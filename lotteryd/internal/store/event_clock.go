package store

import (
	"context"
	"lotteryd/api/events"
	"time"
)

// Clock work is separate from requests and runs only when a matching rule exists.
func (s *Store) EmitEventClock(ctx context.Context, now time.Time) error {
	var ages, upcoming bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM gameplay_rules WHERE enabled AND event_type='user.age_updated'),EXISTS(SELECT 1 FROM gameplay_rules WHERE enabled AND event_type='schedule.upcoming')`).Scan(&ages, &upcoming); err != nil {
		return err
	}
	minute := now.UTC().Format("2006-01-02T15:04")
	if ages {
		rows, err := s.db.QueryContext(ctx, `SELECT user_id,registered_at FROM user_registered`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var uid int64
			var raw string
			if err = rows.Scan(&uid, &raw); err != nil {
				rows.Close()
				return err
			}
			registered := parseTime(raw)
			if registered.IsZero() || registered.After(now) {
				continue
			}
			days := int64(now.Sub(registered).Hours() / 24)
			events.Publish("user.age_updated", uid, events.S(uid)+":"+events.S(days), map[string]string{"registered_at": registered.Format(time.RFC3339), "days": events.S(days)})
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	if upcoming {
		personal, err := s.db.QueryContext(ctx, `SELECT user_id,resource,source_id,expires_at FROM gameplay_schedules WHERE expires_at>$1 AND expires_at<$2`, now, now.Add(24*time.Hour))
		if err != nil {
			return err
		}
		for personal.Next() {
			var uid int64
			var resource, id string
			var at time.Time
			if err = personal.Scan(&uid, &resource, &id, &at); err != nil {
				personal.Close()
				return err
			}
			events.Publish("schedule.upcoming", uid, resource+":"+id+":"+at.Format(time.RFC3339)+":"+minute, map[string]string{"resource": resource, "source_id": id, "expires_at": at.Format(time.RFC3339), "duration_seconds": events.S(int64(at.Sub(now).Seconds()))})
		}
		err = personal.Err()
		personal.Close()
		if err != nil {
			return err
		}
		rows, err := s.db.QueryContext(ctx, `SELECT id,starts_at,draws_at FROM activities WHERE status='active' AND drawn_at=''`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			var starts, draws string
			if err = rows.Scan(&id, &starts, &draws); err != nil {
				rows.Close()
				return err
			}
			for phase, raw := range map[string]string{"starts": starts, "draws": draws} {
				at := parseTime(raw)
				if at.After(now) && at.Before(now.Add(24*time.Hour)) {
					events.Publish("schedule.upcoming", 0, "activity:"+events.S(id)+":"+phase+":"+minute, map[string]string{"activity_id": events.S(id), "resource": "activity", "action": phase, "expires_at": at.Format(time.RFC3339), "duration_seconds": events.S(int64(at.Sub(now).Seconds()))})
				}
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
