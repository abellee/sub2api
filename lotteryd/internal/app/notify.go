package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"lotteryd/internal/lottery"
	"lotteryd/internal/push"
	"lotteryd/internal/store"
)

// ActivityNotificationView 是管理端看到的阶段通知结果。
// 指针为 nil 表示这个阶段还没发送；0 表示发送过，但没有可通知的用户。
type ActivityNotificationView struct {
	Enabled     bool     `json:"enabled"`
	Stages      []string `json:"stages"`
	Channels    []string `json:"channels"`
	BeforeStart *int     `json:"before_start,omitempty"`
	Started     *int     `json:"started,omitempty"`
	BeforeDraw  *int     `json:"before_draw,omitempty"`
	Results     *int     `json:"results,omitempty"`
}

func stageCountPtr(counts map[string]int, stage string) *int {
	if counts == nil {
		return nil
	}
	value, ok := counts[stage]
	if !ok {
		return nil
	}
	return &value
}

func (ap *App) pushReady() bool {
	return ap != nil && ap.Push != nil && ap.Push.Configured()
}

// NotifyDueActivities 给打开了通知的活动发送到期阶段提醒。
func (ap *App) NotifyDueActivities(ctx context.Context, now time.Time) error {
	if !ap.pushReady() {
		return nil
	}
	selected, err := ap.Store.ListActivityNotificationSettings()
	if err != nil {
		return err
	}
	logs, err := ap.Store.ListActivityPushLogs()
	if err != nil {
		return err
	}
	activities, err := ap.Store.ListActivities()
	if err != nil {
		return err
	}
	var firstErr error
	for i := range activities {
		activity := &activities[i]
		if activity.Status == "deleted" {
			continue
		}
		allowed := map[string]struct{}{}
		settings := selected[activity.ID]
		if len(settings.Channels) == 0 {
			continue
		}
		for _, stage := range settings.Stages {
			allowed[stage] = struct{}{}
		}
		if len(allowed) == 0 {
			continue
		}
		sent := map[string]struct{}{}
		for stage := range logs[activity.ID] {
			sent[stage] = struct{}{}
		}
		for _, stage := range lottery.DueNotifyStages(activity, now, sent) {
			if _, ok := allowed[stage]; !ok {
				continue
			}
			notified, sendErr := ap.sendActivityStage(ctx, activity, stage, now, settings.Channels)
			if sendErr != nil {
				slog.Warn("activity notification failed", "activity", activity.ID, "stage", stage, "err", sendErr)
				if firstErr == nil {
					firstErr = sendErr
				}
				continue
			}
			if markErr := ap.Store.MarkActivityPush(activity.ID, stage, notified); markErr != nil && firstErr == nil {
				firstErr = markErr
			}
		}
	}
	return firstErr
}

func (ap *App) sendActivityStage(ctx context.Context, activity *lottery.Activity, stage string, now time.Time, channels []string) (int, error) {
	browser := lottery.HasNotifyChannel(channels, lottery.NotifyBrowser)
	userIDs := make([]int64, 0)
	if browser && stage == lottery.StageResults {
		winners, err := ap.Store.ListWinnersByActivity(activity.ID)
		if err != nil {
			return 0, err
		}
		seen := map[int64]struct{}{}
		for _, winner := range winners {
			if winner.UserID <= 0 {
				continue
			}
			if _, ok := seen[winner.UserID]; ok {
				continue
			}
			seen[winner.UserID] = struct{}{}
			userIDs = append(userIDs, winner.UserID)
		}
	} else if browser {
		ids, err := ap.activityNotifyUserIDs(ctx, activity, now)
		if err != nil {
			return 0, err
		}
		userIDs = ids
	}
	title, body := activityPushCopy(activity, stage)
	var group *push.GroupNotification
	if lottery.HasNotifyChannel(channels, lottery.NotifyGroup) {
		group = &push.GroupNotification{Body: activityGroupCopy(activity, stage), Generate: true}
	}
	result, err := ap.Push.Notify(ctx, push.Message{
		Title:               title,
		Body:                body,
		URL:                 "/lottery",
		DedupeKey:           fmt.Sprintf("lottery:%d:%s", activity.ID, stage),
		UserIDs:             userIDs,
		BrowserNotification: &browser,
		GroupNotification:   group,
	})
	if err != nil {
		return 0, err
	}
	return result.Notified, nil
}

// activityNotifyUserIDs 只保留当前能参与这场抽奖、且开着通知的用户。
// 空名单必须保持非 nil，调用方不能把它当成“通知所有人”。
func (ap *App) activityNotifyUserIDs(ctx context.Context, activity *lottery.Activity, now time.Time) ([]int64, error) {
	subscribers, err := ap.Push.ListSubscribers(ctx)
	if err != nil {
		return nil, err
	}
	usages, err := ap.usageWindow(ctx, maxWindowDays([]lottery.Activity{*activity}))
	if err != nil {
		return nil, err
	}
	if usages == nil {
		usages = map[int64]*lottery.UserUsage{}
	}
	if activityNeedsRegistration(activity) || needsHistoricalRecharge(activity.Conditions) {
		for _, user := range subscribers {
			if user.ID <= 0 {
				continue
			}
			usage := usages[user.ID]
			if usage == nil {
				usage = &lottery.UserUsage{UserID: user.ID, Email: user.Email}
				usages[user.ID] = usage
			}
			if activityNeedsRegistration(activity) && usage.RegisteredAt == nil {
				usage.RegisteredAt = ap.registeredAt(ctx, user.ID, user.Email, nil)
			}
			if err := ap.fillHistoricalRecharge(ctx, usages, user.ID, user.Email, activity.Conditions); err != nil {
				return nil, err
			}
		}
	}
	return eligibleActivityUserIDs(activity, subscribers, usages, now, func(email string) bool {
		return activity.Visibility.ShowsTo(email, true)
	}, func(userID int64) bool {
		blocked, _ := ap.repeatPolicyBlocked(activity, userID)
		removed, err := ap.Store.HasRemovedParticipant(activity.ID, userID)
		return blocked || removed || err != nil
	}), nil
}

func activityNeedsRegistration(activity *lottery.Activity) bool {
	if activity == nil {
		return false
	}
	for _, condition := range activity.Conditions {
		if condition.Dimension == lottery.DimensionRegisteredDays {
			return true
		}
	}
	return false
}

func eligibleActivityUserIDs(activity *lottery.Activity, subscribers []push.Subscriber, usages map[int64]*lottery.UserUsage, now time.Time, allowed func(string) bool, blocked func(int64) bool) []int64 {
	ids := make([]int64, 0)
	if activity == nil {
		return ids
	}
	seen := map[int64]struct{}{}
	for _, user := range subscribers {
		if user.ID <= 0 {
			continue
		}
		if _, ok := seen[user.ID]; ok {
			continue
		}
		var usage *lottery.UserUsage
		if usages != nil {
			usage = usages[user.ID]
		}
		email := strings.TrimSpace(user.Email)
		if email == "" && usage != nil {
			email = strings.TrimSpace(usage.Email)
		}
		if allowed != nil && !allowed(email) {
			continue
		}
		if usage == nil {
			usage = &lottery.UserUsage{UserID: user.ID, Email: email}
		}
		if !activity.Evaluate(usage, now).Eligible {
			continue
		}
		if blocked != nil && blocked(user.ID) {
			continue
		}
		seen[user.ID] = struct{}{}
		ids = append(ids, user.ID)
	}
	return ids
}

func activityPushCopy(activity *lottery.Activity, stage string) (string, string) {
	switch stage {
	case lottery.StageBeforeStart:
		return "抽奖即将开始", fmt.Sprintf("「%s」将在 10 分钟后开始", activity.Name)
	case lottery.StageStarted:
		return "抽奖已开始", fmt.Sprintf("「%s」已开始，现在可以参与", activity.Name)
	case lottery.StageBeforeDraw:
		return "即将开奖", fmt.Sprintf("「%s」将在 5 分钟后开奖", activity.Name)
	default:
		return "抽奖结果", fmt.Sprintf("「%s」已开奖，你中奖了", activity.Name)
	}
}

func activityGroupCopy(activity *lottery.Activity, stage string) string {
	stages := map[string]string{lottery.StageBeforeStart: "开始前10分钟", lottery.StageStarted: "正式开始", lottery.StageBeforeDraw: "开奖前5分钟", lottery.StageResults: "已开奖，提醒查看结果，不代表所有人中奖"}
	type publicPrize struct {
		Name  string  `json:"名称"`
		Type  string  `json:"类型"`
		Value float64 `json:"面额美元"`
		Stock int64   `json:"数量"`
	}
	prizes := make([]publicPrize, 0, len(activity.Prizes))
	for _, prize := range activity.Prizes {
		prizes = append(prizes, publicPrize{prize.Name, prize.PrizeType, prize.Value, prize.Stock})
	}
	info := map[string]any{"活动名称": activity.Name, "活动说明": activity.Description, "阶段": stages[stage], "开始时间（北京时间）": activity.StartsAt.In(lottery.Beijing()).Format("2006-01-02 15:04"), "开奖时间（北京时间）": activity.DrawsAt.In(lottery.Beijing()).Format("2006-01-02 15:04"), "参与人数上限（0为不限）": activity.MaxParticipants, "条件满足方式": activity.ConditionMatch, "参与条件": activity.Conditions, "奖品": prizes}
	data, _ := json.Marshal(info)
	return string(data)
}

// NotifyDueTaskRewards 通知已经拿到奖励、当前仍符合参与条件、且开启了通知的用户。
func (ap *App) NotifyDueTaskRewards(ctx context.Context) error {
	if !ap.pushReady() {
		return nil
	}
	rows, err := ap.Store.ListUnnotifiedDoneTaskRewards()
	if err != nil {
		return err
	}
	var firstErr error
	for _, row := range rows {
		task, taskErr := ap.Store.GetTask(row.TaskID)
		if taskErr != nil {
			if errors.Is(taskErr, store.ErrNotFound) {
				if markErr := ap.Store.MarkTaskRewardNotified(row.RewardID, row.TaskID, row.UserID, false); markErr != nil && firstErr == nil {
					firstErr = markErr
				}
				continue
			}
			if firstErr == nil {
				firstErr = taskErr
			}
			continue
		}
		eligible, eligibleErr := ap.taskRewardEligible(ctx, task, row.UserID, row.Email)
		if eligibleErr != nil {
			slog.Warn("task reward eligibility check failed", "reward", row.RewardID, "err", eligibleErr)
			if firstErr == nil {
				firstErr = eligibleErr
			}
			continue
		}
		if !eligible {
			if markErr := ap.Store.MarkTaskRewardNotified(row.RewardID, row.TaskID, row.UserID, false); markErr != nil && firstErr == nil {
				firstErr = markErr
			}
			continue
		}
		result, sendErr := ap.Push.Notify(ctx, push.Message{
			Title:     "任务奖励已到账",
			Body:      fmt.Sprintf("「%s」的奖励已发放", row.TaskName),
			URL:       "/tasks",
			DedupeKey: fmt.Sprintf("task-reward:%d", row.RewardID),
			UserIDs:   []int64{row.UserID},
		})
		if sendErr != nil {
			slog.Warn("task reward notification failed", "reward", row.RewardID, "err", sendErr)
			if firstErr == nil {
				firstErr = sendErr
			}
			continue
		}
		if markErr := ap.Store.MarkTaskRewardNotified(row.RewardID, row.TaskID, row.UserID, result.Notified > 0); markErr != nil && firstErr == nil {
			firstErr = markErr
		}
	}
	return firstErr
}

// taskRewardEligible 用和结算相同的可见性、白黑名单和可参与条件判断这个用户现在能否收到奖励通知。
func (ap *App) taskRewardEligible(ctx context.Context, task *lottery.Task, userID int64, email string) (bool, error) {
	var usage *lottery.UserUsage
	if len(task.Conditions) > 0 {
		maxWindow := 0
		for _, condition := range task.Conditions {
			if condition.WindowDays > maxWindow {
				maxWindow = condition.WindowDays
			}
		}
		loaded, err := ap.loadUserUsage(ctx, userID, email, nil, maxWindow)
		if err != nil {
			return false, err
		}
		usage = loaded
	}
	return taskParticipationMet(task, email, usage, time.Now(), task.Visibility.ShowsTo(email, true)), nil
}

func taskParticipationMet(task *lottery.Task, email string, usage *lottery.UserUsage, now time.Time, visible bool) bool {
	if task == nil || !visible || !task.UserAllowed(email) {
		return false
	}
	if usage == nil {
		usage = &lottery.UserUsage{Email: email}
	}
	return taskConditionsMet(task, usage, now)
}
