package lottery

import (
	"math/rand"
	"sort"
	"strconv"
	"time"
)

// WeightedPick 按权重随机挑选一个元素（权重 <=0 的元素不参与）。
// rng 可注入以便测试；返回选中下标，无候选时返回 -1。
func WeightedPick[T any](items []T, weight func(T) float64, rng *rand.Rand) int {
	total := 0.0
	for _, it := range items {
		if w := weight(it); w > 0 {
			total += w
		}
	}
	if total <= 0 {
		return -1
	}
	r := rng.Float64() * total
	var acc float64
	for i, it := range items {
		if w := weight(it); w > 0 {
			acc += w
			if r < acc {
				return i
			}
		}
	}
	// 浮点兜底：返回最后一个有权重的元素。
	for i := len(items) - 1; i >= 0; i-- {
		if weight(items[i]) > 0 {
			return i
		}
	}
	return -1
}

// WinnerDraft 开奖产生的一条中奖记录（尚未落库/发放）。
type WinnerDraft struct {
	UserID    int64
	Email     string
	Prize     Prize
	Weight    float64
	CreatedAt time.Time
}

// DrawPlan 开奖：按奖品权重决定发放顺序，每个奖品在未中奖参与者中
// 按用户权重加权随机抽一人；每人限中一个奖品；库存发完或人选抽尽即止。
func DrawPlan(a *Activity, participants []Participant, rng *rand.Rand) []WinnerDraft {
	now := time.Now().UTC()
	remaining := make([]Participant, len(participants))
	copy(remaining, participants)
	// 参与时间早者优先的确定性排序基础（再由加权随机决定中奖者）。
	sort.SliceStable(remaining, func(i, j int) bool {
		return remaining[i].JoinedAt.Before(remaining[j].JoinedAt)
	})

	prizes := make([]Prize, len(a.Prizes))
	copy(prizes, a.Prizes)

	var drafts []WinnerDraft
	for len(remaining) > 0 {
		// 仍有库存的奖品。
		available := make([]Prize, 0, len(prizes))
		for _, p := range prizes {
			if p.GrantedCount < p.Stock {
				available = append(available, p)
			}
		}
		if len(available) == 0 {
			break
		}
		pi := WeightedPick(available, Prize.EffectiveWeight, rng)
		if pi < 0 {
			break
		}
		prize := available[pi]

		ui := WeightedPick(remaining, func(p Participant) float64 {
			if p.Weight > 0 {
				return p.Weight
			}
			return 1
		}, rng)
		if ui < 0 {
			break
		}
		winner := remaining[ui]
		remaining = append(remaining[:ui], remaining[ui+1:]...)

		drafts = append(drafts, WinnerDraft{
			UserID:    winner.UserID,
			Email:     winner.Email,
			Prize:     prize,
			Weight:    winner.Weight,
			CreatedAt: now,
		})
		// 标记库存占用，保持本地与落库一致。
		for i := range prizes {
			if prizes[i].ID == prize.ID {
				prizes[i].GrantedCount++
			}
		}
	}
	return drafts
}

// FormatValue 金额展示：去掉多余的尾零（5.00 → "5"，5.50 → "5.5"）。
func FormatValue(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
