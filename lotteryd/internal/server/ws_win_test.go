package server

import (
	"testing"

	"github.com/stretchr/testify/require"

	"lotteryd/internal/lottery"
)

func TestPendingWinMessagesSkipsExistingThenSendsNew(t *testing.T) {
	known := map[int64]bool{}
	older := lottery.Winner{ID: 1, PrizeName: "旧奖"}
	msgs, ready := pendingWinMessages(known, false, []lottery.Winner{older})
	require.True(t, ready)
	require.Empty(t, msgs)
	require.True(t, known[1])

	fresh := []lottery.Winner{
		{ID: 3, PrizeName: "后到"},
		{ID: 2, PrizeName: "先到"},
	}
	msgs, ready = pendingWinMessages(known, true, fresh)
	require.True(t, ready)
	require.Len(t, msgs, 2)
	require.Equal(t, "lottery_win", msgs[0].Type)
	require.Equal(t, int64(2), msgs[0].Winner.ID)
	require.Equal(t, int64(3), msgs[1].Winner.ID)

	again, _ := pendingWinMessages(known, true, fresh)
	require.Empty(t, again)
}
