package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"lotteryd/internal/lottery"
	"lotteryd/internal/store/storetest"
	"lotteryd/internal/sub2api"
)

func TestRechargeCacheIsRequestLocal(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprintf(w, `{"code":0,"data":{"total_recharged":%d}}`, calls*10)
	}))
	defer srv.Close()
	ap := New(nil, sub2api.NewClient(srv.URL, "key"), Config{})
	u := map[int64]*lottery.UserUsage{}
	ctx := context.Background()
	require.NoError(t, ap.fillHistoricalRecharge(ctx, u, 7, "a@b.c", nil))
	require.Zero(t, calls)
	conditions := []lottery.ConditionDef{{Dimension: lottery.DimensionHistoricalRecharge}}
	require.NoError(t, ap.fillHistoricalRecharge(ctx, u, 7, "a@b.c", conditions))
	require.NoError(t, ap.fillHistoricalRecharge(ctx, u, 7, "a@b.c", conditions))
	require.Equal(t, 1, calls)
	require.Equal(t, 10.0, *u[7].TotalRecharged)
	u = map[int64]*lottery.UserUsage{}
	require.NoError(t, ap.fillHistoricalRecharge(ctx, u, 7, "a@b.c", conditions))
	require.Equal(t, 20.0, *u[7].TotalRecharged)
}

func TestDailyConfigBlacklistCopiesAndUpdatesUndrawnRound(t *testing.T) {
	s := storetest.Open(t)
	ap := New(s, nil, Config{})
	cfg := lottery.DailyConfig{Enabled: true, Name: "daily", DurationHours: 1, StartTime: "08:00", ConditionMatch: lottery.MatchAll, Blacklist: []string{" A@B.C "}, Conditions: []lottery.ConditionDef{{Dimension: lottery.DimensionHistoricalRecharge, Threshold: 10, BonusMode: lottery.BonusNone}}, Prizes: []lottery.PrizeSpec{{Name: "prize", PrizeType: lottery.PrizeBalance, Value: 1, Stock: 1}}}
	saved, err := ap.SaveDailyConfig(cfg)
	require.NoError(t, err)
	id, err := s.CreateActivity(dailyActivityFromConfig(saved, time.Now()))
	require.NoError(t, err)
	saved.Blacklist = []string{"new@b.c"}
	_, err = ap.SaveDailyConfig(saved)
	require.NoError(t, err)
	a, err := s.GetActivity(id)
	require.NoError(t, err)
	require.Equal(t, []string{"new@b.c"}, a.Blacklist)
	require.Equal(t, lottery.DimensionHistoricalRecharge, a.Conditions[0].Dimension)
}
