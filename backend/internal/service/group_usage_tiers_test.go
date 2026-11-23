package service

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func tierTime(day string) time.Time {
	v, _ := time.ParseInLocation("2006-01-02 15:04", day, UsageTierLocation)
	return v
}
func tierConfig() GroupUsageTierConfig {
	return GroupUsageTierConfig{Enabled: true, Tiers: []UsageTier{{DailyCost: 10, ConsecutiveDays: 2, MaxMissedDays: 2, RateMultiplier: 0.8}, {DailyCost: 20, ConsecutiveDays: 2, MaxMissedDays: 3, RateMultiplier: 0.5}}}
}

func TestUsageTierAdvancementAndMaintenance(t *testing.T) {
	c := tierConfig()
	start := tierTime("2026-11-01 00:00")
	daily := map[string]float64{"2026-11-01": 10, "2026-11-02": 50, "2026-11-03": 20, "2026-11-04": 20}
	s := AdvanceUsageTier(c, UsageTierState{}, start, tierTime("2026-11-01 20:00"), daily)
	require.Equal(t, 0, s.Stage)
	require.Equal(t, 1, s.Streak)
	// Refreshing or using another key must not count the same day twice.
	require.Equal(t, s, AdvanceUsageTier(c, s, start, tierTime("2026-11-01 23:59"), daily))
	s = AdvanceUsageTier(c, s, start, tierTime("2026-11-02 20:00"), daily)
	require.Equal(t, 1, s.Stage)
	require.Zero(t, s.Streak)
	require.True(t, s.Promoted)
	require.Equal(t, s, AdvanceUsageTier(c, s, start, tierTime("2026-11-02 23:59"), daily))
	s = AdvanceUsageTier(c, s, start, tierTime("2026-11-03 20:00"), daily)
	require.Equal(t, 1, s.Stage)
	require.Equal(t, 1, s.Streak)
	s = AdvanceUsageTier(c, s, start, tierTime("2026-11-04 20:00"), daily)
	require.Equal(t, 2, s.Stage)
	require.Zero(t, s.Streak)
	s = AdvanceUsageTier(c, s, start, tierTime("2026-11-06 00:00"), daily)
	require.Equal(t, 1, s.Missed)
	require.Equal(t, 2, s.Stage)
	// A qualifying day resets the complete missed-day countdown immediately.
	daily["2026-11-06"] = 20
	s = AdvanceUsageTier(c, s, start, tierTime("2026-11-06 12:00"), daily)
	require.Zero(t, s.Missed)
	require.True(t, s.MaintenanceHit)
	s = AdvanceUsageTier(c, s, start, tierTime("2026-11-10 00:00"), daily)
	require.Zero(t, s.Stage)
	require.Zero(t, s.Streak)
	// Expiration returns to the first tier rather than the preceding tier.
	daily["2026-11-10"] = 20
	s = AdvanceUsageTier(c, s, start, tierTime("2026-11-10 12:00"), daily)
	require.Zero(t, s.Stage)
	require.Equal(t, 1, s.Streak)
}

func TestUsageTierMissedDaysAndTimezone(t *testing.T) {
	c := tierConfig()
	start := tierTime("2026-11-01 00:00")
	s := AdvanceUsageTier(c, UsageTierState{}, start, tierTime("2026-11-04 12:00"), map[string]float64{"2026-11-01": 10, "2026-11-03": 10, "2026-11-04": 10})
	require.Equal(t, 1, s.Stage) // Nov 2 broke the original streak.
	s = AdvanceUsageTier(c, s, start, tierTime("2026-11-06 23:59"), nil)
	require.Equal(t, 1, s.Stage)
	require.Equal(t, 1, s.Missed) // The open day has not failed yet.
	utc, _ := time.Parse(time.RFC3339, "2026-11-06T16:00:00Z")
	s = AdvanceUsageTier(c, s, start, utc, nil)
	require.Zero(t, s.Stage)
	require.Equal(t, "2026-11-07", s.Day)
}

func TestUsageTierExactMoneyThreshold(t *testing.T) {
	c := tierConfig()
	c.Tiers[0].DailyCost = 1e-10
	c.Tiers[0].ConsecutiveDays = 1
	start := tierTime("2026-11-01 00:00")
	s := AdvanceUsageTier(c, UsageTierState{}, start, start, nil)
	require.Zero(t, s.Stage)
	s = AdvanceUsageTier(c, s, start, start, map[string]float64{"2026-11-01": 1e-10})
	require.Equal(t, 1, s.Stage)
}

func TestUsageTierValidate(t *testing.T) {
	require.NoError(t, tierConfig().Validate(1))
	for _, change := range []func(*GroupUsageTierConfig){
		func(c *GroupUsageTierConfig) { c.Tiers = nil },
		func(c *GroupUsageTierConfig) { c.Tiers[0].DailyCost = math.Inf(1) },
		func(c *GroupUsageTierConfig) { c.Tiers[0].ConsecutiveDays = 0 },
		func(c *GroupUsageTierConfig) { c.Tiers[0].MaxMissedDays = 0 },
		func(c *GroupUsageTierConfig) { c.Tiers[1].RateMultiplier = 0.9 },
		func(c *GroupUsageTierConfig) { c.Tiers[1].DailyCost = 1 },
		func(c *GroupUsageTierConfig) { c.LinkedGroupIDs = []int64{1} },
		func(c *GroupUsageTierConfig) { c.ExcludedUserIDs = []int64{2, 2} },
	} {
		c := tierConfig()
		change(&c)
		require.Error(t, c.Validate(1))
	}
}

type usageTierResolverStub struct {
	UserGroupRateRepository
	GroupUsageTierRepository
	rate  *float64
	tier  *float64
	calls int
}

func (s *usageTierResolverStub) GetByUserAndGroup(context.Context, int64, int64) (*float64, error) {
	s.calls++
	return s.rate, nil
}
func (s *usageTierResolverStub) GetUsageTierRate(context.Context, int64, int64) (*float64, error) {
	return s.tier, nil
}

func TestUsageTierBypassesBaseRateCache(t *testing.T) {
	manual, earned := 0.7, 0.5
	repo := &usageTierResolverStub{rate: &manual}
	resolver := newUserGroupRateResolver(repo, nil, time.Hour, nil, "")
	require.Equal(t, 0.7, resolver.Resolve(context.Background(), 2, 1, 1))
	repo.tier = &earned
	require.Equal(t, 0.5, resolver.Resolve(context.Background(), 2, 1, 1))
	repo.tier = nil
	require.Equal(t, 0.7, resolver.Resolve(context.Background(), 2, 1, 1))
	earned = 0.8
	repo.tier = &earned
	require.Equal(t, 0.7, resolver.Resolve(context.Background(), 2, 1, 1))
	require.Equal(t, 1, repo.calls)
}
