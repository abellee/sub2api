package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIAccountSummaryRejectsScopedAdmin(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/api-summary", nil).
		WithContext(adminaccess.WithPrincipal(context.Background(), &adminaccess.Principal{}))
	(&AccountHandler{}).GetAPIAccountSummary(c)
	require.Equal(t, http.StatusForbidden, c.Writer.Status())
}
