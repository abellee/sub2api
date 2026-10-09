package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"lotteryd/internal/lottery"
)

// 任务模块表结构（独立 schemaSQL，由 migrate 一并执行；全部 IF NOT EXISTS，
// 对既有库无影响）。
const taskSchemaSQL = `
CREATE TABLE IF NOT EXISTS tasks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	cover TEXT NOT NULL DEFAULT '',
	progress_svg TEXT NOT NULL DEFAULT '',
	progress_color TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	group_id INTEGER NOT NULL DEFAULT 0,
	group_name TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL DEFAULT '',
	start_date TEXT NOT NULL,
	duration_days INTEGER NOT NULL,
	settle_time TEXT NOT NULL,
	threshold_tokens REAL NOT NULL,
	reward_type TEXT NOT NULL,
	reward_value REAL NOT NULL,
	repeat_policy TEXT NOT NULL DEFAULT 'unlimited',
	whitelist_json TEXT NOT NULL DEFAULT '[]',
	blacklist_json TEXT NOT NULL DEFAULT '[]',
	conditions_json TEXT NOT NULL DEFAULT '[]',
	visibility_json TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'active',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS task_codes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	task_id INTEGER NOT NULL,
	code TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'available',
	reward_id INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	UNIQUE(task_id, code)
);
CREATE INDEX IF NOT EXISTS idx_task_codes_pool ON task_codes(task_id, status);

CREATE TABLE IF NOT EXISTS task_rewards (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	task_id INTEGER NOT NULL,
	settle_date TEXT NOT NULL,
	user_id INTEGER NOT NULL,
	email TEXT NOT NULL DEFAULT '',
	tokens REAL NOT NULL DEFAULT 0,
	units INTEGER NOT NULL DEFAULT 0,
	reward_type TEXT NOT NULL,
	reward_value REAL NOT NULL DEFAULT 0,
	fulfillment TEXT NOT NULL DEFAULT 'pending',
	codes_json TEXT NOT NULL DEFAULT '[]',
	note TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	UNIQUE(task_id, settle_date, user_id)
);
CREATE INDEX IF NOT EXISTS idx_task_rewards_user ON task_rewards(user_id);
CREATE INDEX IF NOT EXISTS idx_task_rewards_fulfill ON task_rewards(task_id, fulfillment);

-- 本地测试消耗：只加到用户侧进度条，不参与结算。
CREATE TABLE IF NOT EXISTS task_test_usage (
	date TEXT NOT NULL,
	group_id INTEGER NOT NULL DEFAULT 0,
	model TEXT NOT NULL DEFAULT '',
	tokens REAL NOT NULL DEFAULT 0,
	PRIMARY KEY (date, group_id, model)
);
`

const taskColumns = `id, name, cover, progress_svg, progress_color, description, group_id, group_name, model, start_date, duration_days, settle_time,
	threshold_tokens, reward_type, reward_value, repeat_policy, whitelist_json, blacklist_json, conditions_json,
	visibility_json, status, created_at, updated_at`

// ErrCodePoolEmpty 码池可用数量不足。
var ErrCodePoolEmpty = errors.New("task code pool has not enough available codes")

// TaskTestUsage 读取某一天、某一分组和模型的本地测试消耗。没有记录时返回 0。
// 只给进度条用，结算不读这张表。
func (s *Store) TaskTestUsage(date string, groupID int64, model string) (float64, error) {
	var tokens float64
	err := s.db.QueryRow(
		`SELECT tokens FROM task_test_usage WHERE date = ? AND group_id = ? AND model = ?`,
		date, groupID, model,
	).Scan(&tokens)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return tokens, err
}

// UpsertTaskTestUsage 写入本地测试消耗。
func (s *Store) UpsertTaskTestUsage(date string, groupID int64, model string, tokens float64) error {
	_, err := s.db.Exec(
		`INSERT INTO task_test_usage (date, group_id, model, tokens) VALUES (?, ?, ?, ?)
		 ON CONFLICT(date, group_id, model) DO UPDATE SET tokens = excluded.tokens`,
		date, groupID, model, tokens,
	)
	return err
}

func scanTaskRow(row interface{ Scan(...any) error }) (*lottery.Task, error) {
	var t lottery.Task
	var whitelistJSON, blacklistJSON, conditionsJSON, visibilityJSON string
	if err := row.Scan(&t.ID, &t.Name, &t.Cover, &t.ProgressSvg, &t.ProgressColor, &t.Description, &t.GroupID, &t.GroupName, &t.Model, &t.StartDate,
		&t.DurationDays, &t.SettleTime, &t.ThresholdTokens, &t.RewardType, &t.RewardValue, &t.RepeatPolicy,
		&whitelistJSON, &blacklistJSON, &conditionsJSON, &visibilityJSON, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(whitelistJSON), &t.Whitelist)
	_ = json.Unmarshal([]byte(blacklistJSON), &t.Blacklist)
	_ = json.Unmarshal([]byte(conditionsJSON), &t.Conditions)
	if visibilityJSON != "" {
		_ = json.Unmarshal([]byte(visibilityJSON), &t.Visibility)
	}
	t.Visibility = lottery.NormalizeItemVisibility(t.Visibility)
	if t.RepeatPolicy == "" {
		t.RepeatPolicy = lottery.TaskRepeatUnlimited
	}
	return &t, nil
}

// UpsertTask 新建（ID=0）或更新任务。
func (s *Store) UpsertTask(t *lottery.Task) (int64, error) {
	if err := t.Validate(); err != nil {
		return 0, err
	}
	whitelistJSON, _ := json.Marshal(t.Whitelist)
	blacklistJSON, _ := json.Marshal(t.Blacklist)
	conditionsJSON, _ := json.Marshal(t.Conditions)
	visibilityJSON, err := json.Marshal(lottery.NormalizeItemVisibility(t.Visibility))
	if err != nil {
		return 0, err
	}
	now := fmtTime(time.Now().UTC())
	if t.Status == "" {
		t.Status = lottery.TaskActive
	}

	if t.ID > 0 {
		res, err := s.db.Exec(
			`UPDATE tasks SET name=?, cover=?, progress_svg=?, progress_color=?, description=?, group_id=?, group_name=?, model=?, start_date=?, duration_days=?,
			settle_time=?, threshold_tokens=?, reward_type=?, reward_value=?, repeat_policy=?, whitelist_json=?,
			blacklist_json=?, conditions_json=?, visibility_json=?, status=?, updated_at=? WHERE id=?`,
			t.Name, t.Cover, t.ProgressSvg, t.ProgressColor, t.Description, t.GroupID, t.GroupName, t.Model, t.StartDate, t.DurationDays,
			t.SettleTime, t.ThresholdTokens, t.RewardType, t.RewardValue, t.RepeatPolicy, string(whitelistJSON),
			string(blacklistJSON), string(conditionsJSON), string(visibilityJSON), t.Status, now, t.ID,
		)
		if err != nil {
			return 0, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return 0, ErrNotFound
		}
		return t.ID, nil
	}

	res, err := s.db.Exec(
		`INSERT INTO tasks (name, cover, progress_svg, progress_color, description, group_id, group_name, model, start_date, duration_days, settle_time,
		threshold_tokens, reward_type, reward_value, repeat_policy, whitelist_json, blacklist_json, conditions_json,
		visibility_json, status, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.Name, t.Cover, t.ProgressSvg, t.ProgressColor, t.Description, t.GroupID, t.GroupName, t.Model, t.StartDate, t.DurationDays,
		t.SettleTime, t.ThresholdTokens, t.RewardType, t.RewardValue, t.RepeatPolicy, string(whitelistJSON),
		string(blacklistJSON), string(conditionsJSON), string(visibilityJSON), t.Status, now, now,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	t.ID = id
	return id, err
}

// GetTask 读取单个任务。
func (s *Store) GetTask(id int64) (*lottery.Task, error) {
	row := s.db.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE id = ?`, id)
	t, err := scanTaskRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

// ListTasks 读取全部任务（创建时间倒序）。
func (s *Store) ListTasks() ([]lottery.Task, error) {
	rows, err := s.db.Query(`SELECT ` + taskColumns + ` FROM tasks ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []lottery.Task{}
	for rows.Next() {
		t, err := scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// SetTaskStatus 归档/恢复任务状态。
func (s *Store) SetTaskStatus(id int64, status string) error {
	res, err := s.db.Exec(`UPDATE tasks SET status=?, updated_at=? WHERE id=?`, status, fmtTime(time.Now().UTC()), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteTask 删除任务及其码池与结算记录（管理员二次确认后调用）。
func (s *Store) DeleteTask(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, stmt := range []string{
		`DELETE FROM task_codes WHERE task_id = ?`,
		`DELETE FROM task_reward_push WHERE task_id = ?`,
		`DELETE FROM task_rewards WHERE task_id = ?`,
		`DELETE FROM tasks WHERE id = ?`,
	} {
		if _, err := tx.Exec(stmt, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---- 码池 ----

// ReplaceAvailableTaskCodes 用给定列表整体替换「可用」码；已发放（granted）记录保留。
func (s *Store) ReplaceAvailableTaskCodes(taskID int64, codes []string) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM task_codes WHERE task_id = ? AND status = 'available'`, taskID); err != nil {
		return 0, err
	}
	now := fmtTime(time.Now().UTC())
	n := int64(0)
	for _, code := range codes {
		res, err := tx.Exec(
			`INSERT OR IGNORE INTO task_codes (task_id, code, status, created_at) VALUES (?, ?, 'available', ?)`,
			taskID, code, now,
		)
		if err != nil {
			return 0, err
		}
		if affected, _ := res.RowsAffected(); affected > 0 {
			n++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}

// TaskCodeCounts 返回码池可用/已发放数量。
func (s *Store) TaskCodeCounts(taskID int64) (available int64, granted int64, err error) {
	row := s.db.QueryRow(`SELECT
		COALESCE(SUM(CASE WHEN status = 'available' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status = 'granted' THEN 1 ELSE 0 END), 0)
		FROM task_codes WHERE task_id = ?`, taskID)
	return available, granted, row.Scan(&available, &granted)
}

// ListTaskCodes 列出码池（管理端展示）。
func (s *Store) ListTaskCodes(taskID int64) ([]lottery.TaskCode, error) {
	rows, err := s.db.Query(
		`SELECT id, task_id, code, status, reward_id, created_at FROM task_codes WHERE task_id = ? ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []lottery.TaskCode{}
	for rows.Next() {
		var c lottery.TaskCode
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Code, &c.Status, &c.RewardID, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// TakeRandomTaskCodes 随机抽取 n 张可用码并标记为 granted（绑定 rewardID）。
// 可用数量不足时不做任何修改，返回 ErrCodePoolEmpty。
func (s *Store) TakeRandomTaskCodes(taskID int64, n int, rewardID int64) ([]string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var available int64
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM task_codes WHERE task_id = ? AND status = 'available'`, taskID).Scan(&available); err != nil {
		return nil, err
	}
	if available < int64(n) {
		return nil, ErrCodePoolEmpty
	}
	rows, err := tx.Query(
		`SELECT id, code FROM task_codes WHERE task_id = ? AND status = 'available' ORDER BY RANDOM() LIMIT ?`,
		taskID, n)
	if err != nil {
		return nil, err
	}
	type picked struct {
		id   int64
		code string
	}
	var picks []picked
	for rows.Next() {
		var p picked
		if err := rows.Scan(&p.id, &p.code); err != nil {
			rows.Close()
			return nil, err
		}
		picks = append(picks, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for _, p := range picks {
		if _, err := tx.Exec(
			`UPDATE task_codes SET status = 'granted', reward_id = ? WHERE id = ?`, rewardID, p.id); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	codes := make([]string, 0, len(picks))
	for _, p := range picks {
		codes = append(codes, p.code)
	}
	return codes, nil
}

// ---- 结算记录 ----

const taskRewardColumns = `id, task_id, settle_date, user_id, email, tokens, units, reward_type, reward_value, fulfillment, codes_json, note, error, created_at`

func scanTaskReward(row interface{ Scan(...any) error }) (*lottery.TaskReward, error) {
	var r lottery.TaskReward
	var codesJSON string
	if err := row.Scan(&r.ID, &r.TaskID, &r.SettleDate, &r.UserID, &r.Email, &r.Tokens, &r.Units,
		&r.RewardType, &r.RewardValue, &r.Fulfillment, &codesJSON, &r.Note, &r.Err, &r.CreatedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(codesJSON), &r.Codes)
	return &r, nil
}

func scanTaskRewardWithTaskName(row interface{ Scan(...any) error }) (*lottery.TaskReward, error) {
	var r lottery.TaskReward
	var codesJSON string
	if err := row.Scan(&r.ID, &r.TaskID, &r.SettleDate, &r.UserID, &r.Email, &r.Tokens, &r.Units,
		&r.RewardType, &r.RewardValue, &r.Fulfillment, &codesJSON, &r.Note, &r.Err, &r.CreatedAt, &r.TaskName); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(codesJSON), &r.Codes)
	return &r, nil
}

// InsertTaskReward 插入一条结算记录（UNIQUE(task_id,settle_date,user_id) 幂等；
// 已存在时返回 (0, false, nil)）。
func (s *Store) InsertTaskReward(r *lottery.TaskReward) (int64, bool, error) {
	codesJSON, _ := json.Marshal(r.Codes)
	res, err := s.db.Exec(
		`INSERT OR IGNORE INTO task_rewards (task_id, settle_date, user_id, email, tokens, units, reward_type,
		reward_value, fulfillment, codes_json, note, error, created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.TaskID, r.SettleDate, r.UserID, r.Email, r.Tokens, r.Units, r.RewardType,
		r.RewardValue, r.Fulfillment, string(codesJSON), r.Note, r.Err, fmtTime(time.Now().UTC()),
	)
	if err != nil {
		return 0, false, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, false, nil
	}
	id, err := res.LastInsertId()
	r.ID = id
	return id, true, err
}

// UpdateTaskReward 更新发放状态/兑换码/备注。
func (s *Store) UpdateTaskReward(id int64, fulfillment string, codes []string, note, errMsg string) error {
	codesJSON, _ := json.Marshal(codes)
	_, err := s.db.Exec(
		`UPDATE task_rewards SET fulfillment=?, codes_json=?, note=?, error=? WHERE id=?`,
		fulfillment, string(codesJSON), note, errMsg, id,
	)
	return err
}

// ListTaskRewards 某任务的全部结算记录（含任务名，管理端发放记录页）。
func (s *Store) ListTaskRewards(taskID int64) ([]lottery.TaskReward, error) {
	rows, err := s.db.Query(
		`SELECT r.id, r.task_id, r.settle_date, r.user_id, r.email, r.tokens, r.units, r.reward_type,
		r.reward_value, r.fulfillment, r.codes_json, r.note, r.error, r.created_at,
		COALESCE((SELECT name FROM tasks WHERE tasks.id = r.task_id), '')
		FROM task_rewards r WHERE r.task_id = ? ORDER BY r.settle_date DESC, r.id DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []lottery.TaskReward{}
	for rows.Next() {
		r, err := scanTaskRewardWithTaskName(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// ListUnfulfilledTaskRewards 某任务 pending/failed 的结算记录（重试发放用）。
func (s *Store) ListUnfulfilledTaskRewards(taskID int64) ([]lottery.TaskReward, error) {
	rows, err := s.db.Query(
		`SELECT `+taskRewardColumns+` FROM task_rewards WHERE task_id = ? AND fulfillment IN ('pending','failed') ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []lottery.TaskReward{}
	for rows.Next() {
		r, err := scanTaskReward(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// ListUserTaskRewards 某用户的全部结算记录（用户侧「我的奖励」，含任务名）。
func (s *Store) ListUserTaskRewards(userID int64) ([]lottery.TaskReward, error) {
	rows, err := s.db.Query(
		`SELECT r.id, r.task_id, r.settle_date, r.user_id, r.email, r.tokens, r.units, r.reward_type,
		r.reward_value, r.fulfillment, r.codes_json, r.note, r.error, r.created_at,
		COALESCE((SELECT name FROM tasks WHERE tasks.id = r.task_id), '')
		FROM task_rewards r WHERE r.user_id = ? ORDER BY r.created_at DESC, r.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []lottery.TaskReward{}
	for rows.Next() {
		r, err := scanTaskRewardWithTaskName(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// HasUserTaskReward 用户在某任务下是否已有任意发放记录（重复参与策略封顶判断）。
func (s *Store) HasUserTaskReward(taskID, userID int64) (bool, error) {
	var n int64
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM task_rewards WHERE task_id = ? AND user_id = ?`, taskID, userID).Scan(&n)
	return n > 0, err
}
