package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"lotteryd/internal/lottery"
	"lotteryd/internal/push"
	"lotteryd/internal/store/storetest"
)

func TestEligibleActivityUserIDsFiltersParticipation(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	activity := &lottery.Activity{
		Blacklist:      []string{" ONE@Test.com "},
		ConditionMatch: lottery.MatchAll,
		Conditions: []lottery.ConditionDef{{
			Dimension:  lottery.DimensionTokenUsage,
			WindowDays: 1,
			Mode:       "total",
			Threshold:  100,
			BonusMode:  lottery.BonusNone,
		}},
	}
	usages := map[int64]*lottery.UserUsage{
		1: {UserID: 1, Email: "one@test.com", Daily: []lottery.DailyToken{{Date: "2026-09-28", TotalTokens: 150}}},
		2: {UserID: 2, Email: "two@test.com", Daily: []lottery.DailyToken{{Date: "2026-09-28", TotalTokens: 10}}},
		4: {UserID: 4, Email: "four@test.com", Daily: []lottery.DailyToken{{Date: "2026-09-28", TotalTokens: 200}}},
	}
	subscribers := []push.Subscriber{
		{ID: 1, Email: "one@test.com"},
		{ID: 2, Email: "two@test.com"},
		{ID: 3, Email: "three@test.com"},
		{ID: 4, Email: "four@test.com"},
		{ID: 1, Email: "one@test.com"},
	}
	ids := eligibleActivityUserIDs(activity, subscribers, usages, now, func(email string) bool {
		return email != "four@test.com"
	}, func(userID int64) bool {
		return userID == 4
	})
	require.Equal(t, []int64{1}, ids)
}

func TestEligibleActivityUserIDsWithoutConditionsKeepsVisibleUsers(t *testing.T) {
	activity := &lottery.Activity{ConditionMatch: lottery.MatchAll}
	ids := eligibleActivityUserIDs(activity, []push.Subscriber{
		{ID: 8, Email: "allowed@test.com"},
		{ID: 9, Email: "hidden@test.com"},
	}, nil, time.Now(), func(email string) bool {
		return email == "allowed@test.com"
	}, func(userID int64) bool {
		return false
	})
	require.Equal(t, []int64{8}, ids)
}

func TestTaskParticipationMet(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	task := &lottery.Task{
		Whitelist: []string{"one@test.com"},
		Conditions: []lottery.ConditionDef{{
			Dimension:  lottery.DimensionTokenUsage,
			WindowDays: 1,
			Mode:       "total",
			Threshold:  100,
		}},
	}
	usage := &lottery.UserUsage{UserID: 1, Email: "one@test.com", Daily: []lottery.DailyToken{{Date: "2026-09-28", TotalTokens: 150}}}
	require.True(t, taskParticipationMet(task, "one@test.com", usage, now, true))
	require.False(t, taskParticipationMet(task, "other@test.com", usage, now, true))
	require.False(t, taskParticipationMet(task, "one@test.com", usage, now, false))
	require.False(t, taskParticipationMet(task, "one@test.com", &lottery.UserUsage{Email: "one@test.com"}, now, true))
}

func TestActivityGroupCopyAllStagesIsPublic(t *testing.T) {
	activity := &lottery.Activity{Name: "双11抽奖", Description: "活动说明", Prizes: []lottery.Prize{{Name: "奖品", PrizeType: lottery.PrizeRedeemCode, Stock: 2, Value: 5, Codes: []string{"SECRET-CODE"}}}}
	want := map[string]string{
		lottery.StageBeforeStart: "开始前10分钟",
		lottery.StageStarted:     "正式开始",
		lottery.StageBeforeDraw:  "开奖前5分钟",
		lottery.StageResults:     "已开奖",
	}
	for _, stage := range lottery.AllNotifyStages() {
		t.Run(stage, func(t *testing.T) {
			text := activityGroupCopy(activity, stage)
			require.Contains(t, text, activity.Name)
			require.Contains(t, text, want[stage])
			require.NotContains(t, text, "你中奖了")
			require.NotContains(t, text, "SECRET-CODE")
			var data map[string]any
			require.NoError(t, json.Unmarshal([]byte(text), &data))
			require.Equal(t, "活动说明", data["活动说明"])
			require.NotContains(t, text, "https://")
		})
	}
}

func TestGroupOnlyStagesBypassBrowserEligibility(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		require.Equal(t, "/push-api/v1/internal/notify", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, false, body["browserNotification"])
		require.Equal(t, []any{}, body["userIDs"])
		group := body["groupNotification"].(map[string]any)
		require.Equal(t, true, group["generate"])
		require.Contains(t, group["body"], "艺术节抽奖")
		_ = json.NewEncoder(w).Encode(push.Result{})
	}))
	defer srv.Close()
	// No store: group-only stages must not query browser recipients or winner identities.
	ap := &App{Push: push.New(srv.URL, "test-only")}
	for _, stage := range lottery.AllNotifyStages() {
		_, err := ap.sendActivityStage(context.Background(), &lottery.Activity{ID: 1, Name: "艺术节抽奖"}, stage, time.Now(), []string{lottery.NotifyGroup})
		require.NoError(t, err)
	}
	require.Equal(t, 4, requests)
}

func TestActivitySelectedChannelsPersistAndDispatch(t *testing.T) {
	s := storetest.Open(t)
	var sent []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"users": []any{}})
			return
		}
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		sent = append(sent, body)
		_ = json.NewEncoder(w).Encode(push.Result{})
	}))
	defer srv.Close()
	ap := &App{Store: s, Push: push.New(srv.URL, "test-only")}
	for _, channels := range [][]string{{lottery.NotifyBrowser}, {lottery.NotifyGroup}, {lottery.NotifyBrowser, lottery.NotifyGroup}, {}} {
		now := time.Now()
		id, err := s.CreateActivity(&lottery.Activity{Name: "test", StartsAt: now.Add(-time.Minute), DrawsAt: now.Add(time.Hour), ConditionMatch: lottery.MatchAll})
		require.NoError(t, err)
		ap.SaveActivityNotify(id, []string{lottery.StageStarted}, false, channels)
		before := len(sent)
		require.NoError(t, ap.NotifyDueActivities(context.Background(), now))
		if len(channels) == 0 {
			require.Len(t, sent, before)
			continue
		}
		require.Len(t, sent, before+1)
		body := sent[len(sent)-1]
		require.Equal(t, lottery.HasNotifyChannel(channels, lottery.NotifyBrowser), body["browserNotification"])
		_, hasGroup := body["groupNotification"]
		require.Equal(t, lottery.HasNotifyChannel(channels, lottery.NotifyGroup), hasGroup)
		require.NoError(t, ap.NotifyDueActivities(context.Background(), now))
		require.Len(t, sent, before+1)
	}
}

func TestDailyNotificationChannelsReachCreatedRounds(t *testing.T) {
	s := storetest.Open(t)
	ap := New(s, nil, Config{})
	for _, channels := range [][]string{nil, {}, {lottery.NotifyGroup}} {
		cfg, err := ap.SaveDailyConfig(lottery.DailyConfig{Enabled: true, Name: "daily", DurationHours: 1, StartTime: "08:00", ConditionMatch: lottery.MatchAll, NotifyStages: []string{lottery.StageStarted}, NotifyChannels: channels, Prizes: []lottery.PrizeSpec{{Name: "prize", PrizeType: lottery.PrizeBalance, Value: 1, Stock: 1}}})
		require.NoError(t, err)
		want, err := lottery.ResolveNotifyChannels(channels)
		require.NoError(t, err)
		require.Equal(t, want, cfg.NotifyChannels)
		acts, err := s.ListActivities()
		require.NoError(t, err)
		found := false
		for _, activity := range acts {
			if activity.DailyConfigID != cfg.ID {
				continue
			}
			found = true
			settings, err := s.ActivityNotificationSettings(activity.ID)
			require.NoError(t, err)
			require.Equal(t, want, settings.Channels)
			require.Equal(t, []string{lottery.StageStarted}, settings.Stages)
		}
		require.True(t, found)
	}
}
