package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestMediaAdminConfigAndAgent(t *testing.T) {
	var calls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer management-private" {
			t.Error("missing management token")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"schemas":[]}`))
	}))
	defer upstream.Close()
	root := t.TempDir()
	defaults := DefaultMediaHubSettings()
	defaults.Enabled = false
	defaults.MediadBaseURL = upstream.URL
	defaults.ManagementToken = "management-private"
	defaults.RelaySecret = "relay-private"
	defaults.CallbackSecret = "callback-private"
	defaults.StorePath = filepath.Join(root, "bindings.json")

	h, err := NewMediaHubController(filepath.Join(root, "settings.json"), defaults, nil)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	admin := router.Group("/admin", func(c *gin.Context) {
		if c.GetHeader("Authorization") != "Bearer admin" {
			c.AbortWithStatus(401)
		}
	})
	h.RegisterAdmin(admin)
	request := func(method, path, body, auth string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", auth)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "/admin/media-service/config", "", ""); w.Code != 401 {
		t.Fatal("unauthenticated configuration read", w.Code)
	}
	if w := request("GET", "/admin/media-service/config", "", "Bearer admin"); w.Code != 200 || strings.Contains(w.Body.String(), "-private") {
		t.Fatal("secret exposed or get failed", w.Body.String())
	}
	if w := request("PUT", "/admin/media-service/config", `{"management_token":"","forward_timeout_seconds":300}`, "Bearer admin"); w.Code != 200 || !strings.Contains(w.Body.String(), `"restart_required":true`) {
		t.Fatal(w.Body.String())
	}
	if w := request("GET", "/admin/media-service/agent/schemas", "", "Bearer admin"); w.Code != 200 || calls != 1 {
		t.Fatal("management proxy failed", w.Body.String())
	}
	if w := request("POST", "/admin/media-service/agent/schemas", "{}", "Bearer admin"); w.Code != 404 || calls != 1 {
		t.Fatal("unlisted method proxied")
	}
	if w := request("GET", "/admin/media-service/agent/tasks/secret", "", "Bearer admin"); w.Code != 404 || calls != 1 {
		t.Fatal("unlisted path proxied")
	}
	if w := request("PUT", "/admin/media-service/bindings/0", `{"schema_id":"one","estimate_kind":"image"}`, "Bearer admin"); w.Code != 400 {
		t.Fatal("invalid group accepted")
	}
	if w := request("PUT", "/admin/media-service/bindings/1", `{"schema_id":"one","estimate_kind":"image","enabled":false}`, "Bearer admin"); w.Code != 200 {
		t.Fatal("binding save failed", w.Body.String())
	}
	if w := request("PUT", "/admin/media-service/agent/callback", `{"callback_secret":"not-from-browser"}`, "Bearer admin"); w.Code != 400 {
		t.Fatal("independent callback key accepted")
	}
}
