// Package migrator 一次性把旧 SQLite 库的数据整体搬进 PostgreSQL。
// 目标表非空时拒绝执行；插入保留原 id，结束后把 identity 序列拨到 max(id)+1。
package migrator

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"

	_ "modernc.org/sqlite"

	"lotteryd/internal/store"
)

type tableSpec struct {
	name     string
	cols     []string
	boolCols map[string]bool // SQLite INTEGER 0/1 → PostgreSQL BOOLEAN
	identity bool            // 自增主键表，迁移后需要修正序列
}

var tables = []tableSpec{
	{name: "activities", identity: true, boolCols: map[string]bool{"show_participant_count": true, "visible_to_all": true}, cols: []string{
		"id", "name", "description", "starts_at", "draws_at", "max_participants", "show_participant_count",
		"condition_match", "auto_bonus_percent", "status", "drawn_at", "visible_to_all", "visible_users_json",
		"daily_config_id", "repeat_group", "repeat_policy", "visibility_json", "created_at", "updated_at"}},
	{name: "activity_conditions", identity: true, cols: []string{"id", "activity_id", "sort_order", "def_json"}},
	{name: "prizes", identity: true, cols: []string{"id", "activity_id", "name", "prize_type", "value", "weight", "stock", "granted_count", "codes_json"}},
	{name: "participants", identity: true, cols: []string{"id", "activity_id", "user_id", "email", "weight", "joined_at"}},
	{name: "winners", identity: true, cols: []string{"id", "activity_id", "user_id", "email", "prize_id", "prize_name", "prize_type", "value", "fulfillment", "redeem_code", "note", "error", "created_at"}},
	{name: "user_daily_tokens", cols: []string{"date", "user_id", "email", "total_tokens"}},
	{name: "sync_state", cols: []string{"key", "value"}},
	{name: "user_registered", cols: []string{"user_id", "email", "registered_at"}},
	{name: "tasks", identity: true, cols: []string{
		"id", "name", "cover", "progress_svg", "progress_color", "description", "group_id", "group_name", "model",
		"start_date", "duration_days", "settle_time", "threshold_tokens", "reward_type", "reward_value",
		"repeat_policy", "whitelist_json", "blacklist_json", "conditions_json", "visibility_json", "status",
		"created_at", "updated_at"}},
	{name: "task_codes", identity: true, cols: []string{"id", "task_id", "code", "status", "reward_id", "created_at"}},
	{name: "task_rewards", identity: true, cols: []string{"id", "task_id", "settle_date", "user_id", "email", "tokens", "units", "reward_type", "reward_value", "fulfillment", "codes_json", "note", "error", "created_at"}},
	{name: "task_test_usage", cols: []string{"date", "group_id", "model", "tokens"}},
	{name: "activity_notify", boolCols: map[string]bool{"enabled": true}, cols: []string{"activity_id", "enabled", "stages"}},
	{name: "activity_push_log", cols: []string{"activity_id", "stage", "notified", "sent_at"}},
	{name: "task_reward_push", boolCols: map[string]bool{"notified": true}, cols: []string{"reward_id", "task_id", "user_id", "notified", "sent_at"}},
}

// Run 执行 SQLite → PostgreSQL 迁移。
func Run(pgDSN, sqlitePath string) error {
	if _, err := os.Stat(sqlitePath); err != nil {
		return fmt.Errorf("sqlite file: %w", err)
	}
	if err := store.EnsureDatabase(pgDSN); err != nil {
		return err
	}
	sq, err := sql.Open("sqlite", sqlitePath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	defer func() { _ = sq.Close() }()
	if _, err := sq.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return fmt.Errorf("wal checkpoint: %w", err)
	}

	pg, err := store.Open(pgDSN)
	if err != nil {
		return err
	}
	defer func() { _ = pg.Close() }()

	tx, err := pg.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// 目标非空即拒绝，防止重复导入产生主键冲突或脏数据。
	for _, t := range tables {
		var n int64
		if err := tx.QueryRow(`SELECT COUNT(*) FROM ` + t.name).Scan(&n); err != nil {
			return fmt.Errorf("count %s: %w", t.name, err)
		}
		if n > 0 {
			return fmt.Errorf("target table %s already has %d rows; refusing to migrate into a non-empty database", t.name, n)
		}
	}

	report := map[string]int64{}
	for _, t := range tables {
		n, err := copyTable(tx, sq, t)
		if err != nil {
			return fmt.Errorf("copy %s: %w", t.name, err)
		}
		report[t.name] = n
		if t.identity && n > 0 {
			if _, err := tx.Exec(fmt.Sprintf(
				`SELECT setval(pg_get_serial_sequence('%s', 'id'), COALESCE((SELECT MAX(id) FROM %s), 0) + 1, false)`,
				t.name, t.name)); err != nil {
				return fmt.Errorf("fix sequence %s: %w", t.name, err)
			}
		}
	}

	// 提交前逐表比对行数，任何不一致都回滚。
	for _, t := range tables {
		var want, got int64
		if err := sq.QueryRow(`SELECT COUNT(*) FROM ` + t.name).Scan(&want); err != nil {
			return fmt.Errorf("verify source %s: %w", t.name, err)
		}
		if err := tx.QueryRow(`SELECT COUNT(*) FROM ` + t.name).Scan(&got); err != nil {
			return fmt.Errorf("verify target %s: %w", t.name, err)
		}
		if want != got {
			return fmt.Errorf("row count mismatch for %s: sqlite=%d postgres=%d", t.name, want, got)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	for _, t := range tables {
		slog.Info("migrated", "table", t.name, "rows", report[t.name])
	}
	slog.Info("sqlite → postgres migration complete", "sqlite", sqlitePath)
	return nil
}

// copyTable 把一张表的所有行复制进目标库，返回复制的行数。
// 列取 SQLite 与 PostgreSQL 两边的交集，SQLite 缺列时走 PG 默认值。
func copyTable(tx *sql.Tx, sq *sql.DB, t tableSpec) (int64, error) {
	present, err := sqliteColumns(sq, t.name)
	if err != nil {
		return 0, err
	}
	if len(present) == 0 {
		slog.Warn("source table missing, skipped", "table", t.name)
		return 0, nil
	}
	cols := make([]string, 0, len(t.cols))
	for _, c := range t.cols {
		if _, ok := present[c]; ok {
			cols = append(cols, c)
		}
	}
	if len(cols) == 0 {
		slog.Warn("no shared columns, skipped", "table", t.name)
		return 0, nil
	}

	rows, err := sq.Query(`SELECT ` + strings.Join(cols, ", ") + ` FROM ` + t.name)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	stmt := `INSERT INTO ` + t.name + ` (` + strings.Join(cols, ", ") + `) VALUES (` + strings.Join(placeholders, ", ") + `)`

	var copied int64
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return 0, err
		}
		for i, c := range cols {
			if t.boolCols[c] {
				vals[i] = toBool(vals[i])
			}
		}
		if _, err := tx.Exec(stmt, vals...); err != nil {
			return 0, err
		}
		copied++
	}
	return copied, rows.Err()
}

func toBool(v any) any {
	switch x := v.(type) {
	case int64:
		return x != 0
	case []byte:
		return string(x) == "1" || string(x) == "true"
	case string:
		return x == "1" || x == "true"
	case bool:
		return x
	case nil:
		return false
	default:
		return v
	}
}

// sqliteColumns 返回 SQLite 表的列名集合；表不存在时返回空集。
func sqliteColumns(sq *sql.DB, table string) (map[string]struct{}, error) {
	rows, err := sq.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = struct{}{}
	}
	return out, rows.Err()
}
