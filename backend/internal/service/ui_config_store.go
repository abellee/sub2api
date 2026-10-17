package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// openConfigDB also imports the old category JSON once. The source file is
// retained, and a database entry always takes precedence over the legacy file.
func (s *GroupCategoryStore) openConfigDB(ctx context.Context) (*sql.DB, error) {
	path := s.resolvedPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	ok := false
	defer func() {
		if !ok {
			_ = db.Close()
		}
	}()
	if _, err = db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		return nil, err
	}
	if _, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS ui_configuration (
		key TEXT PRIMARY KEY, value TEXT NOT NULL
	)`); err != nil {
		return nil, err
	}
	var exists int
	err = db.QueryRowContext(ctx, "SELECT 1 FROM ui_configuration WHERE key = 'group_categories'").Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		legacyPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".json"
		raw, readErr := os.ReadFile(legacyPath)
		if readErr != nil && !os.IsNotExist(readErr) {
			return nil, readErr
		}
		var snapshot GroupCategorySnapshot
		if readErr == nil {
			if err = json.Unmarshal(raw, &snapshot); err != nil {
				return nil, fmt.Errorf("import legacy group categories: %w", err)
			}
		}
		encoded, marshalErr := json.Marshal(snapshot)
		if marshalErr != nil {
			return nil, marshalErr
		}
		_, err = db.ExecContext(ctx, "INSERT OR IGNORE INTO ui_configuration(key, value) VALUES ('group_categories', ?)", string(encoded))
	}
	if err != nil {
		return nil, err
	}
	ok = true
	return db, nil
}

func (s *GroupCategoryStore) readConfig(ctx context.Context, key string, value any) error {
	db, err := s.openConfigDB(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	var raw string
	err = db.QueryRowContext(ctx, "SELECT value FROM ui_configuration WHERE key = ?", key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(raw), value)
}

func (s *GroupCategoryStore) writeConfig(ctx context.Context, key string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	db, err := s.openConfigDB(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	_, err = db.ExecContext(ctx, `INSERT INTO ui_configuration(key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, string(encoded))
	return err
}
