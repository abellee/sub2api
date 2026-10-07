// Package store provides the SQLite persistence layer of lotteryd.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"lotteryd/internal/lottery"
)

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
}

// Open opens (and creates) the SQLite database at path.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// modernc/sqlite serializes writes; a single connection avoids
	// SQLITE_BUSY under concurrent request + scheduler traffic.
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	if _, err := s.db.Exec(schemaSQL); err != nil {
		return err
	}
	if _, err := s.db.Exec(taskSchemaSQL); err != nil {
		return err
	}
	// 任务表旧库升级：补 description 列。
	if err := s.ensureColumn("tasks", "description", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	// 旧库升级：CREATE IF NOT EXISTS 不会改已存在的表，逐列补齐。
	if err := s.ensureColumn("prizes", "codes_json", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	if err := s.ensureColumn("activities", "visible_to_all", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	if err := s.ensureColumn("activities", "daily_config_id", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn("activities", "visible_users_json", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	// 旧活动默认继续向用户显示参与人数。
	return s.ensureColumn("activities", "show_participant_count", "INTEGER NOT NULL DEFAULT 0")
}

// ensureColumn 若表缺少指定列则 ALTER TABLE 补上。
func (s *Store) ensureColumn(table, column, ddl string) error {
	rows, err := s.db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return err
	}
	defer rows.Close()
	has := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			has = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if has {
		return nil
	}
	_, err = s.db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, ddl))
	return err
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS activities (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	starts_at TEXT NOT NULL,
	draws_at TEXT NOT NULL,
	max_participants INTEGER NOT NULL DEFAULT 0,
	show_participant_count INTEGER NOT NULL DEFAULT 0,
	condition_match TEXT NOT NULL DEFAULT 'all',
	auto_bonus_percent REAL NOT NULL DEFAULT 25,
	status TEXT NOT NULL DEFAULT 'active',
	drawn_at TEXT NOT NULL DEFAULT '',
	visible_to_all INTEGER NOT NULL DEFAULT 1,
	visible_users_json TEXT NOT NULL DEFAULT '[]',
	daily_config_id INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS activity_conditions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	activity_id INTEGER NOT NULL REFERENCES activities(id) ON DELETE CASCADE,
	sort_order INTEGER NOT NULL DEFAULT 0,
	def_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS prizes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	activity_id INTEGER NOT NULL REFERENCES activities(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	prize_type TEXT NOT NULL,
	value REAL NOT NULL,
	weight REAL NOT NULL DEFAULT 0,
	stock INTEGER NOT NULL DEFAULT 1,
	granted_count INTEGER NOT NULL DEFAULT 0,
	codes_json TEXT NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS participants (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	activity_id INTEGER NOT NULL REFERENCES activities(id) ON DELETE CASCADE,
	user_id INTEGER NOT NULL,
	email TEXT NOT NULL,
	weight REAL NOT NULL DEFAULT 1,
	joined_at TEXT NOT NULL,
	UNIQUE(activity_id, user_id)
);

CREATE TABLE IF NOT EXISTS winners (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	activity_id INTEGER NOT NULL REFERENCES activities(id) ON DELETE CASCADE,
	user_id INTEGER NOT NULL,
	email TEXT NOT NULL,
	prize_id INTEGER NOT NULL,
	prize_name TEXT NOT NULL,
	prize_type TEXT NOT NULL,
	value REAL NOT NULL,
	fulfillment TEXT NOT NULL DEFAULT 'pending',
	redeem_code TEXT NOT NULL DEFAULT '',
	note TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	UNIQUE(activity_id, user_id)
);

CREATE TABLE IF NOT EXISTS user_daily_tokens (
	date TEXT NOT NULL,
	user_id INTEGER NOT NULL,
	email TEXT NOT NULL DEFAULT '',
	total_tokens REAL NOT NULL DEFAULT 0,
	PRIMARY KEY (date, user_id)
);

CREATE TABLE IF NOT EXISTS sync_state (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS user_registered (
	user_id INTEGER PRIMARY KEY,
	email TEXT NOT NULL DEFAULT '',
	registered_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_activities_draws_at ON activities(draws_at);
CREATE INDEX IF NOT EXISTS idx_participants_user ON participants(user_id);
CREATE INDEX IF NOT EXISTS idx_winners_user ON winners(user_id);
CREATE INDEX IF NOT EXISTS idx_user_daily_tokens_user_date ON user_daily_tokens(user_id, date);
`

const timeLayout = time.RFC3339Nano

func fmtTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTime(s string) time.Time {
	t, _ := time.Parse(timeLayout, s)
	return t
}

// CreateActivity inserts an activity with its conditions and prizes.
func (s *Store) CreateActivity(a *lottery.Activity) (int64, error) {
	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	visibleUsersJSON, err := json.Marshal(a.VisibleUsers)
	if err != nil {
		return 0, err
	}
	res, err := tx.Exec(
		`INSERT INTO activities (name, description, starts_at, draws_at, max_participants, show_participant_count, condition_match, auto_bonus_percent, status, visible_to_all, visible_users_json, daily_config_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, ?, ?, ?)`,
		a.Name, a.Description, fmtTime(a.StartsAt), fmtTime(a.DrawsAt),
		a.MaxParticipants, a.ShowParticipantCount, a.ConditionMatch, a.AutoBonusPercent, a.VisibleToAll, string(visibleUsersJSON), a.DailyConfigID, fmtTime(now), fmtTime(now),
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := insertChildren(tx, id, a); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func insertChildren(tx *sql.Tx, activityID int64, a *lottery.Activity) error {
	for i, c := range a.Conditions {
		def, err := conditionJSON(c)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(
			`INSERT INTO activity_conditions (activity_id, sort_order, def_json) VALUES (?, ?, ?)`,
			activityID, i, def,
		); err != nil {
			return err
		}
	}
	for _, p := range a.Prizes {
		codes, err := json.Marshal(p.Codes)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(
			`INSERT INTO prizes (activity_id, name, prize_type, value, weight, stock, granted_count, codes_json) VALUES (?, ?, ?, ?, ?, ?, 0, ?)`,
			activityID, p.Name, p.PrizeType, p.Value, p.Weight, p.Stock, string(codes),
		); err != nil {
			return err
		}
	}
	return nil
}

func conditionJSON(c lottery.ConditionDef) (string, error) {
	b, err := json.Marshal(c)
	return string(b), err
}

func conditionFromJSON(s string) (lottery.ConditionDef, error) {
	var c lottery.ConditionDef
	if err := json.Unmarshal([]byte(s), &c); err != nil {
		return lottery.ConditionDef{}, err
	}
	return c, nil
}

// UpdateActivity replaces the mutable fields, conditions and prizes of an activity.
// 已开奖活动不允许编辑。
func (s *Store) UpdateActivity(a *lottery.Activity) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var drawn int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM winners WHERE activity_id = ?`, a.ID).Scan(&drawn); err != nil {
		return err
	}
	if drawn > 0 {
		return ErrActivityDrawn
	}
	visibleUsersJSON, err := json.Marshal(a.VisibleUsers)
	if err != nil {
		return err
	}
	res, err := tx.Exec(
		`UPDATE activities SET name=?, description=?, starts_at=?, draws_at=?, max_participants=?, show_participant_count=?, condition_match=?, auto_bonus_percent=?, visible_to_all=?, visible_users_json=?, updated_at=? WHERE id=?`,
		a.Name, a.Description, fmtTime(a.StartsAt), fmtTime(a.DrawsAt),
		a.MaxParticipants, a.ShowParticipantCount, a.ConditionMatch, a.AutoBonusPercent, a.VisibleToAll, string(visibleUsersJSON), fmtTime(time.Now().UTC()), a.ID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err = tx.Exec(`DELETE FROM activity_conditions WHERE activity_id = ?`, a.ID); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM prizes WHERE activity_id = ?`, a.ID); err != nil {
		return err
	}
	if err := insertChildren(tx, a.ID, a); err != nil {
		return err
	}
	return tx.Commit()
}

// MarkDrawn stamps the draw time of an activity (idempotent per draw run).
func (s *Store) MarkDrawn(id int64, at time.Time) error {
	_, err := s.db.Exec(`UPDATE activities SET drawn_at = ?, updated_at = ? WHERE id = ? AND drawn_at = ''`,
		fmtTime(at), fmtTime(time.Now().UTC()), id)
	return err
}

// SetActivityStatus archives or restores an activity.
func (s *Store) SetActivityStatus(id int64, status string) error {
	res, err := s.db.Exec(`UPDATE activities SET status=?, updated_at=? WHERE id=?`, status, fmtTime(time.Now().UTC()), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanActivities(rows *sql.Rows) ([]lottery.Activity, error) {
	var out []lottery.Activity
	for rows.Next() {
		var a lottery.Activity
		var startsAt, drawsAt, createdAt, updatedAt, drawnAt string
		var visibleUsersJSON string
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &startsAt, &drawsAt, &a.MaxParticipants, &a.ShowParticipantCount,
			&a.ConditionMatch, &a.AutoBonusPercent, &a.Status, &drawnAt, &a.VisibleToAll, &visibleUsersJSON, &a.DailyConfigID, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if visibleUsersJSON != "" && visibleUsersJSON != "[]" {
			_ = json.Unmarshal([]byte(visibleUsersJSON), &a.VisibleUsers)
		}
		a.StartsAt, a.DrawsAt = parseTime(startsAt), parseTime(drawsAt)
		a.CreatedAt, a.UpdatedAt = parseTime(createdAt), parseTime(updatedAt)
		if drawnAt != "" {
			a.DrawnAt = parseTime(drawnAt)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

const activityColumns = `id, name, description, starts_at, draws_at, max_participants, show_participant_count, condition_match, auto_bonus_percent, status, drawn_at, visible_to_all, visible_users_json, daily_config_id, created_at, updated_at`

// ListActivities returns all activities ordered by draws_at.
func (s *Store) ListActivities() ([]lottery.Activity, error) {
	rows, err := s.db.Query(`SELECT ` + activityColumns + ` FROM activities ORDER BY draws_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	acts, err := scanActivities(rows)
	if err != nil {
		return nil, err
	}
	for i := range acts {
		if err := s.loadChildren(&acts[i]); err != nil {
			return nil, err
		}
	}
	return acts, nil
}

// GetActivity loads one activity with conditions and prizes.
func (s *Store) GetActivity(id int64) (*lottery.Activity, error) {
	rows, err := s.db.Query(`SELECT `+activityColumns+` FROM activities WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	acts, err := scanActivities(rows)
	if err != nil {
		return nil, err
	}
	if len(acts) == 0 {
		return nil, ErrNotFound
	}
	if err := s.loadChildren(&acts[0]); err != nil {
		return nil, err
	}
	return &acts[0], nil
}

// ListDrawableActivities returns active activities whose draw time has passed
// and that have not been drawn yet.
func (s *Store) ListDrawableActivities(now time.Time) ([]lottery.Activity, error) {
	rows, err := s.db.Query(
		`SELECT `+activityColumns+` FROM activities WHERE status='active' AND drawn_at = '' AND draws_at <= ? ORDER BY draws_at ASC`,
		fmtTime(now),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	acts, err := scanActivities(rows)
	if err != nil {
		return nil, err
	}
	for i := range acts {
		if err := s.loadChildren(&acts[i]); err != nil {
			return nil, err
		}
	}
	return acts, nil
}

func (s *Store) loadChildren(a *lottery.Activity) error {
	crows, err := s.db.Query(
		`SELECT def_json FROM activity_conditions WHERE activity_id = ? ORDER BY sort_order, id`, a.ID)
	if err != nil {
		return err
	}
	defer crows.Close()
	a.Conditions = nil
	for crows.Next() {
		var def string
		if err := crows.Scan(&def); err != nil {
			return err
		}
		c, err := conditionFromJSON(def)
		if err != nil {
			return err
		}
		a.Conditions = append(a.Conditions, c)
	}
	if err := crows.Err(); err != nil {
		return err
	}
	prows, err := s.db.Query(
		`SELECT id, activity_id, name, prize_type, value, weight, stock, granted_count, codes_json FROM prizes WHERE activity_id = ? ORDER BY id`, a.ID)
	if err != nil {
		return err
	}
	defer prows.Close()
	a.Prizes = nil
	for prows.Next() {
		var p lottery.Prize
		var codesJSON string
		if err := prows.Scan(&p.ID, &p.ActivityID, &p.Name, &p.PrizeType, &p.Value, &p.Weight, &p.Stock, &p.GrantedCount, &codesJSON); err != nil {
			return err
		}
		if codesJSON != "" && codesJSON != "[]" {
			_ = json.Unmarshal([]byte(codesJSON), &p.Codes)
		}
		a.Prizes = append(a.Prizes, p)
	}
	return prows.Err()
}
