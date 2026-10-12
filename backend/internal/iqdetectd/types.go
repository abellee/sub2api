package iqdetectd

import "time"

const (
	// DefaultListen is the default HTTP listen address of iqdetectd.
	DefaultListen = "127.0.0.1:18102"
	// DefaultUpstreamBaseURL is the default OpenAI-compatible endpoint the
	// probes are sent to. In a standard sub2api deployment this is the main
	// gateway itself.
	DefaultUpstreamBaseURL = "http://127.0.0.1:8080"

	// Run statuses.
	RunStatusPending   = "pending"
	RunStatusRunning   = "running"
	RunStatusCompleted = "completed"
	RunStatusFailed    = "failed"
)

// MonitoredModel is a model enrolled for IQ probing.
type MonitoredModel struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Label     string    `json:"label,omitempty"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateModelInput is the payload for enrolling or updating a model.
type CreateModelInput struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Enabled *bool  `json:"enabled,omitempty"`
}

// ProbeInfo is the public shape of a built-in probe question.
type ProbeInfo struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Prompt   string `json:"prompt"`
}

// Run is one IQ detection batch over one or more models.
type Run struct {
	ID          int64      `json:"id"`
	Status      string     `json:"status"`
	Models      []string   `json:"models"`
	TotalProbes int        `json:"total_probes"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// ProbeOutcome is the graded result of a single probe against a single model.
type ProbeOutcome struct {
	ProbeID   string `json:"probe_id"`
	Category  string `json:"category"`
	Correct   bool   `json:"correct"`
	Expected  string `json:"expected,omitempty"`
	Answer    string `json:"answer,omitempty"`
	Error     string `json:"error,omitempty"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
}

// ModelResult is the aggregate IQ score of one model within one run.
type ModelResult struct {
	ID        int64          `json:"id"`
	RunID     int64          `json:"run_id"`
	Model     string         `json:"model"`
	IQ        float64        `json:"iq"`
	Correct   int            `json:"correct"`
	Total     int            `json:"total"`
	Error     string         `json:"error,omitempty"`
	Details   []ProbeOutcome `json:"details,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// RunDetail pairs a run with its per-model results.
type RunDetail struct {
	Run
	Results []ModelResult `json:"results"`
}

// ScorePoint is one historical IQ sample of a model.
type ScorePoint struct {
	RunID int64     `json:"run_id"`
	IQ    float64   `json:"iq"`
	RanAt time.Time `json:"ran_at"`
}

// ScoreSummary aggregates the latest, best and historical IQ of a model.
type ScoreSummary struct {
	Model     string       `json:"model"`
	LatestIQ  float64      `json:"latest_iq"`
	BestIQ    float64      `json:"best_iq"`
	Runs      int          `json:"runs"`
	LastRunAt *time.Time   `json:"last_run_at,omitempty"`
	History   []ScorePoint `json:"history,omitempty"`
}

// TriggerRunInput is the payload for starting a detection run. When Models is
// empty every enabled monitored model is probed.
type TriggerRunInput struct {
	Models []string `json:"models,omitempty"`
}

// AccountCredential is the per-account upstream credential stored locally by
// iqdetectd. The API key itself is write-only: it is never returned by the
// API, only its presence (HasAPIKey).
type AccountCredential struct {
	AccountID int64     `json:"account_id"`
	APIURL    string    `json:"api_url"`
	HasAPIKey bool      `json:"has_api_key"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpsertAccountCredentialInput saves or updates the credential of one account.
// An empty APIKey keeps the previously stored key.
type UpsertAccountCredentialInput struct {
	AccountID int64  `json:"account_id"`
	APIURL    string `json:"api_url"`
	APIKey    string `json:"api_key,omitempty"`
}

// SelectionAccount is one account chosen for detection with its model list.
type SelectionAccount struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name,omitempty"`
	Models []string `json:"models,omitempty"`
}

// SelectionGroup is one chosen group with its metadata and chosen accounts.
// IntervalMinutes drives the per-group scheduled detection (0 = manual only).
type SelectionGroup struct {
	ID              int64              `json:"id"`
	Name            string             `json:"name"`
	RateMultiplier  float64            `json:"rate_multiplier"`
	IntervalMinutes int                `json:"interval_minutes,omitempty"`
	Accounts        []SelectionAccount `json:"accounts"`
}

// DetectionSelection is the persisted group/account/model selection. A nil
// UpdatedAt means nothing has been saved yet.
type DetectionSelection struct {
	Groups    []SelectionGroup `json:"groups"`
	UpdatedAt *time.Time       `json:"updated_at,omitempty"`
}

// DetectionModel is one model name configured for detection runs.
type DetectionModel struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// DetectionModelSettings is the payload/response of the detection settings
// endpoints: the model list plus the custom probe prompts and call settings.
type DetectionModelSettings struct {
	Items           []DetectionModel `json:"items"`
	Total           int              `json:"total"`
	PelicanPrompt   string           `json:"pelican_prompt"`
	CandyPrompt     string           `json:"candy_prompt"`
	CandyAnswer     string           `json:"candy_answer"`
	ReasoningEffort string           `json:"reasoning_effort"`
	Temperature     *float64         `json:"temperature,omitempty"`
}

// PutDetectionModelsInput replaces the model list, prompts and call settings.
type PutDetectionModelsInput struct {
	Names           []string `json:"names"`
	PelicanPrompt   string   `json:"pelican_prompt"`
	CandyPrompt     string   `json:"candy_prompt"`
	CandyAnswer     string   `json:"candy_answer"`
	ReasoningEffort string   `json:"reasoning_effort"`
	Temperature     *float64 `json:"temperature,omitempty"`
}

// DetectionPrompts is the stored custom probe prompts and call settings.
type DetectionPrompts struct {
	PelicanPrompt   string   `json:"pelican_prompt"`
	CandyPrompt     string   `json:"candy_prompt"`
	CandyAnswer     string   `json:"candy_answer"`
	ReasoningEffort string   `json:"reasoning_effort"`
	Temperature     *float64 `json:"temperature,omitempty"`
}

// TestResult is one detection outcome pair (pelican + candy) for an account.
// CandyCorrect is nil when no correct answer is configured. Status is
// 'running' from the moment the pair is triggered, then 'completed' or
// 'failed' once both probes finish. PelicanResponse (the full model reply)
// never leaves the sidecar; clients only get PelicanHTML.
type TestResult struct {
	ID               int64     `json:"id"`
	GroupID          int64     `json:"group_id"`
	GroupName        string    `json:"group_name"`
	AccountID        int64     `json:"account_id"`
	AccountName      string    `json:"account_name"`
	Model            string    `json:"model"`
	Status           string    `json:"status"`
	PelicanHTML      string    `json:"pelican_html,omitempty"`
	PelicanHTMLLen   int64     `json:"pelican_html_len,omitempty"`
	PelicanResponse  string    `json:"-"`
	PelicanError     string    `json:"pelican_error,omitempty"`
	PelicanLatencyMS int64     `json:"pelican_latency_ms,omitempty"`
	CandyAnswer      string    `json:"candy_answer,omitempty"`
	CandyError       string    `json:"candy_error,omitempty"`
	CandyLatencyMS   int64     `json:"candy_latency_ms,omitempty"`
	CandyCorrect     *bool     `json:"candy_correct,omitempty"`
	CreatedAt        time.Time `json:"created_at"`

	PelicanStartedAt  string `json:"pelican_started_at,omitempty"`
	PelicanFinishedAt string `json:"pelican_finished_at,omitempty"`
	CandyStartedAt    string `json:"candy_started_at,omitempty"`
	CandyFinishedAt   string `json:"candy_finished_at,omitempty"`
	PelicanReview     string `json:"pelican_review"`
	CandyReview       string `json:"candy_review"`
}

// TestResultReviewInput is the body of PUT /v1/test-results/{id}/review.
type TestResultReviewInput struct {
	Probe   string `json:"probe"`   // pelican | candy
	Verdict string `json:"verdict"` // approved | rejected
}

// TestRunCredential is one account's upstream credential injected by the main
// service at trigger time (scheme B: resolved from the main database so the
// sidecar copy never goes stale). The sidecar falls back to its own stored
// credentials when an account is absent from the map.
type TestRunCredential struct {
	AccountID int64  `json:"account_id"`
	APIURL    string `json:"api_url"`
	APIKey    string `json:"api_key"`
}

// TriggerTestRunInput is the optional body of POST /v1/test-runs. A zero
// GroupID triggers every selected group concurrently.
type TriggerTestRunInput struct {
	Credentials []TestRunCredential `json:"credentials,omitempty"`
	GroupID     int64               `json:"group_id,omitempty"`
}

// TestRunPlanItem reports what a triggered test run will do per model.
type TestRunPlanItem struct {
	Model       string `json:"model"`
	GroupID     int64  `json:"group_id,omitempty"`
	GroupName   string `json:"group_name,omitempty"`
	AccountID   int64  `json:"account_id,omitempty"`
	AccountName string `json:"account_name,omitempty"`
	Skipped     bool   `json:"skipped"`
	Reason      string `json:"reason,omitempty"`
}

// TestRunPlan is the immediate response of triggering a test run.
type TestRunPlan struct {
	Items     []TestRunPlanItem `json:"items"`
	Triggered int               `json:"triggered"`
}
