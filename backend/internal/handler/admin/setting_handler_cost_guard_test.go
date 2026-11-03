package admin

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettingsCostGuardPersistsSwitchAndPreservesOmission(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{})
	guard := service.NewCostGuardService(h.settingService)
	t.Cleanup(guard.Stop)
	require.False(t, service.CostGuardEnabled())
	for _, enabled := range []bool{true, false} {
		rec := doUpdateSettings(t, h, map[string]any{"cost_guard_enabled": enabled}, nil)
		require.Equal(t, 200, rec.Code, rec.Body.String())
		require.Equal(t, enabled, service.CostGuardEnabled(), "save should take effect immediately")
		want := "false"
		if enabled {
			want = "true"
		}
		require.Equal(t, want, repo.values[service.SettingKeyCostGuardEnabled])
		require.Contains(t, rec.Body.String(), `"cost_guard_enabled":`+want)
		rec = doUpdateSettings(t, h, map[string]any{"site_name": "preserve"}, nil)
		require.Equal(t, 200, rec.Code, rec.Body.String())
		require.Equal(t, enabled, service.CostGuardEnabled())
		require.Equal(t, want, repo.values[service.SettingKeyCostGuardEnabled])
	}
}

func TestSettingsCostGuardLoadsSavedSwitchAtStartup(t *testing.T) {
	h, _ := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyCostGuardEnabled:         "true",
		service.SettingKeyCostGuardErrorStatusCode: "402",
		service.SettingKeyCostGuardErrorMessage:    "请充值后重试",
	})
	guard := service.NewCostGuardService(h.settingService)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(guard.Stop)
	require.NoError(t, guard.Start(ctx))
	require.True(t, service.CostGuardEnabled())
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/settings", nil)
	h.GetSettings(c)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"cost_guard_enabled":true`)
	require.Contains(t, rec.Body.String(), `"cost_guard_error_status_code":402`)
	require.Contains(t, rec.Body.String(), `"cost_guard_error_message":"请充值后重试"`)
	var rejection *service.CostGuardRejectedError
	require.ErrorAs(t, service.CostGuardRejection(), &rejection)
	require.Equal(t, 402, rejection.StatusCode)
	require.Equal(t, "请充值后重试", rejection.Message)
}

func TestSettingsCostGuardErrorResponseSaveOmissionAndValidation(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{})
	guard := service.NewCostGuardService(h.settingService)
	t.Cleanup(guard.Stop)
	rec := doUpdateSettings(t, h, map[string]any{
		"cost_guard_enabled": true, "cost_guard_error_status_code": 429,
		"cost_guard_error_message": "预估费用超出可用余额",
	}, nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var rejection *service.CostGuardRejectedError
	require.ErrorAs(t, service.CostGuardRejection(), &rejection)
	require.Equal(t, 429, rejection.StatusCode)
	require.Equal(t, "预估费用超出可用余额", rejection.Message)
	rec = doUpdateSettings(t, h, map[string]any{"site_name": "preserve"}, nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Equal(t, "429", repo.values[service.SettingKeyCostGuardErrorStatusCode])
	require.Equal(t, rejection.Message, repo.values[service.SettingKeyCostGuardErrorMessage])
	for _, body := range []map[string]any{
		{"cost_guard_error_status_code": 200}, {"cost_guard_error_status_code": 600},
		{"cost_guard_error_status_code": 0}, {"cost_guard_error_message": strings.Repeat("长", 2001)},
	} {
		rec = doUpdateSettings(t, h, body, nil)
		require.Equal(t, 400, rec.Code, rec.Body.String())
		require.Equal(t, "429", repo.values[service.SettingKeyCostGuardErrorStatusCode])
	}
	rec = doUpdateSettings(t, h, map[string]any{"cost_guard_error_message": ""}, nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Equal(t, service.DefaultCostGuardErrorMessage, repo.values[service.SettingKeyCostGuardErrorMessage])
	// Errors already returned keep their original response even after a setting save.
	require.Equal(t, "预估费用超出可用余额", rejection.Message)
}
