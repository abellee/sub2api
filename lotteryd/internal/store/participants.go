package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"lotteryd/internal/lottery"
)

// Sentinel errors surfaced to the HTTP layer.
var (
	ErrParticipantRemoved = errors.New("您已被移出本场抽奖，不能再次参与")
	ErrNotFound           = errors.New("not found")
	ErrActivityDrawn      = errors.New("activity already drawn")
	ErrAlreadyJoined      = errors.New("already joined")
	ErrActivityFull       = errors.New("activity is full")
	ErrNotJoinable        = errors.New("activity is not open for participation")
	ErrNotEligible        = errors.New("conditions not met")
	ErrNotVisible         = errors.New("lottery is not available to this user")
	ErrRepeatJoin         = errors.New("you have already joined this lottery series")
	ErrRepeatWin          = errors.New("you have already won in this lottery series")
)

// ---- Participants ----

// CountParticipants returns the number of participants of an activity.
func (s *Store) CountParticipants(activityID int64) (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM participants WHERE activity_id = $1 AND removed_at = ''`, activityID).Scan(&n)
	return n, err
}

// HasParticipant reports whether the user already joined the activity.
func (s *Store) HasParticipant(activityID, userID int64) (bool, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM participants WHERE activity_id = $1 AND user_id = $2 AND removed_at = ''`, activityID, userID).Scan(&n)
	return n > 0, err
}

// AddParticipant joins a user into an activity. The uniqueness and capacity
// checks run inside a transaction so concurrent joins cannot overbook.
func (s *Store) AddParticipant(activityID, userID int64, email string, weight float64, maxParticipants int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockUndrawnActivity(tx, activityID); err != nil {
		return err
	}
	var status string
	if err := tx.QueryRow(`SELECT max_participants, status FROM activities WHERE id=$1`, activityID).Scan(&maxParticipants, &status); err != nil {
		return err
	}
	if status != "active" {
		return ErrNotJoinable
	}
	var removedAt string
	err = tx.QueryRow(`SELECT removed_at FROM participants WHERE activity_id=$1 AND user_id=$2`, activityID, userID).Scan(&removedAt)
	if err == nil {
		if removedAt != "" {
			return ErrParticipantRemoved
		}
		return ErrAlreadyJoined
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if maxParticipants > 0 {
		var n int64
		if err := tx.QueryRow(
			`SELECT COUNT(*) FROM participants WHERE activity_id = $1 AND removed_at = ''`, activityID).Scan(&n); err != nil {
			return err
		}
		if n >= maxParticipants {
			return ErrActivityFull
		}
	}
	_, err = tx.Exec(
		`INSERT INTO participants (activity_id, user_id, email, weight, joined_at) VALUES ($1, $2, $3, $4, $5)`,
		activityID, userID, email, weight, fmtTime(time.Now().UTC()),
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ListParticipants returns all participants of an activity.
func (s *Store) ListParticipants(activityID int64) ([]lottery.Participant, error) {
	return listParticipants(s.db, activityID)
}

func listParticipants(db activityQuerier, activityID int64) ([]lottery.Participant, error) {
	rows, err := db.Query(
		`SELECT id, activity_id, user_id, email, weight, joined_at FROM participants WHERE activity_id = $1 AND removed_at = '' ORDER BY id`,
		activityID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []lottery.Participant
	for rows.Next() {
		var p lottery.Participant
		var joinedAt string
		if err := rows.Scan(&p.ID, &p.ActivityID, &p.UserID, &p.Email, &p.Weight, &joinedAt); err != nil {
			return nil, err
		}
		p.JoinedAt = parseTime(joinedAt)
		out = append(out, p)
	}
	return out, rows.Err()
}

// HasWinner reports whether the user already won in the activity.
func (s *Store) HasWinner(activityID, userID int64) (bool, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM winners WHERE activity_id = $1 AND user_id = $2`, activityID, userID).Scan(&n)
	return n > 0, err
}

// ---- Winners ----

// InsertWinner records a winner and increments the prize granted count in one tx.
func (s *Store) InsertWinner(w *lottery.Winner) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`INSERT INTO winners (activity_id, user_id, email, prize_id, prize_name, prize_type, value, fulfillment, redeem_code, note, error, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		w.ActivityID, w.UserID, w.Email, w.PrizeID, w.PrizeName, w.PrizeType, w.Value,
		w.Fulfillment, w.RedeemCode, w.Note, w.Error, fmtTime(w.CreatedAt),
	); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE prizes SET granted_count = granted_count + 1 WHERE id = $1`, w.PrizeID); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateWinnerFulfillment updates the fulfillment state of a winner record.
func (s *Store) UpdateWinnerFulfillment(id int64, fulfillment, redeemCode, note, errMsg string) error {
	_, err := s.db.Exec(
		`UPDATE winners SET fulfillment=$1, redeem_code=$2, note=$3, error=$4 WHERE id=$5`,
		fulfillment, redeemCode, note, errMsg, id,
	)
	return err
}

func scanWinners(rows *sql.Rows) ([]lottery.Winner, error) {
	var out []lottery.Winner
	for rows.Next() {
		var w lottery.Winner
		var createdAt string
		if err := rows.Scan(&w.ID, &w.ActivityID, &w.UserID, &w.Email, &w.PrizeID, &w.PrizeName,
			&w.PrizeType, &w.Value, &w.Fulfillment, &w.RedeemCode, &w.Note, &w.Error, &createdAt); err != nil {
			return nil, err
		}
		w.CreatedAt = parseTime(createdAt)
		out = append(out, w)
	}
	return out, rows.Err()
}

const winnerColumns = `id, activity_id, user_id, email, prize_id, prize_name, prize_type, value, fulfillment, redeem_code, note, error, created_at`

// ListWinnersByActivity returns all winner records of an activity (raw email).
func (s *Store) ListWinnersByActivity(activityID int64) ([]lottery.Winner, error) {
	rows, err := s.db.Query(
		`SELECT `+winnerColumns+` FROM winners WHERE activity_id = $1 ORDER BY id`, activityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWinners(rows)
}

// ListWinnersByUser returns all winner records of a user across activities,
// with the activity name joined for display.
func (s *Store) ListWinnersByUser(userID int64) ([]lottery.Winner, error) {
	rows, err := s.db.Query(
		`SELECT w.id, w.activity_id,
		        (SELECT name FROM activities WHERE id = w.activity_id) AS activity_name,
		        w.user_id, w.email, w.prize_id, w.prize_name, w.prize_type, w.value,
		        w.fulfillment, w.redeem_code, w.note, w.error, w.created_at
		 FROM winners w
		 WHERE w.user_id = $1 ORDER BY w.created_at DESC, w.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []lottery.Winner
	for rows.Next() {
		var w lottery.Winner
		var createdAt string
		if err := rows.Scan(&w.ID, &w.ActivityID, &w.ActivityName, &w.UserID, &w.Email, &w.PrizeID, &w.PrizeName,
			&w.PrizeType, &w.Value, &w.Fulfillment, &w.RedeemCode, &w.Note, &w.Error, &createdAt); err != nil {
			return nil, err
		}
		w.CreatedAt = parseTime(createdAt)
		out = append(out, w)
	}
	return out, rows.Err()
}

// ListUnfulfilledWinners returns pending/failed winner records of an activity.
func (s *Store) ListUnfulfilledWinners(activityID int64) ([]lottery.Winner, error) {
	rows, err := s.db.Query(
		`SELECT `+winnerColumns+` FROM winners WHERE activity_id = $1 AND fulfillment IN ('pending','failed') ORDER BY id`,
		activityID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWinners(rows)
}

// CountWinners returns the number of winners of an activity.
func (s *Store) CountWinners(activityID int64) (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM winners WHERE activity_id = $1`, activityID).Scan(&n)
	return n, err
}

// ---- User daily tokens ----

// ReplaceDailyTokens atomically replaces a complete daily snapshot, including users now absent.
func (s *Store) ReplaceDailyTokens(date string, tokens []lottery.DailyToken) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM user_daily_tokens WHERE date = $1`, date); err != nil {
		return err
	}

	for _, t := range tokens {
		if _, err := tx.Exec(
			`INSERT INTO user_daily_tokens (date, user_id, email, total_tokens) VALUES ($1, $2, $3, $4)
			 ON CONFLICT(date, user_id) DO UPDATE SET email = excluded.email, total_tokens = excluded.total_tokens`,
			date, t.UserID, t.Email, t.TotalTokens,
		); err != nil {
			return fmt.Errorf("upsert %d: %w", t.UserID, err)
		}
	}
	return tx.Commit()
}

// DailyTokensWindow returns all per-user daily token rows within [start, end] (inclusive dates).
func (s *Store) DailyTokensWindow(start, end string) (map[int64]*lottery.UserUsage, error) {
	rows, err := s.db.Query(
		`SELECT date, user_id, email, total_tokens FROM user_daily_tokens
		 WHERE date >= $1 AND date <= $2 ORDER BY date ASC`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]*lottery.UserUsage{}
	for rows.Next() {
		var date, email string
		var userID int64
		var tokens float64
		if err := rows.Scan(&date, &userID, &email, &tokens); err != nil {
			return nil, err
		}
		u := out[userID]
		if u == nil {
			u = &lottery.UserUsage{UserID: userID, Email: email}
			out[userID] = u
		}
		if email != "" {
			u.Email = email
		}
		u.Daily = append(u.Daily, lottery.DailyToken{Date: date, UserID: userID, Email: email, TotalTokens: tokens})
	}
	return out, rows.Err()
}

// ---- Sync state ----

// GetState returns a state value ("" when absent).
func (s *Store) GetState(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM sync_state WHERE key = $1`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

// SetState stores a state value.
func (s *Store) SetState(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO sync_state (key, value) VALUES ($1, $2)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// ---- Registered dates ----

// GetRegisteredAt 返回缓存的用户注册时间（未缓存时返回 nil, nil）。
func (s *Store) GetRegisteredAt(userID int64) (*time.Time, error) {
	var raw string
	err := s.db.QueryRow(`SELECT registered_at FROM user_registered WHERE user_id = $1`, userID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t := parseTime(raw)
	if t.IsZero() {
		return nil, nil
	}
	return &t, nil
}

// UpsertRegisteredAt 缓存用户注册时间（注册时间不可变，重复写入无害）。
func (s *Store) UpsertRegisteredAt(userID int64, email string, t time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO user_registered (user_id, email, registered_at) VALUES ($1, $2, $3)
		 ON CONFLICT(user_id) DO UPDATE SET email = excluded.email, registered_at = excluded.registered_at`,
		userID, email, fmtTime(t))
	return err
}

// ---- 系列重复参与查询（日常定时抽奖） ----

// HasUserParticipatedInConfig 用户是否参与过该配置系列的任意场次。
func (s *Store) HasUserParticipatedInConfig(configID, userID int64) (bool, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM participants p JOIN activities a ON p.activity_id = a.id
		 WHERE a.daily_config_id = $1 AND p.user_id = $2 AND p.removed_at = ''`, configID, userID).Scan(&n)
	return n > 0, err
}

// HasUserWonInConfig 用户是否在该配置系列中中过奖。
func (s *Store) HasUserWonInConfig(configID, userID int64) (bool, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM winners w JOIN activities a ON w.activity_id = a.id
		 WHERE a.daily_config_id = $1 AND w.user_id = $2`, configID, userID).Scan(&n)
	return n > 0, err
}

// HasUserRecordInRepeatSet 用户在共用参与组里是否已有参与或中奖记录。
// group 匹配场次上写下的组名；configIDs 覆盖还没写上组名的旧场次。
// kind 为 "win" 时查中奖，否则查参与。
func (s *Store) HasUserRecordInRepeatSet(kind, group string, configIDs []int64, userID int64) (bool, error) {
	group = strings.TrimSpace(group)
	ids := make([]int64, 0, len(configIDs))
	for _, id := range configIDs {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	if group == "" && len(ids) == 0 {
		return false, nil
	}
	table := "participants"
	if kind == "win" {
		table = "winners"
	}
	args := make([]any, 0, 1+len(ids))
	clauses := make([]string, 0, 2)
	if group != "" {
		args = append(args, group)
		clauses = append(clauses, fmt.Sprintf("a.repeat_group = $%d", len(args)))
	}
	for _, id := range ids {
		args = append(args, id)
	}
	if len(ids) > 0 {
		placeholders := make([]string, len(ids))
		for i := range ids {
			placeholders[i] = fmt.Sprintf("$%d", len(args)-len(ids)+i+1)
		}
		clauses = append(clauses, "a.daily_config_id IN ("+strings.Join(placeholders, ",")+")")
	}
	args = append(args, userID)
	query := fmt.Sprintf(
		`SELECT COUNT(*) FROM %s t JOIN activities a ON t.activity_id = a.id WHERE t.user_id = $%d AND (%s)`,
		table, len(args), strings.Join(clauses, " OR "))
	if kind != "win" {
		query += " AND t.removed_at = ''"
	}
	var n int
	err := s.db.QueryRow(query, args...).Scan(&n)
	return n > 0, err
}
