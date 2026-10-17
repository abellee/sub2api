package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"math/rand"
	"time"

	"lotteryd/internal/lottery"
)

// Every participant mutation and draw locks the same activity row. A successful
// removal or blacklist update therefore cannot race a draw using an older list.
func lockUndrawnActivity(tx *sql.Tx, id int64) error {
	var drawnAt string
	err := tx.QueryRow(`SELECT drawn_at FROM activities WHERE id=$1 FOR UPDATE`, id).Scan(&drawnAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if drawnAt != "" {
		return ErrActivityDrawn
	}
	// Protect legacy partially recorded draws as well.
	var winners int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM winners WHERE activity_id=$1`, id).Scan(&winners); err != nil {
		return err
	}
	if winners > 0 {
		return ErrActivityDrawn
	}
	return nil
}

func (s *Store) HasRemovedParticipant(activityID, userID int64) (bool, error) {
	var removed bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM participants WHERE activity_id=$1 AND user_id=$2 AND removed_at <> '')`, activityID, userID).Scan(&removed)
	return removed, err
}

// RemoveParticipant keeps the exclusion so the user cannot immediately rejoin.
func (s *Store) RemoveParticipant(activityID, userID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockUndrawnActivity(tx, activityID); err != nil {
		return err
	}
	result, err := tx.Exec(`UPDATE participants SET removed_at=$1 WHERE activity_id=$2 AND user_id=$3 AND removed_at=''`, fmtTime(time.Now()), activityID, userID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

func (s *Store) SetUndrawnActivityBlacklist(configID int64, emails []string) error {
	raw, err := json.Marshal(lottery.NormalizeBlacklist(emails))
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE activities SET blacklist_json=$1, updated_at=$2 WHERE daily_config_id=$3 AND drawn_at='' AND status='active'`, string(raw), fmtTime(time.Now()), configID)
	return err
}

// DrawActivity reloads the blacklist and participants under the activity lock,
// and commits winners, prize stock and draw time together.
func (s *Store) DrawActivity(id int64) (*lottery.Activity, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := lockUndrawnActivity(tx, id); err != nil {
		return nil, err
	}
	a, err := getActivity(tx, id)
	if err != nil {
		return nil, err
	}
	if a.Status != "active" {
		return nil, ErrNotJoinable
	}
	participants, err := listParticipants(tx, id)
	if err != nil {
		return nil, err
	}
	drafts := lottery.DrawPlan(a, participants, rand.New(rand.NewSource(time.Now().UnixNano())))
	codeCursor := map[int64]int{}
	for _, p := range a.Prizes {
		codeCursor[p.ID] = int(p.GrantedCount)
	}
	for _, d := range drafts {
		code := ""
		if d.Prize.PrizeType == lottery.PrizeRedeemCode {
			idx := codeCursor[d.Prize.ID]
			if idx < len(d.Prize.Codes) {
				code = d.Prize.Codes[idx]
			}
			codeCursor[d.Prize.ID]++
		}
		_, err := tx.Exec(`INSERT INTO winners (activity_id,user_id,email,prize_id,prize_name,prize_type,value,fulfillment,redeem_code,note,error,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'','',$10)`, id, d.UserID, d.Email, d.Prize.ID, d.Prize.Name, d.Prize.PrizeType, d.Prize.Value, lottery.FulfillmentPending, code, fmtTime(d.CreatedAt))
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`UPDATE prizes SET granted_count=granted_count+1 WHERE id=$1`, d.Prize.ID); err != nil {
			return nil, err
		}
	}
	a.DrawnAt = time.Now().UTC()
	if _, err := tx.Exec(`UPDATE activities SET drawn_at=$1, updated_at=$1 WHERE id=$2`, fmtTime(a.DrawnAt), id); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return a, nil
}
