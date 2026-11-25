package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func errorRecords(t *testing.T, store *MemoryStore, taskID string) []upstreamErrorRecord {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(store.root, "upstream-errors", taskID, "*.body.json"))
	if err != nil {
		t.Fatal(err)
	}
	records := make([]upstreamErrorRecord, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var record upstreamErrorRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
		if runtime.GOOS != "windows" {
			for _, p := range []string{path, filepath.Join(filepath.Dir(path), record.BodyFile)} {
				info, err := os.Stat(p)
				if err != nil || info.Mode().Perm() != 0600 {
					t.Fatalf("private permissions: %v %v", info, err)
				}
			}
		}
	}
	return records
}

func assertErrorBody(t *testing.T, store *MemoryStore, record upstreamErrorRecord, want []byte) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(store.root, "upstream-errors", record.TaskID, record.BodyFile))
	if err != nil || !bytes.Equal(got, want) || record.BodyBytes != int64(len(want)) {
		t.Fatalf("error body not retained verbatim: bytes=%d want=%d err=%v", len(got), len(want), err)
	}
}

func diagnosticTracker(t *testing.T) (*MemoryStore, *Tracker, *Task) {
	t.Helper()
	store, err := NewDiskStore(filepath.Join(t.TempDir(), "tasks"))
	if err != nil {
		t.Fatal(err)
	}
	schema := testSchema()
	task := &Task{ID: "mtask_errors", Schema: schema, SchemaID: schema.ID, UserID: "2", KeyID: "test-key",
		GroupID: "test-group", Model: "test-model", Status: StatusQueued, CreatedAt: time.Now()}
	tracker := NewTracker(store, func(string) (*Schema, bool) { return schema, true }, nil, log.New(io.Discard, "", 0))
	t.Cleanup(tracker.Stop)
	return store, tracker, task
}

func TestCreateErrorRecordsSurviveCallbackRestartAndTaskGC(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   []byte
	}{
		{"json400", 400, []byte(`{"error":{"message":"invalid image","code":"bad_image","details":{"field":"image","nested":[1,2]}}}`)},
		{"html429", 429, []byte("<html>too many requests</html>\n")},
		{"empty403", 403, nil},
		{"binary500", 500, []byte{0, 255, 128, 1}},
		{"timeout408", 408, []byte("request timeout")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, tracker, task := diagnosticTracker(t)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("X-Upstream-Request-Id", "provider-request")
				w.Header().Add("X-Upstream-Detail", "one")
				w.Header().Add("X-Upstream-Detail", "two")
				w.WriteHeader(tc.status)
				w.Write(tc.body)
			}))
			defer upstream.Close()
			task.Request = &UpstreamRequest{Method: "POST", URL: upstream.URL + "/images?api_key=secret", Header: http.Header{"Authorization": {"Bearer secret"}}, Body: []byte(`{"prompt":"private"}`)}
			if err := store.Create(task); err != nil {
				t.Fatal(err)
			}
			tracker.submit(task, task.Schema)
			records := errorRecords(t, store, task.ID)
			if len(records) != 1 {
				t.Fatalf("records=%d", len(records))
			}
			record := records[0]
			if record.Phase != "create" || record.StatusCode != tc.status || record.UserID != "2" || record.Model != task.Model || record.Method != "POST" || record.URL != upstream.URL+"/images" || len(record.Header.Values("X-Upstream-Detail")) != 2 || record.Header.Get("X-Upstream-Request-Id") != "provider-request" {
				t.Fatalf("incomplete metadata: %+v", record)
			}
			assertErrorBody(t, store, record, tc.body)
			saved, _ := store.Get(task.ID)
			expected := StatusFailed
			if tc.status >= 500 || tc.status == 408 {
				expected = StatusRecoveryRequired
			}
			if saved.Status != expected {
				t.Fatalf("changed failure policy: %s", saved.Status)
			}
			// A recovery response must resolve a synchronous wait without resubmission.
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			h := NewRelayHandler(nil, store, nil, relaySecret, nil)
			result := httptest.NewRecorder()
			h.waitStandardResult(result, httptest.NewRequest("GET", "/", nil).WithContext(ctx), saved, "openai_url")
			if result.Code < 400 {
				t.Fatalf("wait hid the upstream failure: %d", result.Code)
			}
			{
				saved.Response = nil // simulate a compacted task snapshot
				diag := httptest.NewRecorder()
				h.handleStandardResult(diag, httptest.NewRequest("GET", "/", nil), saved, "error_context")
				if diag.Header().Get("X-Media-Error-Status") != fmt.Sprint(tc.status) || !bytes.Equal(diag.Body.Bytes(), tc.body) {
					t.Fatalf("lost original diagnostic: %d %q", diag.Code, diag.Body.Bytes())
				}
			}
			// Simulate callback acknowledgement / cleanup, then a restart and task expiry.
			saved.Status, saved.CallbackState, saved.FilesAcknowledged = StatusFailed, CallbackConfirmed, true
			old := time.Now().Add(-48 * time.Hour)
			saved.CompletedAt = &old
			if err := store.Update(saved); err != nil {
				t.Fatal(err)
			}
			public, _ := store.PublicTask(saved.ID)
			publicJSON, _ := json.Marshal(public)
			callback, err := BuildCallbackPayload(saved)
			if err != nil {
				t.Fatal(err)
			}
			for _, raw := range [][]byte{publicJSON, callback} {
				if bytes.Contains(raw, []byte("provider-request")) || bytes.Contains(raw, []byte("upstream-errors")) {
					t.Fatal("private diagnostics leaked")
				}
			}
			restarted, err := NewDiskStore(store.root)
			if err != nil {
				t.Fatal(err)
			}
			if restarted.GC(time.Hour) != 1 {
				t.Fatal("task did not expire")
			}
			records = errorRecords(t, restarted, task.ID)
			if len(records) != 1 {
				t.Fatal("task GC erased error history")
			}
			assertErrorBody(t, restarted, records[0], tc.body)
		})
	}
}

func TestQueryErrorHistoryRetainedAfterSuccess(t *testing.T) {
	store, tracker, task := diagnosticTracker(t)
	task.Status = StatusProcessing
	task.UpstreamTaskID = "provider-task"
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls <= 2 {
			w.Header().Set("Trailer", "X-Error-Trailer")
			w.WriteHeader(503)
			fmt.Fprintf(w, `{"error":{"message":"busy %d"}}`, calls)
			w.Header().Set("X-Error-Trailer", "final-provider-detail")
			return
		}
		io.WriteString(w, `{"data":{"status":"completed"}}`)
	}))
	defer upstream.Close()
	task.UpstreamURL = upstream.URL
	for i := 0; i < 3; i++ {
		status, _, release, err := tracker.pollOnce(task, task.Schema)
		defer release()
		if i < 2 && err == nil {
			t.Fatal("query HTTP error lost")
		}
		if i == 2 && (err != nil || status != "completed") {
			t.Fatalf("success changed: %s %v", status, err)
		}
	}
	records := errorRecords(t, store, task.ID)
	if len(records) != 2 {
		t.Fatalf("query history overwritten: %d", len(records))
	}
	for _, record := range records {
		if record.Phase != "query" || record.StatusCode != 503 || record.Trailer.Get("X-Error-Trailer") != "final-provider-detail" {
			t.Fatalf("missing query metadata: %+v", record)
		}
	}
}

func TestHTTP200UpstreamFailuresAlsoRecorded(t *testing.T) {
	for _, tc := range []struct{ phase, body string }{
		{"query", `{"data":{"status":"failed","error":{"message":"image rejected","code":"policy","details":"all details"}}}`},
		{"query", `<html>unexpected response</html>`},
		{"create", `{"error":{"message":"rejected","code":"bad_input"}}`},
		{"create", `{"message":"missing task id"}`},
	} {
		t.Run(tc.phase+tc.body, func(t *testing.T) {
			store, tracker, task := diagnosticTracker(t)
			resp := &http.Response{StatusCode: 200, Status: "200 OK", Body: io.NopCloser(strings.NewReader(tc.body))}
			if _, err := tracker.readUpstreamResponse(task, tc.phase, resp, 1<<20); err != nil {
				t.Fatal(err)
			}
			records := errorRecords(t, store, task.ID)
			if len(records) != 1 {
				t.Fatalf("logical error missing: %d", len(records))
			}
			assertErrorBody(t, store, records[0], []byte(tc.body))
		})
	}
}

func TestDownloadErrorsRetainFullResponsesAcrossRetries(t *testing.T) {
	store, tracker, task := diagnosticTracker(t)
	tracker.ConfigureFiles(t.TempDir(), 1<<20)
	body := []byte("<html>CDN denied this image: full detail</html>")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-CDN-Request-ID", "cdn-id")
		w.WriteHeader(403)
		w.Write(body)
	}))
	defer upstream.Close()
	for i := 0; i < 2; i++ {
		_, err := tracker.stager.save(context.Background(), task, "f0000", upstream.URL+"/result?token=private")
		if err == nil || err.Error() != "download status 403" {
			t.Fatalf("changed download failure: %v", err)
		}
	}
	records := errorRecords(t, store, task.ID)
	if len(records) != 2 {
		t.Fatalf("download history missing: %d", len(records))
	}
	for _, record := range records {
		if record.Phase != "download" || record.Header.Get("X-CDN-Request-ID") != "cdn-id" || strings.Contains(record.URL, "token") {
			t.Fatalf("download metadata: %+v", record)
		}
		assertErrorBody(t, store, record, body)
	}
}

type brokenUpstreamBody struct{ io.Reader }

func (b brokenUpstreamBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	if err == io.EOF {
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}

func TestPartialAndOversizedResponsesAreNotSilentlyTruncated(t *testing.T) {
	for _, status := range []int{200, 400} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			store, tracker, task := diagnosticTracker(t)
			// Oversized relative to the parser limit: the private archive must still be complete.
			want := bytes.Repeat([]byte("full error details\n"), 4096)
			resp := &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(want))}
			if _, err := tracker.readUpstreamResponse(task, "create", resp, 64); err == nil {
				t.Fatal("parser limit changed")
			}
			records := errorRecords(t, store, task.ID)
			if len(records) != 1 {
				t.Fatal("oversized response missing")
			}
			assertErrorBody(t, store, records[0], want)
		})
		t.Run(fmt.Sprint(status)+"/partial", func(t *testing.T) {
			store, tracker, task := diagnosticTracker(t)
			want := []byte("received before upstream disconnected")
			resp := &http.Response{StatusCode: status, Body: io.NopCloser(brokenUpstreamBody{bytes.NewReader(want)})}
			if _, err := tracker.readUpstreamResponse(task, "create", resp, 1<<20); err == nil {
				t.Fatal("incomplete response was accepted")
			}
			records := errorRecords(t, store, task.ID)
			if len(records) != 1 || !strings.Contains(records[0].ReadError, "unexpected EOF") {
				t.Fatalf("partial response not marked: %+v", records)
			}
			assertErrorBody(t, store, records[0], want)
		})
	}
}

func TestSuccessfulResponseDoesNotProduceErrorArchive(t *testing.T) {
	store, tracker, task := diagnosticTracker(t)
	task.Schema.Mode = "sync"
	raw := []byte(`{"data":[{"url":"image.png"}]}`)
	resp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(raw))}
	got, err := tracker.readUpstreamResponse(task, "create", resp, 1<<20)
	if err != nil || !bytes.Equal(raw, got) || len(errorRecords(t, store, task.ID)) != 0 {
		t.Fatal("successful result changed")
	}
}

func TestErrorRecordStorageFailureDoesNotChangeHTTPFailurePolicy(t *testing.T) {
	store, tracker, task := diagnosticTracker(t)
	if err := os.WriteFile(filepath.Join(store.root, "upstream-errors"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	tracker.logger = log.New(&logs, "", 0)
	want := []byte(`{"error":{"message":"bad input"}}`)
	resp := &http.Response{StatusCode: 400, Body: io.NopCloser(bytes.NewReader(want))}
	got, err := tracker.readUpstreamResponse(task, "create", resp, 1<<20)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("archive failure changed original response: %v", err)
	}
	if !strings.Contains(logs.String(), "persist upstream error") {
		t.Fatal("storage failure was not reported")
	}
}

func TestPrivateDiagnosticDoesNotReuseHTTPErrorAfterDisconnect(t *testing.T) {
	store, tracker, task := diagnosticTracker(t)
	_, err := tracker.readUpstreamResponse(task, "create", &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader("old input error"))}, 1024)
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { conn, _, _ := w.(http.Hijacker).Hijack(); conn.Close() }))
	defer upstream.Close()
	task.Request = &UpstreamRequest{Method: "POST", URL: upstream.URL + "/image?key=private"}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}
	tracker.submit(task, task.Schema)
	saved, _ := store.Get(task.ID)
	if saved.Status != StatusRecoveryRequired {
		t.Fatal(saved.Status)
	}
	w := httptest.NewRecorder()
	NewRelayHandler(nil, store, nil, relaySecret, nil).writeErrorContext(w, saved)
	var diag struct {
		Message string
		Status  int `json:"status_code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &diag); err != nil || !strings.Contains(diag.Message, "EOF") || strings.Contains(diag.Message, "private") || diag.Status != 0 {
		t.Fatalf("unexpected transport diagnostic: %s", w.Body.String())
	}
}
