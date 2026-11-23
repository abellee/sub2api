package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"
	"strconv"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

var _ service.GroupUsageTierRepository = (*userGroupRateRepository)(nil)

func emptyUsageTierConfig() service.GroupUsageTierConfig {
	return service.GroupUsageTierConfig{Tiers: []service.UsageTier{}, Models: []string{}, LinkedGroupIDs: []int64{}, ExcludedUserIDs: []int64{}}
}

func (r *userGroupRateRepository) GetUsageTierConfig(ctx context.Context, groupID int64) (service.GroupUsageTierConfig, error) {
	c := emptyUsageTierConfig()
	var raw []byte
	err := scanSingleRow(ctx, r.sql, `SELECT config FROM group_usage_tier_rules WHERE group_id=$1`, []any{groupID}, &raw)
	if err == sql.ErrNoRows {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(raw, &c)
	return c, err
}

func (r *userGroupRateRepository) SaveUsageTierConfig(ctx context.Context, groupID int64, c service.GroupUsageTierConfig) error {
	if err := c.Validate(groupID); err != nil {
		return err
	}
	if c.Tiers == nil {
		c.Tiers = []service.UsageTier{}
	}
	if c.LinkedGroupIDs == nil {
		c.LinkedGroupIDs = []int64{}
	}
	if c.ExcludedUserIDs == nil {
		c.ExcludedUserIDs = []int64{}
	}
	if c.Models == nil {
		c.Models = []string{}
	}
	slices.Sort(c.Models)
	tx, err := r.beginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Lock referenced groups against deletion while validating the submitted IDs.
	ids := append([]int64{groupID}, c.LinkedGroupIDs...)
	rows, err := tx.QueryContext(ctx, `SELECT id FROM groups WHERE id=ANY($1) AND deleted_at IS NULL ORDER BY id FOR SHARE`, pq.Array(ids))
	if err != nil {
		return err
	}
	count := 0
	for rows.Next() {
		count++
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	if count != len(ids) {
		return infraerrors.BadRequest("INVALID_USAGE_TIER_GROUP", "分组不存在或已删除")
	}
	if len(c.ExcludedUserIDs) > 0 {
		var users int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id=ANY($1) AND deleted_at IS NULL`, pq.Array(c.ExcludedUserIDs)).Scan(&users); err != nil {
			return err
		}
		if users != len(c.ExcludedUserIDs) {
			return infraerrors.BadRequest("INVALID_USAGE_TIER_USER", "不参与列表中存在无效用户")
		}
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO group_usage_tier_rules(group_id,config) VALUES($1,$2::jsonb)
 ON CONFLICT(group_id) DO UPDATE SET config=EXCLUDED.config,
 revision=group_usage_tier_rules.revision+CASE WHEN group_usage_tier_rules.config->'tiers' IS DISTINCT FROM EXCLUDED.config->'tiers' OR group_usage_tier_rules.config->'enabled' IS DISTINCT FROM EXCLUDED.config->'enabled' OR group_usage_tier_rules.config->'models' IS DISTINCT FROM EXCLUDED.config->'models' THEN 1 ELSE 0 END,
 started_at=CASE WHEN group_usage_tier_rules.config->'tiers' IS DISTINCT FROM EXCLUDED.config->'tiers' OR group_usage_tier_rules.config->'enabled' IS DISTINCT FROM EXCLUDED.config->'enabled' OR group_usage_tier_rules.config->'models' IS DISTINCT FROM EXCLUDED.config->'models' THEN NOW() ELSE group_usage_tier_rules.started_at END
 WHERE group_usage_tier_rules.config IS DISTINCT FROM EXCLUDED.config`, groupID, string(raw))
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if r.tierGroupCache != nil {
		r.tierGroupCache.Flush()
	}
	return nil
}

func (r *userGroupRateRepository) GetUsageTierProgress(ctx context.Context, userID int64, groupIDs []int64) (map[int64][]service.GroupUsageTierProgress, error) {
	out := map[int64][]service.GroupUsageTierProgress{}
	if len(groupIDs) == 0 {
		return out, nil
	}
	tx, err := r.beginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT r.group_id,g.name,r.config,r.revision,r.started_at
 FROM group_usage_tier_rules r JOIN groups g ON g.id=r.group_id
 WHERE g.deleted_at IS NULL AND g.status='active' AND r.config->>'enabled'='true'
 AND (r.group_id=ANY($1) OR EXISTS(SELECT 1 FROM unnest($1::bigint[]) AS target(id) WHERE r.config->'linked_group_ids' @> jsonb_build_array(target.id)))
 ORDER BY r.group_id FOR SHARE OF r`, pq.Array(groupIDs))
	if err != nil {
		return nil, err
	}
	type rule struct {
		id       int64
		name     string
		config   service.GroupUsageTierConfig
		revision int64
		start    time.Time
	}
	rules := []rule{}
	for rows.Next() {
		var v rule
		var raw []byte
		if err = rows.Scan(&v.id, &v.name, &raw, &v.revision, &v.start); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if err = json.Unmarshal(raw, &v.config); err != nil {
			_ = rows.Close()
			return nil, err
		}
		rules = append(rules, v)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	today := now.In(service.UsageTierLocation).Format(time.DateOnly)
	for _, rule := range rules {
		p := service.GroupUsageTierProgress{SourceGroupID: rule.id, SourceGroupName: rule.name, Tiers: rule.config.Tiers, Models: rule.config.Models, Excluded: slices.Contains(rule.config.ExcludedUserIDs, userID)}
		if !p.Excluded {
			// Daily totals are small and retained. Replay them so late charges are never skipped.
			from := rule.start.In(service.UsageTierLocation).Format(time.DateOnly)
			dailyRows, queryErr := tx.QueryContext(ctx, `SELECT day::text,total_tokens
    FROM user_group_usage_tier_token_daily WHERE user_id=$1 AND group_id=$2 AND revision=$3 AND day>=$4::date AND day<=$5::date`, userID, rule.id, rule.revision, from, today)
			if queryErr != nil {
				return nil, queryErr
			}
			daily := map[string]int64{}
			for dailyRows.Next() {
				var day string
				var amount int64
				if err = dailyRows.Scan(&day, &amount); err != nil {
					_ = dailyRows.Close()
					return nil, err
				}
				daily[day] = amount
			}
			err = dailyRows.Err()
			_ = dailyRows.Close()
			if err != nil {
				return nil, err
			}
			state := service.AdvanceUsageTier(rule.config, service.UsageTierState{}, rule.start, now, daily)
			p.Stage = state.Stage
			p.Streak = state.Streak
			p.Missed = state.Missed
			p.TodayTokens = daily[today]
			p.TodayQualified = state.MaintenanceHit
			p.PromotedToday = state.Promoted
			if state.Stage > 0 {
				rate := rule.config.Tiers[state.Stage-1].RateMultiplier
				p.RateMultiplier = &rate
			}
		}
		for _, id := range groupIDs {
			if id == rule.id || slices.Contains(rule.config.LinkedGroupIDs, id) {
				out[id] = append(out[id], p)
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *userGroupRateRepository) GetUsageTierRate(ctx context.Context, userID, groupID int64) (*float64, error) {
	// Only rule applicability is cached. Earned/expired rates are evaluated on every call.
	key := strconv.FormatInt(groupID, 10)
	applicable := false
	hit := false
	if r.tierGroupCache != nil {
		if value, ok := r.tierGroupCache.Get(key); ok {
			applicable, hit = value.(bool)
		}
	}
	if !hit {
		err := scanSingleRow(ctx, r.sql, `SELECT EXISTS(SELECT 1 FROM group_usage_tier_rules r JOIN groups g ON g.id=r.group_id
   WHERE g.deleted_at IS NULL AND g.status='active' AND r.config->>'enabled'='true'
   AND (r.group_id=$1 OR r.config->'linked_group_ids' @> jsonb_build_array($1::bigint)))`, []any{groupID}, &applicable)
		if err != nil {
			return nil, err
		}
		if r.tierGroupCache != nil {
			r.tierGroupCache.SetDefault(key, applicable)
		}
	}
	if !applicable {
		return nil, nil
	}
	progress, err := r.GetUsageTierProgress(ctx, userID, []int64{groupID})
	if err != nil {
		return nil, err
	}
	var rate *float64
	for _, p := range progress[groupID] {
		if p.RateMultiplier != nil && (rate == nil || *p.RateMultiplier < *rate) {
			v := *p.RateMultiplier
			rate = &v
		}
	}
	return rate, nil
}
