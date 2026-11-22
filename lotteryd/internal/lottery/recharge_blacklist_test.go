package lottery

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHistoricalRecharge(t *testing.T) {
	c := ConditionDef{Dimension: DimensionHistoricalRecharge, Threshold: 10, BonusMode: BonusNone}
	require.NoError(t, c.validate())
	for _, amount := range []float64{0, 9.99, 10, 100, -1, math.NaN(), math.Inf(1)} {
		ok, _ := EvaluateCondition(c, &UserUsage{TotalRecharged: &amount}, baseTime())
		require.Equal(t, amount >= 10 && !math.IsInf(amount, 0), ok)
	}
	ok, _ := EvaluateCondition(c, nil, baseTime())
	require.False(t, ok)
	ok, _ = EvaluateCondition(c, &UserUsage{}, baseTime())
	require.False(t, ok)
	for _, threshold := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		c.Threshold = threshold
		require.Error(t, c.validate())
	}
	c.Threshold = 10
	zero := 0.0
	a := Activity{ConditionMatch: MatchAll, Conditions: []ConditionDef{c, {Dimension: DimensionTokenUsage, WindowDays: 1, Mode: "total", Threshold: 1}}}
	u := usageWith(map[int]float64{1: 2})
	u.TotalRecharged = &zero
	require.False(t, a.Evaluate(u, baseTime()).Eligible)
	a.ConditionMatch = MatchAny
	require.True(t, a.Evaluate(u, baseTime()).Eligible)
}

func TestDrawExcludesBlacklistRegardlessOfWeight(t *testing.T) {
	a := Activity{Blacklist: []string{" BLOCKED@Test.com "}, Prizes: []Prize{{ID: 1, Stock: 10}}}
	participants := []Participant{{UserID: 1, Email: "blocked@test.COM", Weight: 1e20}, {UserID: 2, Email: "ok@test.com", Weight: 1, JoinedAt: time.Now()}}
	for seed := int64(0); seed < 100; seed++ {
		wins := DrawPlan(&a, participants, rand.New(rand.NewSource(seed)))
		require.Len(t, wins, 1)
		require.Equal(t, int64(2), wins[0].UserID)
	}
	require.Equal(t, []string{"blocked@test.com"}, NormalizeBlacklist([]string{" BLOCKED@Test.com ", "", "blocked@test.com"}))
	require.Empty(t, DrawPlan(&a, participants[:1], rand.New(rand.NewSource(1))))
}
