package lottery

import (
	"fmt"
	"math"
	"time"
)

// 任务状态。
const (
	TaskActive   = "active"   // 进行中（含未开始）
	TaskEnded    = "ended"    // 已结束（到达期）
	TaskArchived = "archived" // 已归档（管理端手动隐藏，用户侧不再展示）
)

// 任务奖励类型（与抽奖奖品的 balance/redeem_code 对齐）。
const (
	TaskRewardBalance    = "balance"
	TaskRewardRedeemCode = "redeem_code"
)

// 任务重复参与策略（自动结算模式，均以奖励次数封顶口径实现）。
const (
	TaskRepeatUnlimited    = "unlimited"     // 不限：每个结算日达标即发
	TaskRepeatJoinOnce     = "join_once"     // 限参与一次：任务周期内最多 1 次奖励
	TaskRepeatAchievedOnce = "achieved_once" // 达成后不能再参与：获得奖励后不再结算
)

// Task 持续型消耗任务：参与用户在任务周期内按结算日消耗指定分组/模型的
// token 量自动达成并叠加发放奖励（自动参与，无需报名）。
type Task struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Cover string `json:"cover,omitempty"` // 封面（URL 或 data URL）
	// ProgressSvg 进度条小动画：管理端上传的 SVG 源码文本。
	ProgressSvg string `json:"progress_svg,omitempty"`
	// ProgressColor 进度条颜色：#RGB 或 #RRGGBB；空表示用默认色。
	ProgressColor string `json:"progress_color,omitempty"`
	// Description 任务说明（纯文本/Markdown），用户侧展示在任务卡片里。
	Description string `json:"description,omitempty"`
	// GroupID 0 = 全部分组；Model 空 = 不限模型（全部分组时强制为空）。
	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name,omitempty"`
	Model     string `json:"model,omitempty"`
	// StartDate 起始日（YYYY-MM-DD，第 1 天）；持续 DurationDays 天。
	StartDate    string `json:"start_date"`
	DurationDays int    `json:"duration_days"`
	// SettleTime 每日结算时刻 "HH:MM"（北京时间，次日该时刻结算前一天）。
	SettleTime string `json:"settle_time"`
	// ThresholdTokens 达成条件：每个结算日的 token 消耗量（原始 token 数）。
	// 进度阶段上限是它的整数倍：0 时上限为 1 倍，超过 1 倍后上限改为 2 倍，依次类推。
	ThresholdTokens float64 `json:"threshold_tokens"`
	// RewardType balance | redeem_code；RewardValue 每达成单位的奖励值。
	// 兑换码从管理员预录入的码池随机抽取，不走上游生成接口。
	RewardType   string  `json:"reward_type"`
	RewardValue  float64 `json:"reward_value"`
	RepeatPolicy string  `json:"repeat_policy"`
	// Whitelist 非空时仅白名单邮箱可参与；Blacklist 命中即排除。
	Whitelist  []string       `json:"whitelist,omitempty"`
	Blacklist  []string       `json:"blacklist,omitempty"`
	Conditions []ConditionDef `json:"conditions"` // 复用注册时长/Token消耗/活跃度，全部满足（AND）
	// Visibility 这张任务对谁可见。未填写时按全部用户。
	Visibility Visibility `json:"visibility"`
	Status     string     `json:"status"` // active | ended | archived

	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// TaskCode 任务兑换码池里的一张码（管理员手动录入）。
type TaskCode struct {
	ID     int64  `json:"id"`
	TaskID int64  `json:"task_id"`
	Code   string `json:"code"`
	Status string `json:"status"` // available | granted
	// RewardID 发放到的 task_rewards 行（0 = 未发放）。
	RewardID  int64  `json:"reward_id,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// TaskReward 一条结算发放记录（一个任务的一个结算日对一个用户）。
type TaskReward struct {
	ID          int64   `json:"id"`
	TaskID      int64   `json:"task_id"`
	TaskName    string  `json:"task_name,omitempty"` // 联表展示用
	SettleDate  string  `json:"settle_date"`         // 被结算的那一天
	UserID      int64   `json:"user_id"`
	Email       string  `json:"email"`
	Tokens      float64 `json:"tokens"` // 当日该口径消耗量
	Units       int     `json:"units"`  // 达成单位数 = floor(tokens/threshold)
	RewardType  string  `json:"reward_type"`
	RewardValue float64 `json:"reward_value"`
	// Fulfillment pending | done | failed（余额发放失败可重试；码池不足 failed，补码后重试）。
	Fulfillment string   `json:"fulfillment"`
	Codes       []string `json:"codes,omitempty"` // 兑换码奖励发放的码（可多个）
	Note        string   `json:"note,omitempty"`
	Err         string   `json:"error,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty"`
}

// NextStageProgress 把当日消耗映射到当前阶段上限。
// 阈值为 100 时：没有任何消耗，上限是 100；恰好 100，上限仍是 100；超过 100，上限改为 200。
// units 是到达该上限时对应的达成单位数，奖励按这个单位数计算。
func NextStageProgress(threshold, usage float64) (cap float64, units int) {
	if threshold <= 0 || math.IsNaN(threshold) || math.IsInf(threshold, 0) {
		return 0, 0
	}
	if usage <= 0 || math.IsNaN(usage) || math.IsInf(usage, 0) {
		return threshold, 1
	}
	completed := math.Floor(usage/threshold + 1e-9)
	if completed < 1 {
		return threshold, 1
	}
	// 刚好落在整数倍上时，进度条停在这一档并显示满格；再多一点才进入下一档。
	if math.Abs(usage-completed*threshold) <= threshold*1e-6 {
		return completed * threshold, int(completed)
	}
	next := completed + 1
	return next * threshold, int(next)
}

// ParseDate 解析 YYYY-MM-DD（UTC 锚定，仅做日期算术）。
func ParseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// FormatDate 输出 YYYY-MM-DD。
func FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// EndDate 任务最后一天（YYYY-MM-DD）。
func (t *Task) EndDate() string {
	start, err := ParseDate(t.StartDate)
	if err != nil {
		return t.StartDate
	}
	return FormatDate(start.AddDate(0, 0, t.DurationDays-1))
}

// CoversDate 判断 date（YYYY-MM-DD）是否在任务周期内（含首尾）。
func (t *Task) CoversDate(date string) bool {
	return date >= t.StartDate && date <= t.EndDate()
}

// SettleMomentFor 计算结算日 D 的结算时间点：D+1 日的 SettleTime（北京时间）。
func (t *Task) SettleMomentFor(date string) (time.Time, error) {
	d, err := ParseDate(date)
	if err != nil {
		return time.Time{}, fmt.Errorf("task: bad settle date %q: %w", date, err)
	}
	hour, min := 0, 0
	if _, err := fmt.Sscanf(t.SettleTime, "%d:%d", &hour, &min); err != nil {
		return time.Time{}, fmt.Errorf("task: bad settle_time %q", t.SettleTime)
	}
	next := d.AddDate(0, 0, 1)
	return time.Date(next.Year(), next.Month(), next.Day(), hour, min, 0, 0, Beijing()), nil
}

// UserAllowed 白名单/黑名单过滤（邮箱小写比较；名单空表示不限制）。
func (t *Task) UserAllowed(email string) bool {
	email = lower(email)
	if email == "" {
		return false
	}
	if len(t.Whitelist) > 0 && !containsFold(t.Whitelist, email) {
		return false
	}
	if len(t.Blacklist) > 0 && containsFold(t.Blacklist, email) {
		return false
	}
	return true
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

func containsFold(list []string, email string) bool {
	for _, item := range list {
		if lower(item) == email {
			return true
		}
	}
	return false
}

// Validate 校验任务配置。
// isHexColor 只接受 #RGB 与 #RRGGBB 两种写法。
func isHexColor(s string) bool {
	if len(s) != 4 && len(s) != 7 || s[0] != '#' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' {
			continue
		}
		return false
	}
	return true
}

func (t *Task) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("task: name is required")
	}
	if len(t.Name) > 100 {
		return fmt.Errorf("task: name too long")
	}
	if len(t.Description) > 5000 {
		return fmt.Errorf("task: description too long")
	}
	if t.ProgressColor != "" && !isHexColor(t.ProgressColor) {
		return fmt.Errorf("task: progress_color must be #RGB or #RRGGBB")
	}
	if _, err := ParseDate(t.StartDate); err != nil {
		return fmt.Errorf("task: start_date must be YYYY-MM-DD")
	}
	if t.DurationDays < 1 || t.DurationDays > 3650 {
		return fmt.Errorf("task: duration_days must be 1..3650")
	}
	if len(t.SettleTime) != 5 || t.SettleTime[2] != ':' {
		return fmt.Errorf("task: settle_time must be HH:MM")
	}
	var hour, min int
	if _, err := fmt.Sscanf(t.SettleTime, "%d:%d", &hour, &min); err != nil || hour < 0 || hour > 23 || min < 0 || min > 59 {
		return fmt.Errorf("task: settle_time must be HH:MM")
	}
	if t.ThresholdTokens <= 0 {
		return fmt.Errorf("task: threshold_tokens must be > 0")
	}
	if t.RewardValue <= 0 {
		return fmt.Errorf("task: reward_value must be > 0")
	}
	switch t.RewardType {
	case TaskRewardBalance, TaskRewardRedeemCode:
	default:
		return fmt.Errorf("task: unknown reward_type %q", t.RewardType)
	}
	switch t.RepeatPolicy {
	case "", TaskRepeatUnlimited, TaskRepeatJoinOnce, TaskRepeatAchievedOnce:
	default:
		return fmt.Errorf("task: unknown repeat_policy %q", t.RepeatPolicy)
	}
	if t.GroupID < 0 {
		return fmt.Errorf("task: invalid group_id")
	}
	for _, c := range t.Conditions {
		if err := c.validate(); err != nil {
			return err
		}
	}
	for _, e := range append(append([]string{}, t.Whitelist...), t.Blacklist...) {
		if e == "" {
			return fmt.Errorf("task: whitelist/blacklist emails must not be empty")
		}
	}
	t.Visibility = NormalizeItemVisibility(t.Visibility)
	if err := t.Visibility.Validate(); err != nil {
		return err
	}
	return nil
}
