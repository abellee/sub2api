package config

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsPersistRedactAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	defaults := Config{Listen: "127.0.0.1:8090", DataDir: t.TempDir(), PersistCredentials: true, ManagementToken: "private-management", RelaySecret: "private-relay"}
	store, err := OpenSettings(path, defaults)
	if err != nil {
		t.Fatal(err)
	}
	next := store.Active()
	next.MaxConcurrent = 3
	next.FileRetentionHours = 72
	next.ManagementToken = ""
	next.RelaySecret = ""
	if err = store.Update(next); err != nil {
		t.Fatal(err)
	}
	if store.Active().MaxConcurrent != 8 {
		t.Fatal("active workers changed before restart")
	}
	if store.FileRetentionHours() != 72 {
		t.Fatal("file retention did not apply immediately")
	}
	b, _ := json.Marshal(store.View())
	if strings.Contains(string(b), "private-") {
		t.Fatal("secret exposed")
	}
	if !store.View()["restart_required"].(bool) {
		t.Fatal("restart must be reported")
	}
	reopened, err := OpenSettings(path, defaults)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Active().MaxConcurrent != 3 || reopened.Active().ManagementToken != "private-management" || reopened.View()["restart_required"].(bool) {
		t.Fatal("saved configuration was not applied at restart")
	}
	next = reopened.Active()
	next.MaxConcurrent = 0
	if reopened.Update(next) == nil {
		t.Fatal("invalid concurrency accepted")
	}
	again, err := OpenSettings(path, defaults)
	if err != nil || again.Active().MaxConcurrent != 3 {
		t.Fatal("rejected update changed disk")
	}
}

func TestFileRetentionDefaultAndNoPolicyMaximum(t *testing.T) {
	s := DefaultSettings(Config{PersistCredentials: true})
	if s.FileRetentionHours != 1 {
		t.Fatal("default retention must be one hour")
	}
	s.FileRetentionHours = int(^uint(0) >> 1)
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	s.FileRetentionHours = -1
	if s.Validate() == nil {
		t.Fatal("negative retention accepted")
	}
}
