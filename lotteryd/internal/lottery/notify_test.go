package lottery

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDueNotifyStages(t *testing.T) {
	start := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	draw := start.Add(2 * time.Hour)
	activity := &Activity{Status: "active", StartsAt: start, DrawsAt: draw}

	require.Empty(t, DueNotifyStages(activity, start.Add(-11*time.Minute), nil))
	require.Equal(t, []string{StageBeforeStart}, DueNotifyStages(activity, start.Add(-10*time.Minute), nil))
	require.Equal(t, []string{StageStarted}, DueNotifyStages(activity, start, nil))
	require.Equal(t, []string{StageStarted, StageBeforeDraw}, DueNotifyStages(activity, draw.Add(-5*time.Minute), nil))

	sent := map[string]struct{}{StageStarted: {}}
	require.Equal(t, []string{StageBeforeDraw}, DueNotifyStages(activity, draw.Add(-4*time.Minute), sent))

	activity.DrawnAt = draw
	require.Equal(t, []string{StageResults}, DueNotifyStages(activity, draw, nil))
	require.Empty(t, DueNotifyStages(activity, draw, map[string]struct{}{StageResults: {}}))

	closed := &Activity{Status: "archived", StartsAt: start, DrawsAt: draw}
	require.Empty(t, DueNotifyStages(closed, start, nil))
	closed.DrawnAt = draw
	require.Equal(t, []string{StageResults}, DueNotifyStages(closed, draw.Add(time.Minute), nil))
}

func TestResolveNotifyStages(t *testing.T) {
	require.Empty(t, ResolveNotifyStages(nil, false))
	require.Equal(t, AllNotifyStages(), ResolveNotifyStages(nil, true))
	require.Empty(t, ResolveNotifyStages([]string{}, true))
	require.Equal(t, []string{StageStarted, StageResults}, ResolveNotifyStages([]string{"results", "nope", "started", "started"}, false))
}

func TestNextStageProgress(t *testing.T) {
	cap, units := NextStageProgress(100, 0)
	require.Equal(t, 100.0, cap)
	require.Equal(t, 1, units)

	cap, units = NextStageProgress(100, 50)
	require.Equal(t, 100.0, cap)
	require.Equal(t, 1, units)

	cap, units = NextStageProgress(100, 100)
	require.Equal(t, 100.0, cap)
	require.Equal(t, 1, units)

	cap, units = NextStageProgress(100, 100.1)
	require.Equal(t, 200.0, cap)
	require.Equal(t, 2, units)

	cap, units = NextStageProgress(100, 200)
	require.Equal(t, 200.0, cap)
	require.Equal(t, 2, units)

	cap, units = NextStageProgress(1e8, 2.5e8)
	require.Equal(t, 3e8, cap)
	require.Equal(t, 3, units)

	cap, units = NextStageProgress(0, 10)
	require.Equal(t, 0.0, cap)
	require.Equal(t, 0, units)
}
