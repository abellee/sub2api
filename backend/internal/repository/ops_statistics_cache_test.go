package repository

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestOpsStatisticsCacheKeys(t *testing.T) {
	now := time.Now().UTC().Truncate(30 * time.Second).Add(10 * time.Second)
	// Keep both live requests within the accepted live tolerance.
	if time.Since(now) > 4*time.Second || time.Until(now) > 4*time.Second {
		now = time.Now().UTC()
	}
	f := service.OpsDashboardFilter{StartTime: now.Add(-time.Hour), EndTime: now, QueryMode: service.OpsQueryModeRaw}
	canonical := cachedOpsFilter(&f)
	require.Equal(t, canonical.EndTime.Add(-time.Hour), canonical.StartTime)
	require.Equal(t, canonical.EndTime.Truncate(30*time.Second), canonical.EndTime)
	require.Equal(t, now, f.EndTime)
	other := canonical
	other.Platform = "gemini"
	require.NotEqual(t, opsStatisticsKey(&canonical), opsStatisticsKey(&other))
	id := int64(2)
	other = canonical
	other.GroupID = &id
	require.NotEqual(t, opsStatisticsKey(&canonical), opsStatisticsKey(&other))
	other = canonical
	other.QueryMode = service.OpsQueryModePreagg
	require.NotEqual(t, opsStatisticsKey(&canonical), opsStatisticsKey(&other))
	historical := f
	historical.EndTime = now.Add(-24 * time.Hour)
	historical.StartTime = historical.EndTime.Add(-time.Hour)
	require.Equal(t, historical, cachedOpsFilter(&historical))
	custom := f
	custom.StartTime = now.Add(-93 * time.Minute)
	require.Equal(t, custom, cachedOpsFilter(&custom))
}
