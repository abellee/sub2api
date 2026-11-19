package routes

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

type frontendTestSettings struct{ enabled bool }

func (s frontendTestSettings) GetPublicTransitRuntime(context.Context) service.PublicTransitRuntime {
	return service.PublicTransitRuntime{PageEnabled: s.enabled}
}
func TestFrontendPageGate(t *testing.T) {
	for _, tc := range []struct {
		path    string
		enabled bool
		status  int
	}{
		{"/", false, 204}, {"/admin/users", false, 204},
		{"/public/transit", false, 403}, {"/public/transit/?x=1", false, 403},
		{"/public/transit", true, 204}, {"/public%2ftransit", false, 403}, {"", false, 403},
	} {
		r := gin.New()
		RegisterFrontendRoutes(r, frontendTestSettings{tc.enabled})
		req := httptest.NewRequest("GET", "/internal/frontend/check", nil)
		req.Header.Set("X-Frontend-Path", tc.path)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Errorf("path %q enabled %v: got %d want %d", tc.path, tc.enabled, w.Code, tc.status)
		}
	}
}
