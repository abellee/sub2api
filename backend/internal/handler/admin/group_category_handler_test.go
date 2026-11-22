//go:build unit

package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupCategoryHandlerTags(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewGroupCategoryHandler(service.NewGroupCategoryStore(filepath.Join(t.TempDir(), "config.db")))
	router := gin.New()
	router.PUT("/group-tags/:group_id", handler.SetGroupTags)
	// The authenticated user route uses this same List handler.
	router.GET("/groups/categories", handler.List)
	for _, tc := range []struct {
		id, body string
		status   int
	}{
		{"42", `{"tags":[{"name":" 快速 ","description":" 悬停说明 "},{"name":"稳定"}]}`, 200},
		{"0", `{"tags":[]}`, 400},
		{"abc", `{"tags":[]}`, 400},
		{"42", `{}`, 400},
		{"42", `{"tags":null}`, 400},
		{"42", `{"tags":[{"name":" "}]}`, 400},
		{"42", `{"tags":"invalid"}`, 400},
	} {
		w := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/group-tags/"+tc.id, strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, request)
		require.Equal(t, tc.status, w.Code, w.Body.String())
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/groups/categories", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var result struct {
		Data service.GroupCategorySnapshot `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.Equal(t, []service.GroupTag{{Name: "快速", Description: "悬停说明"}, {Name: "稳定"}}, result.Data.GroupTags["42"])
	w = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/group-tags/42", strings.NewReader(`{"tags":[]}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, request)
	require.Equal(t, http.StatusOK, w.Code)
	snapshot, err := handler.store.List()
	require.NoError(t, err)
	require.Empty(t, snapshot.GroupTags)
}
