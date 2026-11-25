package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLegacyImagesMigrateWithoutAccumulatingInMemory(t *testing.T) {
	root := t.TempDir()
	body := bytes.Repeat([]byte("image-reference"), 300000)
	for i := range 12 {
		task := &Task{ID: fmtID(i), UserID: "2", KeyID: "2", Status: StatusRecoveryRequired, CreatedAt: time.Now(), Request: &UpstreamRequest{Body: body, Method: "POST"}, Response: []byte(`{"error":"upstream unavailable"}`)}
		if err := WriteState(filepath.Join(root, task.ID+".json"), task); err != nil {
			t.Fatal(err)
		}
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	store, err := NewDiskStore(root)
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	if after.TotalAlloc-before.TotalAlloc > 32<<20 {
		t.Fatalf("migration allocated %d bytes for metadata", after.TotalAlloc-before.TotalAlloc)
	}
	for _, task := range store.ListAll() {
		if task.Request == nil || len(task.Request.Body) != 0 || len(task.Response) != 0 || task.RequestData == nil {
			t.Fatal("payload retained in memory")
		}
		r, n, err := store.requestReader(task)
		if err != nil {
			t.Fatal(err)
		}
		if n != int64(len(body)) {
			t.Fatal("stream length changed")
		}
		got, err := io.ReadAll(r)
		r.Close()
		if err != nil || !bytes.Equal(got, body) {
			t.Fatal("migration changed body")
		}
	}
	reopened, err := NewDiskStore(root)
	if err != nil || len(reopened.ListAll()) != 12 {
		t.Fatal("restart failed", err)
	}
}
func fmtID(i int) string { return "mtask_" + string(rune('a'+i)) }

func TestRecoveryExpiresAndKeepsOnlyDiagnosticAndFailure(t *testing.T) {
	root, media := t.TempDir(), t.TempDir()
	store, err := NewDiskStore(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	old := now.Add(-31 * time.Minute)
	fresh := now.Add(-29 * time.Minute)
	for _, v := range []struct {
		id string
		at time.Time
	}{{"old", old}, {"fresh", fresh}} {
		task := &Task{ID: v.id, UserID: "2", KeyID: "2", GroupID: "7", Status: StatusRecoveryRequired, CreatedAt: old.Add(-time.Hour), RecoveryAt: &v.at, CallbackState: CallbackPending, Credential: "secret", ResponseStatus: 503, ErrorMessage: "upstream HTTP 503; acceptance unknown", Request: &UpstreamRequest{Body: []byte("private-image")}, Response: []byte(`{"error":"private detail"}`), Result: map[string]any{"image": "private-image"}}
		if err = store.Create(task); err != nil {
			t.Fatal(err)
		}
		for _, dir := range []string{filepath.Join(media, v.id), filepath.Join(root, "upstream-errors", v.id)} {
			os.MkdirAll(dir, 0700)
			os.WriteFile(filepath.Join(dir, "private"), []byte("image"), 0600)
		}
	}
	stale, _ := store.Get("old")
	var logs bytes.Buffer
	ids, err := store.ExpireRecovery(now, media, log.New(&logs, "", 0))
	if err != nil || len(ids) != 1 || ids[0] != "old" {
		t.Fatal(ids, err)
	}
	expired, _ := store.Get("old")
	if expired.Status != StatusFailed || expired.ErrorCode != "recovery_expired" || expired.CallbackState != CallbackPending || expired.Usage != nil || expired.Credential != "" {
		t.Fatal("expiry state incorrect")
	}
	for _, dir := range []string{store.payloadDir("old"), filepath.Join(media, "old"), filepath.Join(root, "upstream-errors", "old")} {
		if _, err = os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("data remains: %s", dir)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "expired-tasks", "old.json"))
	if err != nil || !bytes.Contains(raw, []byte("503")) || bytes.Contains(raw, []byte("private-image")) || bytes.Contains(raw, []byte("secret")) {
		t.Fatal("diagnostic missing or contains payload")
	}
	if !strings.Contains(logs.String(), "old") {
		t.Fatal("missing expiry log")
	}
	if store.Update(stale) == nil {
		t.Fatal("stale worker resurrected expired task")
	}
	freshTask, _ := store.Get("fresh")
	if freshTask.Status != StatusRecoveryRequired || freshTask.RequestData == nil {
		t.Fatal("fresh task removed")
	}
	store, err = NewDiskStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if task, _ := store.Get("old"); task.ErrorCode != "recovery_expired" {
		t.Fatal("expiry not durable")
	}
	if _, err = store.ExpireRecovery(now.Add(time.Minute), media, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExpiryRequiresDurableDiagnosticBeforeDeletingData(t *testing.T) {
	store, _ := NewDiskStore(t.TempDir())
	old := time.Now().Add(-time.Hour)
	task := &Task{ID: "log_failure", Status: StatusRecoveryRequired, CreatedAt: old, Request: &UpstreamRequest{Body: []byte("keep")}}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(store.root, "expired-tasks", task.ID+".json"), 0700)
	if _, err := store.ExpireRecovery(time.Now(), "", nil); err == nil {
		t.Fatal("expected log write failure")
	}
	got, _ := store.Get(task.ID)
	if got.Status != StatusRecoveryRequired || got.RequestData == nil {
		t.Fatal("removed data without diagnostics")
	}
}

func TestMemoryReservationsFollowAvailableMemoryAndRelease(t *testing.T) {
	available := int64(2 << 30)
	b := memoryBudget{available: func() int64 { return available }}
	release, ok := b.try(1 << 30)
	if !ok {
		t.Fatal("adequate memory rejected")
	}
	if _, ok = b.try(1 << 30); ok {
		t.Fatal("pending allocation overcommitted")
	}
	release()
	release()
	available = 600 << 20
	if _, ok = b.try(128 << 20); ok {
		t.Fatal("OS reserve consumed")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := b.wait(ctx, 128<<20); err == nil {
		t.Fatal("memory wait ignored cancellation")
	}
	available = 2 << 30
	release, ok = b.try(1 << 30)
	if !ok {
		t.Fatal("reservation leaked")
	}
	release()
}

type countResponse struct {
	header http.Header
	bytes  int64
}

func (w *countResponse) Header() http.Header         { return w.header }
func (w *countResponse) WriteHeader(int)             {}
func (w *countResponse) Write(p []byte) (int, error) { w.bytes += int64(len(p)); return len(p), nil }
func TestBase64ResultStreamsWithoutImageAllocation(t *testing.T) {
	store := NewMemoryStore(nil)
	tracker := NewTracker(store, nil, nil, nil)
	defer tracker.Stop()
	media := t.TempDir()
	tracker.ConfigureFiles(media, 64<<20)
	task := &Task{ID: "large", Status: StatusSucceeded, CallbackState: CallbackConfirmed, CreatedAt: time.Now(), Files: []ResultFile{{ID: "f0000", ContentType: "image/png", DownloadURL: "https://media.example/image"}}}
	os.MkdirAll(filepath.Join(media, task.ID), 0700)
	f, err := os.Create(filepath.Join(media, task.ID, "f0000"))
	if err != nil {
		t.Fatal(err)
	}
	block := make([]byte, 64<<10)
	for range 512 {
		if _, err = f.Write(block); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()
	h := NewRelayHandler(nil, store, tracker, relaySecret, nil)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	w := &countResponse{header: make(http.Header)}
	h.handleStandardResult(w, httptest.NewRequest("GET", "/", nil), task, "openai_b64_json")
	runtime.ReadMemStats(&after)
	if w.bytes < 40<<20 {
		t.Fatal("image response incomplete")
	}
	if after.TotalAlloc-before.TotalAlloc > 2<<20 {
		t.Fatalf("image loaded into memory: %d", after.TotalAlloc-before.TotalAlloc)
	}
}

func TestMetadataStripsInlineImagesPreservingTextAndUsage(t *testing.T) {
	raw := []byte(`{"candidates":[{"content":{"parts":[{"text":"a \"quoted\" note"},{"inlineData":{"mimeType":"image/png","data":"AAAA"}}]}}],"usageMetadata":{"count":1}}`)
	summary := compactResponse(raw)
	var out map[string]any
	if json.Unmarshal(summary, &out) != nil || bytes.Contains(summary, []byte("AAAA")) || !bytes.Contains(summary, []byte("usageMetadata")) {
		t.Fatal(string(summary))
	}
}

func TestExpiredTaskNotifiesMainWithoutUpstreamOrSchema(t *testing.T) {
	callbacks := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		callbacks <- payload
		w.WriteHeader(200)
	}))
	defer server.Close()
	store, _ := NewDiskStore(t.TempDir())
	task := &Task{ID: "expired_callback", UserID: "2", KeyID: "2", Status: StatusRecoveryRequired, CreatedAt: time.Now().Add(-time.Hour), CallbackState: CallbackPending, Request: &UpstreamRequest{Body: []byte("image")}}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}
	tracker := NewTracker(store, func(string) (*Schema, bool) { return nil, false }, nil, nil)
	defer tracker.Stop()
	tracker.SetCallback(CallbackConfig{URL: server.URL, Enabled: true, Secret: "test", MaxAttempts: 1})
	tracker.ExpireRecovery()
	select {
	case payload := <-callbacks:
		if payload["status"] != StatusFailed || payload["usage"] != nil {
			t.Fatal("incorrect failure notification")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("missing failure callback")
	}
	awaitTask(t, store, task.ID, func(task *Task) bool { return task.CallbackState == CallbackConfirmed })
}

func TestStreamedRequestRewritesModelWithoutImageAllocation(t *testing.T) {
	body := `{"model":"old","images":[{"image_url":"data:image/png;base64,` + strings.Repeat("AAAA", 2<<20) + `"}],"n":2}`
	var out bytes.Buffer
	if err := rewriteRequestObject(strings.NewReader(body), &out, map[string]any{"model": "mapped", "size": "auto"}); err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["model"] != "mapped" || parsed["size"] != "auto" || parsed["n"] != float64(2) {
		t.Fatal("model mapping lost")
	}
	count := &countResponse{header: make(http.Header)}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if err := rewriteRequestObject(strings.NewReader(body), count, nil); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	if after.TotalAlloc-before.TotalAlloc > 1<<20 {
		t.Fatal("streamed request buffered image")
	}
	for _, bad := range []string{`{"x":"\q"}`, `{"x":bad}`, `{"x":1,}`, `{"x":"\uXYZZ"}`, `{"x":true} trailing`, `{"x":[1,]}`} {
		if rewriteRequestObject(strings.NewReader(bad), io.Discard, nil) == nil {
			t.Fatalf("accepted invalid JSON %s", bad)
		}
	}
}

func TestLegacyCompletedImageRepairsUsageFromSavedFiles(t *testing.T) {
	store, _ := NewDiskStore(t.TempDir())
	schema := testSchema()
	schema.Mode = "sync"
	schema.Response.Usage.Quantity = "$result_files"
	schema.Response.Usage.Unit = UnitImage
	task := &Task{ID: "old_usage", UserID: "2", KeyID: "2", Schema: schema, Status: StatusSucceeded, CreatedAt: time.Now(), Files: []ResultFile{{ID: "f0000"}, {ID: "f0001"}}, CallbackState: CallbackPending}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}
	tracker := NewTracker(store, nil, nil, nil)
	defer tracker.Stop()
	tracker.deliver(task)
	saved, _ := store.Get(task.ID)
	if saved.Usage == nil || saved.Usage.Quantity != 2 {
		t.Fatal("file-based usage lost after offload")
	}
}

func TestContainerMemoryAccountsForReclaimableCache(t *testing.T) {
	if n := cgroupAvailable(4<<30, 3<<30, "inactive_file 2147483648\n"); n != 3<<30 {
		t.Fatalf("inactive cache blocked workers: %d", n)
	}
	if n := cgroupAvailable(4<<30, 3<<30, "inactive_file 2147483648\ntotal_inactive_file 2147483648\n"); n != 3<<30 {
		t.Fatalf("cache counted twice: %d", n)
	}
	if n := cgroupAvailable(4<<30, 3<<30, "inactive_file -1\n"); n != 1<<30 {
		t.Fatal("invalid cache value used")
	}
}
