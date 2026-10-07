package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"lotteryd/internal/lottery"
	"lotteryd/internal/store"
	"lotteryd/internal/sub2api"
)

const stateTaskSettledPrefix = "task_settled:"

// ---- 视图 ----

// AdminTaskView 管理端任务视图：任务 + 码池统计。
type AdminTaskView struct {
	lottery.Task
	CodesAvailable int64 `json:"codes_available"`
	CodesGranted   int64 `json:"codes_granted"`
}

// UserTaskView 用户侧任务视图：不暴露白/黑名单、条件明细中的运营字段。
type UserTaskView struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	Cover           string  `json:"cover,omitempty"`
	Description     string  `json:"description,omitempty"`
	Status          string  `json:"status"`
	GroupName       string  `json:"group_name,omitempty"`
	Model           string  `json:"model,omitempty"`
	StartDate       string  `json:"start_date"`
	EndDate         string  `json:"end_date"`
	DaysLeft        int     `json:"days_left"`
	SettleTime      string  `json:"settle_time"`
	ThresholdTokens float64 `json:"threshold_tokens"`
	RewardType      string  `json:"reward_type"`
	RewardValue     float64 `json:"reward_value"`
}

// ---- 管理端 CRUD ----

// CreateTask 新建任务（默认 active）。
func (ap *App) CreateTask(t *lottery.Task) (int64, error) {
	if t.Status == "" {
		t.Status = lottery.TaskActive
	}
	if t.RepeatPolicy == "" {
		t.RepeatPolicy = lottery.TaskRepeatUnlimited
	}
	return ap.Store.UpsertTask(t)
}

// UpdateTask 更新任务。
func (ap *App) UpdateTask(t *lottery.Task) error {
	if _, err := ap.Store.GetTask(t.ID); err != nil {
		return err
	}
	_, err := ap.Store.UpsertTask(t)
	return err
}

// DeleteTask 删除任务（含码池与结算记录）。
func (ap *App) DeleteTask(id int64) error {
	return ap.Store.DeleteTask(id)
}

// AdminListTasks 管理端任务列表（含码池统计）。
func (ap *App) AdminListTasks() ([]AdminTaskView, error) {
	tasks, err := ap.Store.ListTasks()
	if err != nil {
		return nil, err
	}
	out := make([]AdminTaskView, 0, len(tasks))
	for _, t := range tasks {
		v := AdminTaskView{Task: t}
		if t.RewardType == lottery.TaskRewardRedeemCode {
			v.CodesAvailable, v.CodesGranted, _ = ap.Store.TaskCodeCounts(t.ID)
		}
		out = append(out, v)
	}
	return out, nil
}

// SetTaskCodes 整体重置任务可用码池（已发放记录保留）。
func (ap *App) SetTaskCodes(taskID int64, codes []string) (int64, error) {
	if _, err := ap.Store.GetTask(taskID); err != nil {
		return 0, err
	}
	return ap.Store.ReplaceAvailableTaskCodes(taskID, codes)
}

// ListTaskCodes 任务码池明细。
func (ap *App) ListTaskCodes(taskID int64) ([]lottery.TaskCode, error) {
	return ap.Store.ListTaskCodes(taskID)
}

// ListTaskRewards 任务发放记录。
func (ap *App) ListTaskRewards(taskID int64) ([]lottery.TaskReward, error) {
	return ap.Store.ListTaskRewards(taskID)
}

// ---- 集成代理（分组/模型下拉数据） ----

// TaskGroups 代理 sub2api 集成接口的分组列表。
func (ap *App) TaskGroups(ctx context.Context) ([]sub2api.IntegrationGroup, error) {
	return ap.Sub2API.ListIntegrationGroups(ctx)
}

// TaskGroupModels 代理 sub2api 集成接口的分组模型列表。
func (ap *App) TaskGroupModels(ctx context.Context, groupID int64) ([]string, error) {
	return ap.Sub2API.ListGroupModels(ctx, groupID)
}

// ---- 用户侧 ----

// userTaskView 任务 → 用户侧视图（隐藏运营字段）。
func userTaskView(t lottery.Task, now time.Time) UserTaskView {
	end := t.EndDate()
	daysLeft := 0
	if endTs, err := lottery.ParseDate(end); err == nil {
		if today, err2 := lottery.ParseDate(dateStr(now)); err2 == nil {
			daysLeft = int(endTs.Sub(today).Hours() / 24)
			if daysLeft < 0 {
				daysLeft = 0
			}
		}
	}
	return UserTaskView{
		ID: t.ID, Name: t.Name, Cover: t.Cover, Description: t.Description, Status: t.Status,
		GroupName: t.GroupName, Model: t.Model,
		StartDate: t.StartDate, EndDate: end, DaysLeft: daysLeft,
		SettleTime:      t.SettleTime,
		ThresholdTokens: t.ThresholdTokens,
		RewardType:      t.RewardType, RewardValue: t.RewardValue,
	}
}

// ListUserTasks 用户侧进行中的任务列表（status=active，含未开始）。
// 白名单外 / 黑名单内的用户看不到对应任务。
func (ap *App) ListUserTasks(email string, now time.Time) ([]UserTaskView, error) {
	tasks, err := ap.Store.ListTasks()
	if err != nil {
		return nil, err
	}
	out := []UserTaskView{}
	for _, t := range tasks {
		if t.Status != lottery.TaskActive {
			continue
		}
		if !t.UserAllowed(email) {
			continue
		}
		out = append(out, userTaskView(t, now))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartDate < out[j].StartDate })
	return out, nil
}

// TaskPromptView 任务引导弹窗载荷：只带封面、名称和说明。
type TaskPromptView struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Cover       string `json:"cover,omitempty"`
	Description string `json:"description,omitempty"`
}

// TaskPromptList 返回该用户当前可参与、进行中的任务，供 WebSocket 引导弹窗。
// 显隐与任务列表一致（管理员可见）；任务白/黑名单和可参与条件
// （全部满足，空条件视为可参与）都要过。
func (ap *App) TaskPromptList(ctx context.Context, userID int64, email, role string, registeredAt *time.Time) ([]TaskPromptView, error) {
	if !ap.IsUserAllowedTask(role, email) {
		return []TaskPromptView{}, nil
	}
	tasks, err := ap.Store.ListTasks()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	candidates := make([]lottery.Task, 0)
	maxWindow := 0
	for _, t := range tasks {
		if t.Status != lottery.TaskActive || !t.UserAllowed(email) {
			continue
		}
		if dateStr(now) > t.EndDate() {
			continue
		}
		for _, c := range t.Conditions {
			if c.WindowDays > maxWindow {
				maxWindow = c.WindowDays
			}
		}
		candidates = append(candidates, t)
	}
	if len(candidates) == 0 {
		return []TaskPromptView{}, nil
	}
	var window map[int64]*lottery.UserUsage
	if maxWindow > 0 {
		window, err = ap.usageWindow(ctx, maxWindow)
		if err != nil {
			return nil, err
		}
	}
	usage := (*lottery.UserUsage)(nil)
	if window != nil {
		usage = window[userID]
	}
	if usage == nil {
		usage = &lottery.UserUsage{UserID: userID, Email: email}
	}
	usage.RegisteredAt = ap.registeredAt(ctx, userID, email, registeredAt)

	out := make([]TaskPromptView, 0, len(candidates))
	for _, t := range candidates {
		if !taskConditionsMet(&t, usage, now) {
			continue
		}
		out = append(out, TaskPromptView{
			ID: t.ID, Name: t.Name, Cover: t.Cover, Description: t.Description,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

// taskConditionsMet 可参与条件全部满足。未配置条件时任何人可参与。
func taskConditionsMet(t *lottery.Task, usage *lottery.UserUsage, now time.Time) bool {
	for _, c := range t.Conditions {
		ok, _ := lottery.EvaluateCondition(c, usage, now)
		if !ok {
			return false
		}
	}
	return true
}

// TasksPhase 用户侧角标：该用户有可见的进行中任务返回 "active"，否则 "none"。
func (ap *App) TasksPhase(email string, now time.Time) (string, error) {
	tasks, err := ap.ListUserTasks(email, now)
	if err != nil {
		return "", err
	}
	if len(tasks) > 0 {
		return "active", nil
	}
	return "none", nil
}

// MyTaskRewards 用户自己的结算/奖励记录（含兑换码）。
func (ap *App) MyTaskRewards(userID int64) ([]lottery.TaskReward, error) {
	return ap.Store.ListUserTaskRewards(userID)
}

// ---- 结算引擎 ----

// SettleDueTasks 调度 tick：结算所有到期的任务结算日，并把过期任务置为 ended。
// 每个结算日以 sync_state 键 task_settled:<taskID>:<date> 幂等；失败不写键，
// 下个 tick 自动重试。
func (ap *App) SettleDueTasks(ctx context.Context, now time.Time) {
	tasks, err := ap.Store.ListTasks()
	if err != nil {
		slog.Error("task settle: list tasks failed", "err", err)
		return
	}
	for _, t := range tasks {
		if t.Status != lottery.TaskActive {
			continue
		}
		// 先结算到期未结算的结算日（最后一天的结算点在过期后才到达），
		// 再把已过期的任务置为 ended。
		ap.settleTaskDueDays(ctx, &t, now)
		if dateStr(now) > t.EndDate() {
			if err := ap.Store.SetTaskStatus(t.ID, lottery.TaskEnded); err != nil {
				slog.Error("task settle: mark ended failed", "task", t.ID, "err", err)
			}
		}
	}
}

func (ap *App) settleTaskDueDays(ctx context.Context, t *lottery.Task, now time.Time) {
	start, err := lottery.ParseDate(t.StartDate)
	if err != nil {
		slog.Error("task settle: bad start_date", "task", t.ID, "start", t.StartDate)
		return
	}
	end := now.AddDate(0, 0, -1) // 最多结算到昨天
	if endTs, err := lottery.ParseDate(t.EndDate()); err == nil && endTs.Before(end) {
		end = endTs
	}
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		date := lottery.FormatDate(d)
		settleAt, err := t.SettleMomentFor(date)
		if err != nil {
			slog.Error("task settle: bad settle_time", "task", t.ID, "err", err)
			return
		}
		if now.Before(settleAt) {
			continue
		}
		key := fmt.Sprintf("%s%d:%s", stateTaskSettledPrefix, t.ID, date)
		if raw, err := ap.Store.GetState(key); err == nil && raw != "" {
			continue
		}
		if err := ap.settleTaskDay(ctx, t, date); err != nil {
			slog.Error("task settle failed", "task", t.ID, "date", date, "err", err)
			continue // 不写幂等键，下个 tick 重试
		}
		if err := ap.Store.SetState(key, "done"); err != nil {
			slog.Error("task settle: write idempotency key failed", "task", t.ID, "date", date, "err", err)
		}
		slog.Info("task settled", "task", t.ID, "date", date)
	}
}

// settleTaskDay 结算某个结算日：拉当日分组+模型消耗，评估资格后发奖。
func (ap *App) settleTaskDay(ctx context.Context, t *lottery.Task, date string) error {
	usage, err := ap.Sub2API.FetchDailyModelUsage(ctx, date, t.GroupID, t.Model)
	if err != nil {
		return fmt.Errorf("fetch daily model usage: %w", err)
	}
	if len(usage) == 0 {
		return nil
	}
	sort.Slice(usage, func(i, j int) bool { return usage[i].UserID < usage[j].UserID })

	// 条件评估所需的窗口用量与注册时间
	var maxWindow int
	for _, c := range t.Conditions {
		if c.WindowDays > maxWindow {
			maxWindow = c.WindowDays
		}
	}
	var window map[int64]*lottery.UserUsage
	if maxWindow > 0 {
		window, err = ap.usageWindow(ctx, maxWindow)
		if err != nil {
			return fmt.Errorf("usage window: %w", err)
		}
	}

	for _, u := range usage {
		// 全局显隐：任务中心对该用户不可见时不参与结算
		if !ap.IsUserAllowedTask("user", u.Email) {
			continue
		}
		if !t.UserAllowed(u.Email) {
			continue
		}
		if u.TotalTokens < t.ThresholdTokens {
			continue
		}
		units := int(u.TotalTokens / t.ThresholdTokens)
		if units < 1 {
			continue
		}
		// 可参与条件（全部满足）
		if len(t.Conditions) > 0 {
			wu := window[u.UserID]
			if wu == nil {
				wu = &lottery.UserUsage{UserID: u.UserID, Email: u.Email}
			}
			wu.RegisteredAt = ap.registeredAt(ctx, u.UserID, u.Email, nil)
			for _, c := range t.Conditions {
				if ok, _ := lottery.EvaluateCondition(c, wu, time.Now()); !ok {
					units = 0
					break
				}
			}
			if units == 0 {
				continue
			}
		}
		// 重复参与策略：join_once / achieved_once 均为任务周期内奖励次数封顶 1
		if t.RepeatPolicy == lottery.TaskRepeatJoinOnce || t.RepeatPolicy == lottery.TaskRepeatAchievedOnce {
			if has, err := ap.Store.HasUserTaskReward(t.ID, u.UserID); err == nil && has {
				continue
			}
		}
		if err := ap.settleTaskUser(ctx, t, date, u, units); err != nil {
			return err
		}
	}
	return nil
}

// settleTaskUser 写入并发放单个用户的单个结算日奖励。
func (ap *App) settleTaskUser(ctx context.Context, t *lottery.Task, date string, u lottery.DailyToken, units int) error {
	reward := &lottery.TaskReward{
		TaskID: t.ID, SettleDate: date, UserID: u.UserID, Email: u.Email,
		Tokens: u.TotalTokens, Units: units,
		RewardType: t.RewardType, RewardValue: t.RewardValue,
		Fulfillment: lottery.FulfillmentPending,
	}
	if _, inserted, err := ap.Store.InsertTaskReward(reward); err != nil {
		return err
	} else if !inserted {
		return nil // 幂等：该日该用户已有记录
	}
	ap.fulfillTaskReward(ctx, t, reward)
	return nil
}

// fulfillTaskReward 单条发放：余额直接入账；兑换码从码池随机抽取。
func (ap *App) fulfillTaskReward(ctx context.Context, t *lottery.Task, reward *lottery.TaskReward) {
	var codes []string
	note := ""
	var err error
	switch t.RewardType {
	case lottery.TaskRewardBalance:
		total := float64(reward.Units) * t.RewardValue
		note = fmt.Sprintf("任务: %s +%s", t.Name, lottery.FormatValue(total))
		err = ap.Sub2API.AddBalance(ctx, reward.UserID, total, note)
	case lottery.TaskRewardRedeemCode:
		codes, err = ap.Store.TakeRandomTaskCodes(t.ID, reward.Units, reward.ID)
		if errors.Is(err, store.ErrCodePoolEmpty) {
			err = fmt.Errorf("兑换码码池可用数量不足（需要 %d 张），请补码后重试", reward.Units)
		}
	default:
		err = fmt.Errorf("unknown reward type %q", t.RewardType)
	}
	if err != nil {
		slog.Error("task fulfill failed", "reward", reward.ID, "err", err)
		if uerr := ap.Store.UpdateTaskReward(reward.ID, lottery.FulfillmentFailed, nil, note, err.Error()); uerr != nil {
			slog.Error("task fulfill update failed", "reward", reward.ID, "err", uerr)
		}
		return
	}
	if uerr := ap.Store.UpdateTaskReward(reward.ID, lottery.FulfillmentDone, codes, note, ""); uerr != nil {
		slog.Error("task fulfill update failed", "reward", reward.ID, "err", uerr)
		return
	}
	slog.Info("task fulfilled", "task", t.ID, "user", reward.UserID, "units", reward.Units, "type", t.RewardType)
}

// RetryTaskFulfillment 管理端重试某任务 pending/failed 的发放（如补码后）。
func (ap *App) RetryTaskFulfillment(ctx context.Context, taskID int64) error {
	t, err := ap.Store.GetTask(taskID)
	if err != nil {
		return err
	}
	rewards, err := ap.Store.ListUnfulfilledTaskRewards(taskID)
	if err != nil {
		return err
	}
	for i := range rewards {
		ap.fulfillTaskReward(ctx, t, &rewards[i])
	}
	return nil
}
