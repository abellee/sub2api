package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEventManagementRequiresAdministrator(t *testing.T) {
	handler := New(nil, "events-test", "test").Handler()
	for _, route := range []struct{ method, path string }{{"GET", "/v1/admin/events/catalog"}, {"GET", "/v1/admin/events/rules"}, {"POST", "/v1/admin/events/rules"}, {"PUT", "/v1/admin/events/rules/1"}, {"DELETE", "/v1/admin/events/rules/1"}, {"GET", "/v1/admin/events/logs"}, {"GET", "/v1/admin/events/executions"}, {"GET", "/v1/admin/events/settings"}, {"PUT", "/v1/admin/events/settings"}} {
		for _, authenticated := range []bool{false, true} {
			req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`))
			if authenticated {
				req.Header.Set("Authorization", "Bearer "+signJWT("events-test", 2, "user"))
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			want := http.StatusUnauthorized
			if authenticated {
				want = http.StatusForbidden
			}
			if w.Code != want {
				t.Fatalf("%s %s: got %d want %d", route.method, route.path, w.Code, want)
			}
		}
	}
}

func TestEventCatalogIsAvailableToAdministratorWithoutRewardExecution(t *testing.T) {
	handler := New(nil, "events-test", "test").Handler()
	req := httptest.NewRequest("GET", "/v1/admin/events/catalog", nil)
	req.Header.Set("Authorization", "Bearer "+signJWT("events-test", 2, "admin"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "billing.completed") || !strings.Contains(w.Body.String(), "shop.redeemed") {
		t.Fatalf("catalog unavailable: %d", w.Code)
	}
}
