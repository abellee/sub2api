package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountInflationDirectHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AccountHandler{}
	router := gin.New()
	router.GET("/capabilities", h.InflationCapabilities)
	router.POST("/direct", h.ProbeInflationDirect)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/capabilities", nil))
	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), `"direct_probe":true`)
	require.Contains(t, rec.Body.String(), `"custom_timeout":true`)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/direct", strings.NewReader(`{"api_key":"test-secret",bad`)))
	require.Equal(t, 400, rec.Code)
	require.NotContains(t, rec.Body.String(), "test-secret")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/direct", strings.NewReader(`{"api_key":"test-secret"}`)))
	require.Equal(t, 500, rec.Code)
	require.NotContains(t, rec.Body.String(), "test-secret")
}
