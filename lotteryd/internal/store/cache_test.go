package store_test

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"lotteryd/internal/cache"
	"lotteryd/internal/lottery"
	"lotteryd/internal/store"
	"lotteryd/internal/store/storetest"
)

func cachedStore(t *testing.T) (*store.Store, *miniredis.Miniredis) {
	t.Helper()
	s := storetest.Open(t)
	r := miniredis.RunT(t)
	c, err := cache.New("redis://"+r.Addr(), t.Name())
	require.NoError(t, err)
	t.Cleanup(func() { c.Close() })
	s.SetCache(c)
	return s, r
}

func TestNotificationChannelEditsInvalidateRedisSettings(t *testing.T) {
	s, _ := cachedStore(t)
	require.NoError(t, s.SetActivityNotify(1, []string{lottery.StageStarted}, []string{lottery.NotifyBrowser}))
	settings, err := s.ListActivityNotificationSettings()
	require.NoError(t, err)
	require.Equal(t, []string{lottery.NotifyBrowser}, settings[1].Channels)
	require.NoError(t, s.SetActivityNotify(1, []string{lottery.StageResults}, []string{lottery.NotifyGroup}))
	settings, err = s.ListActivityNotificationSettings()
	require.NoError(t, err)
	require.Equal(t, []string{lottery.NotifyGroup}, settings[1].Channels)
	require.Equal(t, []string{lottery.StageResults}, settings[1].Stages)
}

func TestActivityCacheTracksEditsParticipationRemovalDrawAndFulfillment(t *testing.T) {
	s, r := cachedStore(t)
	id := moderationActivity(t, s)
	acts, err := s.ListActivities()
	require.NoError(t, err)
	require.Len(t, acts, 1)
	acts[0].Name = "edited"
	acts[0].Prizes[0].Codes[0] = "updated-code"
	require.NoError(t, s.UpdateActivity(&acts[0]))
	acts, err = s.ListActivities()
	require.NoError(t, err)
	require.Equal(t, "edited", acts[0].Name)
	require.Equal(t, "updated-code", acts[0].Prizes[0].Codes[0])
	state, err := s.UserActivityRecords(1)
	require.NoError(t, err)
	require.False(t, state.Joined[id])
	require.NoError(t, s.AddParticipant(id, 1, "one@test.com", 1, 0))
	state, err = s.UserActivityRecords(1)
	require.NoError(t, err)
	require.True(t, state.Joined[id])
	require.Equal(t, int64(1), state.Counts[id])
	require.NoError(t, s.RemoveParticipant(id, 1))
	state, err = s.UserActivityRecords(1)
	require.NoError(t, err)
	require.False(t, state.Joined[id])
	require.True(t, state.Removed[id])
	require.NoError(t, s.AddParticipant(id, 2, "two@test.com", 1, 0))
	_, err = s.ListWinnersByUser(2)
	require.NoError(t, err)
	_, err = s.DrawActivity(id)
	require.NoError(t, err)
	acts, err = s.ListActivities()
	require.NoError(t, err)
	require.False(t, acts[0].DrawnAt.IsZero())
	require.Equal(t, int64(1), acts[0].Prizes[0].GrantedCount)
	wins, err := s.ListWinnersByUser(2)
	require.NoError(t, err)
	require.Len(t, wins, 1)
	require.Equal(t, "updated-code", wins[0].RedeemCode)
	require.NoError(t, s.UpdateWinnerFulfillment(wins[0].ID, "done", wins[0].RedeemCode, "delivered", ""))
	wins, err = s.ListWinnersByUser(2)
	require.NoError(t, err)
	require.Equal(t, "done", wins[0].Fulfillment)
	other, err := s.ListWinnersByUser(1)
	require.NoError(t, err)
	require.Empty(t, other)
	// A write during an outage succeeds in PostgreSQL and is visible via fallback.
	r.SetError("offline")
	require.NoError(t, s.SetActivityStatus(id, "archived"))
	acts, err = s.ListActivities()
	require.NoError(t, err)
	require.Equal(t, "archived", acts[0].Status)
}

func TestWarmCacheServesReadsWithoutDatabaseAndUpdatesSettingsAndTasks(t *testing.T) {
	s, _ := cachedStore(t)
	moderationActivity(t, s)
	require.NoError(t, s.SetState("visibility_json", `{"mode":"all"}`))
	v, err := s.GetState("visibility_json")
	require.NoError(t, err)
	require.Contains(t, v, "all")
	require.NoError(t, s.SetState("visibility_json", `{"mode":"partial"}`))
	v, err = s.GetState("visibility_json")
	require.NoError(t, err)
	require.Contains(t, v, "partial")
	task := &lottery.Task{Name: "task", StartDate: time.Now().Format("2006-01-02"), DurationDays: 2, SettleTime: "00:10", ThresholdTokens: 100, RewardType: lottery.TaskRewardBalance, RewardValue: 1, RepeatPolicy: lottery.TaskRepeatUnlimited, Status: lottery.TaskActive}
	id, err := s.UpsertTask(task)
	require.NoError(t, err)
	tasks, err := s.ListTasks()
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.NoError(t, s.SetTaskStatus(id, lottery.TaskArchived))
	tasks, err = s.ListTasks()
	require.NoError(t, err)
	require.Equal(t, lottery.TaskArchived, tasks[0].Status)
	require.NoError(t, s.DeleteTask(id))
	tasks, err = s.ListTasks()
	require.NoError(t, err)
	require.Empty(t, tasks)
	require.NoError(t, s.Close())
	// These methods must actually read Redis, rather than silently querying SQL again.
	acts, err := s.ListActivities()
	require.NoError(t, err)
	require.Len(t, acts, 1)
	tasks, err = s.ListTasks()
	require.NoError(t, err)
	require.Empty(t, tasks)
	v, err = s.GetState("visibility_json")
	require.NoError(t, err)
	require.Contains(t, v, "partial")
}
