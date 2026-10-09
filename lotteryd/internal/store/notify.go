package store

import (
	"database/sql"
	"strings"
	"time"

	"lotteryd/internal/lottery"
)

const notifySchemaSQL = `
CREATE TABLE IF NOT EXISTS activity_notify (
	activity_id INTEGER PRIMARY KEY,
	enabled INTEGER NOT NULL,
	stages TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS activity_push_log (
	activity_id INTEGER NOT NULL,
	stage TEXT NOT NULL,
	notified INTEGER NOT NULL,
	sent_at TEXT NOT NULL,
	PRIMARY KEY (activity_id, stage)
);

CREATE TABLE IF NOT EXISTS task_reward_push (
	reward_id INTEGER PRIMARY KEY,
	task_id INTEGER NOT NULL,
	user_id INTEGER NOT NULL,
	notified INTEGER NOT NULL,
	sent_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_task_reward_push_task ON task_reward_push(task_id, notified);
`

func (s *Store) migrateNotify() error {
	if _, err := s.db.Exec(notifySchemaSQL); err != nil {
		return err
	}
	return s.ensureColumn("activity_notify", "stages", "TEXT NOT NULL DEFAULT ''")
}

// SetActivityNotify 保存勾选的通知阶段。空列表表示不发送。
func (s *Store) SetActivityNotify(activityID int64, stages []string) error {
	normalized := lottery.NormalizeNotifyStages(stages)
	flag := 0
	if len(normalized) > 0 {
		flag = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO activity_notify(activity_id, enabled, stages) VALUES(?, ?, ?)
		ON CONFLICT(activity_id) DO UPDATE SET enabled = excluded.enabled, stages = excluded.stages
	`, activityID, flag, strings.Join(normalized, ","))
	return err
}

// ActivityNotifyStages 返回这场活动勾选的阶段。
// 旧数据只记了 enabled=1、没有阶段列表时，视为四个阶段都勾选。没有记录时为空。
func (s *Store) ActivityNotifyStages(activityID int64) ([]string, error) {
	var enabled int
	var raw string
	err := s.db.QueryRow(`SELECT enabled, stages FROM activity_notify WHERE activity_id = ?`, activityID).Scan(&enabled, &raw)
	if err == sql.ErrNoRows {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return decodeNotifyStages(enabled, raw), nil
}

// ListActivityNotifyStages 返回每场活动勾选的阶段。
func (s *Store) ListActivityNotifyStages() (map[int64][]string, error) {
	rows, err := s.db.Query(`SELECT activity_id, enabled, stages FROM activity_notify`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]string{}
	for rows.Next() {
		var id int64
		var enabled int
		var raw string
		if err := rows.Scan(&id, &enabled, &raw); err != nil {
			return nil, err
		}
		out[id] = decodeNotifyStages(enabled, raw)
	}
	return out, rows.Err()
}

func decodeNotifyStages(enabled int, raw string) []string {
	parts := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		if strings.TrimSpace(part) != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) > 0 {
		return lottery.NormalizeNotifyStages(parts)
	}
	if enabled != 0 {
		return lottery.AllNotifyStages()
	}
	return []string{}
}

// ListActivityPushLogs 返回每个活动各阶段已通知的用户数。
func (s *Store) ListActivityPushLogs() (map[int64]map[string]int, error) {
	rows, err := s.db.Query(`SELECT activity_id, stage, notified FROM activity_push_log`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]map[string]int{}
	for rows.Next() {
		var id int64
		var stage string
		var notified int
		if err := rows.Scan(&id, &stage, &notified); err != nil {
			return nil, err
		}
		if out[id] == nil {
			out[id] = map[string]int{}
		}
		out[id][stage] = notified
	}
	return out, rows.Err()
}

// MarkActivityPush 记录一个阶段已经通知过，避免调度器重复发送。
func (s *Store) MarkActivityPush(activityID int64, stage string, notified int) error {
	_, err := s.db.Exec(`
		INSERT INTO activity_push_log(activity_id, stage, notified, sent_at) VALUES(?, ?, ?, ?)
		ON CONFLICT(activity_id, stage) DO UPDATE SET notified = excluded.notified, sent_at = excluded.sent_at
	`, activityID, stage, notified, time.Now().UTC().Format(time.RFC3339))
	return err
}

// TaskRewardPush 一条已完成、尚未尝试通知的任务奖励。
type TaskRewardPush struct {
	RewardID int64
	TaskID   int64
	UserID   int64
	Email    string
	TaskName string
}

// ListUnnotifiedDoneTaskRewards 找出发放成功但还没走过通知的奖励。
func (s *Store) ListUnnotifiedDoneTaskRewards() ([]TaskRewardPush, error) {
	rows, err := s.db.Query(`
		SELECT r.id, r.task_id, r.user_id, r.email, t.name
		FROM task_rewards r
		JOIN tasks t ON t.id = r.task_id
		LEFT JOIN task_reward_push p ON p.reward_id = r.id
		WHERE r.fulfillment = 'done' AND p.reward_id IS NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TaskRewardPush
	for rows.Next() {
		var row TaskRewardPush
		if err := rows.Scan(&row.RewardID, &row.TaskID, &row.UserID, &row.Email, &row.TaskName); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// MarkTaskRewardNotified 记下这条奖励已经通知过。notified 为 false 表示对方当时没有开通知。
func (s *Store) MarkTaskRewardNotified(rewardID, taskID, userID int64, notified bool) error {
	flag := 0
	if notified {
		flag = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO task_reward_push(reward_id, task_id, user_id, notified, sent_at) VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(reward_id) DO UPDATE SET notified = excluded.notified, sent_at = excluded.sent_at
	`, rewardID, taskID, userID, flag, time.Now().UTC().Format(time.RFC3339))
	return err
}

// CountNotifiedTaskUsers 统计每个任务实际通知到的不同用户数。
func (s *Store) CountNotifiedTaskUsers() (map[int64]int64, error) {
	rows, err := s.db.Query(`SELECT task_id, COUNT(DISTINCT user_id) FROM task_reward_push WHERE notified = 1 GROUP BY task_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int64{}
	for rows.Next() {
		var taskID, count int64
		if err := rows.Scan(&taskID, &count); err != nil {
			return nil, err
		}
		out[taskID] = count
	}
	return out, rows.Err()
}

// NotifiedTaskUser 成功收到任务奖励通知的用户。
type NotifiedTaskUser struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
}

// ListNotifiedTaskUsers 返回该任务实际通知成功的用户，同一用户只出现一次。
func (s *Store) ListNotifiedTaskUsers(taskID int64) ([]NotifiedTaskUser, error) {
	rows, err := s.db.Query(`
		SELECT p.user_id, COALESCE(MAX(r.email), '')
		FROM task_reward_push p
		LEFT JOIN task_rewards r ON r.id = p.reward_id
		WHERE p.task_id = ? AND p.notified = 1
		GROUP BY p.user_id
		ORDER BY 2 COLLATE NOCASE, p.user_id
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]NotifiedTaskUser, 0)
	for rows.Next() {
		var row NotifiedTaskUser
		if err := rows.Scan(&row.UserID, &row.Email); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
