//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type mergedGroupUsageRepo struct {
	UsageLogRepository
}

func (mergedGroupUsageRepo) GetAllGroupUsageSummary(context.Context, time.Time) ([]usagestats.GroupUsageSummary, error) {
	return []usagestats.GroupUsageSummary{{GroupID: 2, TodayCost: 1, YesterdayCost: 2, TotalCost: 9}, {GroupID: 1, TotalCost: 3}}, nil
}
func (mergedGroupUsageRepo) GetGroupCacheUsageSummary(_ context.Context, since24h, since7d time.Time) ([]usagestats.GroupCacheUsageSummary, error) {
	return []usagestats.GroupCacheUsageSummary{
		{GroupID: 1, Total: usagestats.GroupCacheUsageWindow{CacheReadTokens: 10}},
		{GroupID: 2, Today: usagestats.GroupCacheUsageWindow{InputTokens: 4, CacheReadTokens: 6, CacheHitRate: 60}, Last24h: usagestats.GroupCacheUsageWindow{CacheReadTokens: 20}, Last7d: usagestats.GroupCacheUsageWindow{CacheReadTokens: 30}, Total: usagestats.GroupCacheUsageWindow{CacheReadTokens: 40}},
	}, nil
}
func TestDashboardGroupUsagePreservesOfficialCostsAndLocalCacheWindows(t *testing.T) {
	svc := &DashboardService{usageRepo: mergedGroupUsageRepo{}}
	rows, err := svc.GetGroupUsageSummary(context.Background(), time.Now())
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, float64(2), rows[0].YesterdayCost)
	require.Equal(t, float64(9), rows[0].TotalCost)
	require.Equal(t, int64(4), rows[0].TodayInputTokens)
	require.Equal(t, float64(60), rows[0].TodayCacheHitRate)
	require.Equal(t, int64(20), rows[0].Last24hCacheReadTokens)
	require.Equal(t, int64(30), rows[0].Last7dCacheReadTokens)
	require.Equal(t, int64(40), rows[0].TotalCacheReadTokens)
	require.Equal(t, int64(10), rows[1].TotalCacheReadTokens)
}
