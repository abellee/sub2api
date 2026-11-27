package store_test

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"lotteryd/internal/store"
	"lotteryd/internal/store/storetest"
)

func TestDailyCheckInCycleResetAndBeijingMidnight(t *testing.T) {
	st := storetest.Open(t)
	rewards := []int64{10, 25, 12, 40, 15, 7, 9}
	require.NoError(t, st.SavePointsSettings(store.PointsSettings{DailyCheckInRewards: rewards}))
	start := time.Date(2026, 12, 27, 16, 0, 0, 0, time.UTC) // Beijing Dec 28, crossing into a new year below.
	var total int64
	for day := 1; day <= 8; day++ {
		now := start.AddDate(0, 0, day-1)
		status, err := st.GetDailyCheckIn(2, now)
		require.NoError(t, err)
		require.False(t, status.Claimed)
		require.Equal(t, (day-1)%7+1, status.Day)
		reward := rewards[(day-1)%7]
		require.Equal(t, rewards, status.Rewards)
		status, balance, granted, err := st.ClaimDailyCheckIn(2, now)
		total += reward
		require.NoError(t, err)
		require.True(t, granted)
		require.True(t, status.Claimed)
		require.Equal(t, reward, status.TodayReward)
		require.Equal(t, total, balance.Balance)
		_, same, granted, err := st.ClaimDailyCheckIn(2, now.Add(time.Hour))
		require.NoError(t, err)
		require.False(t, granted)
		require.Equal(t, total, same.Balance)
	}
	beforeMidnight, err := st.GetDailyCheckIn(2, start.AddDate(0, 0, 8).Add(-time.Nanosecond))
	require.NoError(t, err)
	require.True(t, beforeMidnight.Claimed)
	afterMidnight, err := st.GetDailyCheckIn(2, start.AddDate(0, 0, 8))
	require.NoError(t, err)
	require.False(t, afterMidnight.Claimed)
	require.Equal(t, 2, afterMidnight.Day)
	missed, err := st.GetDailyCheckIn(2, start.AddDate(0, 0, 9))
	require.NoError(t, err)
	require.Equal(t, 1, missed.Day)
	require.Equal(t, 0, missed.CompletedDays)
	_, transactions, err := st.ListPointsTransactions(2, "grant", 50, 0)
	require.NoError(t, err)
	require.EqualValues(t, 8, transactions)
}

func TestDailyCheckInConcurrentFirstGrant(t *testing.T) {
	st := storetest.Open(t)
	now := time.Now()
	var wg sync.WaitGroup
	grants := make(chan bool, 24)
	errors := make(chan error, 24)
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, granted, err := st.ClaimDailyCheckIn(2, now)
			grants <- granted
			errors <- err
		}()
	}
	wg.Wait()
	close(grants)
	close(errors)
	count := 0
	for granted := range grants {
		if granted {
			count++
		}
	}
	for err := range errors {
		require.NoError(t, err)
	}
	require.Equal(t, 1, count)
	balance, err := st.GetPointsBalance(2)
	require.NoError(t, err)
	require.EqualValues(t, 10, balance.Balance)
	_, total, err := st.ListPointsTransactions(2, "all", 10, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
}

func TestCheckInSettingsAndFailedGrantRollback(t *testing.T) {
	st := storetest.Open(t)
	for _, rewards := range [][]int64{nil, {}, {1}, {1, 2, 3, 4, 5, 6}, {1, 2, 3, 4, 5, 6, 7, 8}, {1, 2, 3, 4, 5, 6, 0}, {1, 2, 3, 4, 5, 6, -1}, {1, 2, 3, 4, 5, 6, 2000001}} {
		require.Error(t, st.SavePointsSettings(store.PointsSettings{DailyCheckInRewards: rewards}))
	}
	rewards := []int64{25, 18, 45, 12, 10, 7, 9}
	require.NoError(t, st.SavePointsSettings(store.PointsSettings{DailyCheckInRewards: rewards}))
	now := time.Now()
	for day := 0; day < 7; day++ {
		status, _, _, err := st.ClaimDailyCheckIn(2, now.AddDate(0, 0, day))
		require.NoError(t, err)
		require.EqualValues(t, rewards[day], status.TodayReward)
		if day == 2 {
			updated := []int64{40, 30, 20, 19, 18, 17, 16}
			require.NoError(t, st.SavePointsSettings(store.PointsSettings{DailyCheckInRewards: updated}))
			copy(rewards[3:], updated[3:])
		}
	}
	require.NoError(t, st.SavePointsSettings(store.PointsSettings{DailyCheckInRewards: []int64{40, 40, 40, 40, 40, 40, 40}}))
	alreadyClaimed, err := st.GetDailyCheckIn(2, now.AddDate(0, 0, 6))
	require.NoError(t, err)
	require.EqualValues(t, 16, alreadyClaimed.TodayReward)
	require.EqualValues(t, 25, alreadyClaimed.Rewards[0])
	newCycle, err := st.GetDailyCheckIn(2, now.AddDate(0, 0, 7))
	require.NoError(t, err)
	require.EqualValues(t, 40, newCycle.TodayReward)
	// A credit overflow must roll back both the day marker and the transaction.
	other := storetest.Open(t)
	_, _, err = other.GrantPoints(2, math.MaxInt64-5, "test", "", "", "", nil)
	require.NoError(t, err)
	_, _, _, err = other.ClaimDailyCheckIn(2, now)
	require.Error(t, err)
	status, err := other.GetDailyCheckIn(2, now)
	require.NoError(t, err)
	require.False(t, status.Claimed)
	balance, err := other.GetPointsBalance(2)
	require.NoError(t, err)
	require.EqualValues(t, math.MaxInt64-5, balance.Balance)
}

func TestAdminPointsRechargeConcurrentRetryAndConflict(t *testing.T) {
	st := storetest.Open(t)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := st.GrantAdminPoints(2, 2, 100, "same-request", "manual credit")
			require.NoError(t, err)
		}()
	}
	wg.Wait()
	balance, err := st.GetPointsBalance(2)
	require.NoError(t, err)
	require.EqualValues(t, 100, balance.Balance)
	_, _, err = st.GrantAdminPoints(2, 2, 200, "same-request", "manual credit")
	require.ErrorIs(t, err, store.ErrPointsRechargeConflict)
	_, _, err = st.GrantAdminPoints(2, 2, 100, "same-request", "changed note")
	require.ErrorIs(t, err, store.ErrPointsRechargeConflict)
	_, granted, err := st.GrantAdminPoints(2, 2, 50, "next-request", "next credit")
	require.NoError(t, err)
	require.True(t, granted)
	balance, err = st.GetPointsBalance(2)
	require.NoError(t, err)
	require.EqualValues(t, 150, balance.Balance)
	rows, total, err := st.ListPointsTransactions(2, "grant", 10, 0)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Equal(t, "manual credit", rows[1].Note)
}
