package iqdetectd

import "time"

const (
	// DefaultListen is the default HTTP listen address of iqdetectd.
	DefaultListen = "127.0.0.1:18100"
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
