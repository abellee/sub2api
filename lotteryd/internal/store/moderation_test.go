package store_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"lotteryd/internal/lottery"
	"lotteryd/internal/store"
	"lotteryd/internal/store/storetest"
)

func moderationActivity(t *testing.T, s *store.Store) int64 {
	t.Helper()
	id, err := s.CreateActivity(&lottery.Activity{Name: "moderation", StartsAt: time.Now().Add(-time.Hour), DrawsAt: time.Now().Add(time.Hour), ConditionMatch: lottery.MatchAll, Prizes: []lottery.Prize{{Name: "code", PrizeType: lottery.PrizeRedeemCode, Value: 1, Stock: 3, Codes: []string{"a", "b", "c"}}}})
	require.NoError(t, err)
	return id
}

func TestModerationPersistsAndDrawReloads(t *testing.T) {
	s := storetest.Open(t)
	id := moderationActivity(t, s)
	for uid := int64(1); uid <= 3; uid++ {
		require.NoError(t, s.AddParticipant(id, uid, []string{"one@test.com", "two@test.com", "three@test.com"}[uid-1], 1, 0))
	}
	stale, err := s.GetActivity(id)
	require.NoError(t, err)
	require.NoError(t, s.RemoveParticipant(id, 1))
	require.ErrorIs(t, s.AddParticipant(id, 1, "one@test.com", 1, 0), store.ErrParticipantRemoved)
	stale.Blacklist = []string{" TWO@test.COM "}
	require.NoError(t, s.UpdateActivity(stale))
	require.NoError(t, s.AddParticipant(id, 4, "two@test.com", 1, 0))
	count, err := s.CountParticipants(id)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
	drawn, err := s.DrawActivity(id)
	require.NoError(t, err)
	require.False(t, drawn.DrawnAt.IsZero())
	wins, err := s.ListWinnersByActivity(id)
	require.NoError(t, err)
	require.Len(t, wins, 1)
	require.Equal(t, int64(3), wins[0].UserID)
	require.Equal(t, "a", wins[0].RedeemCode)
	require.ErrorIs(t, s.RemoveParticipant(id, 3), store.ErrActivityDrawn)
	require.ErrorIs(t, s.UpdateActivity(stale), store.ErrActivityDrawn)
	_, err = s.DrawActivity(id)
	require.ErrorIs(t, err, store.ErrActivityDrawn)
	emptyID := moderationActivity(t, s)
	_, err = s.DrawActivity(emptyID)
	require.NoError(t, err)
	require.ErrorIs(t, s.RemoveParticipant(emptyID, 1), store.ErrActivityDrawn)
	empty, err := s.GetActivity(emptyID)
	require.NoError(t, err)
	require.ErrorIs(t, s.UpdateActivity(empty), store.ErrActivityDrawn)
}

func TestRemovalSerializesWithDraw(t *testing.T) {
	s := storetest.Open(t)
	for i := 0; i < 12; i++ {
		id := moderationActivity(t, s)
		require.NoError(t, s.AddParticipant(id, 1, "one@test.com", 1, 0))
		start := make(chan struct{})
		var removeErr, drawErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); <-start; removeErr = s.RemoveParticipant(id, 1) }()
		go func() { defer wg.Done(); <-start; _, drawErr = s.DrawActivity(id) }()
		close(start)
		wg.Wait()
		require.NoError(t, drawErr)
		wins, err := s.ListWinnersByActivity(id)
		require.NoError(t, err)
		if removeErr == nil {
			require.Empty(t, wins)
		} else {
			require.True(t, errors.Is(removeErr, store.ErrActivityDrawn))
			require.Len(t, wins, 1)
		}
	}
}
