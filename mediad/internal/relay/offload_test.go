package relay

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPublicTaskResultOnlyExposesSettledLinks(t *testing.T) {
	store, err := NewDiskStore(filepath.Join(t.TempDir(), "tasks"))
	if err != nil {
		t.Fatal(err)
	}
	tracker := NewTracker(store, func(string) (*Schema, bool) { return nil, false }, nil, nil)
	defer tracker.Stop()
	tracker.ConfigureFiles(t.TempDir(), 1<<20)
	tracker.ConfigureDownloads("https://media.example")
	h := NewRelayHandler(nil, store, tracker, relaySecret, nil)
	task := &Task{ID: "mtask_public", UserID: "42", KeyID: "7", Status: StatusQueued, CreatedAt: time.Now(), CallbackState: CallbackPending, Credential: "secret-never-expose", Response: []byte(`{"private":"image-base64-never-expose"}`)}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}
	accepted := httptest.NewRecorder()
	h.writeAccepted(accepted, task, false)
	var creation struct {
		URL string `json:"result_url"`
	}
	if err := json.Unmarshal(accepted.Body.Bytes(), &creation); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(creation.URL, "https://media.example/downloads/mtask_public/result?token=") {
		t.Fatal(creation.URL)
	}
	mux := http.NewServeMux()
	h.Register(mux)
	get := func(target string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", target, nil)
		r.Header.Set("Origin", "https://canvas.example")
		mux.ServeHTTP(w, r)
		return w
	}
	for _, target := range []string{"/downloads/mtask_public/result", creation.URL + "invalid", strings.Replace(creation.URL, "mtask_public", "mtask_other", 1)} {
		if w := get(target); w.Code != 404 {
			t.Fatal("invalid task link accepted", w.Code)
		}
	}
	checkPending := func() {
		t.Helper()
		w := get(creation.URL)
		if w.Code != 200 || strings.Contains(w.Body.String(), "result\"") || !strings.Contains(w.Body.String(), "processing") {
			t.Fatal("unsettled result exposed", w.Body.String())
		}
	}
	checkPending()
	task.Status = StatusSucceeded
	task.Files = []ResultFile{{ID: "f0000", DownloadURL: "https://media.example/downloads/mtask_public/f0000?token=file", ContentType: "image/png"}, {ID: "f0001", DownloadURL: "https://media.example/downloads/mtask_public/f0001?token=file", ContentType: "image/webp"}}
	if err := store.Update(task); err != nil {
		t.Fatal(err)
	}
	checkPending()
	task.CallbackState = CallbackConfirmed
	if err := store.Update(task); err != nil {
		t.Fatal(err)
	}
	w := get(creation.URL)
	if w.Code != 200 || w.Header().Get("Access-Control-Allow-Origin") != "*" || strings.Count(w.Body.String(), "mime_type") != 2 || !strings.Contains(w.Body.String(), "completed") || strings.Contains(w.Body.String(), "secret-never-expose") || strings.Contains(w.Body.String(), "image-base64") {
		t.Fatal(w.Body.String())
	}
	// Tokens stay valid after a process restart; they cannot be used as file tokens.
	restarted, err := NewDiskStore(store.root)
	if err != nil {
		t.Fatal(err)
	}
	mux = http.NewServeMux()
	NewRelayHandler(nil, restarted, tracker, relaySecret, nil).Register(mux)
	if w := get(creation.URL); w.Code != 200 || !strings.Contains(w.Body.String(), "completed") {
		t.Fatal("result link lost after restart", w.Body.String())
	}
	task.FilesAcknowledged = true
	if err := restarted.Update(task); err != nil {
		t.Fatal(err)
	}
	if w := get(creation.URL); w.Code != 410 {
		t.Fatal("expired result returned as ready", w.Code)
	}
}

func TestMediaWaitFlushesAndEncodesEveryImage(t *testing.T) {
	for _, format := range []string{"openai_sse", "openai_edit_sse"} {
		t.Run(format, func(t *testing.T) {
			store := NewMemoryStore(nil)
			task := &Task{ID: "mtask_stream", UserID: "42", KeyID: "7", GroupID: "3", Status: StatusProcessing, CallbackState: CallbackPending, CreatedAt: time.Now()}
			if err := store.Create(task); err != nil {
				t.Fatal(err)
			}
			h := NewRelayHandler(nil, store, nil, relaySecret, nil)
			mux := http.NewServeMux()
			h.Register(mux)
			server := httptest.NewServer(mux)
			defer server.Close()
			target := "/relay/tasks/mtask_stream?response_format=" + format + "&wait=true"
			r := signedRequest(t, "GET", target, nil, nil)
			r.RequestURI = ""
			r.URL.Scheme = "http"
			r.URL.Host = strings.TrimPrefix(server.URL, "http://")
			resp, err := server.Client().Do(r)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			reader := bufio.NewReader(resp.Body)
			line, err := reader.ReadString('\n')
			if err != nil || line != ": keep-alive\n" {
				t.Fatal("no initial heartbeat", line, err)
			}
			task.Status = StatusFailed
			task.ResponseStatus = 400
			task.Response = []byte(`{"error":{"code":"policy_blocked","message":"not allowed"}}`)
			if err := store.Update(task); err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), "event: error") || !strings.Contains(string(body), "policy_blocked") {
				t.Fatal(string(body))
			}
			w := httptest.NewRecorder()
			event := "image_generation.completed"
			if format == "openai_edit_sse" {
				event = "image_edit.completed"
			}
			mediaStreamResult(w, httptest.NewRequest("GET", "/", nil), 200, []byte(`{"created":123,"data":[{"b64_json":"first"},{"b64_json":"second"}]}`), event)
			if strings.Count(w.Body.String(), "event: "+event) != 2 || !strings.Contains(w.Body.String(), `"b64_json":"second"`) || !strings.Contains(w.Body.String(), `"created_at":123`) {
				t.Fatal(w.Body.String())
			}
		})
	}
}

func TestMediaWaitCancellationLeavesGenerationRunning(t *testing.T) {
	store := NewMemoryStore(nil)
	task := &Task{ID: "mtask_cancel", Status: StatusProcessing}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}
	h := NewRelayHandler(nil, store, nil, relaySecret, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h.waitStandardResult(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil).WithContext(ctx), task, "openai_url")
	got, _ := store.Get(task.ID)
	if got.Status != StatusProcessing {
		t.Fatal("client cancellation cancelled paid generation")
	}
}

func TestReferenceUploadConversionRunsOnMedia(t *testing.T) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range map[string]string{"model": "client-alias", "stream": "true", "n": "2", "prompt": "retain prompt"} {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	for _, field := range []string{"image[]", "image[]", "mask"} {
		part, err := w.CreateFormFile(field, "reference.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte("\x89PNG\r\n\x1a\nimage")); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	raw, ct, err := normalizeStandardRequest(AssembleInput{Body: body.Bytes(), ContentType: w.FormDataContentType(), Model: "mapped-model"}, "openai")
	if err != nil || ct != "application/json" {
		t.Fatal(ct, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out["model"] != "mapped-model" || out["n"] != float64(2) || out["prompt"] != "retain prompt" || out["stream"] != true || len(out["images"].([]any)) != 2 || !strings.HasPrefix(out["mask"].(map[string]any)["image_url"].(string), "data:image/png;base64,") {
		t.Fatal("multipart fields or files lost")
	}
	if _, _, err := normalizeStandardRequest(AssembleInput{Body: body.Bytes(), ContentType: "multipart/form-data"}, "openai"); err == nil {
		t.Fatal("bad boundary accepted")
	}
}
