package push

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListSubscribers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/push-api/v1/internal/subscribers", r.URL.Path)
		require.Equal(t, "secret", r.Header.Get("X-Push-Token"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"users": []map[string]any{{"id": 7, "email": "ada@example.com"}},
		})
	}))
	defer server.Close()

	users, err := New(server.URL, "secret").ListSubscribers(context.Background())
	require.NoError(t, err)
	require.Equal(t, []Subscriber{{ID: 7, Email: "ada@example.com"}}, users)
}

func TestNotifyEmptyUserIDsIsExplicit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var body map[string]any
		require.NoError(t, json.Unmarshal(raw, &body))
		ids, ok := body["userIDs"].([]any)
		require.True(t, ok)
		require.Empty(t, ids)
		_ = json.NewEncoder(w).Encode(Result{})
	}))
	defer server.Close()

	_, err := New(server.URL, "secret").Notify(context.Background(), Message{
		Title:   "抽奖即将开始",
		Body:    "没有符合条件的用户",
		URL:     "/lottery",
		UserIDs: []int64{},
	})
	require.NoError(t, err)
}
