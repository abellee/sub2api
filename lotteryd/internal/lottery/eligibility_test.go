package lottery

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func baseTime() time.Time {
	return time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
}

// usageWith 构造窗口内指定日期的用量：dates 为相对今天往前的天数（1=昨天）。
func usageWith(days map[int]float64) *UserUsage {
	now := baseTime()
	u := &UserUsage{UserID: 1, Email: "a@b.com"}
	for offset, tokens := range days {
		u.Daily = append(u.Daily, DailyToken{
			Date:        now.AddDate(0, 0, -offset).Format("2006-01-02"),
			UserID:      1,
			TotalTokens: tokens,
		})
	}
	return u
}

func TestEvaluateCondition_TokenPerDay(t *testing.T) {
	c := ConditionDef{Dimension: DimensionTokenUsage, WindowDays: 7, Mode: "per_day", Threshold: 100e6, BonusMode: BonusNone}
	// 连续 7 天（昨天..7天前）每天 110M → 满足。
	usage := usageWith(map[int]float64{1: 110e6, 2: 110e6, 3: 110e6, 4: 110e6, 5: 110e6, 6: 110e6, 7: 110e6})
	ok, _ := EvaluateCondition(c, usage, baseTime())
	require.True(t, ok)

	// 第 5 天只有 50M → 不满足。
	usage.Daily[4].TotalTokens = 50e6
	ok, _ = EvaluateCondition(c, usage, baseTime())
	require.False(t, ok)

	// 缺一天（无记录视为 0）→ 不满足。
	usage = usageWith(map[int]float64{1: 110e6, 2: 110e6, 3: 110e6, 4: 110e6, 5: 110e6, 6: 110e6})
	ok, _ = EvaluateCondition(c, usage, baseTime())
	require.False(t, ok)
}

func TestEvaluateCondition_TokenTotal(t *testing.T) {
	c := ConditionDef{Dimension: DimensionTokenUsage, WindowDays: 7, Mode: "total", Threshold: 700e6, BonusMode: BonusNone}
	// 累计 7×110M ≥ 700M，即使其中一天不足单日阈值也满足。
	usage := usageWith(map[int]float64{1: 110e6, 2: 110e6, 3: 110e6, 4: 110e6, 5: 110e6, 6: 110e6, 7: 40e6})
	ok, _ := EvaluateCondition(c, usage, baseTime())
	require.True(t, ok)

	usage = usageWith(map[int]float64{1: 100e6, 2: 100e6, 3: 100e6, 4: 100e6, 5: 100e6, 6: 100e6, 7: 50e6})
	ok, _ = EvaluateCondition(c, usage, baseTime())
	require.False(t, ok)
}

func TestEvaluateCondition_ActiveDays(t *testing.T) {
	c := ConditionDef{Dimension: DimensionActiveDays, WindowDays: 7, MinActiveDays: 5, BonusMode: BonusNone}
	usage := usageWith(map[int]float64{1: 10, 2: 10, 3: 10, 4: 10, 5: 10})
	ok, _ := EvaluateCondition(c, usage, baseTime())
	require.True(t, ok)

	usage = usageWith(map[int]float64{1: 10, 2: 10, 3: 10, 4: 10})
	ok, _ = EvaluateCondition(c, usage, baseTime())
	require.False(t, ok)

	// daily_min_tokens 门槛：低于门槛的天不算活跃。
	c.DailyMinTokens = 100
	usage = usageWith(map[int]float64{1: 10, 2: 10, 3: 100, 4: 100, 5: 100})
	ok, _ = EvaluateCondition(c, usage, baseTime())
	require.False(t, ok)
}

func TestEvaluate_AND_OR(t *testing.T) {
	heavy := ConditionDef{Dimension: DimensionTokenUsage, WindowDays: 7, Mode: "total", Threshold: 100e6, BonusMode: BonusManual, BonusPercent: 50}
	active := ConditionDef{Dimension: DimensionActiveDays, WindowDays: 7, MinActiveDays: 3, BonusMode: BonusManual, BonusPercent: 20}
	a := &Activity{ConditionMatch: MatchAll, AutoBonusPercent: 25, Conditions: []ConditionDef{heavy, active}}

	// 满足 token 不满足活跃 → AND 不合格。
	ok := a.Evaluate(usageWith(map[int]float64{1: 100e6}), baseTime())
	require.False(t, ok.Eligible)

	// 全部满足 → 合格，权重 = 1.5 × 1.2 = 1.8。
	ok = a.Evaluate(usageWith(map[int]float64{1: 100e6, 2: 5, 3: 5, 4: 5}), baseTime())
	require.True(t, ok.Eligible)
	require.InDelta(t, 1.8, ok.Weight, 1e-9)

	// OR：满足任一即合格。
	a.ConditionMatch = MatchAny
	ok = a.Evaluate(usageWith(map[int]float64{1: 100e6}), baseTime())
	require.True(t, ok.Eligible)
	// 只满足一个条件：权重只叠加该条件的加成。
	require.InDelta(t, 1.5, ok.Weight, 1e-9)
}

func TestEvaluate_AutoBonus(t *testing.T) {
	c := ConditionDef{Dimension: DimensionActiveDays, WindowDays: 7, MinActiveDays: 1, BonusMode: BonusAuto}
	a := &Activity{ConditionMatch: MatchAll, AutoBonusPercent: 30, Conditions: []ConditionDef{c}}
	ok := a.Evaluate(usageWith(map[int]float64{1: 10}), baseTime())
	require.True(t, ok.Eligible)
	require.InDelta(t, 1.3, ok.Weight, 1e-9)
}

func TestMaskEmail(t *testing.T) {
	require.Equal(t, "a***@example.com", MaskEmail("abellee@example.com"))
	require.Equal(t, "中***@example.com", MaskEmail("中文邮箱@example.com"))
	require.Equal(t, "***", MaskEmail("no-at-sign"))
	require.Equal(t, "***", MaskEmail("@example.com"))
}

func TestActivityValidate(t *testing.T) {
	a := &Activity{
		Name: "t", StartsAt: baseTime().Add(-time.Hour), DrawsAt: baseTime(),
		ConditionMatch: MatchAll, AutoBonusPercent: 25,
		Conditions: []ConditionDef{{Dimension: DimensionActiveDays, WindowDays: 7, MinActiveDays: 1, BonusMode: BonusNone}},
		Prizes:     []Prize{{Name: "p", PrizeType: PrizeBalance, Value: 5, Stock: 1}},
	}
	require.NoError(t, a.Validate())

	bad := *a
	bad.DrawsAt = baseTime().Add(-2 * time.Hour)
	require.Error(t, bad.Validate())

	bad2 := *a
	bad2.ConditionMatch = "xor"
	require.Error(t, bad2.Validate())
}
