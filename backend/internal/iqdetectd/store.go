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
`)
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

type rowScanner interface {
	Scan(dest ...any) error
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
