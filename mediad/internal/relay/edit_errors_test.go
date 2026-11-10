package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func editMultipart(t *testing.T) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range map[string]string{"model": "client-alias", "prompt": "edit the square", "stream": "true", "n": "1", "size": "1024x1024", "response_format": "b64_json"} {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"image[]", "image[]", "mask"} {
		part, err := w.CreateFormFile(name, "reference.png")
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
	return body.Bytes(), w.FormDataContentType()
}

func TestImageEditMultipartReachesProviderAndRecoveryDoesNotRetry(t *testing.T) {
	var calls atomic.Int32
	providerError := `{"error":{"message":"provider.example token=secret-provider account-345","type":"private_provider_error"}}`
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/images/edits" || r.Header.Get("Authorization") != "Bearer sk-upstream-key" {
			t.Error("wrong provider route or credential")
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error("provider did not receive multipart", err)
			w.WriteHeader(400)
			return
		}
		defer r.MultipartForm.RemoveAll()
		for k, want := range map[string]string{"model": "gpt-image-2.6-sunburst", "prompt": "edit the square", "n": "1", "size": "1024x1024", "stream": "false", "response_format": "url"} {
			if got := r.FormValue(k); got != want || len(r.MultipartForm.Value[k]) != 1 {
				t.Errorf("%s=%q, want %q exactly once", k, got, want)
			}
		}
		if len(r.MultipartForm.File["image[]"]) != 2 || len(r.MultipartForm.File["mask"]) != 1 {
			t.Error("reference images or mask lost")
		}
		for _, files := range r.MultipartForm.File {
			for _, file := range files {
				f, err := file.Open()
				if err != nil {
					t.Error(err)
					continue
				}
				raw, err := io.ReadAll(f)
				f.Close()
				if err != nil || !bytes.Equal(raw, []byte("\x89PNG\r\n\x1a\nimage")) || file.Header.Get("Content-Type") != "image/png" {
					t.Error("image bytes or content type changed")
				}
			}
		}
		w.WriteHeader(502)
		io.WriteString(w, providerError)
	}))
	defer up.Close()
	schema := &Schema{ID: "klinkw-gpt-image", Version: 1, Mode: "sync", Upstream: UpstreamSpec{Create: CallSpec{Method: "POST", Path: "/v1/images/generations"}}, Request: RequestSpec{BodyMode: "passthrough", Credential: map[string]string{"Authorization": "Bearer {credential}"}, HeaderParams: map[string]string{"Content-Type": "application/json"}, StaticFields: map[string]any{"stream": false, "response_format": "url"}}}
	registry := NewSchemaRegistry(nil)
	if err := registry.Put(schema); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "tasks")
	store, err := NewDiskStore(root)
	if err != nil {
		t.Fatal(err)
	}
	tracker := NewTracker(store, registry.Get, nil, nil)
	defer tracker.Stop()
	h := NewRelayHandler(registry, store, tracker, relaySecret, nil)
	mux := http.NewServeMux()
	h.Register(mux)
	body, ct := editMultipart(t)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, signedRequest(t, "POST", "/relay/klinkw-gpt-image/v1/images/edits", body, map[string]string{HeaderUpstreamURL: up.URL, HeaderProtocol: "openai", HeaderModel: "gpt-image-2.6-sunburst", "Content-Type": ct}))
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var accepted struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &accepted); err != nil || accepted.TaskID == "" {
		t.Fatal("missing task ID", err)
	}
	// Wait while the provider call is in progress, rather than only querying a terminal fixture.
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, signedRequest(t, "GET", "/relay/tasks/"+accepted.TaskID+"?response_format=openai_url&wait=true", nil, nil).WithContext(ctx))
	if ctx.Err() != nil || w.Code != 502 || !strings.Contains(w.Body.String(), "generation_unavailable") || !strings.Contains(w.Body.String(), accepted.TaskID) || strings.Contains(w.Body.String(), "provider") {
		t.Fatal("recovery must promptly return a safe error", w.Code, w.Body.String())
	}
	saved, _ := store.Get(accepted.TaskID)
	if saved.Status != StatusRecoveryRequired || string(saved.Response) != providerError || saved.Request == nil || !strings.HasPrefix(saved.Request.ContentType, "multipart/form-data;") || saved.CallbackAttempts != 0 {
		t.Fatal("lost recovery state, response, or saved multipart request")
	}
	if saved.Parameters["n"] != float64(1) || saved.Parameters["size"] != "1024x1024" {
		t.Fatal("billing fallback parameters lost", saved.Parameters)
	}
	restarted, err := NewDiskStore(root)
	if err != nil {
		t.Fatal(err)
	}
	resumed := NewTracker(restarted, registry.Get, nil, nil)
	defer resumed.Stop()
	if n := resumed.ResumeAll(); n != 0 {
		t.Fatal("recovery task must not be scheduled on restart", n)
	}
	resumed.Stop()
	if calls.Load() != 1 {
		t.Fatal("unknown acceptance caused duplicate generation", calls.Load())
	}
}

func TestTaskErrorsNeverExposeProviderDetails(t *testing.T) {
	for _, status := range []string{StatusFailed, StatusRecoveryRequired} {
		for _, responseStatus := range []int{200, 400, 401, 403, 413, 415, 422, 429, 502} {
			t.Run(status+"/"+http.StatusText(responseStatus), func(t *testing.T) {
				store := NewMemoryStore(nil)
				private := "provider.example sk-private-token account-345 /www/private"
				task := &Task{ID: "mtask_private", UserID: "42", KeyID: "7", Status: status, ResponseStatus: responseStatus, Response: []byte(`{"error":{"message":"` + private + `","code":"private_code"}}`), ErrorMessage: private, LastCallbackError: private, Result: map[string]any{"error": private}, CallbackState: CallbackPending}
				if err := store.Create(task); err != nil {
					t.Fatal(err)
				}
				h := NewRelayHandler(nil, store, nil, relaySecret, nil)
				mux := http.NewServeMux()
				h.Register(mux)
				for _, format := range []string{"", "state", "openai_url", "openai_b64_json", "gemini", "grok_video", "openai_url&wait=true", "gemini&wait=true", "openai_sse&wait=true", "openai_edit_sse&wait=true", "public"} {
					target := "/relay/tasks/" + task.ID
					if format != "" {
						target += "?response_format=" + format
					}
					if format == "public" {
						target = "/downloads/" + task.ID + "/result?token=" + h.resultToken(task.ID)
					}
					ctx, cancel := context.WithTimeout(t.Context(), time.Second)
					w := httptest.NewRecorder()
					mux.ServeHTTP(w, signedRequest(t, "GET", target, nil, nil).WithContext(ctx))
					cancel()
					for _, secret := range []string{"provider.example", "sk-private-token", "account-345", "/www/private", "private_code"} {
						if strings.Contains(w.Body.String(), secret) {
							t.Fatalf("%s exposed %s: %s", format, secret, w.Body.String())
						}
					}
					if !strings.Contains(w.Body.String(), task.ID) || !strings.Contains(w.Body.String(), "Generation") {
						t.Fatal("missing safe error or task ID", format, w.Code, w.Body.String())
					}
					if format == "openai_url" || format == "gemini" {
						wantStatus := 502
						if status == StatusFailed && (responseStatus == 400 || responseStatus == 413 || responseStatus == 415 || responseStatus == 422 || responseStatus == 429) {
							wantStatus = responseStatus
						}
						if w.Code != wantStatus || w.Header().Get("X-Task-Id") != task.ID {
							t.Fatal("wrong public error status or task header", w.Code, w.Header())
						}
						if format == "gemini" {
							var envelope struct {
								Error struct {
									Code   int    `json:"code"`
									Status string `json:"status"`
								} `json:"error"`
							}
							if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || envelope.Error.Code != wantStatus || envelope.Error.Status == "" {
								t.Fatal("invalid Gemini error envelope", w.Body.String())
							}
						}
					}
					if strings.HasPrefix(format, "openai_") && strings.Contains(format, "sse") && !strings.Contains(w.Body.String(), "event: error") {
						t.Fatal("missing SSE error event")
					}
				}
				saved, _ := store.Get(task.ID)
				if saved.ErrorMessage != private || !strings.Contains(string(saved.Response), private) || saved.Status != status {
					t.Fatal("public error changed private recovery state")
				}
				w := httptest.NewRecorder()
				NewManagementHandler(nil, store, nil, "token").handlePending(w, httptest.NewRequest("GET", "/management/tasks/pending", nil))
				if !strings.Contains(w.Body.String(), private) {
					t.Fatal("private management diagnostic lost")
				}
			})
		}
	}
}

func TestImageEditMultipartValidationAndOtherSchemas(t *testing.T) {
	schema := &Schema{Upstream: UpstreamSpec{Create: CallSpec{Path: "/v1/images/generations"}}, Request: RequestSpec{BodyMode: "passthrough"}}
	body, ct := editMultipart(t)
	for _, configure := range []func(*AssembleInput){
		func(in *AssembleInput) { in.Schema.Request.BodyMode = "mapped" },
		func(in *AssembleInput) {
			in.Schema.Request.Multipart = &MultipartSpec{FilesField: "image", ToBodyField: "images"}
		},
		func(in *AssembleInput) { in.Schema.Upstream.Create.Path = "/custom/create" },
	} {
		copy := *schema
		in := AssembleInput{Schema: &copy, ClientPath: "/v1/images/edits", Body: body, ContentType: ct, Model: "selected"}
		configure(&in)
		raw, contentType, err := normalizeStandardRequest(in, "openai")
		if err != nil || contentType != "application/json" || !json.Valid(raw) {
			t.Fatal("other schema conversion changed", contentType, err)
		}
	}
	for _, tc := range []struct {
		name, field, value, fileField, fileData string
		count                                   int
	}{
		{name: "invalid stream", field: "stream", value: "not-a-bool"},
		{name: "invalid count", field: "n", value: "not-a-number"},
		{name: "unsupported file", fileField: "image", fileData: "not an image", count: 1},
		{name: "unsupported field", fileField: "attachment", fileData: "\x89PNG\r\n\x1a\nimage", count: 1},
		{name: "too many images", fileField: "image[]", fileData: "\x89PNG\r\n\x1a\nimage", count: 17},
		{name: "oversize field", field: "prompt", value: strings.Repeat("x", (20<<20)+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			w := multipart.NewWriter(&body)
			if tc.field != "" {
				if err := w.WriteField(tc.field, tc.value); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < tc.count; i++ {
				part, err := w.CreateFormFile(tc.fileField, "reference.png")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(part, tc.fileData); err != nil {
					t.Fatal(err)
				}
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			if _, _, err := buildImageEditMultipart(AssembleInput{Schema: schema, Body: body.Bytes(), ContentType: w.FormDataContentType()}); err == nil {
				t.Fatal("invalid multipart accepted")
			}
		})
	}
	if _, _, err := buildImageEditMultipart(AssembleInput{Schema: schema, Body: body, ContentType: "multipart/form-data"}); err == nil {
		t.Fatal("missing boundary accepted")
	}
}
