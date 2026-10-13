package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"lotteryd/internal/lottery"
	"lotteryd/internal/push"
)

func TestEligibleActivityUserIDsFiltersParticipation(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	activity := &lottery.Activity{
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
