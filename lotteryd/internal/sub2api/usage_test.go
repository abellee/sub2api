package sub2api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserUsageValidatesAuthoritativeResponse(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		valid      bool
	}{
		{"empty", `{"trend":[]}`, true},
		{"cache_inclusive_total", `{"trend":[{"date":"2026-10-17","input_tokens":1,"cache_read_tokens":999999,"total_tokens":1000000}]}`, true},
		{"missing", `{}`, false},
		{"null", `{"trend":null}`, false},
		{"missing_total", `{"trend":[{"date":"2026-10-17"}]}`, false},
		{"negative", `{"trend":[{"date":"2026-10-17","total_tokens":-1}]}`, false},
		{"outside_window", `{"trend":[{"date":"2026-10-18","total_tokens":1}]}`, false},
		{"duplicate", `{"trend":[{"date":"2026-10-17","total_tokens":1},{"date":"2026-10-17","total_tokens":1}]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/api/v1/usage/dashboard/trend", r.URL.Path)
				require.Equal(t, "Bearer user-token", r.Header.Get("Authorization"))
				require.Empty(t, r.Header.Get("x-api-key"))
				require.Equal(t, "Asia/Shanghai", r.URL.Query().Get("timezone"))
				fmt.Fprintf(w, `{"code":0,"data":%s}`, tc.data)
			}))
			defer srv.Close()
			rows, err := NewClient(srv.URL, "admin-key").FetchUserDailyTokens(context.Background(), "user-token", "2026-10-11", "2026-10-17")
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			if tc.name == "cache_inclusive_total" {
				require.Equal(t, 1e6, rows[0].TotalTokens)
			}
		})
	}
}
