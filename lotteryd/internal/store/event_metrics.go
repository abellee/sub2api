package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"lotteryd/api/events"
)

func derivedEventID(parent, kind string) string {
	sum := sha256.Sum256([]byte(parent + "/" + kind))
	return "derived:" + hex.EncodeToString(sum[:])
}

// Metrics begin when event collection is enabled. They never claim to be historical totals.
func (s *Store) updateEventMetrics(ctx context.Context, tx *sql.Tx, e *events.Event) error {
	if e.UserID <= 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, e.UserID^int64(-495289123911)); err != nil {
		return err
	}
	day := e.OccurredAt.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02")
	amount := "0"
	for _, field := range []string{"actual_cost", "amount", "reward_points", "quantity"} {
		if raw := e.Data[field]; raw != "" {
			if value, ok := numericRat(raw); ok && value.Sign() >= 0 {
				amount = value.FloatString(8)
			}
			break
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO gameplay_metrics(user_id,event_type) VALUES($1,$2) ON CONFLICT DO NOTHING`, e.UserID, e.Type); err != nil {
		return err
	}
	var previousDay string
	var previousAt sql.NullTime
	var previousStreak int64
	if err := tx.QueryRowContext(ctx, `SELECT day,last_at,streak FROM gameplay_metrics WHERE user_id=$1 AND event_type=$2 FOR UPDATE`, e.UserID, e.Type).Scan(&previousDay, &previousAt, &previousStreak); err != nil {
		return err
	}
	streak := previousStreak
	if day > previousDay {
		if previousDay == e.OccurredAt.In(time.FixedZone("CST", 8*3600)).AddDate(0, 0, -1).Format("2006-01-02") {
			streak++
		} else {
			streak = 1
		}
	}
	var count, daily int64
	var total string
	err := tx.QueryRowContext(ctx, `UPDATE gameplay_metrics SET count=count+1,total=total+$3::numeric,daily_count=CASE WHEN day=$4 THEN daily_count+1 WHEN day<$4 THEN 1 ELSE daily_count END,day=GREATEST(day,$4),last_at=GREATEST(COALESCE(last_at,$5),$5),streak=$6 WHERE user_id=$1 AND event_type=$2 RETURNING count,daily_count,total::text`, e.UserID, e.Type, amount, day, e.OccurredAt, streak).Scan(&count, &daily, &total)
	if err != nil {
		return err
	}
	e.Data["event_count"] = strconv.FormatInt(count, 10)
	e.Data["event_daily_count"] = strconv.FormatInt(daily, 10)
	e.Data["event_total"] = total
	if e.Source == "derived" {
		return nil
	}
	resource, sourceID := "", ""
	if strings.HasPrefix(e.Type, "key.") {
		resource, sourceID = "key", e.Data["api_key_id"]
	} else if strings.HasPrefix(e.Type, "subscription.") {
		resource, sourceID = "subscription", e.Data["subscription_id"]
	}
	if resource != "" && sourceID != "" {
		if e.Type == "key.deleted" || e.Type == "subscription.revoked" || e.Type == "subscription.expired" || e.Type == "key.expired_detected" || (e.Type == "key.updated" && e.Data["expires_at"] == "") {
			if _, err = tx.ExecContext(ctx, `DELETE FROM gameplay_schedules WHERE user_id=$1 AND resource=$2 AND source_id=$3`, e.UserID, resource, sourceID); err != nil {
				return err
			}
		} else if expires, parseErr := time.Parse(time.RFC3339, e.Data["expires_at"]); parseErr == nil {
			if _, err = tx.ExecContext(ctx, `INSERT INTO gameplay_schedules(user_id,resource,source_id,expires_at) VALUES($1,$2,$3,$4) ON CONFLICT(user_id,resource,source_id) DO UPDATE SET expires_at=EXCLUDED.expires_at`, e.UserID, resource, sourceID, expires); err != nil {
				return err
			}
		}
	}
	derive := func(kind string, data map[string]string) error {
		copy := make(map[string]string, len(e.Data)+len(data))
		for k, v := range e.Data {
			copy[k] = v
		}
		for k, v := range data {
			copy[k] = v
		}
		copy["source_id"] = e.ID
		return insertGameplayEvent(ctx, tx, events.Event{ID: derivedEventID(e.ID, kind), Type: kind, Source: "derived", UserID: e.UserID, Reference: e.ID, OccurredAt: e.OccurredAt, Data: copy})
	}
	switch e.Type {
	case "user.registered":
		if registered, parseErr := time.Parse(time.RFC3339, e.Data["registered_at"]); parseErr == nil {
			if _, err = tx.ExecContext(ctx, `INSERT INTO user_registered(user_id,email,registered_at) VALUES($1,'',$2) ON CONFLICT DO NOTHING`, e.UserID, fmtTime(registered)); err != nil {
				return err
			}
		}
	case "usage.tier_observed":
		stage, parseErr := strconv.ParseInt(e.Data["stage"], 10, 64)
		if parseErr != nil {
			return nil
		}
		key := "__tier:" + e.Data["group_id"]
		var previous int64
		readErr := tx.QueryRowContext(ctx, `SELECT count FROM gameplay_metrics WHERE user_id=$1 AND event_type=$2`, e.UserID, key).Scan(&previous)
		if readErr != nil && readErr != sql.ErrNoRows {
			return readErr
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO gameplay_metrics(user_id,event_type,count) VALUES($1,$2,$3) ON CONFLICT(user_id,event_type) DO UPDATE SET count=EXCLUDED.count`, e.UserID, key, stage); err != nil {
			return err
		}
		if readErr == nil && previous != stage {
			if err = derive("usage.tier_changed", map[string]string{"previous_stage": events.S(previous), "stage": events.S(stage)}); err != nil {
				return err
			}
		}
	case "user.logged_in":
		if count == 1 {
			if err = derive("user.first_login", nil); err != nil {
				return err
			}
		}
	case "request.completed":
		if count == 1 {
			if err = derive("user.first_call", nil); err != nil {
				return err
			}
		}
		if err = derive("usage.requests_updated", map[string]string{"count": events.S(count)}); err != nil {
			return err
		}
		if e.Data["status"] != "success" && e.Data["status"] != "failed" && e.Data["status"] != "200" {
			break
		}
		current := int64(1)
		if e.Data["status"] == "success" || e.Data["status"] == "200" {
			current = 1
		} else {
			current = -1
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO gameplay_metrics(user_id,event_type,streak,day) VALUES($1,'__request_streak',$2,$3) ON CONFLICT(user_id,event_type) DO UPDATE SET streak=CASE WHEN SIGN(gameplay_metrics.streak)=SIGN(EXCLUDED.streak) THEN gameplay_metrics.streak+EXCLUDED.streak ELSE EXCLUDED.streak END,day=EXCLUDED.day`, e.UserID, current, day); err != nil {
			return err
		}
		var n int64
		if err = tx.QueryRowContext(ctx, `SELECT ABS(streak) FROM gameplay_metrics WHERE user_id=$1 AND event_type='__request_streak'`, e.UserID).Scan(&n); err != nil {
			return err
		}
		if err = derive("request.streak_updated", map[string]string{"streak": events.S(n)}); err != nil {
			return err
		}
	case "payment.balance_completed":
		if count == 1 {
			if err = derive("user.first_recharge", nil); err != nil {
				return err
			}
		}
		if err = derive("usage.recharge_updated", map[string]string{"count": events.S(count), "total_amount": total}); err != nil {
			return err
		}
	case "billing.completed", "billing.captured":
		var spendTotal string
		var spendDaily int64
		if err = tx.QueryRowContext(ctx, `INSERT INTO gameplay_metrics(user_id,event_type,total,day,daily_count) VALUES($1,'__spending',$2::numeric,$3,1) ON CONFLICT(user_id,event_type) DO UPDATE SET total=gameplay_metrics.total+EXCLUDED.total,daily_count=CASE WHEN gameplay_metrics.day=EXCLUDED.day THEN gameplay_metrics.daily_count+1 WHEN gameplay_metrics.day<EXCLUDED.day THEN 1 ELSE gameplay_metrics.daily_count END,day=GREATEST(gameplay_metrics.day,EXCLUDED.day) RETURNING total::text,daily_count`, e.UserID, amount, day).Scan(&spendTotal, &spendDaily); err != nil {
			return err
		}
		if err = derive("usage.cost_updated", map[string]string{"total_amount": spendTotal}); err != nil {
			return err
		}
		if spendDaily == 1 {
			if err = derive("user.daily_spend", nil); err != nil {
				return err
			}
		}
		var tokens int64
		for _, f := range []string{"input_tokens", "output_tokens"} {
			n, _ := strconv.ParseInt(e.Data[f], 10, 64)
			if n > 0 {
				tokens += n
			}
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO gameplay_metrics(user_id,event_type,count) VALUES($1,'__tokens',$2) ON CONFLICT(user_id,event_type) DO UPDATE SET count=gameplay_metrics.count+EXCLUDED.count`, e.UserID, tokens); err != nil {
			return err
		}
		var allTokens int64
		if err = tx.QueryRowContext(ctx, `SELECT count FROM gameplay_metrics WHERE user_id=$1 AND event_type='__tokens'`, e.UserID).Scan(&allTokens); err != nil {
			return err
		}
		if err = derive("usage.tokens_updated", map[string]string{"total_tokens_used": events.S(allTokens)}); err != nil {
			return err
		}
		for _, field := range []string{"model", "group_id"} {
			if value := e.Data[field]; value != "" {
				if _, err = tx.ExecContext(ctx, `INSERT INTO gameplay_distinct(user_id,kind,value) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, e.UserID, field, value); err != nil {
					return err
				}
			}
		}
		var models, groups int64
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FILTER(WHERE kind='model'),COUNT(*) FILTER(WHERE kind='group_id') FROM gameplay_distinct WHERE user_id=$1`, e.UserID).Scan(&models, &groups); err != nil {
			return err
		}
		if err = derive("usage.models_updated", map[string]string{"unique_models": events.S(models), "unique_groups": events.S(groups)}); err != nil {
			return err
		}
	case "affiliate.bound":
		if err = derive("affiliate.count_updated", map[string]string{"count": events.S(count)}); err != nil {
			return err
		}
	case "lottery.loss_streak_updated":
		increment := int64(1)
		if e.Data["status"] == "won" {
			increment = 0
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO gameplay_metrics(user_id,event_type,streak) VALUES($1,'__lottery_losses',$2) ON CONFLICT(user_id,event_type) DO UPDATE SET streak=CASE WHEN $2=0 THEN 0 ELSE gameplay_metrics.streak+1 END`, e.UserID, increment); err != nil {
			return err
		}
		var losses int64
		if err = tx.QueryRowContext(ctx, `SELECT streak FROM gameplay_metrics WHERE user_id=$1 AND event_type='__lottery_losses'`, e.UserID).Scan(&losses); err != nil {
			return err
		}
		e.Data["streak"] = events.S(losses)
	case "checkin.completed":
		if e.Data["day"] == "7" {
			if err = derive("checkin.cycle_completed", nil); err != nil {
				return err
			}
		}
	case "points.granted", "points.spent", "affiliate.accrued", "affiliate.transferred", "user.balance_adjusted":
		if err = derive("balance.threshold_changed", nil); err != nil {
			return err
		}
	}
	// Only user activity counts. Background rewards, mail, retries and admin work cannot create active days.
	if e.Type == "user.logged_in" || e.Type == "request.completed" || e.Type == "checkin.completed" || strings.HasPrefix(e.Type, "content.") {
		var oldDay string
		var oldStreak int64
		var oldAt sql.NullTime
		if _, err = tx.ExecContext(ctx, `INSERT INTO gameplay_metrics(user_id,event_type) VALUES($1,'__active') ON CONFLICT DO NOTHING`, e.UserID); err != nil {
			return err
		}
		if err = tx.QueryRowContext(ctx, `SELECT day,streak,last_at FROM gameplay_metrics WHERE user_id=$1 AND event_type='__active'`, e.UserID).Scan(&oldDay, &oldStreak, &oldAt); err != nil {
			return err
		}
		if day > oldDay {
			n := int64(1)
			if oldDay == e.OccurredAt.In(time.FixedZone("CST", 8*3600)).AddDate(0, 0, -1).Format("2006-01-02") {
				n = oldStreak + 1
			}
			if err = derive("user.daily_active", nil); err != nil {
				return err
			}
			if err = derive("user.active_streak", map[string]string{"streak": events.S(n)}); err != nil {
				return err
			}
			if oldAt.Valid {
				days := int64(e.OccurredAt.Sub(oldAt.Time).Hours() / 24)
				if days >= 1 {
					if err = derive("user.returned", map[string]string{"days": events.S(days)}); err != nil {
						return err
					}
				}
			}
			if _, err = tx.ExecContext(ctx, `UPDATE gameplay_metrics SET day=$2,streak=$3,last_at=$4 WHERE user_id=$1 AND event_type='__active'`, e.UserID, day, n, e.OccurredAt); err != nil {
				return err
			}
		}
	}
	return nil
}
