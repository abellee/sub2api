package relay

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"mediad/internal/config"
)

func TestLiveConcurrencyDoesNotReserveImageMemoryWhileWaiting(t *testing.T) {
	// Enough RAM for small response parsing, far less than the former 768MiB per worker.
	original := mediaMemory.available
	mediaMemory.available = func() int64 { return 640 << 20 }
	defer func() { mediaMemory.available = original }()
	arrived := make(chan string, 12)
	finish := make(chan struct{})
	var once sync.Once
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		arrived <- r.URL.Path
		select {
		case <-finish:
		case <-r.Context().Done():
			return
		}
		io.WriteString(w, `{"data":[{"url":"https://example.test/image.png"}]}`)
	}))
	defer up.Close()
	defer once.Do(func() { close(finish) })
	store, err := NewDiskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tracker := NewTracker(store, nil, up.Client(), nil)
	defer tracker.Stop()
	settings, err := config.OpenSettings(filepath.Join(t.TempDir(), "settings.json"), config.Config{PersistCredentials: true})
	if err != nil {
		t.Fatal(err)
	}
	tracker.runtimeSettings = settings
	tracker.ConfigureRuntime(1, 1, 1800, 120, 1)
	management := NewManagementHandler(nil, store, tracker, "secret")
	management.Settings = settings
	setLimit := func(n int) {
		w := httptest.NewRecorder()
		management.handleConfig(w, httptest.NewRequest("PUT", "/management/config", strings.NewReader(fmt.Sprintf(`{"max_concurrent":%d}`, n))))
		if w.Code != 200 || settings.Active().MaxConcurrent != n {
			t.Fatalf("live config failed: %s", w.Body.String())
		}
	}
	schema := testSchema()
	schema.Mode = "sync"
	schema.Response.Usage.Quantity = float64(1)
	schema.Response.Result = map[string]string{"data": "data"}
	for i := 0; i < 10; i++ {
		task := &Task{ID: fmt.Sprintf("live_%d", i), UserID: "2", KeyID: "2", Status: StatusQueued, CreatedAt: time.Now(), Schema: schema, CallbackState: CallbackPending, Request: &UpstreamRequest{Method: "POST", URL: up.URL + fmt.Sprintf("/%d", i), Body: []byte(`{}`)}}
		if err := store.Create(task); err != nil {
			t.Fatal(err)
		}
		tracker.Track(task)
	}
	wait := func(n int) {
		for range n {
			select {
			case <-arrived:
			case <-time.After(5 * time.Second):
				t.Fatal("provider submission blocked by image reservation or unchanged concurrency")
			}
		}
	}
	wait(1)
	setLimit(10)
	wait(9)
	mediaMemory.mu.Lock()
	reserved := mediaMemory.reserved
	mediaMemory.mu.Unlock()
	if reserved != 0 {
		t.Fatalf("waiting providers reserved %d image bytes", reserved)
	}
	setLimit(1)
	task := &Task{ID: "live_extra", UserID: "2", KeyID: "2", Status: StatusQueued, CreatedAt: time.Now(), Schema: schema, CallbackState: CallbackPending, Request: &UpstreamRequest{Method: "POST", URL: up.URL + "/extra", Body: []byte(`{}`)}}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}
	tracker.Track(task)
	select {
	case <-arrived:
		t.Fatal("lowered limit admitted extra work")
	case <-time.After(100 * time.Millisecond):
	}
	once.Do(func() { close(finish) })
	wait(1)
	for i := 0; i < 10; i++ {
		awaitTask(t, store, fmt.Sprintf("live_%d", i), func(t *Task) bool { return t.Status == StatusSucceeded })
	}
	awaitTask(t, store, "live_extra", func(t *Task) bool { return t.Status == StatusSucceeded })
}

func TestResponseSavedBeforeWaitingForParseMemory(t *testing.T) {
	original := mediaMemory.available
	mediaMemory.available = func() int64 { return 512 << 20 }
	defer func() { mediaMemory.available = original }()
	store, err := NewDiskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tracker := NewTracker(store, nil, nil, nil)
	defer tracker.Stop()
	task := &Task{ID: "saved_before_parse", UserID: "2", Status: StatusSubmitting, CreatedAt: time.Now()}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, release, err := tracker.bufferedResponse(ctx, task, "create", &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[]}`))}, 64<<20)
		release()
		done <- err
	}()
	awaitTask(t, store, task.ID, func(t *Task) bool { return t.ResponseData != nil })
	select {
	case <-done:
		t.Fatal("parsed without available memory")
	default:
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancellation lost")
		}
	case <-time.After(time.Second):
		t.Fatal("memory waiter stuck")
	}
	reopened, err := NewDiskStore(store.root)
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := reopened.Get(task.ID)
	if err := reopened.loadResults(saved); err != nil || string(saved.Response) != `{"data":[]}` {
		t.Fatalf("saved response not recoverable: %v", err)
	}
}

func TestTaskSlotsCancellation(t *testing.T) {
	var slots taskSlots
	slots.setLimit(1)
	if err := slots.acquire(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer slots.release()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if slots.acquire(ctx) == nil {
		t.Fatal("canceled waiter acquired slot")
	}
}

func TestRuntimeSettingsReachWorkersWithoutMutatingActiveDownloads(t *testing.T) {
	settings, err := config.OpenSettings(filepath.Join(t.TempDir(), "settings.json"), config.Config{DataDir: t.TempDir(), PersistCredentials: true})
	if err != nil {
		t.Fatal(err)
	}
	tracker := NewTracker(NewMemoryStore(nil), nil, nil, nil)
	defer tracker.Stop()
	tracker.ConfigureFiles(t.TempDir(), 512<<20)
	tracker.runtimeSettings = settings
	before := tracker.resultStager()
	next := settings.Active()
	next.MaxSyncSeconds = 91
	next.PollIntervalSeconds = 2
	next.MaxPollSeconds = 300
	next.DownloadAttempts = 5
	next.DownloadTimeoutSeconds = 33
	next.MaxFileMiB = 10
	next.DownloadBaseURL = "https://media.example.test"
	next.RelaySecret = "live-relay"
	if err := settings.Update(next); err != nil {
		t.Fatal(err)
	}
	interval, deadline := tracker.pollSettings()
	after := tracker.resultStager()
	if tracker.creationTimeout() != 91*time.Second || interval != 2*time.Second || deadline != 300*time.Second || tracker.stagingAttempts() != 5 || after.Client.Timeout != 33*time.Second || after.MaxBytes != 10<<20 || tracker.downloadBaseURL() != next.DownloadBaseURL {
		t.Fatal("workers still use bootstrap settings")
	}
	if before.Client.Timeout != 120*time.Second || before.MaxBytes != 512<<20 {
		t.Fatal("in-flight download settings were mutated")
	}
	h := NewRelayHandler(nil, tracker.store, tracker, "old-secret", nil)
	if h.relaySecret() != "live-relay" {
		t.Fatal("relay signing secret not updated")
	}
}
