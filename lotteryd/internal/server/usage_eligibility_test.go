package server

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"lotteryd/internal/lottery"
)

func TestEligibilityUsesLiveUsageWithoutAdminKeyOrSnapshot(t *testing.T) {
	e := newTestEnv(t)
	defer e.srv.Close()
	admin := signJWT(e.secret, 1, "admin")
	user := signJWT(e.secret, 527, "user")
	now := time.Now().In(lottery.Beijing())
	input := sampleActivity(now.Add(-time.Hour), now.Add(time.Hour))
	input["conditions"] = []map[string]any{{"dimension": "activity_days", "window_days": 7, "min_active_days": 7, "daily_min_tokens": 1e6, "bonus_mode": "none"}}
	id := int64(mustJSON(t, e.req(t, "POST", "/v1/admin/activities", admin, input))["id"].(float64))
	join := fmt.Sprintf("/v1/activities/%d/participate", id)
	e.app.Sub2API.SetAdminAPIKey("")
	for i := 1; i <= 7; i++ {
		e.fake.dailyTokens[now.AddDate(0, 0, -i).Format("2006-01-02")] = map[int64]float64{527: 1e6, 528: 10e6}
	}
	views := mustJSON(t, e.req(t, "GET", "/v1/activities", user, nil))["activities"].([]any)
	require.True(t, views[0].(map[string]any)["eligible"].(bool))
	require.Len(t, mustJSON(t, e.req(t, "GET", "/v1/eligibility", user, nil))["activities"], 1)
	// WebSocket prompts use the same authenticated source.
	c := &wsClient{userID: 527, email: "user527@test.com", bearer: user, send: make(chan wsOutMessage, 1), lastIDs: map[int64]bool{}}
	h := newWSHub(New(e.app, e.secret, "test"))
	require.True(t, h.pushLottery(context.Background(), c))
	require.Len(t, c.send, 1)
	// A real empty/under-threshold day is ineligible; today's tokens cannot fill yesterday.
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	e.fake.dailyTokens[yesterday][527] = 999999
	e.fake.dailyTokens[now.Format("2006-01-02")] = map[int64]float64{527: 100e6}
	resp := e.req(t, "POST", join, user, nil)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
	// An upstream error must never masquerade as failed activity conditions.
	e.fake.usageStatus = 503
	for _, route := range []struct{ method, path string }{{"GET", "/v1/activities"}, {"GET", "/v1/eligibility"}, {"POST", join}} {
		resp = e.req(t, route.method, route.path, user, nil)
		require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
		resp.Body.Close()
	}
	e.fake.usageStatus = 0
	e.fake.dailyTokens[yesterday][527] = 1e6
	mustJSON(t, e.req(t, "POST", join, user, nil))
	joined, err := e.app.Store.HasParticipant(id, 527)
	require.NoError(t, err)
	require.True(t, joined)
	// User credentials never populate the all-user background snapshot.
	usage, err := e.app.Store.DailyTokensWindow(yesterday, yesterday)
	require.NoError(t, err)
	require.Empty(t, usage)
}
