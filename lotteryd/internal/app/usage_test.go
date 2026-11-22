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
	"lotteryd/internal/store/storetest"
	"lotteryd/internal/sub2api"
)

func TestSyncRepairsFrozenHistoryAndRefreshesPartialDays(t *testing.T) {
	st := storetest.Open(t)
	now := time.Date(2026, 10, 18, 1, 0, 0, 0, lottery.Beijing())
	today := dateStr(now)
	yesterday := dateStr(now.AddDate(0, 0, -1))
	oldDate := dateStr(now.AddDate(0, 0, -6))
	source := map[string]float64{oldDate: 1e6, yesterday: 10, today: 1}
	failDate := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Asia/Shanghai", r.URL.Query().Get("timezone"))
		date := r.URL.Query().Get("start_date")
		if date == failDate {
			w.WriteHeader(503)
			return
		}
		users := []map[string]any{}
		if n, ok := source[date]; ok {
			users = append(users, map[string]any{"user_id": 527, "total_tokens": n})
		}
		json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"users": users}})
	}))
	defer srv.Close()
	ap := New(st, sub2api.NewClient(srv.URL, "key"), Config{BackfillDays: 8})
	require.NoError(t, st.SetState(stateTokensLastDate, today)) // frozen old watermark
	require.NoError(t, st.ReplaceDailyTokens(oldDate, []lottery.DailyToken{{UserID: 527, TotalTokens: 1}}))
	failDate = yesterday
	require.Error(t, ap.SyncTokens(context.Background(), now))
	repaired, err := st.GetState("tokens_snapshot_repair_v1")
	require.NoError(t, err)
	require.Empty(t, repaired)
	failDate = ""
	require.NoError(t, ap.SyncTokens(context.Background(), now))
	usage, err := st.DailyTokensWindow(oldDate, oldDate)
	require.NoError(t, err)
	require.Equal(t, 1e6, usage[527].Daily[0].TotalTokens)
	source[today] = 1e6
	require.NoError(t, ap.SyncTokens(context.Background(), now.Add(12*time.Hour)))
	usage, err = st.DailyTokensWindow(today, today)
	require.NoError(t, err)
	require.Equal(t, 1e6, usage[527].Daily[0].TotalTokens)
	source[today] = 2e6 // final count arrives overnight
	require.NoError(t, ap.SyncTokens(context.Background(), now.AddDate(0, 0, 1)))
	usage, err = st.DailyTokensWindow(today, today)
	require.NoError(t, err)
	require.Equal(t, 2e6, usage[527].Daily[0].TotalTokens)
	delete(source, today) // authoritative correction removes a stale user
	require.NoError(t, ap.SyncTokens(context.Background(), now.AddDate(0, 0, 1)))
	usage, err = st.DailyTokensWindow(today, today)
	require.NoError(t, err)
	require.Empty(t, usage)
}

func TestUserUsageCannotCrossIdentityOrAffectNonUsageConditions(t *testing.T) {
	ap := New(nil, nil, Config{})
	ctx := WithUserUsageBearer(context.Background(), 527, "token")
	acts := []lottery.Activity{{Conditions: []lottery.ConditionDef{{Dimension: lottery.DimensionTokenUsage, WindowDays: 7}}}}
	_, err := ap.userUsageWindow(ctx, 528, "", acts)
	require.ErrorIs(t, err, ErrUsageUnavailable)
	acts[0].Conditions[0].Dimension = lottery.DimensionHistoricalRecharge
	usage, err := ap.userUsageWindow(ctx, 527, "", acts)
	require.NoError(t, err)
	require.Empty(t, usage)
}
