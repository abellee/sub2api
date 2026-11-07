package server

import (
	"mediad/internal/config"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagementConfigurationRequiresTokenAndAppliesOnRestart(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MEDIAD_CONFIG_PATH", filepath.Join(root, "settings.json"))
	defaults := config.Config{Listen: "127.0.0.1:18090", DataDir: filepath.Join(root, "state"), PersistCredentials: true, ManagementToken: "private-token", RelaySecret: "private-relay"}
	s, err := New(defaults, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.tracker.Stop()
	request := func(method, body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/management/config", strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.httpServer.Handler.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "", ""); w.Code != 401 {
		t.Fatal("unauthenticated config access")
	}
	if w := request("GET", "", "private-token"); w.Code != 200 || strings.Contains(w.Body.String(), "private-") {
		t.Fatal("configuration leaked key or failed", w.Body.String())
	}
	if w := request("PUT", `{"max_concurrent":3}`, "private-token"); w.Code != 200 || !strings.Contains(w.Body.String(), `"restart_required":true`) {
		t.Fatal(w.Body.String())
	}
	if s.settings.Active().MaxConcurrent != 8 {
		t.Fatal("worker limit changed before restart")
	}
	second, err := New(defaults, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer second.tracker.Stop()
	if second.settings.Active().MaxConcurrent != 3 {
		t.Fatal("restart did not apply config")
	}
}
