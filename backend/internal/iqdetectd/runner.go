package iqdetectd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Runner executes IQ probe runs against an OpenAI-compatible chat completions
// endpoint. Runs are serialized: only one batch may execute at a time.
type Runner struct {
	Store        *Store
	UpstreamBase string // e.g. http://127.0.0.1:8080 or https://api.example.com/v1
	APIKey       string
	Timeout      time.Duration
	Client       *http.Client
	Logger       *slog.Logger

	mu sync.Mutex
}

func NewRunner(store *Store, upstreamBase, apiKey string, timeout time.Duration, logger *slog.Logger) *Runner {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Runner{
		Store:        store,
		UpstreamBase: strings.TrimRight(strings.TrimSpace(upstreamBase), "/"),
		APIKey:       strings.TrimSpace(apiKey),
		Timeout:      timeout,
		Client:       &http.Client{Timeout: timeout},
		Logger:       logger,
	}
}

// TriggerRun validates the request, records a pending run and executes it in
// the background. It fails when another run is still in flight.
func (r *Runner) TriggerRun(ctx context.Context, input TriggerRunInput) (*Run, error) {
	if r.UpstreamBase == "" {
		return nil, fmt.Errorf("upstream api base url is not configured")
	}
	if r.APIKey == "" {
		return nil, fmt.Errorf("upstream api key is not configured")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	active, err := r.Store.HasActiveRun(ctx)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, fmt.Errorf("another run is already in progress")
	}

	modelNames, err := r.resolveModels(ctx, input.Models)
	if err != nil {
		return nil, err
	}
	if len(modelNames) == 0 {
		return nil, fmt.Errorf("no enabled models to probe")
	}

	run, err := r.Store.CreateRun(ctx, modelNames, len(builtinProbes()))
	if err != nil {
		return nil, err
	}

	go r.execute(context.WithoutCancel(ctx), run.ID, modelNames)
	return run, nil
}

// resolveModels expands an explicit model list or falls back to every enabled
// monitored model. Explicit names that are not enrolled are kept as-is so the
// operator can probe ad-hoc models.
func (r *Runner) resolveModels(ctx context.Context, explicit []string) ([]string, error) {
	if len(explicit) > 0 {
		names := make([]string, 0, len(explicit))
		for _, name := range explicit {
			if name = strings.TrimSpace(name); name != "" {
				names = append(names, name)
			}
		}
		return names, nil
	}
	models, err := r.Store.ListModels(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(models))
	for _, m := range models {
		if m.Enabled {
			names = append(names, m.Name)
		}
	}
	return names, nil
}

func (r *Runner) execute(ctx context.Context, runID int64, models []string) {
	logger := r.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if err := r.Store.UpdateRunStatus(ctx, runID, RunStatusRunning, ""); err != nil {
		logger.Error("iqdetectd: mark run running", "run", runID, "err", err)
		return
	}

	failed := 0
	var runErr string
	for _, model := range models {
		result := r.probeModel(ctx, runID, model)
		if result.Error != "" {
			failed++
			logger.Warn("iqdetectd: model probe failed", "run", runID, "model", model, "err", result.Error)
		}
		if _, err := r.Store.InsertResult(ctx, *result); err != nil {
			logger.Error("iqdetectd: persist result", "run", runID, "model", model, "err", err)
			failed++
		}
	}
	status := RunStatusCompleted
	if failed == len(models) {
		status = RunStatusFailed
		runErr = "all model probes failed"
	}
	if err := r.Store.UpdateRunStatus(ctx, runID, status, runErr); err != nil {
		logger.Error("iqdetectd: finish run", "run", runID, "err", err)
	}
	logger.Info("iqdetectd: run finished", "run", runID, "models", len(models), "failed", failed)
}

func (r *Runner) probeModel(ctx context.Context, runID int64, model string) *ModelResult {
	probes := builtinProbes()
	result := &ModelResult{RunID: runID, Model: model, Total: len(probes), Details: []ProbeOutcome{}}
	for _, probe := range probes {
		outcome := ProbeOutcome{ProbeID: probe.ID, Category: probe.Category, Expected: strings.Join(probe.Answers, "/")}
		answer, latency, err := r.chat(ctx, model, probe.Prompt+instruction)
		outcome.LatencyMS = latency.Milliseconds()
		if err != nil {
			outcome.Error = err.Error()
			result.Error = err.Error()
		} else {
			outcome.Answer = truncate(answer, 200)
			if gradeAnswer(probe, answer) {
				outcome.Correct = true
				result.Correct++
			}
		}
		result.Details = append(result.Details, outcome)
	}
	result.IQ = IQScore(result.Correct, result.Total)
	return result
}

// chat sends one chat completion and returns the assistant text.
func (r *Runner) chat(ctx context.Context, model, prompt string) (string, time.Duration, error) {
	start := time.Now()
	payload := map[string]any{
		"model":       model,
		"messages":    []map[string]string{{"role": "user", "content": prompt}},
		"temperature": 0,
		"max_tokens":  1024,
		"stream":      false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.chatCompletionsURL(), bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.APIKey)
	}
	resp, err := r.Client.Do(req)
	if err != nil {
		return "", time.Since(start), err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", time.Since(start), err
	}
	if resp.StatusCode >= 400 {
		return "", time.Since(start), fmt.Errorf("upstream HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
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

// chatCompletionsURL normalizes common base URL shapes to the chat completions
// endpoint: bare host gains /v1, /v1 keeps /chat/completions appended.
func (r *Runner) chatCompletionsURL() string {
	base := strings.TrimRight(r.UpstreamBase, "/")
	switch {
	case strings.HasSuffix(base, "/chat/completions"):
		return base
	case strings.HasSuffix(base, "/v1"):
		return base + "/chat/completions"
	default:
		return base + "/v1/chat/completions"
	}
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
