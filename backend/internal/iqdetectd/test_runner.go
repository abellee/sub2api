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
	"slices"
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
	groupID     int64
	groupName   string
	model       string
	accountID   int64
	accountName string
	apiURL      string
	apiKey      string
}

// candidate is one selectable account for a given model.
type candidate struct {
	id     int64
	name   string
	apiURL string
	apiKey string
}

// TestRunner executes one detection test per configured model per selected
// group: within a group it rotates through the accounts that expose the model
// (least-recently-tested first, random within a tie), calls the account's own
// upstream concurrently with the two configured probe prompts and stores the
// result pair. Groups run concurrently; each group also carries an optional
// scheduled interval. Per account only the newest testResultsRetention pairs
// are kept.
type TestRunner struct {
	Store   *Store
	Timeout time.Duration
	Client  *http.Client
	Logger  *slog.Logger

	mu             sync.Mutex
	runningGroups  map[int64]bool
	runCancels     map[int64]context.CancelFunc
	quietScheduled bool // 上一个 tick 是否因静默时段跳过（避免每分钟刷日志）
}

func candidateIDs(candidates []candidate) []int64 {
	ids := make([]int64, len(candidates))
	for i, c := range candidates {
		ids[i] = c.id
	}
	return ids
}

// pickCandidate returns the candidate with the oldest last-tested timestamp
// (zero for never-tested accounts), breaking ties randomly.
func pickCandidate(candidates []candidate, lastTested map[int64]time.Time) candidate {
	oldest := lastTested[candidates[0].id]
	for _, c := range candidates[1:] {
		ts := lastTested[c.id] // 零值 = 从未测过，天然最旧
		if ts.Before(oldest) {
			oldest = ts
		}
	}
	var pool []candidate
	for _, c := range candidates {
		if lastTested[c.id].Equal(oldest) {
			pool = append(pool, c)
		}
	}
	return pool[rand.Intn(len(pool))]
}

func NewTestRunner(store *Store, timeout time.Duration, logger *slog.Logger) *TestRunner {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &TestRunner{
		Store:         store,
		Timeout:       timeout,
		Client:        &http.Client{Timeout: timeout},
		Logger:        logger,
		runningGroups: map[int64]bool{},
		runCancels:    map[int64]context.CancelFunc{},
	}
}

func (r *TestRunner) tryLockGroup(groupID int64, cancel context.CancelFunc) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.runningGroups[groupID] {
		return false
	}
	r.runningGroups[groupID] = true
	if cancel != nil {
		r.runCancels[groupID] = cancel
	}
	return true
}

func (r *TestRunner) releaseGroup(groupID int64) {
	r.mu.Lock()
	cancel := r.runCancels[groupID]
	delete(r.runningGroups, groupID)
	delete(r.runCancels, groupID)
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// StopTestRuns cancels every in-flight group run (scheduled or manual) and
// returns how many groups were stopped.
func (r *TestRunner) StopTestRuns() int {
	r.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(r.runCancels))
	for _, cancel := range r.runCancels {
		cancels = append(cancels, cancel)
	}
	r.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	if len(cancels) > 0 {
		r.logInfo("iqdetectd: stop requested, cancelling running groups", "groups", len(cancels))
	}
	return len(cancels)
}

// StartScheduler launches the single scheduler goroutine: every minute it
// checks the saved selection for groups whose interval is due and starts their
// detection concurrently. Due-ness is derived from the last result row of each
// group, so saving a new selection takes effect on the next tick without any
// reconfiguration.
func (r *TestRunner) StartScheduler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.scheduleTick(ctx)
			}
		}
	}()
	r.logInfo("iqdetectd: scheduler started, ticking every minute")
}

// scheduleTick checks every scheduled group and launches one goroutine per due
// group. Groups run concurrently with each other; within a group the per-model
// probe pairs also run concurrently. Accounts are resolved from the sidecar
// database by account id at run time. Ticks falling into a configured quiet
// window (server local time) start nothing; runs already in flight finish.
func (r *TestRunner) scheduleTick(ctx context.Context) {
	selection, err := r.Store.GetSelection(ctx)
	if err != nil {
		r.logError("iqdetectd: scheduler load selection", "err", err)
		return
	}
	lastTested, err := r.Store.LastTestedAtByGroup(ctx)
	if err != nil {
		r.logError("iqdetectd: scheduler load last-tested", "err", err)
		return
	}
	prompts, err := r.Store.GetDetectionPrompts(ctx)
	if err != nil {
		r.logError("iqdetectd: scheduler load prompts", "err", err)
		return
	}
	if inQuietWindows(prompts.QuietWindows, time.Now()) {
		r.mu.Lock()
		first := !r.quietScheduled
		r.quietScheduled = true
		r.mu.Unlock()
		if first {
			r.logInfo("iqdetectd: scheduler paused, inside quiet window")
		}
		return
	}
	r.mu.Lock()
	wasQuiet := r.quietScheduled
	r.quietScheduled = false
	r.mu.Unlock()
	if wasQuiet {
		r.logInfo("iqdetectd: scheduler resumed, quiet window ended")
	}
	for _, group := range selection.Groups {
		if group.IntervalMinutes <= 0 || r.isGroupRunning(group.ID) {
			continue
		}
		if last, ok := lastTested[group.ID]; ok {
			if time.Since(last) < time.Duration(group.IntervalMinutes)*time.Minute {
				continue
			}
		}
		runCtx, cancel := context.WithCancel(ctx)
		if !r.tryLockGroup(group.ID, cancel) {
			cancel()
			continue
		}
		go func(group SelectionGroup) {
			defer r.releaseGroup(group.ID)
			prompts, jobs, err := r.buildGroupJobs(runCtx, group, nil)
			if err != nil {
				r.logError("iqdetectd: scheduled run build failed", "group", group.ID, "err", err)
				return
			}
			if len(jobs) == 0 {
				r.logInfo("iqdetectd: scheduled group skipped, no usable accounts", "group", group.ID)
				return
			}
			r.runGroupJobs(runCtx, jobs, prompts)
			r.logInfo("iqdetectd: scheduled group round finished", "group", group.ID, "models", len(jobs))
		}(group)
	}
}

// buildGroupJobs resolves the per-model jobs of one group. injected holds
// main-database credentials by account id; accounts missing from it fall back
// to the sidecar's own stored credentials.
func (r *TestRunner) buildGroupJobs(ctx context.Context, group SelectionGroup, injected map[int64]TestRunCredential) (*DetectionPrompts, []testJob, error) {
	prompts, err := r.Store.GetDetectionPrompts(ctx)
	if err != nil {
		return nil, nil, err
	}
	models, err := r.Store.ListDetectionModels(ctx)
	if err != nil {
		return nil, nil, err
	}
	var jobs []testJob
	for _, dm := range models {
		candidates := r.groupCandidates(ctx, group, dm.Name, injected)
		if len(candidates) == 0 {
			continue
		}
		// 轮换制：优先选最久没被测过的账号，同批内随机
		lastTested, err := r.Store.LastTestedAtByAccount(ctx, group.ID, dm.Name, candidateIDs(candidates))
		if err != nil {
			return nil, nil, fmt.Errorf("load account rotation: %w", err)
		}
		pick := pickCandidate(candidates, lastTested)
		jobs = append(jobs, testJob{
			groupID: group.ID, groupName: group.Name,
			model: dm.Name, accountID: pick.id, accountName: pick.name,
			apiURL: pick.apiURL, apiKey: pick.apiKey,
		})
	}
	return prompts, jobs, nil
}

// TriggerTestRun plans the run and executes it in the background, returning
// the per-(group,model) plan immediately. Groups run concurrently. A non-zero
// groupID limits the run to one group. injected holds main-database
// credentials by account id; accounts missing from it fall back to the
// sidecar's own stored credentials.
func (r *TestRunner) TriggerTestRun(ctx context.Context, injected []TestRunCredential, groupID int64) (*TestRunPlan, error) {
	injectedCreds := make(map[int64]TestRunCredential, len(injected))
	for _, cred := range injected {
		if strings.TrimSpace(cred.APIURL) != "" && strings.TrimSpace(cred.APIKey) != "" {
			injectedCreds[cred.AccountID] = cred
		}
	}

	selection, err := r.Store.GetSelection(ctx)
	if err != nil {
		return nil, err
	}

	models, err := r.Store.ListDetectionModels(ctx)
	if err != nil {
		return nil, err
	}

	plan := &TestRunPlan{Items: []TestRunPlanItem{}}
	jobsByGroup := map[int64][]testJob{}
	// Reserve groups before reading their history. A concurrent manual request
	// or scheduler tick must not plan from the same pre-run snapshot.
	runCtx := context.WithoutCancel(ctx)
	groupContexts := map[int64]context.Context{}
	reserved := map[int64]context.CancelFunc{}
	dispatched := false
	defer func() {
		if !dispatched {
			for id, cancel := range reserved {
				cancel()
				r.releaseGroup(id)
			}
		}
	}()
	for _, group := range selection.Groups {
		if groupID > 0 && group.ID != groupID {
			continue
		}
		groupCtx, cancel := context.WithCancel(runCtx)
		locked := r.tryLockGroup(group.ID, cancel)
		if locked {
			reserved[group.ID] = cancel
		} else {
			cancel()
		}
		for _, dm := range models {
			item := TestRunPlanItem{Model: dm.Name, GroupID: group.ID, GroupName: group.Name}
			if !locked {
				item.Skipped = true
				item.Reason = "a test for this group is already running"
				plan.Items = append(plan.Items, item)
				continue
			}
			candidates := r.groupCandidates(ctx, group, dm.Name, injectedCreds)
			if len(candidates) == 0 {
				item.Skipped = true
				item.Reason = "no selected account exposes this model with usable credentials"
				plan.Items = append(plan.Items, item)
				continue
			}
			// 轮换制：优先选最久没被测过的账号，同批内随机
			lastTested, err := r.Store.LastTestedAtByAccount(ctx, group.ID, dm.Name, candidateIDs(candidates))
			if err != nil {
				return nil, fmt.Errorf("load account rotation: %w", err)
			}
			pick := pickCandidate(candidates, lastTested)
			item.AccountID = pick.id
			item.AccountName = pick.name
			plan.Items = append(plan.Items, item)
			jobsByGroup[group.ID] = append(jobsByGroup[group.ID], testJob{
				groupID: group.ID, groupName: group.Name,
				model: dm.Name, accountID: pick.id, accountName: pick.name,
				apiURL: pick.apiURL, apiKey: pick.apiKey,
			})
		}
		if locked && len(jobsByGroup[group.ID]) == 0 {
			cancel()
			r.releaseGroup(group.ID)
			delete(reserved, group.ID)
		} else if locked {
			// Keep the same cancellation context registered by tryLockGroup.
			groupContexts[group.ID] = groupCtx
		}
	}
	plan.Triggered = 0
	for _, jobs := range jobsByGroup {
		plan.Triggered += len(jobs)
	}

	if len(jobsByGroup) == 0 {
		return plan, nil
	}
	dispatched = true
	go r.executeGroups(groupContexts, jobsByGroup)
	return plan, nil
}

func (r *TestRunner) isGroupRunning(groupID int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runningGroups[groupID]
}

// executeGroups executes groups reserved by TriggerTestRun concurrently. Each
// group keeps its registered context so StopTestRuns can abort it immediately.
func (r *TestRunner) executeGroups(groupContexts map[int64]context.Context, jobsByGroup map[int64][]testJob) {
	var wg sync.WaitGroup
	for groupID, jobs := range jobsByGroup {
		wg.Add(1)
		go func(groupID int64, jobs []testJob) {
			defer wg.Done()
			runCtx := groupContexts[groupID]
			defer r.releaseGroup(groupID)
			prompts, err := r.Store.GetDetectionPrompts(runCtx)
			if err != nil {
				r.logError("iqdetectd: load prompts failed", "group", groupID, "err", err)
				return
			}
			r.runGroupJobs(runCtx, jobs, prompts)
		}(groupID, jobs)
	}
	wg.Wait()
}

// runGroupJobs persists a running row per job and fires all of the group's
// probe pairs concurrently, waiting for all of them to finish. When the run is
// stopped mid-flight the pairs are persisted as status=stopped using a
// non-cancelable context — the canceled one can no longer write to SQLite.
func (r *TestRunner) runGroupJobs(ctx context.Context, jobs []testJob, prompts *DetectionPrompts) {
	persistCtx := context.WithoutCancel(ctx)
	var wg sync.WaitGroup
	for _, job := range jobs {
		job := job
		// 触发即落库（status=running），列表刷新就能看到进行中的测试
		running := TestResult{GroupID: job.groupID, GroupName: job.groupName,
			AccountID: job.accountID, AccountName: job.accountName, Model: job.model,
			PelicanStartedAt: nowRFC3339(), CandyStartedAt: nowRFC3339()}
		id, err := r.Store.InsertRunningTestResult(ctx, running)
		if err != nil {
			r.logError("iqdetectd: insert running test result", "account", job.accountID, "model", job.model, "err", err)
			continue
		}
		wg.Add(1)
		go func(id int64, job testJob) {
			defer wg.Done()
			result := r.runPair(ctx, job, prompts)
			if ctx.Err() != nil {
				result.Status = RunStatusStopped
				if result.PelicanError == "" {
					result.PelicanError = "detection stopped"
					result.PelicanFinishedAt = nowRFC3339()
				}
				if result.CandyError == "" {
					result.CandyError = "detection stopped"
					result.CandyFinishedAt = nowRFC3339()
				}
			}
			if err := r.Store.CompleteTestResult(persistCtx, id, *result); err != nil {
				r.logError("iqdetectd: persist test result", "account", job.accountID, "model", job.model, "err", err)
				return
			}
			if err := r.Store.TrimTestResults(persistCtx, job.accountID, testResultsRetention); err != nil {
				r.logError("iqdetectd: trim test results", "account", job.accountID, "err", err)
			}
			r.logInfo("iqdetectd: test pair finished", "group", job.groupID, "account", job.accountID, "model", job.model,
				"pelican_ok", result.PelicanError == "", "candy_ok", result.CandyError == "")
		}(id, job)
	}
	wg.Wait()
}

func (r *TestRunner) groupCandidates(ctx context.Context, group SelectionGroup, model string, injected map[int64]TestRunCredential) []candidate {
	var candidates []candidate
	seen := map[int64]bool{}
	for _, account := range group.Accounts {
		if seen[account.ID] || !slices.Contains(account.Models, model) {
			continue
		}
		apiURL, apiKey := "", ""
		if cred, ok := injected[account.ID]; ok {
			apiURL, apiKey = cred.APIURL, cred.APIKey
		} else {
			var err error
			apiURL, apiKey, err = r.Store.GetAccountCallCreds(ctx, account.ID)
			if err != nil {
				continue
			}
		}
		if strings.TrimSpace(apiURL) == "" || strings.TrimSpace(apiKey) == "" {
			continue
		}
		seen[account.ID] = true
		candidates = append(candidates, candidate{
			id: account.ID, name: account.Name, apiURL: apiURL, apiKey: apiKey,
		})
	}
	return candidates
}

func (r *TestRunner) logInfo(msg string, args ...any) {
	if r.Logger != nil {
		r.Logger.Info(msg, args...)
	} else {
		slog.Info(msg, args...)
	}
}

func (r *TestRunner) logError(msg string, args ...any) {
	if r.Logger != nil {
		r.Logger.Error(msg, args...)
	} else {
		slog.Error(msg, args...)
	}
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// runPair fires the two probe calls concurrently (pelican → HTML stored in
// SQLite, candy → graded answer) and returns one result pair.
func (r *TestRunner) runPair(ctx context.Context, job testJob, prompts *DetectionPrompts) *TestResult {
	result := &TestResult{AccountID: job.accountID, AccountName: job.accountName, Model: job.model}
	var wg sync.WaitGroup

	if strings.TrimSpace(prompts.PelicanPrompt) == "" {
		result.PelicanError = "pelican prompt is not configured"
		result.PelicanFinishedAt = nowRFC3339()
	} else {
		wg.Add(1)
		go func() {
			defer wg.Done()
			started := nowRFC3339()
			answer, latency, err := r.callUpstream(ctx, job.apiURL, job.apiKey, job.model, prompts.PelicanPrompt, prompts)
			result.PelicanStartedAt = started
			result.PelicanFinishedAt = nowRFC3339()
			result.PelicanLatencyMS = latency.Milliseconds()
			if err != nil {
				result.PelicanError = err.Error()
				return
			}
			// 完整回复与 HTML 部分分开存；对外接口只返回 HTML 部分
			result.PelicanResponse = truncate(answer, maxStoredHTML)
			result.PelicanHTML = truncate(extractHTMLContent(answer), maxStoredHTML)
		}()
	}

	if strings.TrimSpace(prompts.CandyPrompt) == "" {
		result.CandyError = "candy prompt is not configured"
		result.CandyFinishedAt = nowRFC3339()
	} else {
		wg.Add(1)
		go func() {
			defer wg.Done()
			started := nowRFC3339()
			answer, latency, err := r.callUpstream(ctx, job.apiURL, job.apiKey, job.model, prompts.CandyPrompt, prompts)
			result.CandyStartedAt = started
			result.CandyFinishedAt = nowRFC3339()
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

var fencedHTMLPattern = regexp.MustCompile("(?is)```html\\s*(.*?)```")

// extractHTMLContent pulls the HTML portion out of a model reply: a ```html
// fenced block when present, otherwise the <!DOCTYPE/<html>…</html> span,
// falling back to the whole content.
func extractHTMLContent(content string) string {
	if m := fencedHTMLPattern.FindStringSubmatch(content); len(m) >= 2 && strings.TrimSpace(m[1]) != "" {
		return strings.TrimSpace(m[1])
	}
	lower := strings.ToLower(content)
	idx := strings.Index(lower, "<!doctype html")
	if idx < 0 {
		idx = strings.Index(lower, "<html")
	}
	if idx < 0 {
		return content
	}
	end := strings.Index(lower[idx:], "</html>")
	if end < 0 {
		return content[idx:]
	}
	return content[idx : idx+end+len("</html>")]
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
// It always requests streaming so long generations keep the connection alive —
// a non-streaming wait past ~100s gets cut off by Cloudflare with a 524.
func (r *TestRunner) callUpstream(ctx context.Context, apiURL, apiKey, model, prompt string, prompts *DetectionPrompts) (string, time.Duration, error) {
	start := time.Now()
	payload := map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
		"stream":     true,
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
	content, err := decodeUpstreamContent(raw, resp.Header.Get("Content-Type"))
	if err != nil {
		return "", time.Since(start), err
	}
	if strings.TrimSpace(content) == "" {
		return "", time.Since(start), fmt.Errorf("upstream returned no content")
	}
	return strings.TrimSpace(content), time.Since(start), nil
}

// decodeUpstreamContent extracts the assistant message content from either an
// SSE stream (delta chunks) or a plain JSON completion response; some upstreams
// ignore the stream flag and answer non-streamed.
func decodeUpstreamContent(raw []byte, contentType string) (string, error) {
	if !strings.Contains(contentType, "text/event-stream") {
		return decodeJSONContent(raw)
	}
	var sb strings.Builder
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // 心跳/注释等非 JSON 行，跳过
		}
		if len(chunk.Choices) > 0 {
			_, _ = sb.WriteString(chunk.Choices[0].Delta.Content)
		}
	}
	return sb.String(), nil
}

// decodeJSONContent handles a non-streamed chat completion response.
func decodeJSONContent(raw []byte) (string, error) {
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", fmt.Errorf("invalid upstream response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("upstream returned no choices")
	}
	return decoded.Choices[0].Message.Content, nil
}

// ExtractFinalAnswer exports the candy final-answer extraction for consumers
// of the sidecar package (e.g. the main service's public results endpoint).
func ExtractFinalAnswer(answer string) string {
	return extractFinalAnswer(answer)
}
