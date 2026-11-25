package config

import (
	"encoding/json"
	"fmt"
	"os"
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
	next.FileRetentionMinutes = 72
	next.ManagementToken = ""
	next.RelaySecret = ""
	if err = store.Update(next); err != nil {
		t.Fatal(err)
	}
	if store.Active().MaxConcurrent != 1000000 || store.Active().MemoryLimitMiB != 2048 {
		t.Fatal("concurrency was not applied immediately")
	}
	if store.FileRetentionMinutes() != 72 {
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
	if s.FileRetentionMinutes != 60 {
		t.Fatal("default retention must be 60 minutes")
	}
	s.FileRetentionMinutes = int(^uint(0) >> 1)
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	s.FileRetentionMinutes = -1
	if s.Validate() == nil {
		t.Fatal("negative retention accepted")
	}
}

func TestFileRetentionLegacySettingsAndMinutePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       int
	}{
		{"hours", `{"file_retention_hours":72}`, 4320},
		{"forever", `{"file_retention_hours":0}`, 0},
		{"minutes", `{"file_retention_minutes":5}`, 5},
		{"minutes win", `{"file_retention_hours":72,"file_retention_minutes":0}`, 0},
		{"omitted", `{}`, 60},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			store, err := OpenSettings(path, Config{PersistCredentials: true})
			if err != nil {
				t.Fatal(err)
			}
			if store.FileRetentionMinutes() != tc.want {
				t.Fatalf("got %d minutes, want %d", store.FileRetentionMinutes(), tc.want)
			}
			next := store.Active()
			if err := json.Unmarshal([]byte(`{"max_concurrent":4}`), &next); err != nil || next.FileRetentionMinutes != tc.want {
				t.Fatalf("partial update changed retention: %+v, %v", next, err)
			}
			if err := store.Update(next); err != nil {
				t.Fatal(err)
			}
			buf, err := os.ReadFile(path)
			if err != nil || strings.Contains(string(buf), "file_retention_hours") || !strings.Contains(string(buf), "file_retention_minutes") {
				t.Fatalf("settings not saved in minutes: %s, %v", buf, err)
			}
			reopened, err := OpenSettings(path, Config{PersistCredentials: true})
			if err != nil || reopened.FileRetentionMinutes() != tc.want {
				t.Fatalf("minute settings did not survive restart: %v", err)
			}
		})
	}
	for _, body := range []string{`{"file_retention_hours":-1}`, fmt.Sprintf(`{"file_retention_hours":%d}`, int(^uint(0)>>1))} {
		var settings Settings
		if json.Unmarshal([]byte(body), &settings) == nil {
			t.Fatal("invalid legacy retention accepted", body)
		}
	}
}
