package server

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"lotteryd/internal/app"
	"lotteryd/internal/cache"
	"lotteryd/internal/lottery"
)

func enableRedis(t *testing.T, e *testEnv) *miniredis.Miniredis {
	t.Helper()
	r := miniredis.RunT(t)
	c, err := cache.New("redis://"+r.Addr(), t.Name())
	require.NoError(t, err)
	t.Cleanup(func() { c.Close() })
	e.app.Store.SetCache(c)
	return r
}

func TestCachedPromptsNeverAuthorizeParticipationAndKeepUsersIsolated(t *testing.T) {
	e := newTestEnv(t)
	defer e.srv.Close()
	r := enableRedis(t, e)
	admin, user, other := signJWT(e.secret, 1, "admin"), signJWT(e.secret, 527, "user"), signJWT(e.secret, 528, "user")
	now := time.Now().In(lottery.Beijing())
	input := sampleActivity(now.Add(-time.Hour), now.Add(time.Hour))
	input["conditions"] = []map[string]any{{"dimension": "token_usage", "window_days": 1, "mode": "total", "threshold": 1e6, "bonus_mode": "none"}}
	id := int64(mustJSON(t, e.req(t, "POST", "/v1/admin/activities", admin, input))["id"].(float64))
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	e.fake.dailyTokens[yesterday] = map[int64]float64{527: 1e6, 528: 0}
	require.Len(t, mustJSON(t, e.req(t, "GET", "/v1/eligibility", user, nil))["activities"], 1)
	require.Empty(t, mustJSON(t, e.req(t, "GET", "/v1/eligibility", other, nil))["activities"])
	e.fake.dailyTokens[yesterday][527] = 0
	resp := e.req(t, "POST", fmt.Sprintf("/v1/activities/%d/participate", id), user, nil)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp.Body.Close()
	joined, err := e.app.Store.HasParticipant(id, 527)
	require.NoError(t, err)
	require.False(t, joined)
	r.FastForward(16 * time.Second)
	require.Empty(t, mustJSON(t, e.req(t, "GET", "/v1/eligibility", user, nil))["activities"])
	// A failing authoritative fetch cannot populate the next cache entry.
	r.FastForward(16 * time.Second)
	e.fake.usageStatus = 503
	resp = e.req(t, "GET", "/v1/eligibility", user, nil)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	resp.Body.Close()
	e.fake.usageStatus = 0
	e.fake.dailyTokens[yesterday][527] = 1e6
	require.Len(t, mustJSON(t, e.req(t, "GET", "/v1/eligibility", user, nil))["activities"], 1)
	mustJSON(t, e.req(t, "POST", fmt.Sprintf("/v1/activities/%d/participate", id), user, nil))
	require.Empty(t, mustJSON(t, e.req(t, "GET", "/v1/eligibility", user, nil))["activities"])
}

func TestSocketTabsShareChecksAndSkipAlreadyPromptedActivities(t *testing.T) {
	e := newTestEnv(t)
	defer e.srv.Close()
	enableRedis(t, e)
	admin, user := signJWT(e.secret, 1, "admin"), signJWT(e.secret, 527, "user")
	now := time.Now()
	input := sampleActivity(now.Add(-time.Hour), now.Add(time.Hour))
	input["conditions"] = []any{}
	id := int64(mustJSON(t, e.req(t, "POST", "/v1/admin/activities", admin, input))["id"].(float64))
	h := newWSHub(New(e.app, e.secret, "test"))
	a := &wsClient{userID: 527, email: "user527@test.com", bearer: user, send: make(chan wsOutMessage, 4), lastIDs: map[int64]bool{}, lastTaskIDs: map[int64]bool{}}
	b := &wsClient{userID: 527, email: a.email, bearer: user, send: make(chan wsOutMessage, 4), lastIDs: map[int64]bool{}, lastTaskIDs: map[int64]bool{}}
	h.add(a)
	h.add(b)
	h.broadcastEligibility(context.Background())
	require.Equal(t, id, (<-a.send).Activity.ID)
	require.Equal(t, id, (<-b.send).Activity.ID)
	require.Equal(t, map[int64]bool{id: true}, commonSeen([]*wsClient{a, b}, false))
	h.broadcastEligibility(context.Background())
	require.Empty(t, a.send)
	require.Empty(t, b.send)
	// An already-prompted round must be excluded before its usage API is called.
	activity, err := e.app.Store.GetActivity(id)
	require.NoError(t, err)
	activity.Conditions = []lottery.ConditionDef{{Dimension: lottery.DimensionTokenUsage, WindowDays: 1, Mode: "total", Threshold: 1, BonusMode: lottery.BonusNone}}
	require.NoError(t, e.app.Store.UpdateActivity(activity))
	e.fake.usageStatus = 503
	views, err := e.app.EligibilityListExcluding(app.WithUserUsageBearer(context.Background(), 527, user), 527, a.email, nil, map[int64]bool{id: true})
	require.NoError(t, err)
	require.Empty(t, views)
	h.broadcastEligibility(context.Background())
	require.Empty(t, a.send)
	require.Empty(t, b.send)
	// Closing a connection cannot race a send into a closed channel.
	h.remove(a)
	require.False(t, h.trySend(a, wsOutMessage{Type: "pong"}))
}

func TestSocketCommonSeenKeepsActivitiesNeededByAnotherTab(t *testing.T) {
	a := &wsClient{lastIDs: map[int64]bool{1: true, 2: true}, lastTaskIDs: map[int64]bool{5: true}}
	b := &wsClient{lastIDs: map[int64]bool{2: true, 3: true}, lastTaskIDs: map[int64]bool{6: true}}
	require.Equal(t, map[int64]bool{2: true}, commonSeen([]*wsClient{a, b}, false))
	require.Empty(t, commonSeen([]*wsClient{a, b}, true))
	require.Len(t, a.lastIDs, 2)
}
