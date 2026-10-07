// Package app wires the store, the Sub2API client and the lottery domain
// logic into the application services used by the HTTP layer and schedulers.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sort"
	"strconv"
	"time"

	"lotteryd/internal/lottery"
	"lotteryd/internal/store"
	"lotteryd/internal/sub2api"
)

const stateTokensLastDate = "tokens_last_date"
const stateAdminAPIKey = "admin_api_key"
const stateVisibility = "visibility_json"
const stateTaskVisibility = "task_visibility_json"
const stateDailyConfigs = "daily_activity_configs"
const stateDailyConfigLegacy = "daily_activity_config"

// AdminSettings 管理页「设置」弹窗的数据。
type AdminSettings struct {
	Sub2APIURL        string `json:"sub2api_url"`
	AdminAPIKeySet    bool   `json:"admin_api_key_set"`
	AdminAPIKeyMasked string `json:"admin_api_key_masked"`
}

// Config holds runtime-tunable behaviour.
type Config struct {
	// BackfillDays 在无水位时首次回溯多少天的用量（含今天）。
	BackfillDays int
}

// App is the application core.
type App struct {
	Store   *store.Store
	Sub2API *sub2api.Client
	Cfg     Config
	rng     *rand.Rand
}

// New creates the application core.
func New(st *store.Store, client *sub2api.Client, cfg Config) *App {
	if cfg.BackfillDays <= 0 {
		cfg.BackfillDays = 35
	}
	return &App{Store: st, Sub2API: client, Cfg: cfg, rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// ---- Views ----

// PrizeView 用户可见的奖品信息。
type PrizeView struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	PrizeType    string  `json:"prize_type"`
	Value        float64 `json:"value"`
	Weight       float64 `json:"weight"`
	Stock        int64   `json:"stock"`
	GrantedCount int64   `json:"granted_count"`
}

// ActivityView 用户/管理侧的活动视图。
type ActivityView struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	StartsAt        time.Time `json:"starts_at"`
	DrawsAt         time.Time `json:"draws_at"`
	MaxParticipants int64     `json:"max_participants"`
	// ParticipantCount 关闭“显示参与人数”时为 nil，用户侧 JSON 不包含该字段。
	ParticipantCount     *int64               `json:"participant_count,omitempty"`
	ShowParticipantCount bool                 `json:"show_participant_count"`
	ConditionMatch       string               `json:"condition_match"`
	Phase                string               `json:"phase"`
	Conditions           []lottery.EvalResult `json:"conditions,omitempty"`
	Eligible             bool                 `json:"eligible"`
	EligibleReason       string               `json:"eligible_reason,omitempty"`
	Weight               float64              `json:"weight,omitempty"`
	Joined               bool                 `json:"joined"`
	Won                  *lottery.Winner      `json:"won,omitempty"`
	Prizes               []PrizeView          `json:"prizes"`
	DrawnAt              *time.Time           `json:"drawn_at,omitempty"`
	Winners              []lottery.Winner     `json:"winners,omitempty"`
	Status               string               `json:"status"`
	CreatedAt            time.Time            `json:"created_at"`
	// 来源日常定时抽奖配置 ID（0 = 手动创建；仅管理端视图填充）。
	DailyConfigID int64                  `json:"daily_config_id,omitempty"`
	Extra         map[string]interface{} `json:"-"`
}

func prizeViews(prizes []lottery.Prize) []PrizeView {
	out := make([]PrizeView, 0, len(prizes))
	for _, p := range prizes {
		out = append(out, PrizeView{
			ID: p.ID, Name: p.Name, PrizeType: p.PrizeType,
			Value: p.Value, Weight: p.Weight, Stock: p.Stock, GrantedCount: p.GrantedCount,
		})
	}
	return out
}

func maskWinner(w lottery.Winner) lottery.Winner {
	w.Email = lottery.MaskEmail(w.Email)
	w.RedeemCode = "" // 兑换码只发给中奖者本人
	return w
}

func maxWindowDays(acts []lottery.Activity) int {
	maxDays := 1
	for _, a := range acts {
		for _, c := range a.Conditions {
			if c.WindowDays > maxDays {
				maxDays = c.WindowDays
			}
		}
	}
	return maxDays
}

func dateStr(t time.Time) string { return lottery.InBeijing(t).Format("2006-01-02") }

// registeredAt 解析用户注册时间：优先取缓存，其次用 identity 种子（introspect 带回的
// created_at），最后回退查主服务单用户接口。注册时间不可变，查到即缓存。
func (ap *App) registeredAt(ctx context.Context, userID int64, email string, seed *time.Time) *time.Time {
	if t, err := ap.Store.GetRegisteredAt(userID); err == nil && t != nil {
		return t
	}
	if seed != nil && !seed.IsZero() {
		_ = ap.Store.UpsertRegisteredAt(userID, email, *seed)
		return seed
	}
	if u, err := ap.Sub2API.GetUser(ctx, userID); err == nil && u.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, u.CreatedAt); err == nil {
			_ = ap.Store.UpsertRegisteredAt(userID, u.Email, t)
			return &t
		}
	}
	return nil
}

// applyRegistered 把注册时间写入 usage（map 里可能还没有该用户的条目）。
func applyRegistered(usages map[int64]*lottery.UserUsage, userID int64, email string, ra *time.Time) {
	if ra == nil {
		return
	}
	u := usages[userID]
	if u == nil {
		u = &lottery.UserUsage{UserID: userID, Email: email}
		usages[userID] = u
	}
	u.RegisteredAt = ra
}

// usageWindow 拉取评估窗口（含 maxDays 个完整天，终点为昨天）内的用量。
func (ap *App) usageWindow(ctx context.Context, maxDays int) (map[int64]*lottery.UserUsage, error) {
	end := time.Now().In(lottery.Beijing()).AddDate(0, 0, -1)
	start := end.AddDate(0, 0, -(maxDays - 1))
	return ap.Store.DailyTokensWindow(dateStr(start), dateStr(end))
}

// ---- User endpoints ----

// ListUserActivities 返回该用户可见的活动。
// 全局显隐（部分人可见）之外，未满足参与条件、且未参与/未中奖的场次不下发。
// 管理员不受显隐白名单和参与条件限制，用户侧能看到全部未归档场次。
func (ap *App) ListUserActivities(ctx context.Context, userID int64, email, role string, registeredAt *time.Time) ([]ActivityView, error) {
	acts, err := ap.Store.ListActivities()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	admin := role == "admin"
	visAllowed := admin || ap.IsUserAllowed("user", email)
	candidates := make([]*lottery.Activity, 0, len(acts))
	for i := range acts {
		a := &acts[i]
		if a.Status != "active" && a.DrawnAt.IsZero() {
			continue // 归档且未开奖的活动对用户不可见
		}
		// 部分人可见模式下，未在白名单的用户：任何活动（含已开奖）都不可见
		if !visAllowed {
			continue
		}
		candidates = append(candidates, a)
	}
	var usages map[int64]*lottery.UserUsage
	if len(candidates) > 0 {
		usages, err = ap.usageWindow(ctx, maxWindowDays(acts))
		if err != nil {
			return nil, err
		}
		applyRegistered(usages, userID, email, ap.registeredAt(ctx, userID, email, registeredAt))
	}
	out := make([]ActivityView, 0, len(candidates))
	for _, a := range candidates {
		v, err := ap.buildActivityView(ctx, a, userID, email, registeredAt, now, usages, false)
		if err != nil {
			return nil, err
		}
		if !admin && !userCanSeeActivity(v) {
			continue
		}
		out = append(out, *v)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DrawsAt.Before(out[j].DrawsAt) })
	return out, nil
}

// userCanSeeActivity 已参与或已中奖的场次始终可见；其余场次只有当前满足参与条件才可见。
func userCanSeeActivity(v *ActivityView) bool {
	if v.Joined || v.Won != nil {
		return true
	}
	return v.Eligible
}

// UserCanSeeActivity 用户侧中奖名单等单场接口：与活动列表同一套可见性。管理员始终可见。
func (ap *App) UserCanSeeActivity(ctx context.Context, activityID, userID int64, email, role string, registeredAt *time.Time) (bool, error) {
	a, err := ap.Store.GetActivity(activityID)
	if err != nil {
		return false, err
	}
	if role == "admin" {
		return true, nil
	}
	if a.Status != "active" && a.DrawnAt.IsZero() {
		return false, nil
	}
	if !ap.IsUserAllowed("user", email) {
		return false, nil
	}
	v, err := ap.buildActivityView(ctx, a, userID, email, registeredAt, time.Now(), nil, false)
	if err != nil {
		return false, err
	}
	return userCanSeeActivity(v), nil
}

func (ap *App) buildActivityView(ctx context.Context, a *lottery.Activity, userID int64, email string, registeredAt *time.Time, now time.Time, usages map[int64]*lottery.UserUsage, withWinners bool) (*ActivityView, error) {
	joined, err := ap.Store.HasParticipant(a.ID, userID)
	if err != nil {
		return nil, err
	}
	won, err := ap.Store.HasWinner(a.ID, userID)
	if err != nil {
		return nil, err
	}
	count, err := ap.Store.CountParticipants(a.ID)
	if err != nil {
		return nil, err
	}

	v := &ActivityView{
		ID: a.ID, Name: a.Name, Description: a.Description,
		StartsAt: a.StartsAt, DrawsAt: a.DrawsAt,
		MaxParticipants:      a.MaxParticipants,
		ShowParticipantCount: a.ShowParticipantCount,
		ConditionMatch:       a.ConditionMatch,
		Phase:                a.Phase(now),
		Joined:               joined,
		Prizes:               prizeViews(a.Prizes),
		Status:               a.Status,
		CreatedAt:            a.CreatedAt,
	}
	if a.ShowParticipantCount {
		n := count
		v.ParticipantCount = &n
	}
	if !a.DrawnAt.IsZero() {
		t := a.DrawnAt
		v.DrawnAt = &t
	}
	if won {
		ws, err := ap.Store.ListWinnersByUser(userID)
		if err != nil {
			return nil, err
		}
		for i := range ws {
			if ws[i].ActivityID == a.ID {
				w := ws[i]
				w.Email = lottery.MaskEmail(w.Email)
				v.Won = &w
				break
			}
		}
	}
	// 未参与的场次都评估资格：不符合条件的场次对用户隐藏，角标也不计入。
	if !joined && !won && phaseNeedsEligibility(v.Phase) {
		if usages == nil {
			var loadErr error
			usages, loadErr = ap.usageWindow(ctx, maxWindowDays([]lottery.Activity{*a}))
			if loadErr != nil {
				return nil, loadErr
			}
			applyRegistered(usages, userID, email, ap.registeredAt(ctx, userID, email, registeredAt))
		}
		el := a.Evaluate(usages[userID], now)
		v.Eligible = el.Eligible
		v.EligibleReason = el.Reason
		v.Weight = el.Weight
		v.Conditions = el.Results
		// 重复参与策略：系列内已参与/已中奖 → 显示为不可参与
		if v.Eligible {
			if blocked, reason := ap.repeatPolicyBlocked(a, userID); blocked {
				v.Eligible = false
				v.EligibleReason = reason
			}
		}
		// 用户侧不展示加成/权重（运营工具内部概念）。
		v.Weight = 1
		for i := range v.Conditions {
			v.Conditions[i].BonusPercent = 0
		}
		if !el.Eligible && !v.Joined {
			v.EligibleReason = el.Reason
		}
	}
	if withWinners && !a.DrawnAt.IsZero() {
		ws, err := ap.Store.ListWinnersByActivity(a.ID)
		if err != nil {
			return nil, err
		}
		v.Winners = make([]lottery.Winner, 0, len(ws))
		for _, w := range ws {
			v.Winners = append(v.Winners, maskWinner(w))
		}
	}
	return v, nil
}

// EligibilityList 返回参与中、符合条件且未参与的活动（供全局引导弹窗）。
func (ap *App) EligibilityList(ctx context.Context, userID int64, email string, registeredAt *time.Time) ([]ActivityView, error) {
	acts, err := ap.Store.ListActivities()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	usages, err := ap.usageWindow(ctx, maxWindowDays(acts))
	if err != nil {
		return nil, err
	}
	applyRegistered(usages, userID, email, ap.registeredAt(ctx, userID, email, registeredAt))
	visAllowed := ap.IsUserAllowed("user", email) // 引导弹窗同样遵守用户侧显隐
	out := make([]ActivityView, 0)
	for i := range acts {
		a := &acts[i]
		if a.Status != "active" || !a.DrawnAt.IsZero() || !now.After(a.StartsAt) {
			continue
		}
		if !visAllowed {
			continue
		}
		joined, err := ap.Store.HasParticipant(a.ID, userID)
		if err != nil {
			return nil, err
		}
		if joined {
			continue
		}
		if a.MaxParticipants > 0 {
			count, err := ap.Store.CountParticipants(a.ID)
			if err != nil {
				return nil, err
			}
			if count >= a.MaxParticipants {
				continue
			}
		}
		el := a.Evaluate(usages[userID], now)
		if !el.Eligible {
			continue
		}
		// 重复参与策略：被排除的用户不弹引导窗
		if blocked, _ := ap.repeatPolicyBlocked(a, userID); blocked {
			continue
		}
		// 用户侧不展示加成/权重（运营工具内部概念）。
		for i := range el.Results {
			el.Results[i].BonusPercent = 0
		}
		out = append(out, ActivityView{
			ID: a.ID, Name: a.Name, Description: a.Description,
			StartsAt: a.StartsAt, DrawsAt: a.DrawsAt,
			ConditionMatch: a.ConditionMatch,
			Phase:          a.Phase(now),
			Eligible:       true,
			Weight:         1,
			Conditions:     el.Results,
			Prizes:         prizeViews(a.Prizes),
		})
	}
	return out, nil
}

// Participate 在校验窗口、资格、容量后把用户登记为参与者。
// identity 由 HTTP 层通过 /auth/me 复核后传入。
func (ap *App) Participate(ctx context.Context, activityID, userID int64, identity *sub2api.Identity, now time.Time) (*ActivityView, error) {
	a, err := ap.Store.GetActivity(activityID)
	if err != nil {
		return nil, err
	}
	if a.Status != "active" {
		return nil, store.ErrNotJoinable
	}
	if !a.DrawnAt.IsZero() {
		return nil, store.ErrNotJoinable
	}
	if now.Before(a.StartsAt) {
		return nil, store.ErrNotJoinable
	}
	if !now.Before(a.DrawsAt) {
		return nil, store.ErrNotJoinable
	}
	// 用户侧显隐：部分人可见模式下，白名单外的用户（管理员除外）不可参与
	if !ap.IsUserAllowed(identity.Role, identity.Email) {
		return nil, store.ErrNotVisible
	}
	joined, err := ap.Store.HasParticipant(a.ID, userID)
	if err != nil {
		return nil, err
	}
	if joined {
		return nil, store.ErrAlreadyJoined
	}
	won, err := ap.Store.HasWinner(a.ID, userID)
	if err != nil {
		return nil, err
	}
	if won {
		return nil, store.ErrAlreadyJoined
	}
	if a.MaxParticipants > 0 {
		count, err := ap.Store.CountParticipants(a.ID)
		if err != nil {
			return nil, err
		}
		if count >= a.MaxParticipants {
			return nil, store.ErrActivityFull
		}
	}
	usages, err := ap.usageWindow(ctx, maxWindowDays([]lottery.Activity{*a}))
	if err != nil {
		return nil, err
	}
	applyRegistered(usages, userID, identity.Email, ap.registeredAt(ctx, userID, identity.Email, identity.RegisteredAt))
	el := a.Evaluate(usages[userID], now)
	if !el.Eligible {
		return nil, store.ErrNotEligible
	}
	// 重复参与策略（日常定时抽奖系列）
	if derr := ap.repeatPolicyDeniedErr(a, userID); derr != nil {
		return nil, derr
	}
	if err := ap.Store.AddParticipant(a.ID, userID, identity.Email, el.Weight, a.MaxParticipants); err != nil {
		return nil, err
	}
	return ap.buildActivityView(ctx, a, userID, identity.Email, identity.RegisteredAt, now, nil, false)
}

// phaseNeedsEligibility 这些阶段需要知道用户当前是否满足参与条件。
func phaseNeedsEligibility(phase string) bool {
	switch phase {
	case lottery.ActivityJoining, lottery.ActivityUpcoming, lottery.ActivityDrawn, lottery.ActivityFulfilled:
		return true
	default:
		return false
	}
}

// MyWinnings 用户的中奖记录（兑换码仅本人可见）。
func (ap *App) MyWinnings(userID int64) ([]lottery.Winner, error) {
	ws, err := ap.Store.ListWinnersByUser(userID)
	if err != nil {
		return nil, err
	}
	if ws == nil {
		ws = make([]lottery.Winner, 0)
	}
	for i := range ws {
		ws[i].Email = lottery.MaskEmail(ws[i].Email)
	}
	return ws, nil
}

// ---- Admin ----

// AdminListActivities 管理端活动列表（含各活动参与者与中奖名单摘要）。
func (ap *App) AdminListActivities(ctx context.Context) ([]ActivityView, error) {
	acts, err := ap.Store.ListActivities()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]ActivityView, 0, len(acts))
	for i := range acts {
		a := &acts[i]
		count, err := ap.Store.CountParticipants(a.ID)
		if err != nil {
			return nil, err
		}
		n := count
		v := &ActivityView{
			ID: a.ID, Name: a.Name, Description: a.Description,
			StartsAt: a.StartsAt, DrawsAt: a.DrawsAt,
			MaxParticipants:      a.MaxParticipants,
			ParticipantCount:     &n,
			ShowParticipantCount: a.ShowParticipantCount,
			ConditionMatch:       a.ConditionMatch,
			Phase:                a.Phase(now),
			Prizes:               prizeViews(a.Prizes),
			Status:               a.Status,
			CreatedAt:            a.CreatedAt,
			DailyConfigID:        a.DailyConfigID,
		}
		if !a.DrawnAt.IsZero() {
			t := a.DrawnAt
			v.DrawnAt = &t
		}
		out = append(out, *v)
	}
	return out, nil
}

// AdminWinners 管理端中奖名单（邮箱脱敏）。
func (ap *App) AdminWinners(activityID int64) ([]lottery.Winner, error) {
	// 管理端（运营工具）：保留完整邮箱，便于核对与发放排查。
	return ap.Store.ListWinnersByActivity(activityID)
}

// AdminParticipants 管理端参与者名单（完整邮箱，供运营核对）。
func (ap *App) AdminParticipants(activityID int64) ([]lottery.Participant, error) {
	return ap.Store.ListParticipants(activityID)
}

// LoadRuntimeSettings 启动时把持久化的运行时设置（如管理员 API Key）应用到客户端。
func (ap *App) LoadRuntimeSettings() {
	if key, err := ap.Store.GetState(stateAdminAPIKey); err == nil && key != "" {
		ap.Sub2API.SetAdminAPIKey(key)
	}
}

// GetAdminSettings 返回设置弹窗所需信息（密钥脱敏）。
func (ap *App) GetAdminSettings() AdminSettings {
	key := ap.Sub2API.AdminAPIKey()
	masked := lottery.MaskSecret(key)
	return AdminSettings{
		Sub2APIURL:        ap.Sub2API.BaseURL(),
		AdminAPIKeySet:    key != "",
		AdminAPIKeyMasked: masked,
	}
}

// SetAdminAPIKey 保存并立即应用新的管理员 API Key（无需重启）。
func (ap *App) SetAdminAPIKey(key string) error {
	if err := ap.Store.SetState(stateAdminAPIKey, key); err != nil {
		return err
	}
	ap.Sub2API.SetAdminAPIKey(key)
	return nil
}

// ---- Visibility ----

// GetVisibility 读取抽奖显隐配置（未配置 = 部分人可见，白名单为空即仅管理员可见）。
func (ap *App) GetVisibility() lottery.Visibility {
	return ap.getVisibilityState(stateVisibility)
}

// SetVisibility 保存用户侧显隐配置。
func (ap *App) SetVisibility(v lottery.Visibility) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return ap.Store.SetState(stateVisibility, string(raw))
}

// GetTaskVisibility 读取任务中心显隐配置（未配置 = 部分人可见）。
func (ap *App) GetTaskVisibility() lottery.Visibility {
	return ap.getVisibilityState(stateTaskVisibility)
}

// SetTaskVisibility 保存任务中心显隐配置。
func (ap *App) SetTaskVisibility(v lottery.Visibility) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return ap.Store.SetState(stateTaskVisibility, string(raw))
}

// getVisibilityState 读取显隐配置；未配置/解析失败 = 部分人可见（白名单外不可见）。
func (ap *App) getVisibilityState(key string) lottery.Visibility {
	raw, err := ap.Store.GetState(key)
	if err != nil || raw == "" {
		return lottery.Visibility{Mode: lottery.VisibilityPartial}
	}
	var v lottery.Visibility
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return lottery.Visibility{Mode: lottery.VisibilityPartial}
	}
	return v
}

// IsUserAllowed 抽奖对该邮箱是否可见（管理员不受限）。
func (ap *App) IsUserAllowed(role, email string) bool {
	return isAllowedBy(ap.GetVisibility(), role, email)
}

// IsUserAllowedTask 任务中心对该邮箱是否可见（管理员不受限）。
func (ap *App) IsUserAllowedTask(role, email string) bool {
	return isAllowedBy(ap.GetTaskVisibility(), role, email)
}

func isAllowedBy(v lottery.Visibility, role, email string) bool {
	if role == "admin" {
		return true
	}
	if v.Mode == lottery.VisibilityPartial && !v.IsEmailAllowed(email) {
		return false
	}
	return true
}

// ---- Daily activity ----

// repeatPolicyBlocked 按活动所属日常配置的重复参与策略判断用户是否被排除。
func (ap *App) repeatPolicyBlocked(a *lottery.Activity, userID int64) (bool, string) {
	if a.DailyConfigID <= 0 {
		return false, ""
	}
	var policy string
	for _, dc := range ap.GetDailyConfigs() {
		if dc.ID == a.DailyConfigID {
			policy = dc.RepeatPolicy
			break
		}
	}
	switch policy {
	case lottery.RepeatJoinOnce:
		if joined, err := ap.Store.HasUserParticipatedInConfig(a.DailyConfigID, userID); err == nil && joined {
			return true, "您已参与过该系列抽奖"
		}
	case lottery.RepeatWinOnce:
		if won, err := ap.Store.HasUserWonInConfig(a.DailyConfigID, userID); err == nil && won {
			return true, "您已在该系列抽奖中中奖"
		}
	}
	return false, ""
}

// repeatPolicyDeniedErr 按活动所属日常配置的重复参与策略判断用户是否被排除，
// 被排除时返回对应哨兵错误（HTTP 层映射为 403 中文提示）。
func (ap *App) repeatPolicyDeniedErr(a *lottery.Activity, userID int64) error {
	blocked, reason := ap.repeatPolicyBlocked(a, userID)
	if !blocked {
		return nil
	}
	if reason == "您已在该系列抽奖中中奖" {
		return store.ErrRepeatWin
	}
	return store.ErrRepeatJoin
}

// ---- Daily activity（多配置：每个配置独立启停/删除） ----

// GetDailyConfigs 读取全部日常定时抽奖配置（首次读取时自动迁移旧的单配置数据）。
func (ap *App) GetDailyConfigs() []lottery.DailyConfig {
	raw, err := ap.Store.GetState(stateDailyConfigs)
	if err == nil && raw != "" {
		var list []lottery.DailyConfig
		if err := json.Unmarshal([]byte(raw), &list); err == nil {
			return list
		}
	}
	// 新键为空：迁移旧版单配置（stateDailyConfig 键，对象而非数组）
	if legacy, lerr := ap.Store.GetState(stateDailyConfigLegacy); lerr == nil && legacy != "" {
		var single lottery.DailyConfig
		if err := json.Unmarshal([]byte(legacy), &single); err == nil && single.Name != "" {
			single.ID = 1
			list := []lottery.DailyConfig{single}
			if raw, err := json.Marshal(list); err == nil {
				_ = ap.Store.SetState(stateDailyConfigs, string(raw))
			}
			return list
		}
	}
	return []lottery.DailyConfig{}
}

// SaveDailyConfig 新建（ID=0）或更新日常定时抽奖配置，返回带 ID 的配置。
func (ap *App) SaveDailyConfig(c lottery.DailyConfig) (lottery.DailyConfig, error) {
	list := ap.GetDailyConfigs()
	if c.ID > 0 {
		found := false
		for i := range list {
			if list[i].ID == c.ID {
				list[i] = c
				found = true
				break
			}
		}
		if !found {
			return c, store.ErrNotFound
		}
	} else {
		next := int64(1)
		for _, item := range list {
			if item.ID >= next {
				next = item.ID + 1
			}
		}
		c.ID = next
		list = append(list, c)
	}
	raw, err := json.Marshal(list)
	if err != nil {
		return c, err
	}
	if err := ap.Store.SetState(stateDailyConfigs, string(raw)); err != nil {
		return c, err
	}
	return c, nil
}

// SetDailyConfigEnabled 启用/停用单个配置（保留其余字段）。
// 启用时立即预约下一场次（不等调度器 tick），前端刷新即可见。
func (ap *App) SetDailyConfigEnabled(id int64, enabled bool) (lottery.DailyConfig, error) {
	list := ap.GetDailyConfigs()
	for i := range list {
		if list[i].ID != id {
			continue
		}
		list[i].Enabled = enabled
		raw, err := json.Marshal(list)
		if err != nil {
			return list[i], err
		}
		if err := ap.Store.SetState(stateDailyConfigs, string(raw)); err != nil {
			return list[i], err
		}
		if enabled {
			// 重新启用：清空日期推进记录，从当前时刻重新计算下一场（不往后顺延）
			ap.clearDailyRoundKeys(id, time.Now())
			if err := ap.ensureNextDailyRound(list[i], time.Now()); err != nil {
				slog.Warn("daily round ensure failed on enable", "config", id, "err", err)
			}
		}
		return list[i], nil
	}
	return lottery.DailyConfig{}, store.ErrNotFound
}

// DeleteDailyConfig 删除单个配置（已创建的场次不受影响）。
func (ap *App) DeleteDailyConfig(id int64) error {
	list := ap.GetDailyConfigs()
	out := list[:0]
	for _, item := range list {
		if item.ID != id {
			out = append(out, item)
		}
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return ap.Store.SetState(stateDailyConfigs, string(raw))
}

// UserLotteryPhase 侧边栏角标：只统计该用户可见的场次（可参与，或已参与/已中奖）。
// 管理员统计全部场次。优先级 joining > upcoming > drawn。没有可见场次时返回空串。
func (ap *App) UserLotteryPhase(ctx context.Context, userID int64, email, role string, registeredAt *time.Time) string {
	views, err := ap.ListUserActivities(ctx, userID, email, role, registeredAt)
	if err != nil || len(views) == 0 {
		return ""
	}
	joining, upcoming, drawn := false, false, false
	for i := range views {
		switch views[i].Phase {
		case lottery.ActivityJoining:
			joining = true
		case lottery.ActivityUpcoming:
			upcoming = true
		case lottery.ActivityDrawn, lottery.ActivityFulfilled:
			drawn = true
		}
	}
	switch {
	case joining:
		return lottery.ActivityJoining
	case upcoming:
		return lottery.ActivityUpcoming
	case drawn:
		return lottery.ActivityDrawn
	}
	return ""
}

// CurrentLotteryPhase 全部进行中场次的全局状态（joining/upcoming/drawn，无活动返回空串）。
// 用户侧角标用 UserLotteryPhase，不把该用户不可参与的场次算进去。
func (ap *App) CurrentLotteryPhase() string {
	acts, err := ap.Store.ListActivities()
	if err != nil {
		return ""
	}
	now := time.Now()
	joining, upcoming, drawn := false, false, false
	for i := range acts {
		a := &acts[i]
		if a.Status != "active" {
			continue
		}
		switch {
		case !a.DrawnAt.IsZero():
			drawn = true
		case now.Before(a.StartsAt):
			upcoming = true
		default:
			joining = true
		}
	}
	switch {
	case joining:
		return "joining"
	case upcoming:
		return "upcoming"
	case drawn:
		return "drawn"
	}
	return ""
}

// clearDailyRoundKeys 清空配置的全部日期推进记录（启用时重置计算起点用）。
func (ap *App) clearDailyRoundKeys(cfgID int64, now time.Time) {
	now = now.In(lottery.Beijing())
	for i := -30; i <= 30; i++ {
		key := fmt.Sprintf("daily_created:%d:%s", cfgID, now.AddDate(0, 0, i).Format("2006-01-02"))
		if v, err := ap.Store.GetState(key); err == nil && v != "" {
			_ = ap.Store.SetState(key, "")
		}
	}
}

// MarkDailySkip 若被关闭的活动来自某个日常配置，记录该日期为跳过（不再自动补建当天场次）。
func (ap *App) MarkDailySkip(activityID int64) {
	a, err := ap.Store.GetActivity(activityID)
	if err != nil || a.DailyConfigID <= 0 {
		return
	}
	list := ap.GetDailyConfigs()
	for i := range list {
		if list[i].ID != a.DailyConfigID {
			continue
		}
		list[i].SkipDate = lottery.InBeijing(a.StartsAt).Format("2006-01-02")
		if raw, merr := json.Marshal(list); merr == nil {
			_ = ap.Store.SetState(stateDailyConfigs, string(raw))
		}
		return
	}
}

// CreateDueDailyActivity 每个调度 tick 调用：保证每个启用的配置始终存在一场
// 未开奖的场次——当前场次开奖后立即创建下一次（用户因此总能看到即将到来的活动）。
func (ap *App) CreateDueDailyActivity(now time.Time) error {
	var firstErr error
	for _, cfg := range ap.GetDailyConfigs() {
		if !cfg.Enabled {
			continue
		}
		if err := ap.ensureNextDailyRound(cfg, now); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ensureNextDailyRound 保证该配置始终有一场未开奖的场次：
//  1. 收集该配置创建的场次（近 7 天 + 未来 2 天的记录）：存在进行中或未开始的（active 且未开奖）→ 已预约，不动；
//  2. 否则以「最后创建日期 + 1 天」为下一场日期（手动关闭/开奖的场次视为已跳过那天），
//     无任何记录时以 start_time 的下一次出现为准；
//  3. 若该场开启时刻已过（如隔天 tick），顺延一天。
func (ap *App) ensureNextDailyRound(cfg lottery.DailyConfig, now time.Time) error {
	now = now.In(lottery.Beijing())
	st, err := time.ParseInLocation("15:04", cfg.StartTime, lottery.Beijing())
	if err != nil {
		return fmt.Errorf("daily config %d start_time: %w", cfg.ID, err)
	}
	keyFor := func(t time.Time) string {
		return fmt.Sprintf("daily_created:%d:%s", cfg.ID, t.Format("2006-01-02"))
	}
	// 1) 收集该配置创建过的场次
	ids := map[int64]bool{}
	lastKeyDate := time.Time{}
	for i := -30; i <= 30; i++ {
		t := now.AddDate(0, 0, -i)
		if idStr, err := ap.Store.GetState(keyFor(t)); err == nil && idStr != "" {
			if id, perr := strconv.ParseInt(idStr, 10, 64); perr == nil {
				ids[id] = true
			}
			if lastKeyDate.IsZero() || t.After(lastKeyDate) {
				lastKeyDate = t
			}
		}
	}
	// 2) 存在进行中/未开始的关联场次 → 已预约
	for id := range ids {
		if a, err := ap.Store.GetActivity(id); err == nil && a.Status == "active" && a.DrawnAt.IsZero() {
			return nil
		}
	}
	// 3) 下一场日期
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), st.Hour(), st.Minute(), 0, 0, lottery.Beijing())
	var base time.Time
	if lastKeyDate.IsZero() {
		base = now
		if !now.Before(todayStart) {
			base = now.AddDate(0, 0, 1)
		}
	} else {
		base = lastKeyDate.AddDate(0, 0, 1)
	}
	start := time.Date(base.Year(), base.Month(), base.Day(), st.Hour(), st.Minute(), 0, 0, lottery.Beijing())
	// 场次开启时刻已过（隔天 tick 等场景）→ 顺延一天，避免创建即开奖
	if !start.After(now) {
		start = start.AddDate(0, 0, 1)
		base = start
	}
	// 4) 该日期已有在用场次 → 不重复（已开奖/已关闭/已归档的允许覆盖重建）
	if prev, err := ap.Store.GetState(keyFor(start)); err == nil && prev != "" {
		if pid, perr := strconv.ParseInt(prev, 10, 64); perr == nil {
			if a, gerr := ap.Store.GetActivity(pid); gerr == nil && a.Status == "active" {
				return nil
			}
		}
	}
	id, err := ap.Store.CreateActivity(dailyActivityFromConfig(cfg, start))
	if err != nil {
		return err
	}
	slog.Info("daily activity created", "id", id, "config", cfg.ID, "name", cfg.Name, "start", start.Format(time.RFC3339))
	return ap.Store.SetState(keyFor(start), strconv.FormatInt(id, 10))
}

// dailyActivityFromConfig 按配置构建一场活动（开启时刻由调用方决定）。
func dailyActivityFromConfig(cfg lottery.DailyConfig, start time.Time) *lottery.Activity {
	a := &lottery.Activity{
		Name:                 cfg.Name,
		Description:          cfg.Description,
		StartsAt:             start,
		DrawsAt:              start.Add(time.Duration(cfg.DurationHours * float64(time.Hour))),
		MaxParticipants:      cfg.MaxParticipants,
		ShowParticipantCount: cfg.ShowsParticipants(),
		ConditionMatch:       cfg.ConditionMatch,
		AutoBonusPercent:     cfg.AutoBonusPercent,
		Conditions:           cfg.Conditions,
		DailyConfigID:        cfg.ID,
	}
	if a.ConditionMatch == "" {
		a.ConditionMatch = lottery.MatchAll
	}
	for _, p := range cfg.Prizes {
		a.Prizes = append(a.Prizes, lottery.Prize{
			Name: p.Name, PrizeType: p.PrizeType, Value: p.Value, Weight: p.Weight, Stock: p.Stock,
			Codes: p.Codes,
		})
	}
	return a
}

// ErrRegenerateBlocked 当前场次已有用户参与，不能重新生成。
var ErrRegenerateBlocked = errors.New("当前场次已有用户参与，不能重新生成；请先「关闭」该场次或改日操作")

// RegenerateDailyRound 保存并重建：归档该配置当前未开奖的场次（有用户参与时拒绝），
// 清空日期预约记录，并按最新配置立即创建今天的新场次（开启时刻已过则立即开启）。
func (ap *App) RegenerateDailyRound(cfgID int64, now time.Time) error {
	var cfg *lottery.DailyConfig
	list := ap.GetDailyConfigs()
	for i := range list {
		if list[i].ID == cfgID {
			cfg = &list[i]
			break
		}
	}
	if cfg == nil {
		return store.ErrNotFound
	}
	acts, err := ap.Store.ListActivities()
	if err != nil {
		return err
	}
	for _, a := range acts {
		if a.DailyConfigID != cfgID || a.Status != "active" || !a.DrawnAt.IsZero() {
			continue
		}
		n, err := ap.Store.CountParticipants(a.ID)
		if err != nil {
			return err
		}
		if n > 0 {
			return ErrRegenerateBlocked
		}
		if err := ap.Store.SetActivityStatus(a.ID, "archived"); err != nil {
			return err
		}
		slog.Info("daily round archived for regeneration", "config", cfgID, "activity", a.ID)
	}
	ap.clearDailyRoundKeys(cfgID, now)

	now = now.In(lottery.Beijing())
	st, err := time.ParseInLocation("15:04", cfg.StartTime, lottery.Beijing())
	if err != nil {
		return fmt.Errorf("daily config %d start_time: %w", cfg.ID, err)
	}
	start := time.Date(now.Year(), now.Month(), now.Day(), st.Hour(), st.Minute(), 0, 0, lottery.Beijing())
	if !start.After(now) {
		start = now // 今日开启时刻已过：立即开启
	}
	a := dailyActivityFromConfig(*cfg, start)
	if err := a.Validate(); err != nil {
		return fmt.Errorf("daily activity invalid: %w", err)
	}
	id, err := ap.Store.CreateActivity(a)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("daily_created:%d:%s", cfg.ID, start.Format("2006-01-02"))
	if err := ap.Store.SetState(key, strconv.FormatInt(id, 10)); err != nil {
		return err
	}
	slog.Info("daily round regenerated", "config", cfgID, "activity", id, "start", start.Format(time.RFC3339))
	return nil
}

// ---- Daily sync ----

// SyncTokens 把水位之后（含今天）的每用户每日 token 总量拉回本地快照。
// 幂等：按天 upsert，宕机后从水位续拉。
func (ap *App) SyncTokens(ctx context.Context, now time.Time) error {
	now = now.In(lottery.Beijing())
	last, err := ap.Store.GetState(stateTokensLastDate)
	if err != nil {
		return err
	}
	today := dateStr(now)
	var start time.Time
	if last == "" {
		start = now.AddDate(0, 0, -(ap.Cfg.BackfillDays - 1))
	} else if last >= today {
		return nil // 已同步到今天
	} else {
		t, err := time.ParseInLocation("2006-01-02", last, lottery.Beijing())
		if err != nil {
			start = now.AddDate(0, 0, -(ap.Cfg.BackfillDays - 1))
		} else {
			start = t.AddDate(0, 0, 1)
		}
	}
	for d := start; dateStr(d) <= today; d = d.AddDate(0, 0, 1) {
		date := dateStr(d)
		tokens, err := ap.Sub2API.FetchDailyTokens(ctx, date)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", date, err)
		}
		if err := ap.Store.UpsertDailyTokens(date, tokens); err != nil {
			return fmt.Errorf("upsert %s: %w", date, err)
		}
		if err := ap.Store.SetState(stateTokensLastDate, date); err != nil {
			return err
		}
		slog.Info("lotteryd sync day done", "date", date, "users", len(tokens))
	}
	return nil
}

// ---- Draw & fulfillment ----

// DrawDueActivities 对所有到点未开奖的活动执行开奖并发放。
func (ap *App) DrawDueActivities(ctx context.Context, now time.Time) error {
	acts, err := ap.Store.ListDrawableActivities(now)
	if err != nil {
		return err
	}
	for i := range acts {
		if err := ap.DrawActivity(ctx, &acts[i]); err != nil {
			slog.Error("lotteryd draw failed", "activity", acts[i].ID, "err", err)
		}
	}
	return nil
}

// DrawActivity 对单个活动开奖并登记中奖者（发放随后执行）。
// 兑换码奖若配置了预存码池，则在开奖时按序分配。
func (ap *App) DrawActivity(ctx context.Context, a *lottery.Activity) error {
	participants, err := ap.Store.ListParticipants(a.ID)
	if err != nil {
		return err
	}
	drafts := lottery.DrawPlan(a, participants, ap.rng)
	// 预存兑换码分配游标：prizeID → 下一个可用的 codes 下标（从已发放数起算）。
	codeCursor := map[int64]int{}
	codesOf := map[int64][]string{}
	for _, p := range a.Prizes {
		codeCursor[p.ID] = int(p.GrantedCount)
		codesOf[p.ID] = p.Codes
	}
	for i := range drafts {
		d := &drafts[i]
		w := &lottery.Winner{
			ActivityID: a.ID, UserID: d.UserID, Email: d.Email,
			PrizeID: d.Prize.ID, PrizeName: d.Prize.Name, PrizeType: d.Prize.PrizeType,
			Value: d.Prize.Value, Fulfillment: lottery.FulfillmentPending, CreatedAt: d.CreatedAt,
		}
		// 从预存码池按序取码；池空时 FulfillPending 回退为主服务实时生成。
		if d.Prize.PrizeType == lottery.PrizeRedeemCode {
			idx := codeCursor[d.Prize.ID]
			codeCursor[d.Prize.ID] = idx + 1
			if codes := codesOf[d.Prize.ID]; idx < len(codes) {
				w.RedeemCode = codes[idx]
			}
		}
		if err := ap.Store.InsertWinner(w); err != nil {
			return err
		}
	}
	if err := ap.Store.MarkDrawn(a.ID, time.Now().UTC()); err != nil {
		return err
	}
	slog.Info("lotteryd drawn", "activity", a.ID, "participants", len(participants), "winners", len(drafts))
	return ap.FulfillPending(ctx, a)
}

// FulfillPending 对活动中 pending/failed 的中奖记录执行发放：
// 兑换码奖 → 生成兑换码；余额奖 → 调接口入账并备注「抽奖名称+金额」。
func (ap *App) FulfillPending(ctx context.Context, a *lottery.Activity) error {
	ws, err := ap.Store.ListUnfulfilledWinners(a.ID)
	if err != nil {
		return err
	}
	for i := range ws {
		w := &ws[i]
		var code, note, errMsg string
		switch w.PrizeType {
		case lottery.PrizeBalance:
			note = fmt.Sprintf("抽奖活动: %s +%s", a.Name, lottery.FormatValue(w.Value))
			err = ap.Sub2API.AddBalance(ctx, w.UserID, w.Value, note)
		case lottery.PrizeRedeemCode:
			if w.RedeemCode != "" {
				// 开奖时已从管理员预存的码池分配，无需调用主服务。
				code = w.RedeemCode
			} else {
				code, err = ap.Sub2API.GenerateRedeemCode(ctx, w.Value)
			}
		default:
			err = fmt.Errorf("unknown prize type %q", w.PrizeType)
		}
		if err != nil {
			errMsg = err.Error()
			slog.Error("lotteryd fulfill failed", "winner", w.ID, "err", errMsg)
			if uerr := ap.Store.UpdateWinnerFulfillment(w.ID, lottery.FulfillmentFailed, code, note, errMsg); uerr != nil {
				return uerr
			}
			continue
		}
		if err := ap.Store.UpdateWinnerFulfillment(w.ID, lottery.FulfillmentDone, code, note, ""); err != nil {
			return err
		}
		slog.Info("lotteryd fulfilled", "winner", w.ID, "type", w.PrizeType)
	}
	return nil
}

// RetryFulfillment 管理端手动重试某活动的失败发放。
func (ap *App) RetryFulfillment(ctx context.Context, activityID int64) error {
	a, err := ap.Store.GetActivity(activityID)
	if err != nil {
		return err
	}
	return ap.FulfillPending(ctx, a)
}
