package appcatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

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
CREATE TABLE IF NOT EXISTS apps (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  official_url TEXT NOT NULL DEFAULT '',
  github_url TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL DEFAULT '',
  icon_url TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  download_page_url TEXT NOT NULL DEFAULT '',
  screenshots_json TEXT NOT NULL DEFAULT '[]',
  changelog TEXT NOT NULL DEFAULT '',
  version TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  last_fetched_at TEXT,
  fetch_error TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS crawl_runs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  started_at TEXT NOT NULL,
  finished_at TEXT NOT NULL,
  status TEXT NOT NULL,
  defer_reason TEXT NOT NULL DEFAULT '',
  load_percent REAL,
  error TEXT NOT NULL DEFAULT ''
);
`)
	return err
}

func (s *Store) CreateApp(ctx context.Context, input CreateAppInput) (*App, error) {
	now := time.Now().UTC()
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = fallbackName(input.OfficialURL, input.GitHubURL)
	}
	shots, err := json.Marshal(uniqueNonEmpty(input.Screenshots))
	if err != nil {
		return nil, err
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO apps (official_url, github_url, name, icon_url, description, download_page_url,
                  screenshots_json, changelog, version, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.OfficialURL, input.GitHubURL, name, strings.TrimSpace(input.IconURL),
		strings.TrimSpace(input.Description), strings.TrimSpace(input.DownloadPageURL),
		string(shots), strings.TrimSpace(input.Changelog), strings.TrimSpace(input.Version),
		now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetApp(ctx, id)
}

func (s *Store) UpdateApp(ctx context.Context, id int64, input CreateAppInput) (*App, error) {
	app, err := s.GetApp(ctx, id)
	if err != nil {
		return nil, err
	}
	app.OfficialURL = input.OfficialURL
	app.GitHubURL = input.GitHubURL
	if name := strings.TrimSpace(input.Name); name != "" {
		app.Name = name
	}
	app.IconURL = strings.TrimSpace(input.IconURL)
	app.Description = strings.TrimSpace(input.Description)
	app.DownloadPageURL = strings.TrimSpace(input.DownloadPageURL)
	app.Screenshots = uniqueNonEmpty(input.Screenshots)
	app.Changelog = strings.TrimSpace(input.Changelog)
	if version := strings.TrimSpace(input.Version); version != "" {
		app.Version = version
	}
	app.UpdatedAt = time.Now().UTC()
	if err := s.persistApp(ctx, app, false); err != nil {
		return nil, err
	}
	return s.GetApp(ctx, id)
}

func (s *Store) GetApp(ctx context.Context, id int64) (*App, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, official_url, github_url, name, icon_url, description, download_page_url,
       screenshots_json, changelog, version, created_at, updated_at, last_fetched_at, fetch_error
FROM apps WHERE id = ?`, id)
	app, err := scanApp(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("app not found")
	}
	return app, err
}

func (s *Store) ListApps(ctx context.Context) ([]App, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, official_url, github_url, name, icon_url, description, download_page_url,
       screenshots_json, changelog, version, created_at, updated_at, last_fetched_at, fetch_error
FROM apps ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var items []App
	for rows.Next() {
		app, err := scanApp(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *app)
	}
	if items == nil {
		items = []App{}
	}
	return items, rows.Err()
}

func (s *Store) DeleteApp(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM apps WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("app not found")
	}
	return nil
}

func (s *Store) ApplyFetch(ctx context.Context, id int64, result FetchResult, fetchErr error, mode fetchApplyMode) (*App, error) {
	app, err := s.GetApp(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	app.UpdatedAt = now
	app.LastFetchedAt = &now
	if fetchErr != nil {
		app.FetchError = fetchErr.Error()
	} else {
		app.FetchError = ""
		applyFetchResult(app, result, mode)
	}
	if err := s.persistApp(ctx, app, true); err != nil {
		return nil, err
	}
	return app, nil
}

func applyFetchResult(app *App, result FetchResult, mode fetchApplyMode) {
	if result.OfficialURL != "" && app.OfficialURL == "" {
		app.OfficialURL = result.OfficialURL
	}
	if result.GitHubURL != "" && app.GitHubURL == "" {
		app.GitHubURL = result.GitHubURL
	}
	if result.Name != "" && (app.Name == "" || looksLikeURL(app.Name)) {
		app.Name = result.Name
	}
	if result.IconURL != "" && app.IconURL == "" {
		app.IconURL = result.IconURL
	}
	if result.Description != "" && app.Description == "" {
		app.Description = result.Description
	}
	if len(result.Screenshots) > 0 && len(app.Screenshots) == 0 {
		app.Screenshots = uniqueNonEmpty(result.Screenshots)
	}
	refreshRelease := mode == fetchRefreshRelease
	if result.DownloadPageURL != "" && (refreshRelease || app.DownloadPageURL == "") {
		app.DownloadPageURL = result.DownloadPageURL
	}
	if result.Changelog != "" && (refreshRelease || app.Changelog == "") {
		app.Changelog = result.Changelog
	}
	if result.Version != "" && (refreshRelease || app.Version == "") {
		app.Version = result.Version
	}
}

func (s *Store) persistApp(ctx context.Context, app *App, withFetchMeta bool) error {
	shots, err := json.Marshal(app.Screenshots)
	if err != nil {
		return err
	}
	if app.Screenshots == nil {
		shots = []byte("[]")
	}
	if withFetchMeta {
		var lastFetched any
		if app.LastFetchedAt != nil {
			lastFetched = app.LastFetchedAt.Format(time.RFC3339Nano)
		}
		_, err = s.db.ExecContext(ctx, `
UPDATE apps SET official_url=?, github_url=?, name=?, icon_url=?, description=?, download_page_url=?,
                screenshots_json=?, changelog=?, version=?, updated_at=?, last_fetched_at=?, fetch_error=?
WHERE id=?`,
			app.OfficialURL, app.GitHubURL, app.Name, app.IconURL, app.Description, app.DownloadPageURL,
			string(shots), app.Changelog, app.Version, app.UpdatedAt.Format(time.RFC3339Nano),
			lastFetched, app.FetchError, app.ID)
		return err
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE apps SET official_url=?, github_url=?, name=?, icon_url=?, description=?, download_page_url=?,
                screenshots_json=?, changelog=?, version=?, updated_at=?
WHERE id=?`,
		app.OfficialURL, app.GitHubURL, app.Name, app.IconURL, app.Description, app.DownloadPageURL,
		string(shots), app.Changelog, app.Version, app.UpdatedAt.Format(time.RFC3339Nano), app.ID)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanApp(row rowScanner) (*App, error) {
	var app App
	var created, updated string
	var lastFetched sql.NullString
	var shots string
	if err := row.Scan(
		&app.ID, &app.OfficialURL, &app.GitHubURL, &app.Name, &app.IconURL, &app.Description, &app.DownloadPageURL,
		&shots, &app.Changelog, &app.Version, &created, &updated, &lastFetched, &app.FetchError,
	); err != nil {
		return nil, err
	}
	app.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	app.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	if lastFetched.Valid && lastFetched.String != "" {
		t, err := time.Parse(time.RFC3339Nano, lastFetched.String)
		if err == nil {
			app.LastFetchedAt = &t
		}
	}
	if err := json.Unmarshal([]byte(shots), &app.Screenshots); err != nil || app.Screenshots == nil {
		app.Screenshots = []string{}
	}
	return &app, nil
}

func fallbackName(officialURL, githubURL string) string {
	if githubURL != "" {
		parsed, err := parseURL(githubURL)
		if err == nil {
			if owner, repo, err := githubOwnerRepo(parsed); err == nil {
				return owner + "/" + repo
			}
		}
	}
	if officialURL != "" {
		return officialURL
	}
	return "untitled"
}

func parseURL(raw string) (*url.URL, error) {
	return url.Parse(raw)
}
