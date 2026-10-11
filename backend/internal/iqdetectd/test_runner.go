package iqdetectd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// testResultsRetention is how many result pairs are kept per account.
const testResultsRetention = 60

// maxStoredHTML caps the pelican HTML stored per result row.
const maxStoredHTML = 2 << 20

var finalAnswerPattern = regexp.MustCompile(`(?is)<final_answer>(.*?)</final_answer>`)

type testJob struct {
	model       string
	accountID   int64
	accountName string
	apiURL      string
	apiKey      string
}

// TestRunner executes one detection test per configured model: it picks a
// random account (from the saved selection) that exposes the model, calls the
// account's own upstream concurrently with the two configured probe prompts
// and stores the result pair. Per account only the newest
// testResultsRetention pairs are kept.
type TestRunner struct {
	Store   *Store
	Timeout time.Duration
	Client  *http.Client
	Logger  *slog.Logger

	mu      sync.Mutex
	running bool
}

func NewTestRunner(store *Store, timeout time.Duration, logger *slog.Logger) *TestRunner {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &TestRunner{
		Store:   store,
		Timeout: timeout,
		Client:  &http.Client{Timeout: timeout},
		Logger:  logger,
	}
}

func (r *TestRunner) setRunning(v bool) {
	r.mu.Lock()
	r.running = v
	r.mu.Unlock()
}

// TriggerTestRun plans the run and executes it in the background, returning
// the per-model plan immediately.
func (r *TestRunner) TriggerTestRun(ctx context.Context) (*TestRunPlan, error) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return nil, fmt.Errorf("another test run is already in progress")
	}
	r.running = true
	r.mu.Unlock()

	prompts, err := r.Store.GetDetectionPrompts(ctx)
	if err != nil {
		r.setRunning(false)
		return nil, err
	}
	models, err := r.Store.ListDetectionModels(ctx)
	if err != nil {
		r.setRunning(false)
		return nil, err
	}
	selection, err := r.Store.GetSelection(ctx)
	if err != nil {
		r.setRunning(false)
		return nil, err
	}

	// model -> 已保存选择中的候选账号
	type candidate struct {
		id     int64
		name   string
		apiURL string
		apiKey string
	}
	modelAccounts := map[string][]candidate{}
	for _, group := range selection.Groups {
		for _, account := range group.Accounts {
			apiURL, apiKey, err := r.Store.GetAccountCallCreds(ctx, account.ID)
			if err != nil || strings.TrimSpace(apiURL) == "" || strings.TrimSpace(apiKey) == "" {
				continue
			}
			for _, model := range account.Models {
				modelAccounts[model] = append(modelAccounts[model], candidate{
					id: account.ID, name: account.Name, apiURL: apiURL, apiKey: apiKey,
				})
			}
		}
	}

	plan := &TestRunPlan{Items: []TestRunPlanItem{}}
	var jobs []testJob
	for _, dm := range models {
		item := TestRunPlanItem{Model: dm.Name}
		candidates := modelAccounts[dm.Name]
		if len(candidates) == 0 {
			item.Skipped = true
			item.Reason = "no selected account exposes this model with stored credentials"
			plan.Items = append(plan.Items, item)
			continue
		}
		pick := candidates[rand.Intn(len(candidates))]
		item.AccountID = pick.id
		item.AccountName = pick.name
		plan.Items = append(plan.Items, item)
		jobs = append(jobs, testJob{
			model: dm.Name, accountID: pick.id, accountName: pick.name,
			apiURL: pick.apiURL, apiKey: pick.apiKey,
		})
	}
	plan.Triggered = len(jobs)

	if len(jobs) == 0 {
		r.setRunning(false)
		return plan, nil
	}
	go func() {
		defer r.setRunning(false)
		r.execute(context.WithoutCancel(ctx), jobs, prompts)
	}()
	return plan, nil
}

func (r *TestRunner) execute(ctx context.Context, jobs []testJob, prompts *DetectionPrompts) {
	logger := r.Logger
	if logger == nil {
		logger = slog.Default()
	}
	for _, job := range jobs {
		result := r.runPair(ctx, job, prompts)
		if _, err := r.Store.InsertTestResult(ctx, *result); err != nil {
			logger.Error("iqdetectd: persist test result", "account", job.accountID, "model", job.model, "err", err)
			continue
		}
		if err := r.Store.TrimTestResults(ctx, job.accountID, testResultsRetention); err != nil {
			logger.Error("iqdetectd: trim test results", "account", job.accountID, "err", err)
		}
		logger.Info("iqdetectd: test pair finished", "account", job.accountID, "model", job.model,
			"pelican_ok", result.PelicanError == "", "candy_ok", result.CandyError == "")
	}
}

// runPair fires the two probe calls concurrently (pelican → HTML stored in
// SQLite, candy → graded answer) and returns one result pair.
func (r *TestRunner) runPair(ctx context.Context, job testJob, prompts *DetectionPrompts) *TestResult {
	result := &TestResult{AccountID: job.accountID, AccountName: job.accountName, Model: job.model}
	var wg sync.WaitGroup

	if strings.TrimSpace(prompts.PelicanPrompt) == "" {
		result.PelicanError = "pelican prompt is not configured"
	} else {
		wg.Add(1)
		go func() {
			defer wg.Done()
			answer, latency, err := r.callUpstream(ctx, job.apiURL, job.apiKey, job.model, prompts.PelicanPrompt, prompts)
			result.PelicanLatencyMS = latency.Milliseconds()
			if err != nil {
				result.PelicanError = err.Error()
				return
			}
			result.PelicanHTML = truncate(answer, maxStoredHTML)
		}()
	}

	if strings.TrimSpace(prompts.CandyPrompt) == "" {
		result.CandyError = "candy prompt is not configured"
	} else {
		wg.Add(1)
		go func() {
			defer wg.Done()
			answer, latency, err := r.callUpstream(ctx, job.apiURL, job.apiKey, job.model, prompts.CandyPrompt, prompts)
			result.CandyLatencyMS = latency.Milliseconds()
			if err != nil {
				result.CandyError = err.Error()
				return
			}
			result.CandyAnswer = truncate(answer, 10000)
			// 抽取 <final_answer>…</final_answer> 与配置的正确答案比对
			if expected := strings.TrimSpace(prompts.CandyAnswer); expected != "" {
				correct := gradeCandyAnswer(answer, expected)
				result.CandyCorrect = &correct
			}
		}()
	}

	wg.Wait()
	return result
}

// extractFinalAnswer pulls the value between <final_answer> and
// </final_answer> out of a candy answer; empty when the tag is missing.
func extractFinalAnswer(answer string) string {
	m := finalAnswerPattern.FindStringSubmatch(answer)
	if len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// gradeCandyAnswer compares the extracted final answer with the configured
// expected answer (exact or numeric equality).
func gradeCandyAnswer(answer, expected string) bool {
	got := extractFinalAnswer(answer)
	if got == "" {
		return false
	}
	if strings.EqualFold(got, expected) {
		return true
	}
	gotN, gotErr := strconv.ParseFloat(got, 64)
	wantN, wantErr := strconv.ParseFloat(expected, 64)
	return gotErr == nil && wantErr == nil && gotN == wantN
}

// callUpstream sends one chat completion to the account's own upstream using
// the stored API key, honoring the configured reasoning effort and temperature.
func (r *TestRunner) callUpstream(ctx context.Context, apiURL, apiKey, model, prompt string, prompts *DetectionPrompts) (string, time.Duration, error) {
	start := time.Now()
	payload := map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
		"stream":     false,
		"max_tokens": 8192,
	}
	if prompts != nil && prompts.Temperature != nil {
		payload["temperature"] = *prompts.Temperature
	}
	if prompts != nil && strings.TrimSpace(prompts.ReasoningEffort) != "" {
		payload["reasoning_effort"] = strings.TrimSpace(prompts.ReasoningEffort)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatCompletionsURLFor(apiURL), bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := r.Client.Do(req)
	if err != nil {
		return "", time.Since(start), err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", time.Since(start), err
	}
	if resp.StatusCode >= 400 {
		return "", time.Since(start), fmt.Errorf("upstream HTTP %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", time.Since(start), fmt.Errorf("invalid upstream response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return "", time.Since(start), fmt.Errorf("upstream returned no choices")
	}
	return strings.TrimSpace(decoded.Choices[0].Message.Content), time.Since(start), nil
}
