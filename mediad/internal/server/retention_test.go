package server

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"mediad/internal/config"
	"mediad/internal/relay"
)

func TestFileCleanupEveryMinuteUsesCurrentRetentionWithoutReset(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		root := t.TempDir()
		s, err := New(config.Config{DataDir: root, ResultDir: filepath.Join(root, "media"), PersistCredentials: true}, log.New(io.Discard, "", 0))
		if err != nil {
			t.Fatal(err)
		}
		defer s.tracker.Stop()
		settings := s.settings.Active()
		settings.FileRetentionMinutes = 60
		settings.GCMinutes = 30
		if err := s.settings.Update(settings); err != nil {
			t.Fatal(err)
		}
		add := func(id string, age time.Duration, callback string) {
			completed := time.Now().Add(-age)
			if err := s.store.Create(&relay.Task{ID: id, UserID: "2", Status: relay.StatusSucceeded, CallbackState: callback, CompletedAt: &completed}); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(settings.ResultDir, id)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "f0000"), []byte("media"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		add("expired", 31*time.Minute, relay.CallbackConfirmed)
		add("recent", 28*time.Minute, relay.CallbackConfirmed)
		add("unconfirmed", 31*time.Minute, relay.CallbackPending)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- s.runMaintenance(ctx, make(chan error)) }()
		synctest.Wait()
		time.Sleep(30 * time.Second)
		settings.FileRetentionMinutes = 30
		if err := s.settings.Update(settings); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		time.Sleep(30 * time.Second)
		synctest.Wait()
		if _, err := os.Stat(filepath.Join(settings.ResultDir, "expired")); !os.IsNotExist(err) {
			t.Fatal("expired file was not cleaned on the first minute after changing retention")
		}
		for _, id := range []string{"recent", "unconfirmed"} {
			if _, err := os.Stat(filepath.Join(settings.ResultDir, id, "f0000")); err != nil {
				t.Fatalf("protected file removed: %s: %v", id, err)
			}
		}
		settings.FileRetentionMinutes = 0
		if err := s.settings.Update(settings); err != nil {
			t.Fatal(err)
		}
		add("permanent", 31*time.Minute, relay.CallbackConfirmed)
		time.Sleep(time.Minute)
		synctest.Wait()
		if _, err := os.Stat(filepath.Join(settings.ResultDir, "permanent", "f0000")); err != nil {
			t.Fatalf("permanent file removed: %v", err)
		}
		cancel()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})
}
