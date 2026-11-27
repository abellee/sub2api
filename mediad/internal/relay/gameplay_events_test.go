package relay

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGameplayTransitionsAreSnapshotsWithoutSecrets(t *testing.T) {
	task := &Task{ID: "mtask_event", UserID: "2", KeyID: "7", GroupID: "3", Model: "image-model", Status: StatusQueued, CreatedAt: time.Now(), Credential: "private-credential", UpstreamURL: "https://provider.invalid?key=secret", Request: &UpstreamRequest{}}
	created := taskGameplayChanges(nil, task)
	if len(created) != 1 || created[0].Type != "media.created" || created[0].UserID != 2 {
		t.Fatalf("creation=%+v", created)
	}
	raw, _ := json.Marshal(created)
	if strings.Contains(string(raw), "private-credential") || strings.Contains(string(raw), "provider.invalid") {
		t.Fatal("event contains credentials or upstream URLs")
	}
	previous := *task
	task.Status = StatusSubmitting
	started := taskGameplayChanges(&previous, task)
	if len(started) != 1 || started[0].Type != "media.started" {
		t.Fatalf("start=%+v", started)
	}
	previous = *task
	if got := taskGameplayChanges(&previous, task); len(got) != 0 {
		t.Fatalf("unchanged poll emitted=%+v", got)
	}
	task.Status = StatusSucceeded
	task.Files = []ResultFile{{ID: "f0001"}}
	finished := taskGameplayChanges(&previous, task)
	if len(finished) != 2 || finished[0].Type != "media.succeeded" || finished[1].Type != "media.files_saved" {
		t.Fatalf("finish=%+v", finished)
	}
	replayed := taskGameplayChanges(&previous, task)
	if finished[0].ID != replayed[0].ID {
		t.Fatal("replayed transition changed event identity")
	}
}

func TestGameplayQueueDoesNotWaitForCallbackConfigurationLock(t *testing.T) {
	tracker := &Tracker{ctx: context.Background(), gameplay: make(chan gameplayEvent, 1)}
	tracker.gameplayStarted.Store(true)
	tracker.gameplayConfigured.Store(true)
	tracker.cbMu.Lock()
	done := make(chan struct{})
	go func() { tracker.queueGameplay(gameplayEvent{UserID: 2}); close(done) }()
	select {
	case <-done:
		tracker.cbMu.Unlock()
	case <-time.After(time.Second):
		tracker.cbMu.Unlock()
		t.Fatal("event hook waited for callback configuration lock")
	}
	if len(tracker.gameplay) != 1 {
		t.Fatal("event snapshot was not queued")
	}
	tracker.queueGameplay(gameplayEvent{UserID: 2})
	if tracker.gameplayDropped.Load() != 1 {
		t.Fatal("full queue was not dropped")
	}
}

type gameplayOptimizedWriter struct {
	*httptest.ResponseRecorder
	copied bool
}

func (w *gameplayOptimizedWriter) ReadFrom(r io.Reader) (int64, error) {
	w.copied = true
	return w.Body.ReadFrom(r)
}

func TestGameplayDownloadObserverPreservesOptimizedCopy(t *testing.T) {
	underlying := &gameplayOptimizedWriter{ResponseRecorder: httptest.NewRecorder()}
	w := &gameplayDownloadWriter{ResponseWriter: underlying}
	n, err := w.ReadFrom(strings.NewReader("image-content"))
	if err != nil || n != 13 || !underlying.copied || underlying.Body.String() != "image-content" || w.status != 200 || w.failed {
		t.Fatalf("copy n=%d err=%v optimized=%v status=%d", n, err, underlying.copied, w.status)
	}
}
