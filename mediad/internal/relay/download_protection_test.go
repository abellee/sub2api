package relay

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

type downloadCheckingWriter struct {
	*httptest.ResponseRecorder
	check func()
	fail  bool
}

func (w *downloadCheckingWriter) Write(p []byte) (int, error) {
	if w.check != nil {
		check := w.check
		w.check = nil
		check()
	}
	if w.fail {
		return 0, io.ErrClosedPipe
	}
	return w.ResponseRecorder.Write(p)
}

func downloadProtectionFixture(t *testing.T) (*MemoryStore, string, *Task, *http.ServeMux) {
	t.Helper()
	store, media := diskCleanupFixture(t)
	task := addDiskCleanupTask(t, store, media, "mtask_protected", time.Hour, StatusSucceeded, CallbackConfirmed, "image/png", "image/png")
	task.DownloadToken = "private-download-token"
	if err := store.Update(task); err != nil {
		t.Fatal(err)
	}
	tracker := NewTracker(store, nil, nil, nil)
	t.Cleanup(tracker.Stop)
	tracker.ConfigureFiles(media, 1024)
	mux := http.NewServeMux()
	NewRelayHandler(nil, store, tracker, relaySecret, nil).Register(mux)
	return store, media, task, mux
}

func protectedDownloadRequest(task *Task, method string) *http.Request {
	return httptest.NewRequest(method, "/downloads/"+task.ID+"/f0000.png?token="+downloadToken(task, "f0000"), nil)
}

func TestDownloadProtectionCoversEveryTransferAndCleanup(t *testing.T) {
	for _, format := range []string{"public", "relay", "openai_b64_json", "openai_sse", "openai_edit_sse"} {
		t.Run(format, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				store, media, task, mux := downloadProtectionFixture(t)
				request := protectedDownloadRequest(task, "GET")
				if format == "relay" {
					request = signedRequest(t, "GET", "/relay/tasks/"+task.ID+"/files/f0000", nil, map[string]string{HeaderUserID: "2"})
				} else if strings.HasPrefix(format, "openai_") {
					request = signedRequest(t, "GET", "/relay/tasks/"+task.ID+"?response_format="+format, nil, map[string]string{HeaderUserID: "2"})
				}
				checked := false
				w := &downloadCheckingWriter{ResponseRecorder: httptest.NewRecorder(), check: func() {
					checked = true
					// An active transfer must outlive the retry grace without becoming deletable.
					time.Sleep(3 * downloadRetryGrace)
					if n, err := store.GCFiles(30, media); n != 0 || err != nil {
						t.Fatalf("retention removed active transfer: %d %v", n, err)
					}
					if err := store.deleteResultFile(task.ID, "f0000", task.UserID, media); !errors.Is(err, errDownloadProtected) {
						t.Fatal("manual deletion ignored transfer", err)
					}
					if err := store.Delete(task.ID); !errors.Is(err, errDownloadProtected) {
						t.Fatal("task deletion ignored transfer", err)
					}
					want := 1
					if strings.HasPrefix(format, "openai_") {
						want = 0 // Both inline images are protected before the first byte is sent.
					}
					if n, err := store.gcImagesForDisk(85, media, func(string) (float64, error) { return 99, nil }); n != want || err != nil {
						t.Fatalf("disk cleanup: %d %v, want %d", n, err, want)
					}
					if _, err := os.Stat(filepath.Join(media, task.ID, "f0000")); err != nil {
						t.Fatal("active file disappeared", err)
					}
					// Task-record GC must also respect a transfer even after acknowledgement.
					if err := store.AcknowledgeFiles(task.ID); err != nil {
						t.Fatal(err)
					}
					if n := store.GC(time.Nanosecond); n != 0 {
						t.Fatal("active task collected")
					}
				}}
				mux.ServeHTTP(w, request)
				if !checked || w.Code != 200 {
					t.Fatal("transfer not exercised", w.Code, w.Body.String())
				}
				wantBody := "media"
				if strings.HasPrefix(format, "openai_") {
					wantBody = base64.StdEncoding.EncodeToString([]byte(wantBody))
				}
				if !strings.Contains(w.Body.String(), wantBody) {
					t.Fatal("transfer body corrupted", w.Body.String())
				}
				if n, err := store.GCFiles(30, media); n != 0 || err != nil {
					t.Fatal("retry grace ignored", n, err)
				}
				time.Sleep(downloadRetryGrace - time.Second)
				if err := store.deleteResultFile(task.ID, "f0000", task.UserID, media); !errors.Is(err, errDownloadProtected) {
					t.Fatal("grace expired too early", err)
				}
				time.Sleep(time.Second)
				if n, err := store.GCFiles(30, media); n != 1 || err != nil {
					t.Fatal("expired protection blocked cleanup", n, err)
				}
				if _, err := os.Stat(filepath.Join(media, task.ID)); !os.IsNotExist(err) {
					t.Fatal("expired media remains", err)
				}
				if len(store.downloads) != 0 {
					t.Fatal("expired download records retained")
				}
			})
		})
	}
}

func TestConcurrentDownloadsKeepProtectionUntilLastTransferEnds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store, media, task, mux := downloadProtectionFixture(t)
		started, finish, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
		w := &downloadCheckingWriter{ResponseRecorder: httptest.NewRecorder(), check: func() { close(started); <-finish }}
		go func() { defer close(done); mux.ServeHTTP(w, protectedDownloadRequest(task, "GET")) }()
		<-started
		second := httptest.NewRecorder()
		mux.ServeHTTP(second, protectedDownloadRequest(task, "GET"))
		if second.Code != 200 {
			t.Fatal(second.Code)
		}
		time.Sleep(3 * downloadRetryGrace)
		if n, err := store.GCFiles(30, media); n != 0 || err != nil {
			t.Fatal("first active download lost protection", n, err)
		}
		close(finish)
		<-done
		time.Sleep(downloadRetryGrace - time.Second)
		head := httptest.NewRecorder()
		mux.ServeHTTP(head, protectedDownloadRequest(task, "HEAD"))
		if head.Code != 200 || head.Body.Len() != 0 || head.Header().Get("Content-Length") != "5" {
			t.Fatal("HEAD behavior changed", head.Code, head.Header())
		}
		retry := protectedDownloadRequest(task, "GET")
		retry.Header.Set("Range", "bytes=2-4")
		resumed := httptest.NewRecorder()
		mux.ServeHTTP(resumed, retry)
		if resumed.Code != 206 || resumed.Body.String() != "dia" || resumed.Header().Get("Content-Range") != "bytes 2-4/5" {
			t.Fatal("resume failed", resumed.Code, resumed.Body.String())
		}
		time.Sleep(time.Second)
		if n, err := store.GCFiles(30, media); n != 0 || err != nil {
			t.Fatal("retry did not renew grace", n, err)
		}
		time.Sleep(downloadRetryGrace)
		if n, err := store.gcImagesForDisk(85, media, func(string) (float64, error) { return 99, nil }); n != 2 || err != nil {
			t.Fatal("disk cleanup blocked after grace", n, err)
		}
	})
}

func TestDownloadFailureReleasesProtectionAndPendingIntentWaits(t *testing.T) {
	for _, format := range []string{"public", "openai_b64_json", "openai_sse", "openai_edit_sse"} {
		t.Run(format, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				store, media, task, mux := downloadProtectionFixture(t)
				event := newDeletionEvent(task, []string{"f0000"}, time.Now(), "disk_pressure")
				if err := store.deletions.put(event, false); err != nil {
					t.Fatal(err)
				}
				request := protectedDownloadRequest(task, "GET")
				if format != "public" {
					request = signedRequest(t, "GET", "/relay/tasks/"+task.ID+"?response_format="+format, nil, map[string]string{HeaderUserID: "2"})
				}
				mux.ServeHTTP(&downloadCheckingWriter{ResponseRecorder: httptest.NewRecorder(), fail: true, check: func() {
					if n, err := store.gcImagesForDisk(0, media, nil); n != 0 || err != nil {
						t.Fatal("pending intent removed active transfer", n, err)
					}
				}}, request)
				if n, err := store.gcImagesForDisk(0, media, nil); n != 0 || err != nil {
					t.Fatal("pending intent ignored retry grace", n, err)
				}
				time.Sleep(downloadRetryGrace)
				if n, err := store.gcImagesForDisk(0, media, nil); n != 1 || err != nil {
					t.Fatal("failed transfer leaked protection", n, err)
				}
				if err := store.deleteResultFile(task.ID, "f0001", task.UserID, media); err != nil {
					t.Fatal("manual deletion blocked after grace", err)
				}
			})
		})
	}
}

func TestDownloadProtectionRejectsInvalidAndStaleRequests(t *testing.T) {
	store, media, task, mux := downloadProtectionFixture(t)
	for _, request := range []*http.Request{
		httptest.NewRequest("GET", "/downloads/"+task.ID+"/f0000.png?token=wrong", nil),
		signedRequest(t, "GET", "/relay/tasks/"+task.ID+"/files/f0000", nil, nil),
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, request)
		if w.Code != 404 || len(store.downloads) != 0 {
			t.Fatal("unauthorized request created protection", w.Code)
		}
	}
	if err := os.Remove(filepath.Join(media, task.ID, "f0001")); err != nil {
		t.Fatal(err)
	}
	if _, release, err := store.openDownloads(media, task, task.Files); err == nil {
		release()
		t.Fatal("missing file accepted")
	}
	if len(store.downloads) != 0 {
		t.Fatal("partial open created protection")
	}
	if err := store.deleteResultFile(task.ID, "f0000", task.UserID, media); err != nil {
		t.Fatal("partial open leaked file handle", err)
	}
	if _, release, err := store.openDownloads(media, task, task.Files[:1]); err == nil {
		release()
		t.Fatal("stale task reopened deleted file")
	}
	if len(store.downloads) != 0 {
		t.Fatal("stale request created protection")
	}
}
