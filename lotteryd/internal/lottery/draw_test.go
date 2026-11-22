package lottery

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func testActivity() *Activity {
	return &Activity{
		ID: 1, ConditionMatch: MatchAll,
		Prizes: []Prize{
			{ID: 101, Name: "大奖", PrizeType: PrizeBalance, Value: 100, Weight: 1, Stock: 1},
			{ID: 102, Name: "小奖", PrizeType: PrizeRedeemCode, Value: 5, Weight: 0, Stock: 3},
		},
	}
}

func participants(n int) []Participant {
	out := make([]Participant, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, Participant{UserID: int64(i), Email: "u@x.com", Weight: 1})
	}
	return out
}

func TestDrawPlan_OneWinPerUser(t *testing.T) {
	a := testActivity()
	a.Prizes[1].Stock = 10 // 奖品足够所有人
	ps := participants(5)
	rng := rand.New(rand.NewSource(1))
	drafts := DrawPlan(a, ps, rng)
	require.Len(t, drafts, 5)

	seen := map[int64]bool{}
	for _, d := range drafts {
		require.False(t, seen[d.UserID], "每个参与者最多中一次")
		seen[d.UserID] = true
	}
}

func TestDrawPlan_RespectsStock(t *testing.T) {
	a := testActivity() // 1 + 3 = 4 个奖品
	ps := participants(10)
	rng := rand.New(rand.NewSource(2))
	drafts := DrawPlan(a, ps, rng)
	require.Len(t, drafts, 4, "库存耗尽即停止")

	counts := map[int64]int{}
	for _, d := range drafts {
		counts[d.Prize.ID]++
	}
	require.LessOrEqual(t, counts[101], 1)
	require.LessOrEqual(t, counts[102], 3)
}

func TestDrawPlan_EmptyParticipants(t *testing.T) {
	a := testActivity()
	drafts := DrawPlan(a, nil, rand.New(rand.NewSource(3)))
	require.Empty(t, drafts)
}

func TestDrawPlan_HeavierWeightWinsMore(t *testing.T) {
	// 单奖品单名额：加权用户的夺魁频率应显著高于均权用户。
	a := testActivity()
	a.Prizes = []Prize{{ID: 1, Name: "only", PrizeType: PrizeBalance, Value: 1, Stock: 1, Weight: 1}}
	ps := []Participant{
		{UserID: 1, Weight: 9.0}, // 条件加成用户
		{UserID: 2, Weight: 1.0},
		{UserID: 3, Weight: 1.0},
	}
	rng := rand.New(rand.NewSource(42))
	win1 := 0
	runs := 2000
	for i := 0; i < runs; i++ {
		drafts := DrawPlan(a, ps, rng)
		require.Len(t, drafts, 1)
		if drafts[0].UserID == 1 {
			win1++
		}
	}
	// 权重 9/11 ≈ 81.8%，宽松断言避免随机抖动。
	require.Greater(t, win1, runs*7/10, "加权用户应显著更常中奖")
}

func TestWeightedPick_NoCandidates(t *testing.T) {
	items := []int{1, 2, 3}
	idx := WeightedPick(items, func(int) float64 { return 0 }, rand.New(rand.NewSource(1)))
	require.Equal(t, -1, idx)
}

func TestFormatValue(t *testing.T) {
	require.Equal(t, "5", FormatValue(5))
	require.Equal(t, "5.5", FormatValue(5.5))
	require.Equal(t, "0.01", FormatValue(0.01))
}
