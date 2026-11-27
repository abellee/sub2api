package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"lotteryd/internal/sub2api"
)

func TestPointsRoutesAuthorizationAndInputValidation(t *testing.T) {
	handler := New(nil, "checkin-test", "test").Handler()
	for _, route := range []struct{ method, path string }{
		{"GET", "/v1/me/points/check-in"}, {"POST", "/v1/me/points/check-in"},
		{"GET", "/v1/admin/points/settings"}, {"PUT", "/v1/admin/points/settings"},
		{"GET", "/v1/admin/users/2/points"}, {"POST", "/v1/admin/users/2/points"},
	} {
		for _, role := range []string{"", "user"} {
			if role == "user" && !strings.Contains(route.path, "/admin/") {
				continue
			}
			w := httptest.NewRecorder()
			r := httptest.NewRequest(route.method, route.path, nil)
			if role != "" {
				r.Header.Set("Authorization", "Bearer "+signJWT("checkin-test", 2, role))
			}
			handler.ServeHTTP(w, r)
			if role == "" {
				require.Equal(t, 401, w.Code)
			} else {
				require.Equal(t, 403, w.Code)
			}
		}
	}
	for _, body := range []string{
		`{}`, `{"daily_checkin_points":10}`, `{"daily_checkin_rewards":null}`, `{"daily_checkin_rewards":[]}`,
		`{"daily_checkin_rewards":[10,10,10,10,10,10]}`, `{"daily_checkin_rewards":[10,10,10,10,10,10,10,10]}`,
		`{"daily_checkin_rewards":[10,10,10,10,10,10,0]}`, `{"daily_checkin_rewards":[10,10,10,10,10,10,-1]}`,
		`{"daily_checkin_rewards":[10,10,10,10,10,10,1.5]}`, `{"daily_checkin_rewards":[10,10,10,10,10,10,2000001]}`,
		`{"daily_checkin_rewards":[10,10,10,10,10,10,null]}`, `{"daily_checkin_rewards":[10,10,10,10,10,10,"10"]}`,
		`{"daily_checkin_rewards":[10,10,10,10,10,10,10],"unknown":1}`,
	} {
		r := httptest.NewRequest("PUT", "/v1/admin/points/settings", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+signJWT("checkin-test", 2, "admin"))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		require.Equal(t, 400, w.Code, body)
	}
	for _, amount := range []string{"0", "-1", "1.5", "1000000000001", "no"} {
		body := fmt.Sprintf(`{"amount":%q,"request_id":"e518279a-0ebe-4131-b2a6-85026b853bc8"}`, amount)
		r := httptest.NewRequest("POST", "/v1/admin/users/2/points", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+signJWT("checkin-test", 2, "admin"))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		require.Equal(t, 400, w.Code, body)
	}
}

func TestPointsCheckInSettingsAndAdminRecharge(t *testing.T) {
	e := newTestEnv(t)
	defer e.srv.Close()
	user := signJWT(e.secret, 2, "user")
	admin := signJWT(e.secret, 2, "admin")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/admin/users/2", r.URL.Path)
		require.Equal(t, "Bearer "+admin, r.Header.Get("Authorization"))
		fmt.Fprint(w, `{"code":0,"data":{"id":2,"email":"user2@example.com"}}`)
	}))
	defer upstream.Close()
	e.app.Sub2API = sub2api.NewClient(upstream.URL, "")
	rewards := []int64{30, 18, 50, 12, 40, 7, 9}
	settings := mustJSON(t, e.req(t, "PUT", "/v1/admin/points/settings", admin, map[string]any{"daily_checkin_rewards": rewards}))
	require.Equal(t, []any{float64(30), float64(18), float64(50), float64(12), float64(40), float64(7), float64(9)}, settings["daily_checkin_rewards"])
	settings = mustJSON(t, e.req(t, "GET", "/v1/admin/points/settings", admin, nil))
	require.EqualValues(t, 7, settings["daily_checkin_rewards"].([]any)[5])
	status := mustJSON(t, e.req(t, "GET", "/v1/me/points/check-in?user_id=999", user, nil))
	require.Equal(t, false, status["claimed"])
	require.EqualValues(t, 30, status["today_reward"])
	data := mustJSON(t, e.req(t, "POST", "/v1/me/points/check-in", user, map[string]any{"user_id": 999, "day": 7, "amount": 999999}))
	require.Equal(t, true, data["granted"])
	require.Equal(t, "30", data["points"].(map[string]any)["balance"])
	data = mustJSON(t, e.req(t, "POST", "/v1/me/points/check-in", user, nil))
	require.Equal(t, false, data["granted"])
	require.Equal(t, "30", data["points"].(map[string]any)["balance"])
	body := map[string]any{"amount": "70", "notes": "support credit", "request_id": "e518279a-0ebe-4131-b2a6-85026b853bc8"}
	for i := 0; i < 2; i++ {
		data = mustJSON(t, e.req(t, "POST", "/v1/admin/users/2/points", admin, body))
		require.Equal(t, i == 0, data["granted"])
		require.Equal(t, "100", data["points"].(map[string]any)["balance"])
	}
	data = mustJSON(t, e.req(t, "GET", "/v1/admin/users/2/points", admin, nil))
	require.Equal(t, "100", data["balance"])
	data = mustJSON(t, e.req(t, "GET", "/v1/me/points/transactions", user, nil))
	require.EqualValues(t, 2, data["total"])
	require.Contains(t, data["items"].([]any)[0].(map[string]any)["note"], "support credit")
	body["amount"] = "80"
	resp := e.req(t, "POST", "/v1/admin/users/2/points", admin, body)
	defer resp.Body.Close()
	require.Equal(t, 409, resp.StatusCode)
}
