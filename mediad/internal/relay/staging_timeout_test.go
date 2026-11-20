package relay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStagingFailureDeadlineRetriesRestartAndShutdown(t *testing.T) {
	for _, mode := range []string{"expired_restart", "retry_exhausted", "slow_download", "slow_refresh", "shutdown", "success"} {
		t.Run(mode, func(t *testing.T) {
			var downloads, queries, callbacks atomic.Int32
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/query" {
					queries.Add(1)
					<-r.Context().Done()
					return
				}
				downloads.Add(1)
				if mode == "slow_download" || mode == "shutdown" {
					<-r.Context().Done()
					return
				}
				_, _ = w.Write([]byte("image"))
			}))
			defer up.Close()
			callback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload struct {
					Status string `json:"status"`
					Usage  *Usage `json:"usage"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				if mode != "success" && (payload.Status != StatusFailed || payload.Usage != nil) {
					t.Error("staging failure must not bill successful usage")
				}
				callbacks.Add(1)
				w.WriteHeader(200)
			}))
			defer callback.Close()
			store, err := NewDiskStore(filepath.Join(t.TempDir(), "tasks"))
			if err != nil {
				t.Fatal(err)
			}
			completed := time.Now()
			if mode == "expired_restart" {
				completed = completed.Add(-resultStagingTimeout - time.Second)
			} else if mode == "slow_download" || mode == "slow_refresh" {
				completed = completed.Add(-resultStagingTimeout + 300*time.Millisecond)
			}
			task := &Task{ID: "mtask_staging", UserID: "2", KeyID: "test-key", Status: StatusDownloading,
				Schema:    &Schema{Mode: "sync"},
				CreatedAt: time.Now(), CompletedAt: &completed, CallbackState: CallbackPending,
				Request: &UpstreamRequest{Body: []byte("reference")}, Response: []byte(`{"saved":"upstream evidence"}`),
				Usage: &Usage{Quantity: 1, Unit: UnitImage}, Result: map[string]any{"url": up.URL + "/image"}}
			if mode == "retry_exhausted" || mode == "slow_refresh" {
				task.Result = nil
			}
			if mode == "slow_refresh" {
				task.UpstreamTaskID = "up-existing"
				task.UpstreamURL = up.URL
				task.Schema = &Schema{Upstream: UpstreamSpec{Query: &CallSpec{Path: "/query", Method: "GET"}}}
			}
			if err := store.Create(task); err != nil {
				t.Fatal(err)
			}
			if mode == "expired_restart" {
				store, err = NewDiskStore(store.root)
				if err != nil {
					t.Fatal(err)
				}
				task, _ = store.Get(task.ID)
			}
			tracker := NewTracker(store, func(string) (*Schema, bool) { return nil, false }, nil, nil)
			defer tracker.Stop()
			tracker.ConfigureFiles(t.TempDir(), 1024)
			tracker.ConfigureDownloads("https://media.example")
			tracker.ConfigureRuntime(1, 1, 1800, 120, 1)
			if mode == "slow_refresh" {
				tracker.downloadAttempts = 3
			}
			if err := tracker.SetCallback(CallbackConfig{URL: callback.URL, Secret: "callback", Enabled: true}); err != nil {
				t.Fatal(err)
			}
			if mode == "shutdown" {
				tracker.Track(task)
				deadline := time.Now().Add(time.Second)
				for downloads.Load() == 0 && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
				tracker.Stop()
			} else {
				tracker.finishFiles(task)
			}
			got, _ := store.Get(task.ID)
			if err := store.loadPayloadFields(got, true, false); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "success":
				if got.Status != StatusSucceeded || len(got.Files) != 1 || got.Usage == nil {
					t.Fatal("successful download changed")
				}
			case "shutdown":
				if got.Status != StatusDownloading || got.ErrorCode != "" || got.Request == nil || callbacks.Load() != 0 {
					t.Fatal("shutdown must preserve unfinished work")
				}
			default:
				code := "result_staging_timeout"
				if mode == "retry_exhausted" {
					code = "result_staging_failed"
				}
				if got.Status != StatusFailed || got.ErrorCode != code || got.Usage != nil || got.Request != nil || !got.FilesAcknowledged || got.CallbackState != CallbackConfirmed || callbacks.Load() != 1 || string(got.Response) != `{"saved":"upstream evidence"}` {
					t.Fatal("failed task was not settled safely")
				}
				if mode == "expired_restart" && downloads.Load() != 0 {
					t.Fatal("restart retried an expired download")
				}
				if mode == "slow_refresh" && queries.Load() != 1 {
					t.Fatal("query refresh did not use the staging deadline")
				}
				reopened, err := NewDiskStore(store.root)
				if err != nil {
					t.Fatal(err)
				}
				got, _ = reopened.Get(task.ID)
				if got.ErrorCode != code || got.Status != StatusFailed {
					t.Fatal("restart lost terminal failure")
				}
			}
		})
	}
}

func TestStagingFailureStopsPollingAcrossRoutesAndFormats(t *testing.T) {
	for _, code := range []string{"result_staging_timeout", "result_staging_failed"} {
		store := NewMemoryStore(nil)
		task := &Task{ID: "mtask_stopped", UserID: "2", KeyID: "7", CreatedAt: time.Now().Add(-2 * time.Hour), Status: StatusFailed, ErrorCode: code, ErrorMessage: "Result retrieval ended. Stop polling."}
		if err := store.Create(task); err != nil {
			t.Fatal(err)
		}
		h := NewRelayHandler(nil, store, nil, relaySecret, nil)
		mux := http.NewServeMux()
		h.Register(mux)
		public := "/downloads/" + task.ID + "/result?token=" + h.resultToken(task.ID)
		for _, path := range []string{"/relay/tasks/" + task.ID, "/relay/tasks/" + task.ID + "?response_format=state", "/relay/tasks/" + task.ID + "?response_format=grok_video", public} {
			for range 2 {
				r := signedRequest(t, "GET", path, nil, map[string]string{HeaderUserID: "2", HeaderClientQuery: "status"})
				w := httptest.NewRecorder()
				mux.ServeHTTP(w, r)
				assertStagingStop(t, w, code)
			}
		}
		for _, format := range []string{"state", "openai_url", "openai_b64_json", "gemini", "grok_video"} {
			w := httptest.NewRecorder()
			h.handleStandardResult(w, httptest.NewRequest("GET", "/", nil), task, format)
			assertStagingStop(t, w, code)
		}
		r := signedRequest(t, "GET", "/relay/tasks/"+task.ID, nil, map[string]string{HeaderUserID: "3", HeaderClientQuery: "status"})
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 404 {
			t.Fatal("terminal error bypassed ownership check")
		}
	}
}

func assertStagingStop(t *testing.T, w *httptest.ResponseRecorder, code string) {
	t.Helper()
	if w.Code != http.StatusGone || w.Header().Get("Retry-After") != "" || !strings.Contains(w.Body.String(), `"stop_polling":true`) || !strings.Contains(w.Body.String(), `"retryable":false`) || !strings.Contains(w.Body.String(), code) || !strings.Contains(w.Body.String(), `"task_status":"failed"`) {
		t.Fatal("client was not told to stop polling", w.Code, w.Body.String())
	}
}
