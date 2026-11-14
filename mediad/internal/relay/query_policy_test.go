package relay

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mediad/internal/signature"
)

func TestTaskQueryLimitsAcrossEntrypointsAndRestart(t *testing.T) {
	store, err := NewDiskStore(filepath.Join(t.TempDir(), "tasks"))
	if err != nil {
		t.Fatal(err)
	}
	created := time.Now().UTC().Truncate(time.Second)
	now := created
	task := &Task{ID: "mtask_limits", UserID: "42", KeyID: "7", CreatedAt: created, Status: StatusRecoveryRequired}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}
	h := NewRelayHandler(nil, store, nil, relaySecret, nil)
	h.now = func() time.Time { return now }
	mux := http.NewServeMux()
	h.Register(mux)
	get := func(path, marker, user string) *httptest.ResponseRecorder {
		r := signedRequest(t, "GET", path, nil, map[string]string{HeaderClientQuery: marker, HeaderUserID: user})
		ts := strconv.FormatInt(now.Unix(), 10)
		r.Header.Set(HeaderTimestamp, ts)
		r.Header.Set(HeaderSignature, signature.Sign(relaySecret, ts, "GET", r.URL.Path, nil))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	path := "/relay/tasks/" + task.ID + "?response_format=state"
	if w := get(path, "status", "43"); w.Code != 404 {
		t.Fatal("foreign task exposed", w.Code)
	}
	if w := get(path, "status", "42"); w.Code != 200 || w.Header().Get("Retry-After") != "3" {
		t.Fatal(w.Code, w.Body.String())
	}
	// The public result URL and the content route cannot create extra polling windows.
	public := "/downloads/" + task.ID + "/result?token=" + h.resultToken(task.ID)
	if w := get(public, "", "42"); w.Code != 429 {
		t.Fatal("result URL bypass", w.Code)
	}
	if w := get(path, "content", "42"); w.Code != 429 {
		t.Fatal("content bypass", w.Code)
	}
	now = created.Add(3*time.Second - time.Nanosecond)
	if w := get(path, "status", "42"); w.Code != 429 || w.Header().Get("Retry-After") != "1" {
		t.Fatal("early query allowed", w.Code, w.Header())
	}
	now = created.Add(3 * time.Second)
	if w := get(path, "status", "42"); w.Code != 200 {
		t.Fatal("interval boundary", w.Code)
	}
	now = created.Add(time.Hour)
	for _, target := range []string{path, public} {
		w := get(target, "status", "42")
		if w.Code != 410 || !strings.Contains(w.Body.String(), `"stop_polling":true`) || !strings.Contains(w.Body.String(), `"retryable":false`) {
			t.Fatal("expiry boundary", w.Code, w.Body.String())
		}
	}
	// Internal reads remain available for settlement, diagnostics and manual recovery.
	if w := get(path, "", "42"); w.Code != 200 {
		t.Fatal("internal read expired", w.Code)
	}
	restored, err := NewDiskStore(store.root)
	if err != nil {
		t.Fatal(err)
	}
	h = NewRelayHandler(nil, restored, nil, relaySecret, nil)
	h.now = func() time.Time { return now }
	mux = http.NewServeMux()
	h.Register(mux)
	if w := get(path, "status", "42"); w.Code != 410 {
		t.Fatal("restart reset expiry", w.Code)
	}
	saved, _ := restored.Get(task.ID)
	if saved.Status != StatusRecoveryRequired || !saved.CreatedAt.Equal(created) {
		t.Fatal("query changed task", saved)
	}
}

func TestTaskQueryConcurrentLimitAndCompletedContent(t *testing.T) {
	now := time.Now()
	h := NewRelayHandler(nil, nil, nil, relaySecret, nil)
	h.now = func() time.Time { return now }
	task := &Task{ID: "mtask_concurrent", CreatedAt: now, Status: StatusQueued}
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if h.allowTaskQuery(httptest.NewRecorder(), task, false) {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if allowed.Load() != 1 {
		t.Fatal("concurrent polling bypass", allowed.Load())
	}
	task.Status, task.CallbackState = StatusSucceeded, CallbackConfirmed
	if !h.allowTaskQuery(httptest.NewRecorder(), task, true) {
		t.Fatal("completed video cannot be fetched after checking status")
	}
	now = now.Add(time.Hour)
	if h.allowTaskQuery(httptest.NewRecorder(), task, true) {
		t.Fatal("content bypassed expiry")
	}
}
