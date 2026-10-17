package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"lotteryd/internal/sub2api"
)

func TestAdminUserSearchUsesCurrentAdminSession(t *testing.T) {
	e := newTestEnv(t)
	defer e.srv.Close()
	admin := signJWT(e.secret, 1, "admin")
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "Bearer "+admin, r.Header.Get("Authorization"))
		require.Equal(t, "alice", r.URL.Query().Get("search"))
		fmt.Fprint(w, `{"code":0,"data":{"items":[{"id":7,"email":"alice@test.com","username":"Alice"}]}}`)
	}))
	defer upstream.Close()
	e.app.Sub2API = sub2api.NewClient(upstream.URL, "")
	data := mustJSON(t, e.req(t, "GET", "/v1/admin/users/search?q=alice", admin, nil))
	require.Len(t, data["users"], 1)
	for _, token := range []string{"", signJWT(e.secret, 7, "user")} {
		resp := e.req(t, "GET", "/v1/admin/users/search?q=alice", token, nil)
		require.Contains(t, []int{401, 403}, resp.StatusCode)
		resp.Body.Close()
	}
	require.Equal(t, 1, calls)
}
