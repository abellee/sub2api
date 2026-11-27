package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"lotteryd/api/events"
	"time"
)

var beijing = time.FixedZone("Asia/Shanghai", 8*60*60)

type PointsSettings struct {
	DailyCheckInRewards     []int64 `json:"daily_checkin_rewards"`
	DailyCheckInMultipliers []int   `json:"daily_checkin_multipliers"`
}

func (settings *PointsSettings) Normalize() {
	if settings.DailyCheckInMultipliers == nil {
		settings.DailyCheckInMultipliers = []int{1, 1, 1, 1, 1, 1, 1}
	}
}

func (settings PointsSettings) Validate() error {
	if len(settings.DailyCheckInRewards) != 7 {
		return fmt.Errorf("configure exactly seven daily check-in rewards")
	}
	for _, reward := range settings.DailyCheckInRewards {
		if reward < 1 || reward > 2000000 {
			return fmt.Errorf("each daily check-in reward must be an integer between 1 and 2000000")
		}
	}
	if len(settings.DailyCheckInMultipliers) != 7 {
		return fmt.Errorf("configure exactly seven daily check-in multipliers")
	}
	for _, multiplier := range settings.DailyCheckInMultipliers {
		if multiplier < 1 || multiplier > 10 {
			return fmt.Errorf("each daily check-in multiplier must be an integer between 1 and 10")
		}
	}
	return nil
}

type CheckInStatus struct {
	Date          string  `json:"date"`
	Day           int     `json:"day"`
	CompletedDays int     `json:"completed_days"`
	Claimed       bool    `json:"claimed"`
	Rewards       []int64 `json:"rewards"`
	Multipliers   []int   `json:"multipliers"`
	TodayReward   int64   `json:"today_reward"`
	NextResetAt   string  `json:"next_reset_at"`
}

func (s *Store) GetPointsSettings() (PointsSettings, error) {
	return getPointsSettings(s.db)
}

func getPointsSettings(q interface {
	QueryRow(string, ...any) *sql.Row
}) (PointsSettings, error) {
	var settings PointsSettings
	var raw, multipliers []byte
	if err := q.QueryRow(`SELECT daily_checkin_rewards,daily_checkin_multipliers FROM points_settings WHERE id=TRUE`).Scan(&raw, &multipliers); err != nil {
		return settings, err
	}
	if err := json.Unmarshal(raw, &settings.DailyCheckInRewards); err != nil {
		return settings, err
	}
	if err := json.Unmarshal(multipliers, &settings.DailyCheckInMultipliers); err != nil {
		return settings, err
	}
	settings.Normalize()
	return settings, settings.Validate()
}

func (s *Store) SavePointsSettings(settings PointsSettings) error {
	settings.Normalize()
	if err := settings.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(settings.DailyCheckInRewards)
	if err != nil {
		return err
	}
	multipliers, err := json.Marshal(settings.DailyCheckInMultipliers)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE points_settings SET daily_checkin_rewards=$1,daily_checkin_multipliers=$2 WHERE id=TRUE`, string(raw), string(multipliers))
	return err
}

func checkInStatus(lastDate string, lastDay int, now time.Time, settings PointsSettings) CheckInStatus {
	local := now.In(beijing)
	today := local.Format(time.DateOnly)
	status := CheckInStatus{Date: today, Day: 1, Rewards: make([]int64, 7), Multipliers: append([]int(nil), settings.DailyCheckInMultipliers...)}
	for index, reward := range settings.DailyCheckInRewards {
		status.Rewards[index] = reward * int64(status.Multipliers[index])
	}
	if lastDate == today {
		status.Day, status.Claimed = lastDay, true
	} else if lastDate == local.AddDate(0, 0, -1).Format(time.DateOnly) {
		status.Day = lastDay%7 + 1
	}
	status.CompletedDays = status.Day - 1
	if status.Claimed {
		status.CompletedDays++
	}
	status.TodayReward = status.Rewards[status.Day-1]
	next := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, beijing)
	status.NextResetAt = next.Format(time.RFC3339)
	return status
}

func (s *Store) GetDailyCheckIn(userID int64, now time.Time) (CheckInStatus, error) {
	var lastDate string
	var lastDay int
	err := s.db.QueryRow(`SELECT last_date, cycle_day FROM user_checkins WHERE user_id=$1`, userID).Scan(&lastDate, &lastDay)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return CheckInStatus{}, err
	}
	settings, err := s.GetPointsSettings()
	if err != nil {
		return CheckInStatus{}, err
	}
	status := checkInStatus(lastDate, lastDay, now, settings)
	err = loadCheckInRewards(s.db, userID, &status)
	return status, err
}

// Preserve the amounts actually received when an admin changes rewards mid-cycle.
func loadCheckInRewards(q interface {
	Query(string, ...any) (*sql.Rows, error)
}, userID int64, status *CheckInStatus) error {
	if status.CompletedDays == 0 {
		return nil
	}
	today, err := time.Parse(time.DateOnly, status.Date)
	if err != nil {
		return err
	}
	start := today.AddDate(0, 0, 1-status.Day)
	refs := make(map[string]int, status.CompletedDays)
	for day := 0; day < status.CompletedDays; day++ {
		refs[fmt.Sprintf("%d:%s", userID, start.AddDate(0, 0, day).Format(time.DateOnly))] = day
	}
	rows, err := q.Query(`SELECT ref_id, delta, ext_json FROM points_transactions WHERE user_id=$1 AND kind='grant' AND ref_type='daily_checkin' AND ref_id >= $2 AND ref_id <= $3`,
		userID, fmt.Sprintf("%d:%s", userID, start.Format(time.DateOnly)), fmt.Sprintf("%d:%s", userID, status.Date))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var ref string
		var amount int64
		var ext string
		if err := rows.Scan(&ref, &amount, &ext); err != nil {
			return err
		}
		if index, ok := refs[ref]; ok {
			status.Rewards[index] = amount
			// Old check-ins have no multiplier snapshot and retain their credited amount.
			metadata := struct {
				Multiplier int `json:"multiplier"`
			}{Multiplier: 1}
			if err := json.Unmarshal([]byte(ext), &metadata); err != nil {
				return err
			}
			status.Multipliers[index] = metadata.Multiplier
		}
	}
	if status.Claimed {
		status.TodayReward = status.Rewards[status.Day-1]
	}
	return rows.Err()
}

func pointsBalanceInTx(tx *sql.Tx, userID int64) (PointsBalance, error) {
	var points PointsBalance
	err := tx.QueryRow(`SELECT balance, total_earned, total_spent FROM user_points WHERE user_id=$1`, userID).
		Scan(&points.Balance, &points.TotalEarned, &points.TotalSpent)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return points, err
}

// Lock a real balance row before adjusting it, including the user's first grant.
func ensurePointsRow(tx *sql.Tx, userID int64) error {
	_, err := tx.Exec(`INSERT INTO user_points (user_id, updated_at) VALUES ($1,$2) ON CONFLICT DO NOTHING`, userID, fmtTime(time.Now()))
	if err != nil {
		return err
	}
	var id int64
	return tx.QueryRow(`SELECT user_id FROM user_points WHERE user_id=$1 FOR UPDATE`, userID).Scan(&id)
}

// now is supplied by the server, never by the request body.
func (s *Store) ClaimDailyCheckIn(userID int64, now time.Time) (CheckInStatus, PointsBalance, bool, error) {
	var status CheckInStatus
	var points PointsBalance
	tx, err := s.db.Begin()
	if err != nil {
		return status, points, false, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.Exec(`INSERT INTO user_checkins (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, userID); err != nil {
		return status, points, false, err
	}
	var lastDate string
	var lastDay int
	if err = tx.QueryRow(`SELECT last_date, cycle_day FROM user_checkins WHERE user_id=$1 FOR UPDATE`, userID).Scan(&lastDate, &lastDay); err != nil {
		return status, points, false, err
	}
	settings, err := getPointsSettings(tx)
	if err != nil {
		return status, points, false, err
	}
	status = checkInStatus(lastDate, lastDay, now, settings)
	if lastDate > status.Date {
		return status, points, false, fmt.Errorf("check-in date moved backwards")
	}
	if status.Claimed {
		points, err = pointsBalanceInTx(tx, userID)
		if err == nil {
			err = loadCheckInRewards(tx, userID, &status)
		}
		return status, points, false, err
	}
	if err = ensurePointsRow(tx, userID); err != nil {
		return status, points, false, err
	}
	points, err = s.applyPointsDelta(tx, userID, status.TodayReward)
	if err != nil {
		return status, points, false, err
	}
	note := fmt.Sprintf("每日签到 · 第 %d 天", status.Day)
	ext, _ := json.Marshal(map[string]any{"date": status.Date, "day": status.Day, "base_points": settings.DailyCheckInRewards[status.Day-1], "multiplier": status.Multipliers[status.Day-1], "reward_points": status.TodayReward})
	_, err = insertPointsTxn(tx, PointsTransaction{UserID: userID, Delta: status.TodayReward, BalanceAfter: points.Balance,
		Kind: "grant", Source: "daily_checkin", RefType: "daily_checkin", RefID: fmt.Sprintf("%d:%s", userID, status.Date),
		Note: note, ExtJSON: string(ext), CreatedAt: fmtTime(now)})
	if err != nil {
		return status, points, false, err
	}
	if _, err = tx.Exec(`UPDATE user_checkins SET last_date=$2, cycle_day=$3 WHERE user_id=$1`, userID, status.Date, status.Day); err != nil {
		return status, points, false, err
	}
	status.Claimed = true
	status.CompletedDays = status.Day
	if err = loadCheckInRewards(tx, userID, &status); err != nil {
		return status, points, false, err
	}
	if err = tx.Commit(); err != nil {
		return status, points, false, err
	}
	data := map[string]string{"day": events.S(status.Day), "multiplier": events.S(status.Multipliers[status.Day-1]), "reward_points": events.S(status.TodayReward), "balance_after": events.S(points.Balance)}
	ref := fmt.Sprintf("%d:%s", userID, status.Date)
	events.Publish("checkin.completed", userID, ref, data)
	events.Publish("points.granted", userID, "daily_checkin:"+ref, data)
	return status, points, true, nil
}

var ErrPointsRechargeConflict = errors.New("points recharge request already used with different values")

func (s *Store) GrantAdminPoints(userID, adminID, amount int64, requestID, note string) (PointsBalance, bool, error) {
	var points PointsBalance
	if userID < 1 || adminID < 1 || amount < 1 || amount > 1000000000000 {
		return points, false, fmt.Errorf("invalid points recharge")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return points, false, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = ensurePointsRow(tx, userID); err != nil {
		return points, false, err
	}
	refID := fmt.Sprintf("%d:%s", adminID, requestID)
	var existingUser, existingAmount int64
	var existingNote string
	err = tx.QueryRow(`SELECT user_id, delta, note FROM points_transactions WHERE kind='grant' AND ref_type='admin_points_recharge' AND ref_id=$1`, refID).
		Scan(&existingUser, &existingAmount, &existingNote)
	if err == nil {
		if existingUser != userID || existingAmount != amount || existingNote != note {
			return points, false, ErrPointsRechargeConflict
		}
		points, err = pointsBalanceInTx(tx, userID)
		return points, false, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return points, false, err
	}
	points, err = s.applyPointsDelta(tx, userID, amount)
	if err != nil {
		return points, false, err
	}
	ext, _ := json.Marshal(map[string]int64{"admin_id": adminID})
	_, err = insertPointsTxn(tx, PointsTransaction{UserID: userID, Delta: amount, BalanceAfter: points.Balance,
		Kind: "grant", Source: "admin_recharge", RefType: "admin_points_recharge", RefID: refID,
		Note: note, ExtJSON: string(ext), CreatedAt: fmtTime(time.Now())})
	if err != nil {
		return points, false, err
	}
	if err = tx.Commit(); err != nil {
		return points, false, err
	}
	data := map[string]string{"actor_id": events.S(adminID), "amount": events.S(amount), "balance_after": events.S(points.Balance)}
	events.Publish("points.admin_granted", userID, refID, data)
	events.Publish("points.granted", userID, "admin_points_recharge:"+refID, data)
	return points, true, nil
}
