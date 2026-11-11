package server

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestActivityNotificationChannelsRoundTrip(t *testing.T) {
	e := newTestEnv(t)
	defer e.srv.Close()
	admin := signJWT(e.secret, 1, "admin")
	for _, channels := range [][]string{nil, {}, {"browser"}, {"group"}, {"browser", "group"}} {
		input := sampleActivity(time.Now().Add(time.Hour), time.Now().Add(2*time.Hour))
		input["notify_stages"] = []string{"started", "results"}
		if channels != nil {
			input["notify_channels"] = channels
		}
		id := int64(mustJSON(t, e.req(t, "POST", "/v1/admin/activities", admin, input))["id"].(float64))
		path := fmt.Sprintf("/v1/admin/activities/%d", id)
		detail := mustJSON(t, e.req(t, "GET", path, admin, nil))
		want := []any{}
		if channels == nil {
			want = []any{"browser", "group"}
		} else {
			for _, c := range channels {
				want = append(want, c)
			}
		}
		require.Equal(t, want, detail["notify_channels"])
		input["notify_channels"] = []string{}
		mustJSON(t, e.req(t, "PUT", path, admin, input))
		detail = mustJSON(t, e.req(t, "GET", path, admin, nil))
		require.Equal(t, []any{}, detail["notify_channels"])
		require.Equal(t, false, detail["notify"])
	}
	input := sampleActivity(time.Now().Add(time.Hour), time.Now().Add(2*time.Hour))
	input["notify_channels"] = []string{"bad"}
	response := e.req(t, "POST", "/v1/admin/activities", admin, input)
	require.Equal(t, 400, response.StatusCode)
	response.Body.Close()
}
