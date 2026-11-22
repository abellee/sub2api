package admin

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettingsAccountFailoverSwitchPersistsAndPreservesOmission(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{})
	require.False(t, h.settingService.IsAccountErrorFailoverEnabled(context.Background()))
	for _, enabled := range []bool{true, false} {
		rec := doUpdateSettings(t, h, map[string]any{"account_error_failover_enabled": enabled}, nil)
		require.Equal(t, 200, rec.Code, rec.Body.String())
		require.Equal(t, enabled, h.settingService.IsAccountErrorFailoverEnabled(context.Background()))
		want := "false"
		if enabled {
			want = "true"
		}
		require.Equal(t, want, repo.values[service.SettingKeyAccountErrorFailoverEnabled])
		require.Contains(t, rec.Body.String(), `"account_error_failover_enabled":`+want)
		rec = doUpdateSettings(t, h, map[string]any{"site_name": "preserve"}, nil)
		require.Equal(t, 200, rec.Code, rec.Body.String())
		require.Equal(t, want, repo.values[service.SettingKeyAccountErrorFailoverEnabled])
		require.Equal(t, enabled, h.settingService.IsAccountErrorFailoverEnabled(context.Background()))

		get := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(get)
		c.Request = httptest.NewRequest("GET", "/api/v1/admin/settings", nil)
		h.GetSettings(c)
		require.Equal(t, 200, get.Code, get.Body.String())
		require.Contains(t, get.Body.String(), `"account_error_failover_enabled":`+want)
	}
}
