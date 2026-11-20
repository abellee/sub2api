package config

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsPersistRedactAndLiveUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	defaults := Config{Listen: "127.0.0.1:8090", DataDir: t.TempDir(), PersistCredentials: true, ManagementToken: "private-management", RelaySecret: "private-relay"}
	store, err := OpenSettings(path, defaults)
	if err != nil {
		t.Fatal(err)
	}
	next := store.Active()
	next.MaxConcurrent = 1000000
	next.MemoryLimitMiB = 2048
	next.MaxSyncSeconds = 90
	next.PollIntervalSeconds = 2
	next.MaxPollSeconds = 300
	next.DownloadTimeoutSeconds = 45
	next.DownloadAttempts = 5
	next.MaxFileMiB = 64
	next.GCMinutes = 1
	next.TaskRetentionHours = 12
	next.DownloadBaseURL = "https://media.example.test"
	next.FileRetentionHours = 72
	next.ManagementToken = ""
	next.RelaySecret = ""
	if err = store.Update(next); err != nil {
		t.Fatal(err)
	}
	if store.Active().MaxConcurrent != 1000000 || store.Active().MemoryLimitMiB != 2048 {
		t.Fatal("concurrency was not applied immediately")
	}
	if store.FileRetentionHours() != 72 {
		t.Fatal("file retention did not apply immediately")
	}
	b, _ := json.Marshal(store.View())
	if strings.Contains(string(b), "private-") {
		t.Fatal("secret exposed")
	}
	if store.View()["restart_required"].(bool) {
		t.Fatal("operational settings must not require restart")
	}
	active := store.Active()
	next.ManagementToken, next.RelaySecret = active.ManagementToken, active.RelaySecret
	if active != next {
		t.Fatal("only part of the runtime settings applied")
	}
	reopened, err := OpenSettings(path, defaults)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Active().MaxConcurrent != 1000000 || reopened.Active().MemoryLimitMiB != 2048 || reopened.Active().ManagementToken != "private-management" || reopened.View()["restart_required"].(bool) {
		t.Fatal("saved configuration was not applied at restart")
	}
	next = reopened.Active()
	next.MaxConcurrent = 0
	if reopened.Update(next) == nil {
		t.Fatal("invalid concurrency accepted")
	}
	again, err := OpenSettings(path, defaults)
	if err != nil || again.Active().MaxConcurrent != 1000000 {
		t.Fatal("rejected update changed disk")
	}
}

func TestMemoryLimitValidation(t *testing.T) {
	s := DefaultSettings(Config{PersistCredentials: true})
	for _, value := range []int64{0, 1, 4096, (1<<63 - 1) >> 20} {
		s.MemoryLimitMiB = value
		if err := s.Validate(); err != nil {
			t.Fatalf("valid budget %d: %v", value, err)
		}
	}
	for _, value := range []int64{-1, 1 << 43} {
		s.MemoryLimitMiB = value
		if s.Validate() == nil {
			t.Fatalf("unsafe budget accepted: %d", value)
		}
	}
}

func TestStorageAndListenerChangesRemainPending(t *testing.T) {
	store, err := OpenSettings(filepath.Join(t.TempDir(), "settings.json"), Config{DataDir: t.TempDir(), PersistCredentials: true})
	if err != nil {
		t.Fatal(err)
	}
	before := store.Active()
	next := before
	next.Listen = "127.0.0.1:9999"
	next.ResultDir = t.TempDir()
	next.DataDir = t.TempDir()
	next.MaxConcurrent = 100
	changed := store.Changed()
	if err := store.Update(next); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changed:
	default:
		t.Fatal("settings update was not broadcast")
	}
	a := store.Active()
	if a.Listen != before.Listen || a.DataDir != before.DataDir || a.ResultDir != before.ResultDir || a.MaxConcurrent != 100 {
		t.Fatal("unsafe partial configuration update")
	}
	if !store.StorageChangePending() || len(store.View()["pending_fields"].([]string)) != 3 {
		t.Fatal("wrong pending fields")
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
