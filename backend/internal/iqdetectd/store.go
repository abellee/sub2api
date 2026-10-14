package iqdetectd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store persists monitored models, probe runs and their IQ results in a
// dedicated SQLite file. It shares nothing with the main sub2api database.
type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS models (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  label TEXT NOT NULL DEFAULT '',
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS probe_runs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  status TEXT NOT NULL DEFAULT 'pending',
  models_json TEXT NOT NULL DEFAULT '[]',
  total_probes INTEGER NOT NULL DEFAULT 0,
  started_at TEXT NOT NULL,
  finished_at TEXT,
  error TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS probe_results (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id INTEGER NOT NULL REFERENCES probe_runs(id) ON DELETE CASCADE,
  model TEXT NOT NULL,
  iq REAL NOT NULL DEFAULT 0,
  correct INTEGER NOT NULL DEFAULT 0,
  total INTEGER NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT '',
  details_json TEXT NOT NULL DEFAULT '[]',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_probe_results_run ON probe_results(run_id);
CREATE INDEX IF NOT EXISTS idx_probe_results_model ON probe_results(model, id DESC);
CREATE TABLE IF NOT EXISTS account_credentials (
  account_id INTEGER PRIMARY KEY,
  api_url TEXT NOT NULL DEFAULT '',
  api_key TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS detection_selection (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  payload TEXT NOT NULL DEFAULT '[]',
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS detection_models (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS detection_prompts (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  pelican_prompt TEXT NOT NULL DEFAULT '',
  candy_prompt TEXT NOT NULL DEFAULT '',
  candy_answer TEXT NOT NULL DEFAULT '',
  quiet_windows TEXT NOT NULL DEFAULT '[]',
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS test_results (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  account_id INTEGER NOT NULL,
  account_name TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'completed',
  pelican_html TEXT NOT NULL DEFAULT '',
  pelican_response TEXT NOT NULL DEFAULT '',
  pelican_error TEXT NOT NULL DEFAULT '',
  candy_answer TEXT NOT NULL DEFAULT '',
  candy_error TEXT NOT NULL DEFAULT '',
  candy_correct INTEGER,
  pelican_latency_ms INTEGER NOT NULL DEFAULT 0,
  candy_latency_ms INTEGER NOT NULL DEFAULT 0,
  pelican_started_at TEXT NOT NULL DEFAULT '',
  pelican_finished_at TEXT NOT NULL DEFAULT '',
  candy_started_at TEXT NOT NULL DEFAULT '',
  candy_finished_at TEXT NOT NULL DEFAULT '',
  pelican_review TEXT NOT NULL DEFAULT 'pending',
  candy_review TEXT NOT NULL DEFAULT 'pending',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_test_results_account ON test_results(account_id, id DESC);
`)
	// 旧库升级：检测设置与测试结果增列（新库 CREATE 已包含，ALTER 报 duplicate column 时忽略）
	for _, stmt := range []string{
		`ALTER TABLE detection_prompts ADD COLUMN reasoning_effort TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE detection_prompts ADD COLUMN temperature REAL`,
		`ALTER TABLE detection_prompts ADD COLUMN candy_answer TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE detection_prompts ADD COLUMN quiet_windows TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE test_results ADD COLUMN pelican_html TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE test_results ADD COLUMN candy_correct INTEGER`,
		`ALTER TABLE test_results ADD COLUMN status TEXT NOT NULL DEFAULT 'completed'`,
		`ALTER TABLE test_results ADD COLUMN pelican_response TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE test_results ADD COLUMN pelican_started_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE test_results ADD COLUMN pelican_finished_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE test_results ADD COLUMN candy_started_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE test_results ADD COLUMN candy_finished_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE test_results ADD COLUMN pelican_review TEXT NOT NULL DEFAULT 'pending'`,
		`ALTER TABLE test_results ADD COLUMN candy_review TEXT NOT NULL DEFAULT 'pending'`,
		`ALTER TABLE test_results ADD COLUMN group_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE test_results ADD COLUMN group_name TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
	}
	// 上个进程中断遗留的 running 行不可能再完成，标记为失败
	if _, err := s.db.Exec(`
UPDATE test_results SET status = 'failed',
  pelican_error = CASE WHEN pelican_error = '' THEN 'interrupted: service restarted mid-run' ELSE pelican_error END
WHERE status = 'running'`); err != nil {
		return err
	}
	return err
}

// ---- monitored models ----

func (s *Store) ListModels(ctx context.Context) ([]MonitoredModel, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, label, enabled, created_at, updated_at FROM models ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []MonitoredModel{}
	for rows.Next() {
		m, err := scanModel(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *m)
	}
	return items, rows.Err()
}

func (s *Store) GetModelByName(ctx context.Context, name string) (*MonitoredModel, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, label, enabled, created_at, updated_at FROM models WHERE name = ?`, name)
	m, err := scanModel(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("model not found")
	}
	return m, err
}

func (s *Store) CreateModel(ctx context.Context, input CreateModelInput) (*MonitoredModel, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("model name is required")
	}
	now := time.Now().UTC()
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO models (name, label, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		name, strings.TrimSpace(input.Label), boolToInt(enabled),
		now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("model already exists")
		}
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetModelByID(ctx, id)
}

func (s *Store) GetModelByID(ctx context.Context, id int64) (*MonitoredModel, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, label, enabled, created_at, updated_at FROM models WHERE id = ?`, id)
	m, err := scanModel(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("model not found")
	}
	return m, err
}

func (s *Store) UpdateModel(ctx context.Context, id int64, input CreateModelInput) (*MonitoredModel, error) {
	model, err := s.GetModelByID(ctx, id)
	if err != nil {
		return nil, err
	}
	model.Label = strings.TrimSpace(input.Label)
	if input.Enabled != nil {
		model.Enabled = *input.Enabled
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE models SET label = ?, enabled = ?, updated_at = ? WHERE id = ?`,
		model.Label, boolToInt(model.Enabled), time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return nil, err
	}
	return s.GetModelByID(ctx, id)
}

func (s *Store) DeleteModel(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM models WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("model not found")
	}
	return nil
}

func scanModel(row rowScanner) (*MonitoredModel, error) {
	var m MonitoredModel
	var created, updated string
	var enabled int
	if err := row.Scan(&m.ID, &m.Name, &m.Label, &enabled, &created, &updated); err != nil {
		return nil, err
	}
	m.Enabled = enabled != 0
	m.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	m.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return &m, nil
}

// ---- runs ----

func (s *Store) CreateRun(ctx context.Context, models []string, totalProbes int) (*Run, error) {
	if models == nil {
		models = []string{}
	}
	encoded, err := json.Marshal(models)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
INSERT INTO probe_runs (status, models_json, total_probes, started_at) VALUES (?, ?, ?, ?)`,
		RunStatusPending, string(encoded), totalProbes, now.Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetRun(ctx, id)
}

func (s *Store) UpdateRunStatus(ctx context.Context, id int64, status, runErr string) error {
	var finished any
	if status == RunStatusCompleted || status == RunStatusFailed {
		finished = time.Now().UTC().Format(time.RFC3339Nano)
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE probe_runs SET status = ?, error = ?, finished_at = ? WHERE id = ?`,
		status, runErr, finished, id)
	return err
}

// HasActiveRun reports whether a run is pending or running.
func (s *Store) HasActiveRun(ctx context.Context) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(1) FROM probe_runs WHERE status IN (?, ?)`,
		RunStatusPending, RunStatusRunning).Scan(&n)
	return n > 0, err
}

func (s *Store) GetRun(ctx context.Context, id int64) (*Run, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, status, models_json, total_probes, started_at, finished_at, error FROM probe_runs WHERE id = ?`, id)
	run, err := scanRun(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("run not found")
	}
	return run, err
}

func (s *Store) ListRuns(ctx context.Context, limit int) ([]Run, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, status, models_json, total_probes, started_at, finished_at, error
FROM probe_runs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []Run{}
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *run)
	}
	return items, rows.Err()
}

func scanRun(row rowScanner) (*Run, error) {
	var run Run
	var modelsJSON string
	var started, finished sql.NullString
	if err := row.Scan(&run.ID, &run.Status, &modelsJSON, &run.TotalProbes, &started, &finished, &run.Error); err != nil {
		return nil, err
	}
	run.Models = []string{}
	_ = json.Unmarshal([]byte(modelsJSON), &run.Models)
	run.StartedAt, _ = time.Parse(time.RFC3339Nano, started.String)
	if finished.Valid && finished.String != "" {
		t, err := time.Parse(time.RFC3339Nano, finished.String)
		if err == nil {
			run.FinishedAt = &t
		}
	}
	return &run, nil
}

// ---- results ----

func (s *Store) InsertResult(ctx context.Context, result ModelResult) (*ModelResult, error) {
	if result.Details == nil {
		result.Details = []ProbeOutcome{}
	}
	encoded, err := json.Marshal(result.Details)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
INSERT INTO probe_results (run_id, model, iq, correct, total, error, details_json, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		result.RunID, result.Model, result.IQ, result.Correct, result.Total, result.Error,
		string(encoded), now.Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	result.ID = id
	result.CreatedAt = now
	return &result, nil
}

func (s *Store) ListResultsByRun(ctx context.Context, runID int64) ([]ModelResult, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, run_id, model, iq, correct, total, error, details_json, created_at
FROM probe_results WHERE run_id = ? ORDER BY iq DESC`, runID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanResults(rows)
}

func (s *Store) ListScores(ctx context.Context, historyPerModel int) ([]ScoreSummary, error) {
	if historyPerModel <= 0 {
		historyPerModel = 10
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT model, COUNT(1) AS runs, MAX(iq) AS best_iq, MAX(id) AS last_result_id
FROM probe_results GROUP BY model ORDER BY best_iq DESC`)
	if err != nil {
		return nil, err
	}
	type agg struct {
		model        string
		runs         int
		bestIQ       float64
		lastResultID int64
	}
	var aggs []agg
	for rows.Next() {
		var a agg
		if err := rows.Scan(&a.model, &a.runs, &a.bestIQ, &a.lastResultID); err != nil {
			_ = rows.Close()
			return nil, err
		}
		aggs = append(aggs, a)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()

	summaries := make([]ScoreSummary, 0, len(aggs))
	for _, a := range aggs {
		summary := ScoreSummary{Model: a.model, BestIQ: a.bestIQ, Runs: a.runs}
		latest, err := s.resultByID(ctx, a.lastResultID)
		if err != nil {
			return nil, err
		}
		if latest != nil {
			summary.LatestIQ = latest.IQ
			ranAt := latest.CreatedAt
			summary.LastRunAt = &ranAt
		}
		history, err := s.resultHistory(ctx, a.model, historyPerModel)
		if err != nil {
			return nil, err
		}
		summary.History = history
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func (s *Store) resultByID(ctx context.Context, id int64) (*ModelResult, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, run_id, model, iq, correct, total, error, details_json, created_at
FROM probe_results WHERE id = ?`, id)
	res, err := scanResultRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return res, err
}

func (s *Store) resultHistory(ctx context.Context, model string, limit int) ([]ScorePoint, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT run_id, iq, created_at FROM probe_results
WHERE model = ? ORDER BY id ASC LIMIT ?`, model, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	points := []ScorePoint{}
	for rows.Next() {
		var p ScorePoint
		var created string
		if err := rows.Scan(&p.RunID, &p.IQ, &created); err != nil {
			return nil, err
		}
		p.RanAt, _ = time.Parse(time.RFC3339Nano, created)
		points = append(points, p)
	}
	return points, rows.Err()
}

func scanResults(rows *sql.Rows) ([]ModelResult, error) {
	items := []ModelResult{}
	for rows.Next() {
		res, err := scanResultRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *res)
	}
	return items, rows.Err()
}

func scanResultRow(row rowScanner) (*ModelResult, error) {
	var res ModelResult
	var detailsJSON, created string
	if err := row.Scan(&res.ID, &res.RunID, &res.Model, &res.IQ, &res.Correct, &res.Total, &res.Error, &detailsJSON, &created); err != nil {
		return nil, err
	}
	res.Details = []ProbeOutcome{}
	_ = json.Unmarshal([]byte(detailsJSON), &res.Details)
	res.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return &res, nil
}

// ---- per-account credentials ----

// UpsertAccountCredential saves the account's API URL and (optionally) API
// key. An empty key keeps the previously stored one.
func (s *Store) UpsertAccountCredential(ctx context.Context, input UpsertAccountCredentialInput) (*AccountCredential, error) {
	if input.AccountID <= 0 {
		return nil, fmt.Errorf("account_id is required")
	}
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO account_credentials (account_id, api_url, api_key, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(account_id) DO UPDATE SET
  api_url = excluded.api_url,
  api_key = CASE WHEN excluded.api_key != '' THEN excluded.api_key ELSE account_credentials.api_key END,
  updated_at = excluded.updated_at`,
		input.AccountID, strings.TrimSpace(input.APIURL), strings.TrimSpace(input.APIKey),
		now.Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	return s.GetAccountCredential(ctx, input.AccountID)
}

func (s *Store) GetAccountCredential(ctx context.Context, accountID int64) (*AccountCredential, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT account_id, api_url, api_key, updated_at FROM account_credentials WHERE account_id = ?`, accountID)
	return scanAccountCredential(row)
}

func (s *Store) ListAccountCredentials(ctx context.Context) ([]AccountCredential, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT account_id, api_url, api_key, updated_at FROM account_credentials ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []AccountCredential{}
	for rows.Next() {
		c, err := scanAccountCredential(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *c)
	}
	return items, rows.Err()
}

func scanAccountCredential(row rowScanner) (*AccountCredential, error) {
	var c AccountCredential
	var apiKey, updated string
	if err := row.Scan(&c.AccountID, &c.APIURL, &apiKey, &updated); err != nil {
		return nil, err
	}
	c.HasAPIKey = strings.TrimSpace(apiKey) != ""
	c.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return &c, nil
}

// ---- detection selection (single-row JSON blob) ----

// GetSelection returns the persisted selection; Groups is always non-nil and
// UpdatedAt is nil when nothing has been saved yet.
func (s *Store) GetSelection(ctx context.Context) (*DetectionSelection, error) {
	var payload string
	var updated sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT payload, updated_at FROM detection_selection WHERE id = 1`).Scan(&payload, &updated)
	if err == sql.ErrNoRows {
		return &DetectionSelection{Groups: []SelectionGroup{}}, nil
	}
	if err != nil {
		return nil, err
	}
	selection := DetectionSelection{Groups: []SelectionGroup{}}
	if err := json.Unmarshal([]byte(payload), &selection.Groups); err != nil {
		return nil, fmt.Errorf("corrupt selection payload: %w", err)
	}
	if selection.Groups == nil {
		selection.Groups = []SelectionGroup{}
	}
	if updated.Valid && updated.String != "" {
		if t, err := time.Parse(time.RFC3339Nano, updated.String); err == nil {
			selection.UpdatedAt = &t
		}
	}
	return &selection, nil
}

// SaveSelection replaces the persisted selection as a whole.
func (s *Store) SaveSelection(ctx context.Context, groups []SelectionGroup) (*DetectionSelection, error) {
	if groups == nil {
		groups = []SelectionGroup{}
	}
	encoded, err := json.Marshal(groups)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
INSERT INTO detection_selection (id, payload, updated_at) VALUES (1, ?, ?)
ON CONFLICT(id) DO UPDATE SET payload = excluded.payload, updated_at = excluded.updated_at`,
		string(encoded), now.Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	return s.GetSelection(ctx)
}

// ---- detection model settings ----

func (s *Store) ListDetectionModels(ctx context.Context) ([]DetectionModel, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, created_at FROM detection_models ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []DetectionModel{}
	for rows.Next() {
		var m DetectionModel
		var created string
		if err := rows.Scan(&m.ID, &m.Name, &created); err != nil {
			return nil, err
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		items = append(items, m)
	}
	return items, rows.Err()
}

// ReplaceDetectionModels swaps the whole list. Names are trimmed, emptied-out
// entries dropped and duplicates removed (case-insensitive).
func (s *Store) ReplaceDetectionModels(ctx context.Context, names []string) ([]DetectionModel, error) {
	seen := make(map[string]struct{}, len(names))
	cleaned := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, name)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM detection_models`); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, name := range cleaned {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO detection_models (name, created_at) VALUES (?, ?)`, name, now); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.ListDetectionModels(ctx)
}

// GetDetectionPrompts returns the saved custom probe prompts, the expected
// candy answer and call settings (zero values when never saved).
func (s *Store) GetDetectionPrompts(ctx context.Context) (*DetectionPrompts, error) {
	var prompts DetectionPrompts
	var temperature sql.NullFloat64
	var quietWindows string
	row := s.db.QueryRowContext(ctx,
		`SELECT pelican_prompt, candy_prompt, candy_answer, reasoning_effort, temperature, quiet_windows FROM detection_prompts WHERE id = 1`)
	err := row.Scan(&prompts.PelicanPrompt, &prompts.CandyPrompt, &prompts.CandyAnswer, &prompts.ReasoningEffort, &temperature, &quietWindows)
	if err == sql.ErrNoRows {
		return &DetectionPrompts{}, nil
	}
	if err != nil {
		return nil, err
	}
	if temperature.Valid {
		prompts.Temperature = &temperature.Float64
	}
	prompts.QuietWindows = decodeQuietWindows(quietWindows)
	return &prompts, nil
}

// UpsertDetectionPrompts saves the custom probe prompts and call settings.
func (s *Store) UpsertDetectionPrompts(ctx context.Context, prompts DetectionPrompts) error {
	quietWindows, err := json.Marshal(prompts.QuietWindows)
	if err != nil {
		return err
	}
	if prompts.QuietWindows == nil {
		quietWindows = []byte(`[]`)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO detection_prompts (id, pelican_prompt, candy_prompt, candy_answer, reasoning_effort, temperature, quiet_windows, updated_at)
VALUES (1, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  pelican_prompt = excluded.pelican_prompt,
  candy_prompt = excluded.candy_prompt,
  candy_answer = excluded.candy_answer,
  reasoning_effort = excluded.reasoning_effort,
  temperature = excluded.temperature,
  quiet_windows = excluded.quiet_windows,
  updated_at = excluded.updated_at`,
		prompts.PelicanPrompt, prompts.CandyPrompt, prompts.CandyAnswer, prompts.ReasoningEffort, nullFloat(prompts.Temperature),
		string(quietWindows), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

// decodeQuietWindows unmarshals the stored quiet-window JSON, tolerating
// corrupt/empty payloads by returning an empty list.
func decodeQuietWindows(raw string) []QuietWindow {
	windows := []QuietWindow{}
	if strings.TrimSpace(raw) == "" {
		return windows
	}
	_ = json.Unmarshal([]byte(raw), &windows)
	return windows
}

func nullFloat(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

// ---- test results ----

// InsertRunningTestResult persists one result pair at trigger time so the
// list can show it as in-progress; returns its row id.
func (s *Store) InsertRunningTestResult(ctx context.Context, result TestResult) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx, `
INSERT INTO test_results (group_id, group_name, account_id, account_name, model, status,
                          pelican_started_at, candy_started_at, pelican_review, candy_review, created_at)
VALUES (?, ?, ?, ?, ?, 'running', ?, ?, 'pending', 'pending', ?)`,
		result.GroupID, result.GroupName, result.AccountID, result.AccountName, result.Model,
		result.PelicanStartedAt, result.CandyStartedAt, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CompleteTestResult fills in the outcome of both probes and flips the row
// out of the running state.
func (s *Store) CompleteTestResult(ctx context.Context, id int64, result TestResult) error {
	status := "failed"
	if result.Status == RunStatusStopped {
		status = RunStatusStopped
	} else if result.PelicanError == "" || result.CandyError == "" {
		status = "completed"
	}
	var candyCorrect any
	if result.CandyCorrect != nil {
		candyCorrect = boolToInt(*result.CandyCorrect)
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE test_results SET
  status = ?, pelican_html = ?, pelican_response = ?, pelican_error = ?,
  pelican_latency_ms = ?, pelican_finished_at = ?,
  candy_answer = ?, candy_error = ?, candy_correct = ?,
  candy_latency_ms = ?, candy_finished_at = ?
WHERE id = ?`,
		status, result.PelicanHTML, result.PelicanResponse, result.PelicanError,
		result.PelicanLatencyMS, result.PelicanFinishedAt,
		result.CandyAnswer, result.CandyError, candyCorrect,
		result.CandyLatencyMS, result.CandyFinishedAt, id)
	return err
}

// UpdateTestResultReview records the manual review verdict of one probe.
func (s *Store) UpdateTestResultReview(ctx context.Context, id int64, probe, verdict string) error {
	column := probe + "_review"
	if column != "pelican_review" && column != "candy_review" {
		return fmt.Errorf("unknown probe %q", probe)
	}
	if verdict != "approved" && verdict != "rejected" {
		return fmt.Errorf("unknown verdict %q", verdict)
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE test_results SET `+column+` = ? WHERE id = ?`, verdict, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("test result %d not found", id)
	}
	return nil
}

// TrimTestResults keeps only the newest `keep` result pairs of an account.
func (s *Store) TrimTestResults(ctx context.Context, accountID int64, keep int) error {
	rows, err := s.db.QueryContext(ctx, `
SELECT id FROM test_results WHERE account_id = ? ORDER BY id DESC LIMIT -1 OFFSET ?`,
		accountID, keep)
	if err != nil {
		return err
	}
	var staleIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		staleIDs = append(staleIDs, id)
	}
	_ = rows.Close()
	for _, id := range staleIDs {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM test_results WHERE id = ?`, id); err != nil {
			return err
		}
	}
	return nil
}

// ListTestResults returns the newest result pairs without the pelican HTML
// body (only its length; fetch the body via GetTestResultPelicanHTML).
// hasGroup with groupID >= 0 filters by that group (0 = ungrouped legacy
// rows); otherwise all rows are returned.
func (s *Store) ListTestResults(ctx context.Context, limit, offset int64, hasGroup bool, groupID int64) ([]TestResult, int, error) {
	if limit <= 0 {
		limit = 20
	}
	countQuery := `SELECT count(*) FROM test_results`
	listQuery := `
SELECT id, group_id, group_name, account_id, account_name, model, status, pelican_error, pelican_latency_ms,
       length(pelican_html), candy_answer, candy_error, candy_correct, candy_latency_ms,
       pelican_started_at, pelican_finished_at, candy_started_at, candy_finished_at,
       pelican_review, candy_review, created_at
FROM test_results`
	args := []any{}
	if hasGroup {
		countQuery += ` WHERE group_id = ?`
		listQuery += ` WHERE group_id = ?`
		args = append(args, groupID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listQuery += ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := []TestResult{}
	for rows.Next() {
		var result TestResult
		var candyCorrect sql.NullInt64
		// created_at 在 SQLite 里存的是 RFC3339 文本，需手动解析成 time.Time
		var createdAt string
		if err := rows.Scan(&result.ID, &result.GroupID, &result.GroupName, &result.AccountID,
			&result.AccountName, &result.Model,
			&result.Status, &result.PelicanError, &result.PelicanLatencyMS, &result.PelicanHTMLLen,
			&result.CandyAnswer, &result.CandyError, &candyCorrect, &result.CandyLatencyMS,
			&result.PelicanStartedAt, &result.PelicanFinishedAt, &result.CandyStartedAt,
			&result.CandyFinishedAt, &result.PelicanReview, &result.CandyReview,
			&createdAt); err != nil {
			return nil, 0, err
		}
		if ts, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
			result.CreatedAt = ts
		}
		if candyCorrect.Valid {
			v := candyCorrect.Int64 == 1
			result.CandyCorrect = &v
		}
		items = append(items, result)
	}
	return items, total, rows.Err()
}

// GetTestResultByID returns one result pair with its pelican HTML body (the
// full raw upstream response is never included).
func (s *Store) GetTestResultByID(ctx context.Context, id int64) (*TestResult, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, group_id, group_name, account_id, account_name, model, status,
       pelican_html, length(pelican_html), pelican_error, pelican_latency_ms,
       candy_answer, candy_error, candy_correct, candy_latency_ms,
       pelican_started_at, pelican_finished_at, candy_started_at, candy_finished_at,
       pelican_review, candy_review, created_at
FROM test_results WHERE id = ?`, id)
	var result TestResult
	var candyCorrect sql.NullInt64
	var createdAt string
	err := row.Scan(&result.ID, &result.GroupID, &result.GroupName, &result.AccountID,
		&result.AccountName, &result.Model, &result.Status,
		&result.PelicanHTML, &result.PelicanHTMLLen, &result.PelicanError, &result.PelicanLatencyMS,
		&result.CandyAnswer, &result.CandyError, &candyCorrect, &result.CandyLatencyMS,
		&result.PelicanStartedAt, &result.PelicanFinishedAt, &result.CandyStartedAt,
		&result.CandyFinishedAt, &result.PelicanReview, &result.CandyReview,
		&createdAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("test result %d not found", id)
	}
	if err != nil {
		return nil, err
	}
	if candyCorrect.Valid {
		v := candyCorrect.Int64 == 1
		result.CandyCorrect = &v
	}
	if ts, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		result.CreatedAt = ts
	}
	return &result, nil
}

// GetTestResultPelicanHTML returns the stored pelican HTML of one result pair.
func (s *Store) GetTestResultPelicanHTML(ctx context.Context, id int64) (string, error) {
	var html string
	err := s.db.QueryRowContext(ctx,
		`SELECT pelican_html FROM test_results WHERE id = ?`, id).Scan(&html)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("test result %d not found", id)
	}
	return html, err
}

// BackfillTestResultGroups attributes legacy rows (created before results
// carried group fields, group_id = 0) to the selected group of their account.
// Rows whose account belongs to zero or several selected groups are left
// untouched. Idempotent; safe to run at every startup.
func (s *Store) BackfillTestResultGroups(ctx context.Context) (int64, error) {
	selection, err := s.GetSelection(ctx)
	if err != nil {
		return 0, err
	}
	accountGroups := map[int64][]SelectionGroup{}
	for _, group := range selection.Groups {
		for _, account := range group.Accounts {
			accountGroups[account.ID] = append(accountGroups[account.ID], group)
		}
	}
	var updated int64
	for accountID, groups := range accountGroups {
		if len(groups) != 1 {
			continue
		}
		res, err := s.db.ExecContext(ctx, `
UPDATE test_results SET group_id = ?, group_name = ?
WHERE group_id = 0 AND account_id = ?`,
			groups[0].ID, groups[0].Name, accountID)
		if err != nil {
			return updated, err
		}
		n, _ := res.RowsAffected()
		updated += n
	}
	return updated, nil
}

// LastTestedAtByGroup returns, for each group that has result rows, the
// created_at of its most recent one. Groups missing from the map have never
// been tested and are due immediately.
func (s *Store) LastTestedAtByGroup(ctx context.Context) (map[int64]time.Time, error) {
	result := map[int64]time.Time{}
	rows, err := s.db.QueryContext(ctx,
		`SELECT group_id, max(created_at) FROM test_results WHERE group_id > 0 GROUP BY group_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var created string
		if err := rows.Scan(&id, &created); err != nil {
			return nil, err
		}
		if ts, err := time.Parse(time.RFC3339Nano, created); err == nil {
			result[id] = ts
		}
	}
	return result, rows.Err()
}

// LastTestedAtByAccount returns, for each given account, the created_at of its
// most recent test result row; missing entries mean never tested. Used by the
// round-robin account rotation of the test runner.
func (s *Store) LastTestedAtByAccount(ctx context.Context, accountIDs []int64) (map[int64]time.Time, error) {
	result := make(map[int64]time.Time, len(accountIDs))
	if len(accountIDs) == 0 {
		return result, nil
	}
	placeholders := make([]string, len(accountIDs))
	args := make([]any, len(accountIDs))
	for i, id := range accountIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT account_id, max(created_at) FROM test_results
WHERE account_id IN (`+strings.Join(placeholders, ",")+`)
GROUP BY account_id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var created string
		if err := rows.Scan(&id, &created); err != nil {
			return nil, err
		}
		if ts, err := time.Parse(time.RFC3339Nano, created); err == nil {
			result[id] = ts
		}
	}
	return result, rows.Err()
}

// GetAccountCallCreds returns the stored upstream URL and API key of an
// account for making probe calls. Empty values mean the account is not usable.
func (s *Store) GetAccountCallCreds(ctx context.Context, accountID int64) (apiURL, apiKey string, err error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT api_url, api_key FROM account_credentials WHERE account_id = ?`, accountID)
	err = row.Scan(&apiURL, &apiKey)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	return
}

type rowScanner interface {
	Scan(dest ...any) error
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
