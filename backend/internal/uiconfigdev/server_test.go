package uiconfigdev

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMenuDevelopmentProxyAuthenticationAndPersistence(t *testing.T) {
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/auth/me", r.URL.Path)
		role := "user"
		switch r.Header.Get("Authorization") {
		case "Bearer admin":
			role = "admin"
		case "Bearer user":
		default:
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]string{"role": role}}))
	}))
	defer auth.Close()
	path := filepath.Join(t.TempDir(), "group_categories.db")
	h := NewHandler(service.NewGroupCategoryStore(path), auth.URL)
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, tc := range []struct {
		token string
		want  int
	}{
		{"", http.StatusUnauthorized}, {"expired", http.StatusUnauthorized}, {"user", http.StatusForbidden},
	} {
		require.Equal(t, tc.want, call(http.MethodPut, "/api/v1/admin/user-menu", tc.token, `{"sections":[]}`).Code)
	}
	w := call(http.MethodGet, "/api/v1/admin/user-menu", "admin", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"code":0,"message":"success","data":{"sections":[]}}`, w.Body.String())
	body := `{"sections":[{"id":"models","title":"模型","items":["/keys"]}]}`
	w = call(http.MethodPut, "/api/v1/admin/user-menu", "admin", body)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, http.StatusBadRequest, call(http.MethodPut, "/api/v1/admin/user-menu", "admin", `{"sections":[{"id":"a","title":"Bad","items":["/admin/users"]}]}`).Code)
	// Reopen the store and router, as on a server restart. A regular user reads
	// the exact same persisted configuration that the administrator saved.
	h = NewHandler(service.NewGroupCategoryStore(path), auth.URL)
	w = call(http.MethodGet, "/api/v1/user/menu", "user", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"code":0,"message":"success","data":`+body+`}`, w.Body.String())
}

func TestMenuDevelopmentProxyFailsClosed(t *testing.T) {
	for _, upstreamStatus := range []int{http.StatusInternalServerError, http.StatusFound} {
		t.Run(http.StatusText(upstreamStatus), func(t *testing.T) {
			auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", "http://127.0.0.1:1/")
				w.WriteHeader(upstreamStatus)
			}))
			defer auth.Close()
			h := NewHandler(service.NewGroupCategoryStore(filepath.Join(t.TempDir(), "group_categories.db")), auth.URL)
			r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/user-menu", nil)
			r.Header.Set("Authorization", "Bearer admin")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.Equal(t, http.StatusServiceUnavailable, w.Code)
		})
	}
}
