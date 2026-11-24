package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// Tier days use one fixed calendar, independent of the server's local timezone.
var UsageTierLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

type UsageTier struct {
	DailyTokens     int64   `json:"daily_tokens"`
	DailyCost       float64 `json:"daily_cost"`
	ConsecutiveDays int     `json:"consecutive_days"`
	MaxMissedDays   int     `json:"max_missed_days"`
	RateMultiplier  float64 `json:"rate_multiplier"`
}

type GroupUsageTierConfig struct {
	Enabled         bool        `json:"enabled"`
	Tiers           []UsageTier `json:"tiers"`
	LinkedGroupIDs  []int64     `json:"linked_group_ids"`
	ExcludedUserIDs []int64     `json:"excluded_user_ids"`
	Models          []string    `json:"models"`
	Mode            string      `json:"mode"`
}

func (c GroupUsageTierConfig) EffectiveMode() string {
	if c.Mode == "" {
		for _, tier := range c.Tiers {
			if tier.DailyCost > 0 && tier.DailyTokens == 0 {
				return "cost"
			}
		}
		return "tokens"
	}
	return c.Mode
}

type UsageTierDaily struct {
	Tokens int64
	Cost   float64
}

func (c GroupUsageTierConfig) qualifies(d UsageTierDaily, tier UsageTier) bool {
	if c.EffectiveMode() == "cost" {
		return d.Cost >= tier.DailyCost
	}
	return d.Tokens >= tier.DailyTokens
}

func (c GroupUsageTierConfig) Validate(groupID int64) error {
	if c.EffectiveMode() != "tokens" && c.EffectiveMode() != "cost" {
		return fmt.Errorf("达标模式无效")
	}
	if len(c.Tiers) > 20 || (c.Enabled && len(c.Tiers) == 0) {
		return fmt.Errorf("请设置 1 到 20 个阶梯")
	}
	for i, t := range c.Tiers {
		invalidTarget := t.DailyTokens <= 0 || t.DailyTokens > 9007199254740991
		if c.EffectiveMode() == "cost" {
			invalidTarget = math.IsNaN(t.DailyCost) || math.IsInf(t.DailyCost, 0) || t.DailyCost <= 0
		}
		if invalidTarget || t.ConsecutiveDays < 1 || t.ConsecutiveDays > 365 || t.MaxMissedDays < 1 || t.MaxMissedDays > 365 || math.IsNaN(t.RateMultiplier) || math.IsInf(t.RateMultiplier, 0) || t.RateMultiplier < 0 || t.RateMultiplier > 100 {
			return fmt.Errorf("阶梯 %d 的达标数量、天数或倍率无效", i+1)
		}
		if i > 0 && (!c.qualifies(UsageTierDaily{Tokens: t.DailyTokens, Cost: t.DailyCost}, c.Tiers[i-1]) || t.RateMultiplier >= c.Tiers[i-1].RateMultiplier) {
			return fmt.Errorf("后续阶梯的达标数量不能降低，倍率必须降低")
		}
	}
	for _, ids := range [][]int64{c.LinkedGroupIDs, c.ExcludedUserIDs} {
		if len(ids) > 1000 {
			return fmt.Errorf("关联分组或不参与用户过多")
		}
		seen := map[int64]bool{}
		for _, id := range ids {
			if id <= 0 || seen[id] {
				return fmt.Errorf("分组或用户 ID 无效或重复")
			}
			seen[id] = true
		}
	}
	for _, id := range c.LinkedGroupIDs {
		if id == groupID {
			return fmt.Errorf("不需要关联分组自身")
		}
	}
	if len(c.Models) > 200 {
		return fmt.Errorf("最多选择 200 个模型")
	}
	seenModels := map[string]bool{}
	for _, model := range c.Models {
		if strings.TrimSpace(model) != model || model == "" || len(model) > 200 || seenModels[model] {
			return fmt.Errorf("模型名称无效或重复")
		}
		seenModels[model] = true
	}
	return nil
}

// Day is the currently open day. Flags prevent repeated requests from counting it twice.
type UsageTierState struct {
	Stage          int    `json:"stage"`
	Streak         int    `json:"streak"`
	Missed         int    `json:"missed"`
	Day            string `json:"day"`
	UpgradeCounted bool   `json:"upgrade_counted"`
	MaintenanceHit bool   `json:"maintenance_hit"`
	Promoted       bool   `json:"promoted"`
}

func AdvanceUsageTier(c GroupUsageTierConfig, s UsageTierState, start, now time.Time, daily map[string]UsageTierDaily) UsageTierState {
	today := now.In(UsageTierLocation).Format(time.DateOnly)
	if s.Day == "" {
		s.Day = start.In(UsageTierLocation).Format(time.DateOnly)
	}
	day, _ := time.ParseInLocation(time.DateOnly, s.Day, UsageTierLocation)
	for ; s.Day <= today; day = day.AddDate(0, 0, 1) {
		amount := daily[s.Day]
		if s.Stage > 0 && c.qualifies(amount, c.Tiers[s.Stage-1]) {
			s.Missed = 0
			s.MaintenanceHit = true
		}
		if s.Stage < len(c.Tiers) && !s.UpgradeCounted && !s.Promoted && c.qualifies(amount, c.Tiers[s.Stage]) {
			s.Streak++
			s.UpgradeCounted = true
			if s.Streak >= c.Tiers[s.Stage].ConsecutiveDays {
				s.Stage++
				s.Streak = 0
				s.Missed = 0
				s.Promoted = true
				s.MaintenanceHit = true
			}
		}
		if s.Day == today {
			break
		}
		if !s.UpgradeCounted && !s.Promoted {
			s.Streak = 0
		}
		if s.Stage > 0 && !s.MaintenanceHit {
			s.Missed++
			if s.Missed >= c.Tiers[s.Stage-1].MaxMissedDays {
				s.Stage = 0
				s.Streak = 0
				s.Missed = 0
			}
		}
		s.UpgradeCounted = false
		s.MaintenanceHit = false
		s.Promoted = false
		s.Day = day.AddDate(0, 0, 1).Format(time.DateOnly)
	}
	return s
}

type GroupUsageTierProgress struct {
	Mode            string      `json:"mode"`
	SourceGroupID   int64       `json:"source_group_id"`
	SourceGroupName string      `json:"source_group_name"`
	Tiers           []UsageTier `json:"tiers"`
	Models          []string    `json:"models"`
	Excluded        bool        `json:"excluded"`
	Stage           int         `json:"stage"`
	Streak          int         `json:"streak"`
	Missed          int         `json:"missed"`
	TodayTokens     int64       `json:"today_tokens"`
	TodayCost       float64     `json:"today_cost"`
	TodayQualified  bool        `json:"today_qualified"`
	PromotedToday   bool        `json:"promoted_today"`
	RateMultiplier  *float64    `json:"rate_multiplier,omitempty"`
}

type GroupUsageTierRepository interface {
	GetUsageTierConfig(context.Context, int64) (GroupUsageTierConfig, error)
	SaveUsageTierConfig(context.Context, int64, GroupUsageTierConfig) error
	GetUsageTierProgress(context.Context, int64, []int64) (map[int64][]GroupUsageTierProgress, error)
	GetUsageTierRate(context.Context, int64, int64) (*float64, error)
}
