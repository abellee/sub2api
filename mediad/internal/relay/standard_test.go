package relay

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStandardResultsAndGeminiImageCounting(t *testing.T) {
	for _, kind := range []string{"png", "gif", "svg", "gemini"} {
		t.Run(kind, func(t *testing.T) {
			mime, raw := "image/png", []byte("\x89PNG\r\n\x1a\nimage")
			if kind == "gif" {
				mime = "image/gif"
				raw = []byte("GIF89aimage")
			}
			if kind == "svg" {
				mime = "image/svg+xml"
				raw = []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)
			}
			encoded := base64.StdEncoding.EncodeToString(raw)
			response := map[string]any{"data": []any{map[string]any{"url": "data:" + mime + ";base64," + encoded}}}
			createPath := "/v1/images/generations"
			quantity, result := any("data"), map[string]string{"files": "data"}
			if kind == "gemini" {
				createPath = "/v1beta/models/{model}:generateContent"
				quantity = "$result_files"
				result = map[string]string{"candidates": "candidates"}
				part := map[string]any{"inlineData": map[string]any{"mimeType": mime, "data": encoded}}
				response = map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": "https://example.com is text, not a result"}, part}}}, map[string]any{"content": map[string]any{"parts": []any{part}}}}}
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				want := strings.ReplaceAll(createPath, "{model}", "gemini-3-pro-image")
				if r.URL.Path != want || r.Header.Get("Authorization") != "Bearer sk-upstream-key" {
					t.Error("upstream request mismatch", r.URL.Path)
				}
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer upstream.Close()
			callbacks := make(chan map[string]any, 1)
			callback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var v map[string]any
				_ = json.NewDecoder(r.Body).Decode(&v)
				callbacks <- v
				w.WriteHeader(200)
			}))
			defer callback.Close()
			registry := NewSchemaRegistry(nil)
			schema := &Schema{ID: "standard", Version: 1, Mode: "sync", Upstream: UpstreamSpec{Create: CallSpec{Method: "POST", Path: createPath}}, Request: RequestSpec{BodyMode: "passthrough", Credential: map[string]string{"Authorization": "Bearer {credential}"}}, Response: ResponseSpec{Usage: UsageSpec{Quantity: quantity, Unit: UnitImage}, Result: result}}
			if err := registry.Put(schema); err != nil {
				t.Fatal(err)
			}
			store, err := NewDiskStore(filepath.Join(t.TempDir(), "tasks"))
			if err != nil {
				t.Fatal(err)
			}
			tracker := NewTracker(store, registry.Get, nil, nil)
			defer tracker.Stop()
			tracker.ConfigureFiles(t.TempDir(), 1<<20)
			tracker.ConfigureDownloads("https://d.tokenfree.dev")
			if err := tracker.SetCallback(CallbackConfig{URL: callback.URL, Enabled: true, Secret: "cb"}); err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			h := NewRelayHandler(registry, store, tracker, relaySecret, nil)
			h.Register(mux)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, signedRequest(t, "POST", "/relay/standard", []byte(`{"prompt":"test"}`), map[string]string{HeaderUpstreamURL: upstream.URL, HeaderModel: "gemini-3-pro-image", "Content-Type": "application/json"}))
			if w.Code != 202 {
				t.Fatal(w.Code, w.Body.String())
			}
			var accepted map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &accepted)
			id := accepted["task_id"].(string)
			select {
			case cb := <-callbacks:
				want := float64(1)
				if kind == "gemini" {
					want = 2
				}
				if cb["usage"].(map[string]any)["quantity"] != want || len(cb["files"].([]any)) != int(want) {
					t.Fatal("image count", cb["usage"])
				}
			case <-time.After(3 * time.Second):
				t.Fatal("missing callback")
			}
			deadline := time.Now().Add(3 * time.Second)
			for {
				task, _ := store.Get(id)
				if task.CallbackState == CallbackConfirmed {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("not confirmed")
				}
				time.Sleep(time.Millisecond)
			}
			format := "openai_b64_json"
			if kind == "gemini" {
				format = "gemini"
			}
			target := "/relay/tasks/" + id + "?response_format=" + format
			w = httptest.NewRecorder()
			mux.ServeHTTP(w, signedRequest(t, "GET", target, nil, nil))
			if w.Code != 200 {
				t.Fatal(w.Code, w.Body.String())
			}
			var out map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &out)
			if kind == "gemini" {
				if len(out["candidates"].([]any)) != 2 || len(out["download_urls"].([]any)) != 2 {
					t.Fatal("native Gemini response lost")
				}
			} else if out["data"].([]any)[0].(map[string]any)["b64_json"] != encoded {
				t.Fatal("standard base64 mismatch")
			}
			w = httptest.NewRecorder()
			mux.ServeHTTP(w, signedRequest(t, "GET", target, nil, map[string]string{HeaderUserID: "43"}))
			if w.Code != 404 {
				t.Fatal("owner check bypassed", w.Code)
			}
			task, _ := store.Get(id)
			task.CallbackState = CallbackPending
			if err := store.Update(task); err != nil {
				t.Fatal(err)
			}
			w = httptest.NewRecorder()
			mux.ServeHTTP(w, signedRequest(t, "GET", target, nil, nil))
			if w.Code != 409 {
				t.Fatal("unbilled standard result exposed", w.Code)
			}
		})
	}
}
