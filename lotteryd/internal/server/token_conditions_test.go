package server

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"lotteryd/internal/lottery"
)

// Exercise configuration persistence, authoritative user usage, list eligibility,
// prompts and participation together for all three token condition modes.
func TestTokenConditionsFromConfigurationThroughParticipation(t *testing.T) {
	e := newTestEnv(t)
	defer e.srv.Close()
	e.app.Cfg.DisableSettlement = true
	now := time.Now().In(lottery.Beijing())
	admin := signJWT(e.secret, 1, "admin")
	conditions := []map[string]any{
		{"dimension": "activity_days", "window_days": 7, "min_active_days": 1, "daily_min_tokens": 1e6, "bonus_mode": "none"},
		{"dimension": "token_usage", "window_days": 7, "mode": "per_day", "threshold": 1e6, "bonus_mode": "none"},
		{"dimension": "token_usage", "window_days": 7, "mode": "total", "threshold": 1e6, "bonus_mode": "none"},
	}
	ids := make([]int64, 3)
	for i, condition := range conditions {
		input := sampleActivity(now.Add(-time.Hour), now.Add(time.Hour))
		input["conditions"] = []map[string]any{condition}
		ids[i] = int64(mustJSON(t, e.req(t, "POST", "/v1/admin/activities", admin, input))["id"].(float64))
		saved := mustJSON(t, e.req(t, "GET", fmt.Sprintf("/v1/admin/activities/%d", ids[i]), admin, nil))["activity"].(map[string]any)["conditions"].([]any)[0].(map[string]any)
		for key, value := range condition {
			switch n := value.(type) {
			case int:
				require.Equal(t, float64(n), saved[key])
			default:
				require.Equal(t, value, saved[key])
			}
		}
	}
	e.app.Sub2API.SetAdminAPIKey("") // eligibility must not depend on background snapshots
	for index, tc := range []struct {
		name string
		days map[int]float64
		want [3]bool
	}{
		{"no_usage", nil, [3]bool{false, false, false}},
		{"one_day_exact", map[int]float64{1: 1e6}, [3]bool{true, false, true}},
		{"one_token_short", map[int]float64{1: 999999}, [3]bool{false, false, false}},
		{"split_total_exact", map[int]float64{1: 500000, 2: 500000}, [3]bool{false, false, true}},
		{"split_total_short", map[int]float64{1: 500000, 2: 499999}, [3]bool{false, false, false}},
		{"all_days_exact", map[int]float64{1: 1e6, 2: 1e6, 3: 1e6, 4: 1e6, 5: 1e6, 6: 1e6, 7: 1e6}, [3]bool{true, true, true}},
		{"one_day_short", map[int]float64{1: 999999, 2: 1e6, 3: 1e6, 4: 1e6, 5: 1e6, 6: 1e6, 7: 1e6}, [3]bool{true, false, true}},
		{"missing_day", map[int]float64{1: 100e6, 2: 1e6, 3: 1e6, 4: 1e6, 5: 1e6, 6: 1e6}, [3]bool{true, false, true}},
		{"oldest_day", map[int]float64{7: 1e6}, [3]bool{true, false, true}},
		{"outside_window", map[int]float64{0: 100e6, 8: 100e6}, [3]bool{false, false, false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			userID := int64(600 + index)
			user := signJWT(e.secret, userID, "user")
			for offset, tokens := range tc.days {
				date := now.AddDate(0, 0, -offset).Format("2006-01-02")
				if e.fake.dailyTokens[date] == nil {
					e.fake.dailyTokens[date] = map[int64]float64{}
				}
				e.fake.dailyTokens[date][userID] = tokens
			}
			views := mustJSON(t, e.req(t, "GET", "/v1/activities", user, nil))["activities"].([]any)
			eligible := mustJSON(t, e.req(t, "GET", "/v1/eligibility", user, nil))["activities"].([]any)
			for i, id := range ids {
				found := false
				for _, raw := range views {
					view := raw.(map[string]any)
					if int64(view["id"].(float64)) != id {
						continue
					}
					found = true
					require.Equal(t, tc.want[i], view["eligible"], "list condition %d", i)
					require.Equal(t, tc.want[i], view["conditions"].([]any)[0].(map[string]any)["satisfied"])
				}
				require.True(t, found)
				prompted := false
				for _, raw := range eligible {
					if int64(raw.(map[string]any)["id"].(float64)) == id {
						prompted = true
					}
				}
				require.Equal(t, tc.want[i], prompted, "prompt condition %d", i)
				resp := e.req(t, "POST", fmt.Sprintf("/v1/activities/%d/participate", id), user, nil)
				status := http.StatusForbidden
				if tc.want[i] {
					status = http.StatusOK
				}
				require.Equal(t, status, resp.StatusCode, "participation condition %d", i)
				resp.Body.Close()
				joined, err := e.app.Store.HasParticipant(id, userID)
				require.NoError(t, err)
				require.Equal(t, tc.want[i], joined)
			}
		})
	}
}
