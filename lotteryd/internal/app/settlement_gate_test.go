package app

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"lotteryd/internal/lottery"
	"lotteryd/internal/store"
	"lotteryd/internal/sub2api"
)

func TestSettlementGateDrawsWithoutPaying(t *testing.T) {
	st, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })

	ap := New(st, sub2api.NewClient("http://127.0.0.1:9", "k"), Config{DisableSettlement: true})
	ctx := context.Background()
	now := time.Now().UTC()

	require.NoError(t, ap.DrawDueActivities(ctx, now))
	ap.SettleDueTasks(ctx, now)

	id, err := st.CreateActivity(&lottery.Activity{
		Name:           "本地场次",
		StartsAt:       now.Add(-time.Hour),
		DrawsAt:        now.Add(-time.Minute),
		ConditionMatch: lottery.MatchAll,
		Prizes: []lottery.Prize{{
			Name: "余额奖", PrizeType: lottery.PrizeBalance, Value: 1, Stock: 1,
		}},
	})
	require.NoError(t, err)
	require.NoError(t, st.AddParticipant(id, 7, "a@b.c", 1, 0))
	loaded, err := st.GetActivity(id)
	require.NoError(t, err)
	require.NoError(t, ap.DrawActivity(ctx, loaded))

	wins, err := st.ListWinnersByActivity(id)
	require.NoError(t, err)
	require.Len(t, wins, 1)
	require.Equal(t, lottery.FulfillmentPending, wins[0].Fulfillment)
	require.ErrorIs(t, ap.RetryFulfillment(ctx, id), ErrSettlementDisabled)
	require.ErrorIs(t, ap.RetryTaskFulfillment(ctx, 9), ErrSettlementDisabled)
}
