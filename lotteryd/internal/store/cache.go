package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"lotteryd/internal/cache"
	"lotteryd/internal/lottery"
)

const readCacheTTL = 5 * time.Minute

func (s *Store) SetCache(c *cache.Cache) { s.cache = c }
func (s *Store) Cache() *cache.Cache     { return s.cache }
func (s *Store) Revision() uint64        { return s.revision.Load() }

// A committed write always wins over an old fill. Cache failures must not turn
// a successful participation or payout into an error that the caller retries.
func (s *Store) changed(err error) error {
	if err != nil {
		return err
	}
	s.revision.Add(1)
	if s.cache.Invalidate() {
		if _, err := s.ListActivities(); err != nil {
			slog.Warn("refresh lottery activities cache", "err", err)
		}
		if _, err := s.ListTasks(); err != nil {
			slog.Warn("refresh lottery tasks cache", "err", err)
		}
		if _, err := s.states(); err != nil {
			slog.Warn("refresh lottery settings cache", "err", err)
		}
	}
	return nil
}

func (s *Store) states() (map[string]string, error) {
	return cache.Load(s.cache, context.Background(), "states", readCacheTTL, func() (map[string]string, error) {
		rows, err := s.db.Query(`SELECT key, value FROM sync_state`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := map[string]string{}
		for rows.Next() {
			var k, v string
			if err := rows.Scan(&k, &v); err != nil {
				return nil, err
			}
			out[k] = v
		}
		return out, rows.Err()
	})
}

// Activity lists use one consistent snapshot and three queries regardless of
// the number of historical activities. Draws still reload inside their own tx.
func (s *Store) activitySnapshot() ([]lottery.Activity, error) {
	tx, err := s.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT ` + activityColumns + ` FROM activities ORDER BY draws_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	acts, err := scanActivities(rows)
	rows.Close()
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]*lottery.Activity, len(acts))
	for i := range acts {
		byID[acts[i].ID] = &acts[i]
	}
	rows, err = tx.Query(`SELECT activity_id, def_json FROM activity_conditions ORDER BY activity_id, sort_order, id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var def string
		if err := rows.Scan(&id, &def); err != nil {
			rows.Close()
			return nil, err
		}
		condition, err := conditionFromJSON(def)
		if err != nil {
			rows.Close()
			return nil, err
		}
		if a := byID[id]; a != nil {
			a.Conditions = append(a.Conditions, condition)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.Query(`SELECT id, activity_id, name, prize_type, value, weight, stock, granted_count, codes_json FROM prizes ORDER BY activity_id, id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p lottery.Prize
		var raw string
		if err := rows.Scan(&p.ID, &p.ActivityID, &p.Name, &p.PrizeType, &p.Value, &p.Weight, &p.Stock, &p.GrantedCount, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		if raw != "" && raw != "[]" {
			if err := json.Unmarshal([]byte(raw), &p.Codes); err != nil {
				rows.Close()
				return nil, err
			}
		}
		if a := byID[p.ActivityID]; a != nil {
			a.Prizes = append(a.Prizes, p)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return acts, tx.Commit()
}

func userKey(kind string, id int64) string { return fmt.Sprintf("%s:%d", kind, id) }

func (s *Store) ListTasks() ([]lottery.Task, error) {
	return cache.Load(s.cache, context.Background(), "tasks", readCacheTTL, s.readListTasks)
}
func (s *Store) ListWinnersByUser(id int64) ([]lottery.Winner, error) {
	return cache.Load(s.cache, context.Background(), userKey("user-winners", id), readCacheTTL, func() ([]lottery.Winner, error) { return s.readListWinnersByUser(id) })
}
func (s *Store) ListWinnersByActivity(id int64) ([]lottery.Winner, error) {
	return cache.Load(s.cache, context.Background(), userKey("activity-winners", id), readCacheTTL, func() ([]lottery.Winner, error) { return s.readListWinnersByActivity(id) })
}
func (s *Store) DailyTokensWindow(start, end string) (map[int64]*lottery.UserUsage, error) {
	return cache.Load(s.cache, context.Background(), "daily-tokens:"+start+":"+end, readCacheTTL, func() (map[int64]*lottery.UserUsage, error) { return s.readDailyTokensWindow(start, end) })
}
func (s *Store) GetRegisteredAt(id int64) (*time.Time, error) {
	return cache.Load(s.cache, context.Background(), userKey("registered", id), readCacheTTL, func() (*time.Time, error) { return s.readGetRegisteredAt(id) })
}
func (s *Store) GetState(key string) (string, error) {
	if s.cache == nil {
		return s.readGetState(key)
	}
	values, err := s.states()
	return values[key], err
}
func (s *Store) ListActivityNotificationSettings() (map[int64]lottery.NotificationSettings, error) {
	return cache.Load(s.cache, context.Background(), "notify-settings", readCacheTTL, s.readListActivityNotificationSettings)
}
func (s *Store) ListActivityPushLogs() (map[int64]map[string]int, error) {
	return cache.Load(s.cache, context.Background(), "push-logs", readCacheTTL, s.readListActivityPushLogs)
}
func (s *Store) CountNotifiedTaskUsers() (map[int64]int64, error) {
	return cache.Load(s.cache, context.Background(), "notified-task-users", readCacheTTL, s.readCountNotifiedTaskUsers)
}

// UserActivityRecords is for read-only cards and prompts. Participation and draw
// validation deliberately keep using the database and its activity row lock.
type UserActivityRecords struct {
	Joined  map[int64]bool
	Removed map[int64]bool
	Winners map[int64]lottery.Winner
	Counts  map[int64]int64
}

func (s *Store) UserActivityRecords(userID int64) (*UserActivityRecords, error) {
	return cache.Load(s.cache, context.Background(), userKey("activity-records", userID), readCacheTTL, func() (*UserActivityRecords, error) {
		out := &UserActivityRecords{Joined: map[int64]bool{}, Removed: map[int64]bool{}, Winners: map[int64]lottery.Winner{}}
		rows, err := s.db.Query(`SELECT activity_id, removed_at FROM participants WHERE user_id=$1`, userID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			var removed string
			if err := rows.Scan(&id, &removed); err != nil {
				rows.Close()
				return nil, err
			}
			out.Joined[id] = removed == ""
			out.Removed[id] = removed != ""
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		wins, err := s.ListWinnersByUser(userID)
		if err != nil {
			return nil, err
		}
		for _, w := range wins {
			out.Winners[w.ActivityID] = w
		}
		out.Counts, err = cache.Load(s.cache, context.Background(), "participant-counts", readCacheTTL, func() (map[int64]int64, error) {
			rows, err := s.db.Query(`SELECT activity_id, COUNT(*) FROM participants WHERE removed_at='' GROUP BY activity_id`)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			counts := map[int64]int64{}
			for rows.Next() {
				var id, n int64
				if err := rows.Scan(&id, &n); err != nil {
					return nil, err
				}
				counts[id] = n
			}
			return counts, rows.Err()
		})
		return out, err
	})
}

// FreshDailyTokensWindow is reserved for decisions that may grant a reward.
func (s *Store) FreshDailyTokensWindow(start, end string) (map[int64]*lottery.UserUsage, error) {
	return s.readDailyTokensWindow(start, end)
}
