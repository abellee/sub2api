package relay

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"mediad/internal/config"
)

func TestConfiguredMemoryBudgetAccountsForUsageAndReservations(t *testing.T) {
	used := int64(256 << 20)
	b := memoryBudget{available: func() int64 { return 8 << 30 }, used: func() int64 { return used }, limit: 512 << 20}
	release, ok := b.try(192 << 20)
	if !ok {
		t.Fatal("budget should admit first allocation")
	}
	if _, ok := b.try(128 << 20); ok {
		t.Fatal("pending allocations overcommitted budget")
	}
	b.limit = 128 << 20
	if _, ok := b.try(1); ok {
		t.Fatal("lowered budget ignored existing usage")
	}
	release()
	release()
	b.limit = 1024 << 20
	release, ok = b.try(512 << 20)
	if !ok {
		t.Fatal("increased budget did not unblock admission")
	}
	release()
	b.limit = 0
	release, ok = b.try(2 << 30)
	if !ok {
		t.Fatal("automatic mode retained old budget")
	}
	release()
	b.available = func() int64 { return 600 << 20 }
	b.limit = 8 << 30
	if _, ok := b.try(128 << 20); ok {
		t.Fatal("configured budget overrode available memory")
	}
}

func TestMemoryConfigAppliesToWaitingReadersAndPersists(t *testing.T) {
	beforeLimit := mediaMemory.limit >> 20
	beforeAvailable, beforeUsed := mediaMemory.available, mediaMemory.used
	beforeGo := debug.SetMemoryLimit(-1)
	defer func() {
		ConfigureMemoryLimit(beforeLimit)
		mediaMemory.available, mediaMemory.used = beforeAvailable, beforeUsed
		debug.SetMemoryLimit(beforeGo)
	}()
	mediaMemory.available = func() int64 { return 8 << 30 }
	mediaMemory.used = func() int64 { return 256 << 20 }
	path := filepath.Join(t.TempDir(), "settings.json")
	settings, err := config.OpenSettings(path, config.Config{PersistCredentials: true})
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore(nil)
	tracker := NewTracker(store, nil, nil, nil)
	defer tracker.Stop()
	h := NewManagementHandler(nil, store, tracker, "secret")
	h.Settings = settings
	update := func(body string, status int) {
		t.Helper()
		w := httptest.NewRecorder()
		h.handleConfig(w, httptest.NewRequest("PUT", "/management/config", strings.NewReader(body)))
		if w.Code != status {
			t.Fatalf("config: %d %s", w.Code, w.Body.String())
		}
	}
	update(`{"memory_limit_mib":256,"max_concurrent":1000000}`, 200)
	if _, ok := mediaMemory.try(1 << 20); ok {
		t.Fatal("live limit not enforced")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		release, err := mediaMemory.wait(ctx, 128<<20)
		if release != nil {
			release()
		}
		done <- err
	}()
	update(`{"memory_limit_mib":1024}`, 200)
	if err := <-done; err != nil {
		t.Fatal("reader did not resume", err)
	}
	update(`{"memory_limit_mib":-1}`, 400)
	if mediaMemory.limit != 1024<<20 {
		t.Fatal("rejected update changed running budget")
	}
	reopened, err := config.OpenSettings(path, config.Config{PersistCredentials: true})
	if err != nil || reopened.Active().MemoryLimitMiB != 1024 || reopened.Active().MaxConcurrent != 1000000 {
		t.Fatal("live config was not persisted", err)
	}
	update(`{"memory_limit_mib":0}`, 200)
	if mediaMemory.limit != 0 || debug.SetMemoryLimit(-1) != defaultGoMemoryLimit {
		t.Fatal("automatic mode did not restore original GC budget")
	}
}
