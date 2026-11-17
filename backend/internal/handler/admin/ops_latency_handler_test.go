package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLatencyCollectionInputValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		query string
		valid bool
	}{{"before=-1", false}, {"before=abc", false}, {"before=20", true}, {"", true}} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/?"+tc.query, nil)
		_, ok := latencyCursor(c)
		require.Equal(t, tc.valid, ok)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	_, ok := latencyID(c, "not-a-uuid", false)
	require.False(t, ok)
	require.Equal(t, 400, w.Code)
	_, ok = latencyID(c, "", true)
	require.True(t, ok)
}

func TestLatencyCollectionsNotGrantedToExistingScopedKeys(t *testing.T) {
	policy := adminaccess.Policy{}
	for _, permission := range adminaccess.PermissionCatalog() {
		policy.GlobalPermissions = append(policy.GlobalPermissions, permission.ID)
	}
	for _, route := range []struct{ method, path string }{{"GET", "/ops/latency-collections"}, {"POST", "/ops/latency-collections"}, {"GET", "/ops/latency-collections/:id/records"}, {"POST", "/ops/latency-collections/:id/stop"}} {
		require.False(t, adminaccess.RouteAllowed(policy, route.method, "/api/v1/admin"+route.path))
	}
}
