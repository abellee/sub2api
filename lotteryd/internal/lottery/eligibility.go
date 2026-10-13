package lottery

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

// EvalResult 单条条件的评估明细。
type EvalResult struct {
	Condition ConditionDef `json:"condition"`
	Satisfied bool         `json:"satisfied"`
	// Detail 人类可读的说明（如窗口内总量、活跃天数）。
	Detail string `json:"detail"`
	// BonusPercent 该条件在满足时贡献的加成百分比。
	BonusPercent float64 `json:"bonus_percent"`
}

// Eligibility 活动对某用户的参与资格评估结果。
type Eligibility struct {
	Eligible bool         `json:"eligible"`
	Weight   float64      `json:"weight"`
	Results  []EvalResult `json:"results"`
	Reason   string       `json:"reason,omitempty"` // 不合格时的原因
}

// dateRange 返回窗口内 [today-N+1 .. today] 的日期串（升序，北京时间）。
// 条件评估只用完整天：窗口以「昨天」为终点，避免当天进行中的数据造成误判。
func windowDates(now time.Time, windowDays int) []string {
	if windowDays <= 0 {
		windowDays = 1
	}
	today := InBeijing(now)
	dates := make([]string, 0, windowDays)
	for i := windowDays; i >= 1; i-- {
		dates = append(dates, today.AddDate(0, 0, -i).Format("2006-01-02"))
	}
	return dates
}

// fmtTokens token 数值展示：以 M（百万 tokens）为单位，去掉多余尾零（1e8 → "100M"，5e5 → "0.5M"）。
func fmtTokens(v float64) string {
	return strconv.FormatFloat(v/1_000_000, 'f', -1, 64) + "M"
}

func dailyMap(usage *UserUsage) map[string]float64 {
	m := make(map[string]float64, len(usage.Daily))
	for _, d := range usage.Daily {
		m[d.Date] = d.TotalTokens
	}
	return m
}

// EvaluateCondition 评估单条条件。
func EvaluateCondition(c ConditionDef, usage *UserUsage, now time.Time) (bool, string) {
	dates := windowDates(now, c.WindowDays)
	daily := dailyMap(usage)
	switch c.Dimension {
	case DimensionTokenUsage:
		switch c.Mode {
		case "per_day":
			ok := true
			var minDay float64 = math.MaxFloat64
			for _, d := range dates {
				v := daily[d]
				if v < minDay {
					minDay = v
				}
				if v < c.Threshold {
					ok = false
				}
			}
			if math.IsInf(minDay, 0) {
				minDay = 0
			}
			return ok, fmt.Sprintf("近 %d 天每日最低 %s tokens（要求每日 ≥ %s）", c.WindowDays, fmtTokens(minDay), fmtTokens(c.Threshold))
		default: // total
			var sum float64
			for _, d := range dates {
				sum += daily[d]
			}
			return sum >= c.Threshold, fmt.Sprintf("近 %d 天累计 %s tokens（要求 ≥ %s）", c.WindowDays, fmtTokens(sum), fmtTokens(c.Threshold))
		}
	case DimensionActiveDays:
		minDaily := c.DailyMinTokens
		if minDaily <= 0 {
			minDaily = 1 // 默认当天有任意用量即算活跃
		}
		active := 0
		for _, d := range dates {
			if daily[d] >= minDaily {
				active++
			}
		}
		return active >= c.MinActiveDays, fmt.Sprintf("近 %d 天活跃 %d 天（要求 ≥ %d 天）", c.WindowDays, active, c.MinActiveDays)
	case DimensionRegisteredDays:
		if usage == nil || usage.RegisteredAt == nil || usage.RegisteredAt.IsZero() {
			return false, "注册时间未知"
		}
		// 注册天数按北京时间自然日计算：注册当天 = 第 1 天，当天注册的用户即可满足「注册满 1 天」。
		regDate := InBeijing(*usage.RegisteredAt)
		nowDate := InBeijing(now)
		dayDiff := int(time.Date(nowDate.Year(), nowDate.Month(), nowDate.Day(), 0, 0, 0, 0, Beijing()).
			Sub(time.Date(regDate.Year(), regDate.Month(), regDate.Day(), 0, 0, 0, 0, Beijing())).Hours() / 24)
		days := dayDiff + 1
		if days < 1 {
			days = 1
		}
		if c.MinRegisteredDays > 0 && days < c.MinRegisteredDays {
			return false, fmt.Sprintf("注册第 %d 天（要求 ≥ 第 %d 天）", days, c.MinRegisteredDays)
		}
		if c.MaxRegisteredDays > 0 && days > c.MaxRegisteredDays {
			return false, fmt.Sprintf("注册第 %d 天（要求 ≤ 第 %d 天）", days, c.MaxRegisteredDays)
		}
		return true, fmt.Sprintf("注册第 %d 天", days)
	default:
		return false, "未知维度"
	}
}

// conditionBonus 满足时该条件贡献的加成百分比。
func conditionBonus(c ConditionDef, autoPercent float64) float64 {
	switch c.BonusMode {
	case BonusManual:
		return c.BonusPercent
	case BonusAuto:
		return autoPercent
	default:
		return 0
	}
}

// Evaluate 按活动的 AND/OR 语义评估全部条件并计算用户权重。
// 权重 = 各「已满足且配置了加成」条件的 (1+pct/100) 连乘，基础 1。
// 未配置任何条件时任何人可参与（权重 1）。
func (a *Activity) Evaluate(usage *UserUsage, now time.Time) Eligibility {
	if len(a.Conditions) == 0 {
		return Eligibility{Eligible: true, Weight: 1, Results: []EvalResult{}}
	}
	if usage == nil {
		usage = &UserUsage{UserID: 0}
	}
	results := make([]EvalResult, 0, len(a.Conditions))
	satisfiedCount := 0
	weight := 1.0
	for _, c := range a.Conditions {
		ok, detail := EvaluateCondition(c, usage, now)
		bonus := 0.0
		if ok {
			satisfiedCount++
			bonus = conditionBonus(c, a.AutoBonusPercent)
			weight *= 1 + bonus/100
		}
		results = append(results, EvalResult{
			Condition: c, Satisfied: ok, Detail: detail, BonusPercent: bonus,
		})
	}
	match := a.ConditionMatch == MatchAny && satisfiedCount > 0 ||
		a.ConditionMatch == MatchAll && satisfiedCount == len(a.Conditions)
	e := Eligibility{Eligible: match, Weight: weight, Results: results}
	if !match {
		e.Reason = "未满足参与条件"
	}
	return e
}
