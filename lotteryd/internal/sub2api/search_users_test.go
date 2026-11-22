package sub2api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchUsersAdminAuthentication(t *testing.T) {
	for _, key := range []string{"", "configured-admin-key"} {
		t.Run(key, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/api/v1/admin/users", r.URL.Path)
				require.Equal(t, "Alice", r.URL.Query().Get("search"))
				require.Equal(t, "20", r.URL.Query().Get("page_size"))
				if key == "" {
					require.Equal(t, "Bearer admin-session", r.Header.Get("Authorization"))
					require.Empty(t, r.Header.Get("x-api-key"))
				} else {
					require.Equal(t, key, r.Header.Get("x-api-key"))
					require.Empty(t, r.Header.Get("Authorization"))
				}
				fmt.Fprint(w, `{"code":0,"data":{"items":[{"id":7,"email":"alice@test.com","username":"Alice"}]}}`)
			}))
			defer srv.Close()
			users, err := NewClient(srv.URL, key).SearchUsers(context.Background(), " Alice ", 20, "admin-session")
			require.NoError(t, err)
			require.Equal(t, []SearchUser{{ID: 7, Email: "alice@test.com", Username: "Alice"}}, users)
		})
	}
}
