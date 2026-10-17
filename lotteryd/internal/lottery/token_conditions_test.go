package lottery

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The same usage must produce different results for any-day, every-day and
// window-total conditions. Thresholds are inclusive and missing days count as zero.
func TestTokenConditionSemantics(t *testing.T) {
	conditions := []ConditionDef{
		{Dimension: DimensionActiveDays, WindowDays: 7, MinActiveDays: 1, DailyMinTokens: 1e6, BonusMode: BonusNone},
		{Dimension: DimensionTokenUsage, WindowDays: 7, Mode: "per_day", Threshold: 1e6, BonusMode: BonusNone},
		{Dimension: DimensionTokenUsage, WindowDays: 7, Mode: "total", Threshold: 1e6, BonusMode: BonusNone},
	}
	for _, tc := range []struct {
		name string
		days map[int]float64
		want [3]bool // active, per_day, total
	}{
		{"no_usage", nil, [3]bool{false, false, false}},
		{"single_day_exact", map[int]float64{1: 1e6}, [3]bool{true, false, true}},
		{"single_day_one_token_short", map[int]float64{1: 999999}, [3]bool{false, false, false}},
		{"split_total_exact", map[int]float64{1: 500000, 2: 500000}, [3]bool{false, false, true}},
		{"split_total_one_token_short", map[int]float64{1: 500000, 2: 499999}, [3]bool{false, false, false}},
		{"every_day_exact", map[int]float64{1: 1e6, 2: 1e6, 3: 1e6, 4: 1e6, 5: 1e6, 6: 1e6, 7: 1e6}, [3]bool{true, true, true}},
		{"one_day_one_token_short", map[int]float64{1: 999999, 2: 1e6, 3: 1e6, 4: 1e6, 5: 1e6, 6: 1e6, 7: 1e6}, [3]bool{true, false, true}},
		{"missing_day_despite_large_total", map[int]float64{1: 100e6, 2: 1e6, 3: 1e6, 4: 1e6, 5: 1e6, 6: 1e6}, [3]bool{true, false, true}},
		{"explicit_zero_day", map[int]float64{1: 100e6, 2: 1e6, 3: 1e6, 4: 1e6, 5: 1e6, 6: 1e6, 7: 0}, [3]bool{true, false, true}},
		{"oldest_day_included", map[int]float64{7: 1e6}, [3]bool{true, false, true}},
		{"outside_window_excluded", map[int]float64{0: 100e6, 8: 100e6}, [3]bool{false, false, false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i, c := range conditions {
				require.NoError(t, c.validate())
				got, detail := EvaluateCondition(c, usageWith(tc.days), baseTime())
				require.Equal(t, tc.want[i], got, "condition %d: %s", i, detail)
				a := Activity{ConditionMatch: MatchAll, Conditions: []ConditionDef{c}}
				require.Equal(t, tc.want[i], a.Evaluate(usageWith(tc.days), baseTime()).Eligible)
			}
		})
	}
}

func TestTokenWindowsUseCompleteBeijingDays(t *testing.T) {
	// UTC is still Oct 17, but Beijing has already entered Oct 18.
	now := time.Date(2026, 10, 17, 16, 30, 0, 0, time.UTC)
	require.Equal(t, []string{"2026-10-11", "2026-10-12", "2026-10-13", "2026-10-14", "2026-10-15", "2026-10-16", "2026-10-17"}, windowDates(now, 7))
	for _, c := range []ConditionDef{
		{Dimension: DimensionActiveDays, WindowDays: 1, MinActiveDays: 1, DailyMinTokens: 1e6},
		{Dimension: DimensionTokenUsage, WindowDays: 1, Mode: "per_day", Threshold: 1e6},
		{Dimension: DimensionTokenUsage, WindowDays: 1, Mode: "total", Threshold: 1e6},
	} {
		for _, tc := range []struct {
			date string
			want bool
		}{
			{"2026-10-17", true}, {"2026-10-18", false}, {"2026-10-16", false},
		} {
			got, detail := EvaluateCondition(c, &UserUsage{Daily: []DailyToken{{Date: tc.date, TotalTokens: 1e6}}}, now)
			require.Equal(t, tc.want, got, detail)
		}
	}
}
