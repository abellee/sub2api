package relay

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func grokSchema(t *testing.T, kind string) *Schema {
	t.Helper()
	raw, err := os.ReadFile("../../schemas/grok-" + kind + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var schema Schema
	if err = json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	if err = schema.Validate(); err != nil {
		t.Fatal(err)
	}
	return &schema
}

func TestGrokImageSchemaReferences(t *testing.T) {
	schema := grokSchema(t, "image")
	out, err := Assemble(AssembleInput{Schema: schema, UpstreamURL: "https://example.com", ClientPath: "/v1/images/edits", ContentType: "application/json", Body: []byte(`{"model":"grok-imagine-image","prompt":"edit","images":[{"image_url":"data:image/png;base64,aQ=="},"https://example.com/image.png"],"mask":{"url":"https://example.com/mask.png"}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if out.URL != "https://example.com/v1/images/edits" || !strings.Contains(string(out.Body), `"url":"data:image/png;base64,aQ=="`) || strings.Contains(string(out.Body), "image_url") {
		t.Fatal(out.URL, string(out.Body))
	}
}

func TestGrokVideoSchemaEndToEnd(t *testing.T) {
	schema := grokSchema(t, "video")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-upstream-key" {
			t.Error("credential missing")
		}
		switch r.URL.Path {
		case "/v1/videos/extensions":
			io.WriteString(w, `{"request_id":"up-video"}`)
		case "/v1/videos/up-video":
			io.WriteString(w, `{"status":"done","video":{"url":"data:video/mp4;base64,AAAAHGZ0eXBpc29tAAAAAGlzb21pc28y","duration":6,"resolution":"720p"}}`)
		default:
			t.Error("unexpected upstream path", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	callback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["usage"].(map[string]any)["quantity"] != float64(6) {
			t.Error("wrong duration")
		}
		w.WriteHeader(200)
	}))
	defer callback.Close()
	registry := NewSchemaRegistry(nil)
	if err := registry.Put(schema); err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore(nil)
	tracker := NewTracker(store, registry.Get, nil, nil)
	defer tracker.Stop()
	tracker.ConfigureFiles(t.TempDir(), 1<<20)
	tracker.ConfigureDownloads("https://d.tokenfree.dev")
	tracker.SetCallback(CallbackConfig{URL: callback.URL, Enabled: true, Secret: "cb"})
	mux := http.NewServeMux()
	NewRelayHandler(registry, store, tracker, relaySecret, nil).Register(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, signedRequest(t, "POST", "/relay/grok-video/v1/videos/extensions", []byte(`{"model":"grok-imagine-video","video":{"url":"https://example.com/input.mp4"},"duration":6}`), map[string]string{HeaderUpstreamURL: upstream.URL, HeaderModel: "grok-imagine-video", "Content-Type": "application/json"}))
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var accepted map[string]any
	json.Unmarshal(w.Body.Bytes(), &accepted)
	id := accepted["task_id"].(string)
	deadline := time.Now().Add(4 * time.Second)
	for {
		task, _ := store.Get(id)
		if task.CallbackState == CallbackConfirmed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("video not confirmed", task.Status, task.ErrorMessage)
		}
		time.Sleep(time.Millisecond)
	}
	target := "/relay/tasks/" + id + "?response_format=grok_video"
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, signedRequest(t, "GET", target, nil, nil))
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	if w.Code != 200 || result["status"] != "done" || !strings.Contains(result["video"].(map[string]any)["url"].(string), ".mp4?") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, signedRequest(t, "GET", target, nil, map[string]string{HeaderUserID: "43"}))
	if w.Code != 404 {
		t.Fatal("ownership bypass", w.Code)
	}
}
