package relay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestFailedTaskPreservesPrivateErrorContext(t *testing.T) {
	root := filepath.Join(t.TempDir(), "tasks")
	store, err := NewDiskStore(root)
	if err != nil {
		t.Fatal(err)
	}
	private := "provider.example sk-private-token account-344 "
	body := private + strings.Repeat("x", 10<<10)
	task := &Task{ID: "mtask_error_context", UserID: "42", KeyID: "7", Status: StatusProcessing, ResponseStatus: 400, Response: []byte(body)}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}
	tracker := NewTracker(store, nil, nil, nil)
	defer tracker.Stop()
	tracker.complete(task, StatusFailed, nil, nil, "upstream HTTP 400")
	// Callback updates clear normal response storage; the matching prefix must survive restart.
	restarted, err := NewDiskStore(root)
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := restarted.Get(task.ID)
	if !ok || len(saved.Response) != 0 || !saved.FilesAcknowledged || string(saved.ErrorBody) != body[:8<<10] {
		t.Fatal("private matching prefix lost or unbounded after cleanup")
	}
	h := NewRelayHandler(nil, restarted, nil, relaySecret, nil)
	mux := http.NewServeMux()
	h.Register(mux)
	target := "/relay/tasks/" + task.ID + "?response_format=error_context"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, signedRequest(t, "GET", target, nil, nil))
	var details struct {
		StatusCode int    `json:"status_code"`
		Message    string `json:"message"`
		Body       string `json:"body"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &details); err != nil || w.Code != 200 || details.StatusCode != 400 || details.Message != "upstream HTTP 400" || details.Body != body[:8<<10] || w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("signed context missing", w.Code, err)
	}
	for _, tc := range []struct {
		name string
		req  *http.Request
		code int
	}{
		{"unsigned", httptest.NewRequest("GET", target, nil), 401},
		{"other user", signedRequest(t, "GET", target, nil, map[string]string{HeaderUserID: "43"}), 404},
		{"other key", signedRequest(t, "GET", target, nil, map[string]string{HeaderKeyID: "8"}), 404},
		{"state", signedRequest(t, "GET", "/relay/tasks/"+task.ID+"?response_format=state", nil, nil), 200},
		{"normal query", signedRequest(t, "GET", "/relay/tasks/"+task.ID, nil, nil), 200},
		{"standard result", signedRequest(t, "GET", "/relay/tasks/"+task.ID+"?response_format=openai_url", nil, nil), 400},
		{"public result", httptest.NewRequest("GET", "/downloads/"+task.ID+"/result?response_format=error_context&token="+h.resultToken(task.ID), nil), 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, tc.req)
			if w.Code != tc.code {
				t.Fatal("unexpected status", w.Code, w.Body.String())
			}
			for _, secret := range []string{"provider.example", "sk-private-token", "account-344", "error_body", "upstream HTTP"} {
				if strings.Contains(w.Body.String(), secret) {
					t.Fatal("private error context leaked", w.Body.String())
				}
			}
		})
	}
}

func TestErrorContextOnlyAvailableForTerminalErrors(t *testing.T) {
	for _, status := range []string{StatusProcessing, StatusSucceeded, StatusFailed, StatusRecoveryRequired, StatusInterrupted} {
		t.Run(status, func(t *testing.T) {
			store := NewMemoryStore(nil)
			task := &Task{ID: "mtask_context_status", UserID: "42", KeyID: "7", Status: status, ResponseStatus: 502, Response: []byte("private response")}
			if err := store.Create(task); err != nil {
				t.Fatal(err)
			}
			h := NewRelayHandler(nil, store, nil, relaySecret, nil)
			mux := http.NewServeMux()
			h.Register(mux)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, signedRequest(t, "GET", "/relay/tasks/"+task.ID+"?response_format=error_context", nil, nil))
			want := 200
			if status == StatusProcessing || status == StatusSucceeded {
				want = 409
			}
			if w.Code != want || (want == 200 && !strings.Contains(w.Body.String(), "private response")) || (want != 200 && strings.Contains(w.Body.String(), "private response")) {
				t.Fatal("unexpected context visibility", w.Code, w.Body.String())
			}
		})
	}
}

func TestAsyncFailurePreservesPollResponseAfterRestart(t *testing.T) {
	const failure = `{"data":{"status":"failed","error":{"message":"generation timed out"}}}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Error("must only query the existing task")
		}
		_, _ = w.Write([]byte(failure))
	}))
	defer upstream.Close()
	root := filepath.Join(t.TempDir(), "tasks")
	store, err := NewDiskStore(root)
	if err != nil {
		t.Fatal(err)
	}
	schema := testSchema()
	schema.Response.ErrorMessage = "data.error.message"
	task := &Task{ID: "mtask_poll_failure", UserID: "2", KeyID: "7", Schema: schema, SchemaID: schema.ID,
		Status: StatusProcessing, UpstreamURL: upstream.URL, UpstreamTaskID: "task_accepted",
		Response: []byte(`{"request_id":"task_accepted"}`)}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}
	tracker := NewTracker(store, nil, upstream.Client(), nil)
	defer tracker.Stop()
	tracker.trackLoop(task.ID)
	restarted, err := NewDiskStore(root)
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := restarted.Get(task.ID)
	if !ok || saved.Status != StatusFailed || saved.ErrorMessage != "generation timed out" || string(saved.ErrorBody) != failure {
		t.Fatalf("failure response not preserved: %+v", saved)
	}
	if len(saved.Response) != 0 || saved.Usage != nil || !saved.FilesAcknowledged {
		t.Fatal("failure cleanup or billing behavior changed")
	}
}
