package relay

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileRetentionProtectsUnfinishedResultsAndPersistsExpiry(t *testing.T) {
	root := t.TempDir()
	store, err := NewDiskStore(filepath.Join(root, "tasks"))
	if err != nil {
		t.Fatal(err)
	}
	media := filepath.Join(root, "media")
	old, recent := time.Now().Add(-73*time.Hour), time.Now()
	for _, tc := range []struct {
		id, status, callback string
		completed            *time.Time
	}{
		{"expired", StatusSucceeded, CallbackConfirmed, &old},
		{"recent", StatusSucceeded, CallbackConfirmed, &recent},
		{"unconfirmed", StatusSucceeded, CallbackPending, &old},
		{"recoverable", StatusRecoveryRequired, CallbackConfirmed, &old},
		{"downloading", StatusDownloading, CallbackConfirmed, &old},
		{"unfinished", StatusSucceeded, CallbackConfirmed, nil},
	} {
		task := &Task{ID: tc.id, Status: tc.status, CallbackState: tc.callback, CompletedAt: tc.completed, Response: []byte("embedded media"), Result: map[string]any{"image": "embedded media"}}
		if err := store.Create(task); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(media, tc.id), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(media, tc.id, "f0000"), []byte("media"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := store.GCFiles(0, media); err != nil || n != 0 {
		t.Fatalf("disabled cleanup: %d %v", n, err)
	}
	if n, err := store.GCFiles(int(^uint(0)>>1), media); err != nil || n != 0 {
		t.Fatalf("large retention overflowed: %d %v", n, err)
	}
	if n, err := store.GCFiles(72, media); err != nil || n != 1 {
		t.Fatalf("cleanup: %d %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(media, "expired")); !os.IsNotExist(err) {
		t.Fatal("expired media remains")
	}
	reopened, err := NewDiskStore(filepath.Join(root, "tasks"))
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range reopened.ListAll() {
		if task.ID == "expired" {
			if !task.FilesAcknowledged || len(task.Response) != 0 || task.Result != nil {
				t.Fatal("expiry not persisted")
			}
			w := httptest.NewRecorder()
			(&RelayHandler{}).handleStandardResult(w, httptest.NewRequest(http.MethodGet, "/", nil), task, "openai_url")
			if w.Code != http.StatusGone {
				t.Fatalf("expired result: %d", w.Code)
			}
		} else if _, err := os.Stat(filepath.Join(media, task.ID, "f0000")); err != nil {
			t.Fatalf("protected media removed: %s", task.ID)
		}
	}
}
