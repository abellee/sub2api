package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"mediad/internal/signature"
)

func TestFileDeletionNoticeSurvivesTaskGCAndRestart(t *testing.T) {
	root := t.TempDir()
	media := filepath.Join(root, "media")
	store, err := NewDiskStore(filepath.Join(root, "tasks"))
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	task := &Task{ID: "mtask_expired", UserID: "42", KeyID: "7", Status: StatusSucceeded, CallbackState: CallbackConfirmed, CompletedAt: &old, Files: []ResultFile{{ID: "f0000"}}, Response: []byte(`large-image`)}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(media, task.ID), 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(media, task.ID, "f0000")
	if err := os.WriteFile(file, []byte("image"), 0600); err != nil {
		t.Fatal(err)
	}
	if n, err := store.GCFiles(1, media); n != 1 || err != nil {
		t.Fatal(n, err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("expired file remains")
	}
	var legacy atomic.Bool
	legacy.Store(true)
	var calls atomic.Int32
	main := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		raw := new(bytes.Buffer)
		_, _ = raw.ReadFrom(r.Body)
		if err := signature.Verify("secret", r.Header.Get("X-Media-Timestamp"), r.Method, r.URL.Path, raw.Bytes(), r.Header.Get("X-Media-Signature"), time.Now()); err != nil {
			t.Error(err)
		}
		var ev FileDeletionEvent
		if err := json.Unmarshal(raw.Bytes(), &ev); err != nil {
			t.Error(err)
			return
		}
		if ev.Event != "files.deleted" || ev.DeletedAt.IsZero() || ev.Reason != "retention_expired" {
			t.Error("wrong deletion event", ev)
		}
		if legacy.Load() {
			_, _ = w.Write([]byte(`{"already_settled":true}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"files_deleted": true, "event_id": ev.EventID})
	}))
	defer main.Close()
	tracker := NewTracker(store, nil, nil, nil)
	defer tracker.Stop()
	_ = tracker.SetCallback(CallbackConfig{Enabled: true, URL: main.URL, Secret: "secret"})
	if n := tracker.DeliverFileDeletions(context.Background()); n != 0 || calls.Load() != 1 {
		t.Fatal("legacy billing ACK accepted", n, calls.Load())
	}
	if n := store.GC(time.Hour); n != 1 {
		t.Fatal("task not collected", n)
	}
	reopened, err := NewDiskStore(filepath.Join(root, "tasks"))
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.deletions.items) != 1 {
		t.Fatal("deletion notice lost with task GC or restart")
	}
	legacy.Store(false)
	retry := NewTracker(reopened, nil, nil, nil)
	defer retry.Stop()
	_ = retry.SetCallback(CallbackConfig{Enabled: true, URL: main.URL, Secret: "secret"})
	// Advance the durable retry schedule without sleeping in the test.
	for id, n := range reopened.deletions.items {
		n.NextAttemptAt = time.Time{}
		reopened.deletions.items[id] = n
	}
	if n := retry.DeliverFileDeletions(context.Background()); n != 1 {
		t.Fatal("retry did not acknowledge deletion", n)
	}
	again, err := NewDiskStore(filepath.Join(root, "tasks"))
	if err != nil || len(again.deletions.items) != 0 {
		t.Fatal("confirmed event remains", err)
	}
}

func TestDeletionIntentPrecedesRemovalAndResumesWithRetentionDisabled(t *testing.T) {
	root := t.TempDir()
	media := filepath.Join(root, "media")
	store, _ := NewDiskStore(filepath.Join(root, "tasks"))
	old := time.Now().Add(-48 * time.Hour)
	task := &Task{ID: "mtask_resume", UserID: "42", KeyID: "7", Status: StatusSucceeded, CallbackState: CallbackConfirmed, CompletedAt: &old, Files: []ResultFile{{ID: "f0000"}}}
	_ = store.Create(task)
	_ = os.MkdirAll(filepath.Join(media, task.ID), 0700)
	file := filepath.Join(media, task.ID, "f0000")
	_ = os.WriteFile(file, []byte("image"), 0600)
	// A failed journal write must prevent physical deletion.
	store.deletions.root = filepath.Join(root, "not-a-directory")
	_ = os.WriteFile(store.deletions.root, []byte("x"), 0600)
	if _, err := store.GCFiles(1, media); err == nil {
		t.Fatal("journal failure ignored")
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("file removed before durable deletion intent")
	}
	store.deletions.root = filepath.Join(root, "tasks", "file-deletions")
	if err := store.deletions.put(newDeletionEvent(task, []string{"f0000"}, time.Now(), "retention_expired"), false); err != nil {
		t.Fatal(err)
	}
	store, err := NewDiskStore(filepath.Join(root, "tasks"))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := store.GCFiles(0, media); n != 1 || err != nil {
		t.Fatal("interrupted deletion did not resume", n, err)
	}
	if len(store.deletions.pending(time.Now())) != 1 {
		t.Fatal("removed file lacks pending notification")
	}
}

func TestHistoryReconcileOnlyMarksMissingFilesAndChecksOwner(t *testing.T) {
	root := t.TempDir()
	media := filepath.Join(root, "media")
	_ = os.MkdirAll(media, 0700)
	store := NewMemoryStore(nil)
	now := time.Now()
	_ = store.Create(&Task{ID: "mtask_retained", UserID: "42", KeyID: "7", Status: StatusSucceeded, CallbackState: CallbackConfirmed, CompletedAt: &now, Files: []ResultFile{{ID: "f0000"}}})
	_ = os.MkdirAll(filepath.Join(media, "mtask_retained"), 0700)
	_ = os.WriteFile(filepath.Join(media, "mtask_retained", "f0000"), []byte("image"), 0600)
	tracker := NewTracker(store, nil, nil, nil)
	defer tracker.Stop()
	tracker.ConfigureFiles(media, 1024)
	h := NewManagementHandler(NewSchemaRegistry(nil), store, tracker, "admin")
	mux := http.NewServeMux()
	h.Register(mux)
	call := func(body string) int {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/management/files/reconcile", bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer admin")
		mux.ServeHTTP(w, r)
		return w.Code
	}
	if code := call(`{"records":[{"task_id":"mtask_retained","user_id":"42","key_id":"7","file_ids":["f0000"]},{"task_id":"mtask_purged","user_id":"42","key_id":"7","file_ids":["f0000"]}]}`); code != 200 {
		t.Fatal(code)
	}
	if len(store.deletions.items) != 1 {
		t.Fatal("retained file treated as deleted")
	}
	for _, n := range store.deletions.items {
		if n.Event.TaskID != "mtask_purged" || n.Event.Reason != "files_missing" {
			t.Fatal(n)
		}
	}
	if code := call(`{"records":[{"task_id":"mtask_retained","user_id":"43","key_id":"7","file_ids":["f0000"]}]}`); code != 409 {
		t.Fatal("owner mismatch accepted", code)
	}
	_ = os.RemoveAll(media)
	if code := call(`{"records":[{"task_id":"mtask_another","user_id":"42","key_id":"7","file_ids":["f0000"]}]}`); code != 503 {
		t.Fatal("missing storage mount treated as file expiry", code)
	}
}
