// Package lottery contains the domain model and business logic of lotteryd.
package lottery

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Condition dimensions.
const (
	DimensionTokenUsage     = "token_usage"     // Token 消耗量
	DimensionActiveDays     = "activity_days"   // 活跃度
	DimensionRegisteredDays = "registered_days" // 注册时长
)

// Condition match modes for an activity.
const (
	MatchAll = "all" // AND：满足全部条件
	MatchAny = "any" // OR：满足任一条件
)

// Bonus modes of a condition.
const (
	BonusNone   = "none"   // 不加成
	BonusManual = "manual" // 手动输入百分比
	BonusAuto   = "auto"   // 系统自动叠加（按全局默认规则）
)

// Prize types.
const (
	PrizeRedeemCode = "redeem_code" // 兑换码：中奖者自行兑换
	PrizeBalance    = "balance"     // 余额：开奖时直接入账并备注
)

// Fulfillment states of a winner record.
const (
	FulfillmentPending = "pending"
	FulfillmentDone    = "done"
	FulfillmentFailed  = "failed"
)

// Activity 状态（派生）。
const (
	ActivityUpcoming  = "upcoming"  // 未到参与开始时间
	ActivityJoining   = "joining"   // 参与中
	ActivityDrawn     = "drawn"     // 已开奖
	ActivityFulfilled = "fulfilled" // 已开奖且发放完成
)

// ConditionDef 是一条可参与条件的定义。
type ConditionDef struct {
	Dimension  string `json:"dimension"`
	WindowDays int    `json:"window_days"`
	// Token 消耗量维度：
	//   Mode=per_day 时窗口内每天 total_tokens 都要 >= Threshold；
	//   Mode=total   时窗口内累计 >= Threshold。
	Mode      string  `json:"mode,omitempty"` // per_day | total
	Threshold float64 `json:"threshold,omitempty"`
	// 活跃度维度：窗口内有用量（>= DailyMinTokens，默认 >0）的天数 >= MinActiveDays。
	MinActiveDays  int     `json:"min_active_days,omitempty"`
	DailyMinTokens float64 `json:"daily_min_tokens,omitempty"`
	// 注册时长维度：注册天数在 [MinRegisteredDays, MaxRegisteredDays] 内（0 = 该侧不设限）。
	// Min=0 且 Max=N → 新用户（注册 ≤ N 天）；Min=N → 老用户（注册满 N 天）。
	MinRegisteredDays int `json:"min_registered_days,omitempty"`
	MaxRegisteredDays int `json:"max_registered_days,omitempty"`
	// 概率加成：none / manual（BonusPercent）/ auto（系统按全局默认叠加）。
	BonusMode    string  `json:"bonus_mode"`
	BonusPercent float64 `json:"bonus_percent,omitempty"` // manual 模式：50 表示 +50%
}

func (c ConditionDef) validate() error {
	switch c.Dimension {
	case DimensionTokenUsage:
		if c.WindowDays <= 0 {
			return fmt.Errorf("token_usage: window_days must be > 0")
		}
		if c.Mode != "per_day" && c.Mode != "total" {
			return fmt.Errorf("token_usage: mode must be per_day or total")
		}
		if c.Threshold <= 0 {
			return fmt.Errorf("token_usage: threshold must be > 0")
		}
	case DimensionActiveDays:
		if c.WindowDays <= 0 {
			return fmt.Errorf("activity_days: window_days must be > 0")
		}
		if c.MinActiveDays <= 0 || c.MinActiveDays > c.WindowDays {
			return fmt.Errorf("activity_days: min_active_days must be within (0, window_days]")
		}
	case DimensionRegisteredDays:
		if c.MinRegisteredDays < 0 || c.MaxRegisteredDays < 0 {
			return fmt.Errorf("registered_days: bounds must be >= 0")
		}
		if c.MinRegisteredDays == 0 && c.MaxRegisteredDays == 0 {
			return fmt.Errorf("registered_days: at least one bound is required")
		}
		if c.MinRegisteredDays > 0 && c.MaxRegisteredDays > 0 && c.MinRegisteredDays > c.MaxRegisteredDays {
			return fmt.Errorf("registered_days: min_registered_days must be <= max_registered_days")
		}
	default:
		return fmt.Errorf("unknown dimension %q", c.Dimension)
	}
	switch c.BonusMode {
	case BonusNone, BonusManual, BonusAuto:
	default:
		return fmt.Errorf("unknown bonus_mode %q", c.BonusMode)
	}
	if c.BonusMode == BonusManual && (c.BonusPercent < 0 || c.BonusPercent > 1000) {
		return fmt.Errorf("manual bonus_percent must be within [0, 1000]")
	}
	return nil
}

// Activity 一场抽奖活动。
type Activity struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	StartsAt        time.Time `json:"starts_at"`        // 参与开始时间
	DrawsAt         time.Time `json:"draws_at"`         // 开奖时间
	MaxParticipants int64     `json:"max_participants"` // 0 = 不限
	// ShowParticipantCount 为 false 时，用户侧响应不带参与人数。
	ShowParticipantCount bool      `json:"show_participant_count"`
	ConditionMatch       string    `json:"condition_match"`    // all | any
	AutoBonusPercent     float64   `json:"auto_bonus_percent"` // auto 加成默认百分比
	Status               string    `json:"status"`             // active | archived（管理员手动停用）
	DrawnAt              time.Time `json:"drawn_at"`           // 已开奖时间；零值 = 未开奖
	// 来源：日常定时抽奖配置 ID（0 = 手动创建）。
	DailyConfigID int64 `json:"daily_config_id,omitempty"`
	// 用户侧可见性：全员可见，或仅 VisibleUsers 中的用户可见（管理员不受限）。
	VisibleToAll bool      `json:"visible_to_all"`
	VisibleUsers []int64   `json:"visible_users,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	Conditions []ConditionDef `json:"conditions"`
	Prizes     []Prize        `json:"prizes"`
}

func (a *Activity) Validate() error {
	if a.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !a.StartsAt.Before(a.DrawsAt) {
		return fmt.Errorf("starts_at must be before draws_at")
	}
	if a.MaxParticipants < 0 {
		return fmt.Errorf("max_participants must be >= 0")
	}
	if a.ConditionMatch != MatchAll && a.ConditionMatch != MatchAny {
		return fmt.Errorf("condition_match must be all or any")
	}
	// 无可参与条件 = 任何登录用户均可参与，不做限制。
	if len(a.Prizes) == 0 {
		return fmt.Errorf("at least one prize is required")
	}
	for i, c := range a.Conditions {
		if err := c.validate(); err != nil {
			return fmt.Errorf("condition[%d]: %w", i, err)
		}
	}
	seen := map[string]bool{}
	for i, p := range a.Prizes {
		if err := p.Validate(); err != nil {
			return fmt.Errorf("prize[%d]: %w", i, err)
		}
		if seen[p.Name] {
			return fmt.Errorf("prize[%d]: duplicate name %q", i, p.Name)
		}
		seen[p.Name] = true
	}
	return nil
}

// Phase 返回活动当前所处阶段（相对 now）。
func (a *Activity) Phase(now time.Time) string {
	if !a.DrawnAt.IsZero() {
		if a.allFulfilled() {
			return ActivityFulfilled
		}
		return ActivityDrawn
	}
	if now.Before(a.StartsAt) {
		return ActivityUpcoming
	}
	return ActivityJoining
}

func (a *Activity) allFulfilled() bool {
	for _, p := range a.Prizes {
		if p.GrantedCount < p.Stock {
			return false
		}
	}
	return len(a.Prizes) > 0
}

// Prize 奖池中的一项奖品。
type Prize struct {
	ID           int64   `json:"id"`
	ActivityID   int64   `json:"activity_id,omitempty"`
	Name         string  `json:"name"`
	PrizeType    string  `json:"prize_type"` // redeem_code | balance
	Value        float64 `json:"value"`      // 面额
	Weight       float64 `json:"weight"`     // 相对概率权重；0 表示未设置（随机均等）
	Stock        int64   `json:"stock"`
	GrantedCount int64   `json:"granted_count"`
	// Codes 预存的兑换码池（仅 redeem_code）：管理员录入，开奖时按序发放；
	// 为空时回退为调用主服务实时生成。
	Codes []string `json:"codes,omitempty"`
}

func (p Prize) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("name is required")
	}
	if p.PrizeType != PrizeRedeemCode && p.PrizeType != PrizeBalance {
		return fmt.Errorf("prize_type must be redeem_code or balance")
	}
	if p.Value <= 0 {
		return fmt.Errorf("value must be > 0")
	}
	if p.Weight < 0 {
		return fmt.Errorf("weight must be >= 0 (0 = equal chance)")
	}
	if p.Stock <= 0 {
		return fmt.Errorf("stock must be > 0")
	}
	return nil
}

// EffectiveWeight 未设权重按 1（随机均等）。
func (p Prize) EffectiveWeight() float64 {
	if p.Weight > 0 {
		return p.Weight
	}
	return 1
}

// Participant 一名参与者。
type Participant struct {
	ID         int64     `json:"id"`
	ActivityID int64     `json:"activity_id"`
	UserID     int64     `json:"user_id"`
	Email      string    `json:"email"`
	Weight     float64   `json:"weight"`
	JoinedAt   time.Time `json:"joined_at"`
}

// Winner 一条中奖记录。
type Winner struct {
	ID           int64     `json:"id"`
	ActivityID   int64     `json:"activity_id"`
	ActivityName string    `json:"activity_name,omitempty"`
	UserID       int64     `json:"user_id"`
	Email        string    `json:"email"` // 原始邮箱；对外输出一律走 MaskEmail
	PrizeID      int64     `json:"prize_id"`
	PrizeName    string    `json:"prize_name"`
	PrizeType    string    `json:"prize_type"`
	Value        float64   `json:"value"`
	Fulfillment  string    `json:"fulfillment"`
	RedeemCode   string    `json:"redeem_code,omitempty"`
	Note         string    `json:"note,omitempty"`
	Error        string    `json:"error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// DailyToken 某用户某天的总 token 消耗。
type DailyToken struct {
	Date        string  `json:"date"` // YYYY-MM-DD
	UserID      int64   `json:"user_id"`
	Email       string  `json:"email"`
	TotalTokens float64 `json:"total_tokens"`
}

// UserUsage 评估条件所需的用户用量视图。
type UserUsage struct {
	UserID int64
	Email  string
	// Daily 按日期升序的每日总 token。
	Daily []DailyToken
	// RegisteredAt 账号注册时间（来自主服务；未知为 nil，注册时长条件将不满足）。
	RegisteredAt *time.Time
}

// conditionJSON 序列化 ConditionDef 存库。
func conditionJSON(c ConditionDef) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func conditionFromJSON(s string) (ConditionDef, error) {
	var c ConditionDef
	if err := json.Unmarshal([]byte(s), &c); err != nil {
		return ConditionDef{}, err
	}
	return c, nil
}

// MaskEmail 邮箱脱敏：a***@domain.com；本地部分保留首字符，域保留完整。
func MaskEmail(email string) string {
	at := -1
	for i := 0; i < len(email); i++ {
		if email[i] == '@' {
			at = i
			break
		}
	}
	if at <= 0 {
		return "***"
	}
	local := email[:at]
	domain := email[at:]
	prefix := local[:1]
	// 多字节字符按 rune 截断，避免切出半个字符。
	r := []rune(local)
	if len(r) > 0 {
		prefix = string(r[0])
	}
	return prefix + "***" + domain
}

// MaskSecret 密钥脱敏：保留首尾各 4 位，过短全打码。
func MaskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:6] + "****" + s[len(s)-4:]
}

// 用户侧显隐模式。
const (
	VisibilityAll     = "all"     // 全员可见
	VisibilityPartial = "partial" // 部分人可见（按邮箱白名单）
)

// 重复参与策略（日常定时抽奖配置）。
const (
	RepeatUnlimited = "unlimited" // 每期独立（默认）
	RepeatJoinOnce  = "join_once" // 该系列限参与一次
	RepeatWinOnce   = "win_once"  // 该系列中奖后不能再参与
)

// Visibility 用户侧显隐配置。
type Visibility struct {
	Mode          string   `json:"mode"`           // all | partial
	AllowedEmails []string `json:"allowed_emails"` // partial 模式下的邮箱白名单
}

// DailyConfig 日常定时抽奖配置：调度器每天到达 StartTime 后自动创建一场活动。
type DailyConfig struct {
	ID      int64 `json:"id"`
	Enabled bool  `json:"enabled"`
	// 重复参与策略：unlimited 每期独立 / join_once 该系列限参与一次 / win_once 该系列中奖后不能再参与。
	RepeatPolicy string `json:"repeat_policy"`
	// SkipDate 手动关闭场次时记录的跳过日期（该日期不再自动补建）。
	SkipDate        string  `json:"skip_date,omitempty"`
	StartTime       string  `json:"start_time"`     // 每日开启时刻，"HH:MM"（北京时间）
	DurationHours   float64 `json:"duration_hours"` // 开奖时刻 = 开启 + 时长（小时）
	Name            string  `json:"name"`
	Description     string  `json:"description,omitempty"`
	MaxParticipants int64   `json:"max_participants"` // 0 = 不限
	// ShowParticipantCount 为 true 时，该配置生成的场次向用户下发参与人数。
	// 缺省（旧配置没有该字段）为 false：不显示。
	ShowParticipantCount bool           `json:"show_participant_count"`
	ConditionMatch       string         `json:"condition_match"` // all | any
	AutoBonusPercent     float64        `json:"auto_bonus_percent"`
	Conditions           []ConditionDef `json:"conditions"`
	Prizes               []PrizeSpec    `json:"prizes"`
}

// PrizeSpec 日常活动配置里的奖品（无 ID，创建活动时落库生成）。
type PrizeSpec struct {
	Name      string   `json:"name"`
	PrizeType string   `json:"prize_type"` // balance | redeem_code
	Value     float64  `json:"value"`
	Weight    float64  `json:"weight"`
	Stock     int64    `json:"stock"`
	Codes     []string `json:"codes,omitempty"`
}

// ShowsParticipants 报告该配置生成的场次是否向用户下发参与人数。
// 未写入该字段的旧配置按不显示处理。
func (c DailyConfig) ShowsParticipants() bool {
	return c.ShowParticipantCount
}

// Validate 校验日常配置。
func (c DailyConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Name == "" {
		return fmt.Errorf("daily: name is required")
	}
	if len(c.Prizes) == 0 {
		return fmt.Errorf("daily: at least one prize is required")
	}
	if c.DurationHours <= 0 {
		return fmt.Errorf("daily: duration_hours must be > 0")
	}
	for _, p := range c.Prizes {
		if p.Name == "" || p.Stock <= 0 {
			return fmt.Errorf("daily: prize name and stock are required")
		}
	}
	for _, cond := range c.Conditions {
		if err := cond.validate(); err != nil {
			return err
		}
	}
	switch c.RepeatPolicy {
	case "", RepeatUnlimited, RepeatJoinOnce, RepeatWinOnce:
	default:
		return fmt.Errorf("unknown repeat_policy %q", c.RepeatPolicy)
	}
	return nil
}

// MarshalJSON 保证 allowed_emails 始终输出数组（nil 切片输出 [] 而非 null），
// 避免前端模板对 null 读 .length 抛错。
func (v Visibility) MarshalJSON() ([]byte, error) {
	type alias struct {
		Mode          string   `json:"mode"`
		AllowedEmails []string `json:"allowed_emails"`
	}
	emails := v.AllowedEmails
	if emails == nil {
		emails = []string{}
	}
	return json.Marshal(alias{Mode: v.Mode, AllowedEmails: emails})
}

func (v Visibility) Validate() error {
	switch v.Mode {
	case VisibilityAll, VisibilityPartial:
	default:
		return fmt.Errorf("visibility mode must be all or partial")
	}
	return nil
}

// IsEmailAllowed 判断邮箱是否在白名单内（不区分大小写）。
func (v Visibility) IsEmailAllowed(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, allowed := range v.AllowedEmails {
		if strings.ToLower(strings.TrimSpace(allowed)) == email {
			return true
		}
	}
	return false
}
