//go:build unit

package admin

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupDocumentationBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := service.NewGroupCategoryStore(filepath.Join(t.TempDir(), "groups.db"))
	require.NoError(t, store.SetGroupDocumentation(12, "banana"))
	router := gin.New()
	router.PUT("/documentation/:group_id", NewGroupCategoryHandler(store).SetDocumentation)
	for _, tc := range []struct {
		id, body string
		status   int
		want     string
	}{
		{"12", `{}`, http.StatusBadRequest, "banana"},
		{"12", `{"document_id":null}`, http.StatusBadRequest, "banana"},
		{"12", `{"document_id":"unknown"}`, http.StatusBadRequest, "banana"},
		{"0", `{"document_id":"video"}`, http.StatusBadRequest, "banana"},
		{"12", `{"document_id":"sd-video"}`, http.StatusOK, "sd-video"},
		{"12", `{"document_id":""}`, http.StatusOK, ""},
	} {
		req := httptest.NewRequest(http.MethodPut, "/documentation/"+tc.id, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		require.Equal(t, tc.status, response.Code, response.Body.String())
		snapshot, err := store.List()
		require.NoError(t, err)
		require.Equal(t, tc.want, snapshot.Documentation["12"])
	}
}
