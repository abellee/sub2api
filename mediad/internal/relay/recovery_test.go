package relay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func awaitTask(t *testing.T, s *MemoryStore, id string, fn func(*Task) bool) *Task {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		task, ok := s.Get(id)
		if ok && fn(task) {
			return task
		}
		time.Sleep(10 * time.Millisecond)
	}
	task, _ := s.Get(id)
	t.Fatalf("task did not reach expected state: %+v", task)
	return nil
}

func TestCrashDuringSubmitDoesNotBlindlyCreateAnotherTask(t *testing.T) {
	for _, safe := range []bool{false, true} {
		t.Run(map[bool]string{false: "unknown", true: "provider_idempotent"}[safe], func(t *testing.T) {
			var creates atomic.Int32
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				creates.Add(1)
				if r.Header.Get("Idempotency-Key") != "mtask_crash" {
					t.Error("lost stable provider key")
				}
				w.Write([]byte(`{"data":{"task_id":"up-existing"}}`))
			}))
			defer up.Close()
			schema := testSchema()
			if safe {
				schema.Request.IdempotencyHeader = "Idempotency-Key"
			}
			root := t.TempDir()
			store, err := NewDiskStore(root)
			if err != nil {
				t.Fatal(err)
			}
			task := &Task{ID: "mtask_crash", Schema: schema, SchemaID: schema.ID, Status: StatusSubmitting, UserID: "42", KeyID: "7", CreatedAt: time.Now(), Request: &UpstreamRequest{Method: "POST", URL: up.URL, Header: make(http.Header), Body: []byte(`{}`)}}
			if err := store.Create(task); err != nil {
				t.Fatal(err)
			}
			restarted, err := NewDiskStore(root)
			if err != nil {
				t.Fatal(err)
			}
			tracker := NewTracker(restarted, func(string) (*Schema, bool) { return nil, false }, nil, nil)
			defer tracker.Stop()
			tracker.ResumeAll()
			if safe {
				awaitTask(t, restarted, task.ID, func(t *Task) bool { return t.UpstreamTaskID == "up-existing" })
				if creates.Load() != 1 {
					t.Fatal("expected one idempotent replay")
				}
			} else {
				awaitTask(t, restarted, task.ID, func(t *Task) bool { return t.Status == StatusRecoveryRequired })
				if creates.Load() != 0 {
					t.Fatal("unsafe automatic resubmission")
				}
			}
		})
	}
}

func TestAcceptedTaskIsDurableAndIdempotencyIsScoped(t *testing.T) {
	fx := newFixture(t)
	body := []byte(`{"prompt":"a"}`)
	submit := func(user string, body []byte) *httptest.ResponseRecorder {
		r := signedRequest(t, "POST", "/relay/mj-proxy-v1/x", body, map[string]string{HeaderUpstreamURL: fx.upstreamURL, HeaderIdempotencyKey: "same-client-key", HeaderUserID: user})
		w := httptest.NewRecorder()
		fx.mux.ServeHTTP(w, r)
		return w
	}
	a := submit("42", body)
	b := submit("99", body)
	var aa, bb map[string]any
	json.Unmarshal(a.Body.Bytes(), &aa)
	json.Unmarshal(b.Body.Bytes(), &bb)
	if a.Code != 202 || b.Code != 202 || aa["task_id"] == bb["task_id"] {
		t.Fatal("same client key must not cross user boundaries")
	}
	if w := submit("42", []byte(`{"prompt":"different"}`)); w.Code != 409 {
		t.Fatalf("same key with changed input=%d", w.Code)
	}
	diskFx := newFixture(t)
	diskFx.handler.store.root = filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(diskFx.handler.store.root, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	r := signedRequest(t, "POST", "/relay/mj-proxy-v1/x", body, map[string]string{HeaderUpstreamURL: fx.upstreamURL})
	w := httptest.NewRecorder()
	diskFx.mux.ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatalf("must not accept task if disk write fails: %d", w.Code)
	}
}

func TestUnclaimedFilesSurviveGCAndDownloadFailure(t *testing.T) {
	store := NewMemoryStore(nil)
	old := time.Now().Add(-48 * time.Hour)
	task := &Task{ID: "mtask_keep", Status: StatusSucceeded, CreatedAt: old, CompletedAt: &old, CallbackState: CallbackConfirmed}
	store.Create(task)
	if n := store.GC(time.Hour); n != 0 {
		t.Fatal("unclaimed result deleted")
	}
	task.Status = StatusDownloading
	task.Result = map[string]any{"images": []any{"data:image/png;base64,aGVsbG8="}}
	store.Update(task)
	root := filepath.Join(t.TempDir(), "file")
	os.WriteFile(root, []byte("blocked"), 0600)
	stager := &ResultStager{Root: root, MaxBytes: 100}
	if _, err := stager.Stage(t.Context(), task); err == nil {
		t.Fatal("disk failure must not count as successful staging")
	}
	stager.Root = t.TempDir()
	files, err := stager.Stage(t.Context(), task)
	if err != nil || len(files) != 1 {
		t.Fatalf("retry staging: %v", err)
	}
	task.Files = files
	task.Status = StatusSucceeded
	store.Update(task)
	store.AcknowledgeFiles(task.ID)
	// 通知线程的旧副本不能覆盖文件已交接标记。
	store.Update(task)
	if saved, _ := store.Get(task.ID); !saved.FilesAcknowledged {
		t.Fatal("concurrent callback overwrote file acknowledgement")
	}
}

func TestSavedResponseSurvivesTemporaryStorageOutage(t *testing.T) {
	root := filepath.Join(t.TempDir(), "tasks")
	store, err := NewDiskStore(root)
	if err != nil {
		t.Fatal(err)
	}
	schema := testSchema()
	schema.Mode = "sync"
	schema.Upstream.Query = nil
	schema.Response.Result = map[string]string{"images": "data"}
	task := &Task{ID: "mtask_saved", Schema: schema, SchemaID: schema.ID, Status: StatusSubmitting, CreatedAt: time.Now(), UserID: "42", KeyID: "7", Response: []byte(`{"data":[{"b64_json":"aGVsbG8="}]}`)}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, root+".backup"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root, []byte("temporary storage outage"), 0600); err != nil {
		t.Fatal(err)
	}
	tracker := NewTracker(store, func(string) (*Schema, bool) { return nil, false }, nil, nil)
	defer tracker.Stop()
	tracker.ConfigureFiles(t.TempDir(), 100)
	tracker.Track(task)
	time.Sleep(50 * time.Millisecond)
	if state, _ := store.Get(task.ID); state.Status != StatusSubmitting {
		t.Fatal("storage error must not report successful generation")
	}
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root+".backup", root); err != nil {
		t.Fatal(err)
	}
	state := awaitTask(t, store, task.ID, func(t *Task) bool { return t.Status == StatusSucceeded && len(t.Files) == 1 })
	if state.Files[0].Size != 5 {
		t.Fatalf("saved response was not recovered: %+v", state.Files)
	}
}
