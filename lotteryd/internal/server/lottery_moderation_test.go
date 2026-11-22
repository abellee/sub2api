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

func TestLotteryRechargeBlacklistAndRemoval(t *testing.T) {
	e := newTestEnv(t)
	defer e.srv.Close()
	e.app.Cfg.DisableSettlement = true
	admin := signJWT(e.secret, 1, "admin")
	user := signJWT(e.secret, 7, "user")
	other := signJWT(e.secret, 8, "user")
	mustJSON(t, e.req(t, "PUT", "/v1/admin/settings", admin, map[string]any{"visibility": map[string]any{"mode": "all"}}))
	input := sampleActivity(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	input["conditions"] = []map[string]any{{"dimension": "historical_recharge", "threshold": 10, "window_days": 0, "bonus_mode": "none"}}
	input["blacklist"] = []string{" BLOCKED@test.com ", "blocked@test.com", " USER7@test.com "}
	id := int64(mustJSON(t, e.req(t, "POST", "/v1/admin/activities", admin, input))["id"].(float64))
	path := fmt.Sprintf("/v1/admin/activities/%d", id)
	join := fmt.Sprintf("/v1/activities/%d/participate", id)
	detail := mustJSON(t, e.req(t, "GET", path, admin, nil))["activity"].(map[string]any)
	require.Equal(t, []any{"blocked@test.com", "user7@test.com"}, detail["blacklist"])
	require.Equal(t, "historical_recharge", detail["conditions"].([]any)[0].(map[string]any)["dimension"])
	views := mustJSON(t, e.req(t, "GET", "/v1/activities", user, nil))["activities"].([]any)
	require.False(t, views[0].(map[string]any)["eligible"].(bool))
	require.NotContains(t, views[0].(map[string]any), "blacklist")
	resp := e.req(t, "POST", join, user, nil)
	require.Equal(t, 403, resp.StatusCode)
	resp.Body.Close()
	e.fake.recharged[7] = 10
	eligible := mustJSON(t, e.req(t, "GET", "/v1/eligibility", user, nil))["activities"].([]any)
	require.Len(t, eligible, 1)
	mustJSON(t, e.req(t, "POST", join, user, nil))
	e.fake.recharged[8] = 11
	mustJSON(t, e.req(t, "POST", join, other, nil))
	remove := path + "/participants/7"
	resp = e.req(t, "DELETE", remove, other, nil)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
	mustJSON(t, e.req(t, "DELETE", remove, admin, nil))
	resp = e.req(t, "POST", join, user, nil)
	require.Equal(t, 403, resp.StatusCode)
	resp.Body.Close()
	eligible = mustJSON(t, e.req(t, "GET", "/v1/eligibility", user, nil))["activities"].([]any)
	require.Empty(t, eligible)
	input["blacklist"] = []string{" USER8@TEST.COM "}
	mustJSON(t, e.req(t, "PUT", path, admin, input))
	people := mustJSON(t, e.req(t, "GET", path+"/participants", admin, nil))["participants"].([]any)
	require.Len(t, people, 1)
	stale := &lottery.Activity{ID: id}
	require.NoError(t, e.app.DrawActivity(context.Background(), stale))
	wins, err := e.app.Store.ListWinnersByActivity(id)
	require.NoError(t, err)
	require.Empty(t, wins)
	resp = e.req(t, "DELETE", path+"/participants/8", admin, nil)
	require.Equal(t, 409, resp.StatusCode)
	resp.Body.Close()
	resp = e.req(t, "PUT", path, admin, input)
	require.Equal(t, 409, resp.StatusCode)
	resp.Body.Close()
}

func TestLotteryBlacklistOnlyExcludesWinning(t *testing.T) {
	e := newTestEnv(t)
	defer e.srv.Close()
	e.app.Cfg.DisableSettlement = true
	now := time.Now().In(lottery.Beijing())
	admin := signJWT(e.secret, 1, "admin")
	input := sampleActivity(now.Add(-time.Hour), now.Add(time.Hour))
	input["blacklist"] = []string{" USER7@test.com "}
	input["visibility"] = map[string]any{"mode": "eligible"}
	input["max_participants"] = 2
	input["show_participant_count"] = true
	input["conditions"] = []map[string]any{{"dimension": "token_usage", "window_days": 7, "mode": "total", "threshold": 1e6, "bonus_mode": "none"}}
	input["prizes"] = []map[string]any{{"name": "余额奖", "prize_type": "balance", "value": 1, "weight": 1, "stock": 10}}
	id := int64(mustJSON(t, e.req(t, "POST", "/v1/admin/activities", admin, input))["id"].(float64))
	join := fmt.Sprintf("/v1/activities/%d/participate", id)
	e.app.Sub2API.SetAdminAPIKey("")
	e.fake.dailyTokens[now.AddDate(0, 0, -1).Format("2006-01-02")] = map[int64]float64{7: 1e6, 8: 1e6, 9: 1e6}
	for _, userID := range []int64{7, 8} {
		user := signJWT(e.secret, userID, "user")
		views := mustJSON(t, e.req(t, "GET", "/v1/activities", user, nil))["activities"].([]any)
		require.Len(t, views, 1, "blacklisted eligible users still see eligible-only activities")
		view := views[0].(map[string]any)
		require.True(t, view["eligible"].(bool))
		require.Empty(t, view["eligible_reason"])
		require.NotContains(t, view, "blacklist")
		require.True(t, view["conditions"].([]any)[0].(map[string]any)["satisfied"].(bool))
		prompts := mustJSON(t, e.req(t, "GET", "/v1/eligibility", user, nil))["activities"].([]any)
		require.Len(t, prompts, 1)
		client := &wsClient{userID: userID, email: fmt.Sprintf("user%d@test.com", userID), bearer: user, send: make(chan wsOutMessage, 1), lastIDs: map[int64]bool{}}
		hub := newWSHub(New(e.app, e.secret, "test"))
		require.True(t, hub.pushLottery(context.Background(), client))
		require.Len(t, client.send, 1)
		joined := mustJSON(t, e.req(t, "POST", join, user, nil))
		require.True(t, joined["joined"].(bool))
		require.NotContains(t, joined, "blacklist")
	}
	// Blacklisted entrants count toward capacity and remain in the participant list.
	people := mustJSON(t, e.req(t, "GET", fmt.Sprintf("/v1/admin/activities/%d/participants", id), admin, nil))["participants"].([]any)
	require.Len(t, people, 2)
	count, err := e.app.Store.CountParticipants(id)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
	for _, userID := range []int64{7, 9} {
		resp := e.req(t, "POST", join, signJWT(e.secret, userID, "user"), nil)
		require.Equal(t, http.StatusConflict, resp.StatusCode, "duplicate and capacity rules still apply")
		resp.Body.Close()
	}
	// With more prizes than entrants, only the normal entrant may win.
	require.NoError(t, e.app.DrawActivity(context.Background(), &lottery.Activity{ID: id}))
	winners, err := e.app.Store.ListWinnersByActivity(id)
	require.NoError(t, err)
	require.Len(t, winners, 1)
	require.Equal(t, int64(8), winners[0].UserID)
	joined, err := e.app.Store.HasParticipant(id, 7)
	require.NoError(t, err)
	require.True(t, joined, "excluding a winner must not remove their participation")
	views := mustJSON(t, e.req(t, "GET", "/v1/activities", signJWT(e.secret, 7, "user"), nil))["activities"].([]any)
	require.True(t, views[0].(map[string]any)["joined"].(bool))
	require.NotContains(t, views[0].(map[string]any), "won")
	require.NotContains(t, views[0].(map[string]any), "blacklist")
}
