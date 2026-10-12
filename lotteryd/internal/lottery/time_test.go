package lottery

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBeijingOffset(t *testing.T) {
	utc := time.Date(2026, 10, 7, 18, 30, 0, 0, time.UTC)
	bj := InBeijing(utc)
	require.Equal(t, 2026, bj.Year())
	require.Equal(t, time.October, bj.Month())
	require.Equal(t, 8, bj.Day())
	require.Equal(t, 2, bj.Hour())
	require.Equal(t, 30, bj.Minute())

	wall, err := time.ParseInLocation("15:04", "18:30", Beijing())
	require.NoError(t, err)
	start := time.Date(2026, 10, 8, wall.Hour(), wall.Minute(), 0, 0, Beijing())
	require.Equal(t, "2026-10-08T10:30:00Z", start.UTC().Format(time.RFC3339))
}

func TestSettleMomentIsBeijing(t *testing.T) {
	task := &Task{SettleTime: "00:10"}
	at, err := task.SettleMomentFor("2026-10-07")
	require.NoError(t, err)
	require.Equal(t, "2026-10-07T16:10:00Z", at.UTC().Format(time.RFC3339))
	require.Equal(t, 8, InBeijing(at).Day())
	require.Equal(t, 0, InBeijing(at).Hour())
	require.Equal(t, 10, InBeijing(at).Minute())
}

func TestRegisteredDaysUseBeijingCalendar(t *testing.T) {
	// 北京时间 10月8日 00:30 注册，同一分钟评估，算注册第 1 天。
	reg := time.Date(2026, 10, 7, 16, 30, 0, 0, time.UTC)
	now := time.Date(2026, 10, 7, 16, 30, 0, 0, time.UTC)
	c := ConditionDef{Dimension: DimensionRegisteredDays, MinRegisteredDays: 1, MaxRegisteredDays: 1}
	ok, detail := EvaluateCondition(c, &UserUsage{RegisteredAt: &reg}, now)
	require.True(t, ok, detail)
}
