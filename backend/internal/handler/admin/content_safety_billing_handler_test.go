package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestContentSafetyBillingAdminValidationAndPersistence(t *testing.T) {
	s, err := service.OpenContentSafetyBilling(filepath.Join(t.TempDir(), "billing.sqlite"), nil, nil)
	require.NoError(t, err)
	defer s.Close()
	h := &SettingHandler{}
	h.SetContentSafetyBilling(s)
	r := gin.New()
	r.GET("/config", h.GetContentSafetyBilling)
	r.PUT("/config", h.UpdateContentSafetyBilling)
	r.GET("/records", h.ListContentSafetyDebits)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
	valid := `{"enabled":true,"amount":0.01,"match_description":"SAFETY_CHECK_TYPE","client_message":"Content rejected"}`
	require.Equal(t, 200, request(http.MethodPut, "/config", valid).Code)
	for _, body := range []string{
		`{}`, `{"enabled":true}`, `{"enabled":true,"amount":-1,"match_description":"x","client_message":"y"}`,
		`{"enabled":true,"amount":0,"match_description":"x","client_message":"y"}`,
		`{"enabled":true,"amount":0.01,"match_description":"","client_message":"y"}`,
		`{"enabled":true,"amount":0.01,"match_description":"x","client_message":" "}`,
	} {
		require.Equal(t, 400, request(http.MethodPut, "/config", body).Code)
	}
	config, err := s.Config(context.Background())
	require.NoError(t, err)
	require.True(t, config.Enabled)
	require.Equal(t, 0.01, config.Amount)
	require.Equal(t, "SAFETY_CHECK_TYPE", config.MatchDescription)
	require.Contains(t, request(http.MethodGet, "/config", "").Body.String(), "Content rejected")
	require.Equal(t, 200, request(http.MethodGet, "/records?page=1&page_size=20", "").Code)
	require.Equal(t, 400, request(http.MethodGet, "/records?page=-1", "").Code)
	require.Equal(t, 400, request(http.MethodGet, "/records?page_size=101", "").Code)
}
