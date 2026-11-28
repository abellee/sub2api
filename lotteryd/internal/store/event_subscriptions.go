package store

import (
	"context"
	"strings"
	"time"

	"lotteryd/api/events"
)

type eventSubscriptions struct {
	active map[string]bool
	needed map[string]bool
	until  time.Time
}

// Include only the facts needed by an enabled trigger, including derived triggers.
func eventDependencies(kind string) []string {
	var result []string
	switch kind {
	case "user.registered":
		result = []string{"user.age_updated"}
	case "user.logged_in":
		result = []string{"user.first_login"}
	case "request.completed":
		result = []string{"user.first_call", "usage.requests_updated", "request.streak_updated"}
	case "payment.balance_completed":
		result = []string{"user.first_recharge", "usage.recharge_updated"}
	case "billing.completed", "billing.captured":
		result = []string{"usage.cost_updated", "user.daily_spend", "usage.tokens_updated", "usage.models_updated"}
	case "affiliate.bound":
		result = []string{"affiliate.count_updated"}
	case "checkin.completed":
		result = []string{"checkin.cycle_completed"}
	case "points.granted", "points.spent", "affiliate.accrued", "affiliate.transferred", "user.balance_adjusted":
		result = []string{"balance.threshold_changed"}
	case "usage.tier_observed":
		result = []string{"usage.tier_changed"}
	}
	if kind == "user.logged_in" || kind == "request.completed" || kind == "checkin.completed" || strings.HasPrefix(kind, "content.") {
		result = append(result, "user.daily_active", "user.active_streak", "user.returned")
	}
	if strings.HasPrefix(kind, "key.") || strings.HasPrefix(kind, "subscription.") {
		result = append(result, "schedule.upcoming")
	}
	return result
}

func (s *Store) eventSubscriptions(ctx context.Context) (*eventSubscriptions, error) {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	if s.eventSubs != nil && time.Now().Before(s.eventSubs.until) {
		return s.eventSubs, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT event_type FROM gameplay_rules WHERE enabled`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := &eventSubscriptions{active: map[string]bool{}, needed: map[string]bool{}}
	for rows.Next() {
		var kind string
		if err := rows.Scan(&kind); err != nil {
			return nil, err
		}
		result.active[kind] = true
		result.needed[kind] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, definition := range events.Catalog() {
		for _, target := range eventDependencies(definition.Code) {
			if result.active[target] {
				result.needed[definition.Code] = true
			}
		}
	}
	// Admin edits invalidate immediately; other processes refresh within one second.
	result.until = time.Now().Add(time.Second)
	s.eventSubs = result
	return result, nil
}

func (s *Store) invalidateEventSubscriptions() {
	s.eventMu.Lock()
	s.eventSubs = nil
	s.eventMu.Unlock()
}

// Old empty snapshots are observation work, not promised rewards. Skip in bounded batches.
func (s *Store) SkipUnconfiguredEvents(ctx context.Context) error {
	subs, err := s.eventSubscriptions(ctx)
	if err != nil {
		return err
	}
	kinds := make([]string, 0, len(subs.needed))
	for kind := range subs.needed {
		kinds = append(kinds, kind)
	}
	_, err = s.db.ExecContext(ctx, `WITH candidates AS MATERIALIZED (
	 SELECT id,event_type,rules FROM gameplay_events WHERE status='pending'
	 ORDER BY available_at,seq LIMIT 1000 FOR UPDATE SKIP LOCKED
	), batch AS (
	 SELECT id FROM candidates WHERE rules='[]'::jsonb AND NOT(event_type=ANY($1::text[]))
	) UPDATE gameplay_events e SET status='skipped',processed_at=NOW(),last_error='没有已启用的触发规则'
	FROM batch WHERE e.id=batch.id`, kinds)
	return err
}
