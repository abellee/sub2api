package admin

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountBaseURLFilterPropagatesAcrossListAndExport(t *testing.T) {
	for _, path := range []string{"/api/v1/admin/accounts", "/api/v1/admin/accounts/data", "/api/v1/admin/accounts/upstream-billing-rates"} {
		t.Run(path, func(t *testing.T) {
			svc := newStubAdminService()
			h := NewAccountHandler(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			router := gin.New()
			router.GET("/api/v1/admin/accounts", h.List)
			router.GET("/api/v1/admin/accounts/data", h.ExportData)
			router.GET("/api/v1/admin/accounts/upstream-billing-rates", h.GetUpstreamBillingRates)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("GET", path+"?base_url="+url.QueryEscape("  https://api.example.com/v1  ")+"&platform=openai", nil))
			require.Equal(t, 200, w.Code, w.Body.String())
			require.Equal(t, "https://api.example.com/v1", svc.lastListAccounts.baseURL)
			require.Equal(t, "openai", svc.lastListAccounts.platform)
		})
	}
	filters := toServiceBulkUpdateAccountFilters(&BulkUpdateAccountFilters{BaseURL: "api.example.com"})
	require.Equal(t, "api.example.com", filters.BaseURL)
}
