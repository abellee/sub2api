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
			io.WriteString(w, `{"status":"done","video":{"url":"data:video/mp4;base64,AAAAHGZ0eXBpc29tAAAAAGlzb21pc28y","resolution":"720p"}}`)
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
	media := httptest.NewServer(mux)
	defer media.Close()
	tracker.ConfigureDownloads(media.URL)
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
	public, err := http.Get(accepted["result_url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	var ready struct {
		Status string `json:"status"`
		Result struct {
			Data []struct {
				URL  string `json:"url"`
				MIME string `json:"mime_type"`
			} `json:"data"`
		} `json:"result"`
	}
	err = json.NewDecoder(public.Body).Decode(&ready)
	_ = public.Body.Close()
	if err != nil || ready.Status != "completed" || len(ready.Result.Data) != 1 || ready.Result.Data[0].MIME != "video/mp4" {
		t.Fatal("video result unavailable", ready, err)
	}
	download, _ := http.NewRequest("GET", ready.Result.Data[0].URL, nil)
	download.Header.Set("Range", "bytes=0-7")
	file, err := http.DefaultClient.Do(download)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(file.Body)
	_ = file.Body.Close()
	if err != nil || file.StatusCode != 206 || len(data) != 8 || file.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("video player cannot read a range", file.StatusCode, string(data), err)
	}
	target := "/relay/tasks/" + id + "?response_format=grok_video"
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, signedRequest(t, "GET", target, nil, nil))
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	if w.Code != 200 || result["status"] != "done" || result["id"] != id || result["video_url"] != result["video"].(map[string]any)["url"] || !strings.Contains(result["video_url"].(string), ".mp4?") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, signedRequest(t, "GET", target, nil, map[string]string{HeaderUserID: "43"}))
	if w.Code != 404 {
		t.Fatal("ownership bypass", w.Code)
	}
}

func TestGrokVideoDurationFallback(t *testing.T) {
	schema := grokSchema(t, "video")
	task := &Task{Parameters: map[string]any{"duration": float64(6)}}
	for _, tc := range []struct {
		body string
		want float64
		fail bool
	}{
		{`{"video":{"url":"https://example.com/v.mp4"}}`, 6, false},
		{`{"video":{"duration":8}}`, 8, false},
		{`{"video":{"duration":"bad"}}`, 0, true},
		{`{"video":{"duration":null}}`, 0, true},
	} {
		quantity, unit, err := extractTaskUsage(schema, task, []byte(tc.body))
		if (err != nil) != tc.fail || (!tc.fail && (quantity != tc.want || unit != UnitSecond)) {
			t.Fatal(tc.body, quantity, unit, err)
		}
	}
	task.Parameters = nil
	if _, _, err := extractTaskUsage(schema, task, []byte(`{"video":{}}`)); err == nil {
		t.Fatal("missing duration guessed")
	}
}

func TestRecoverGrokVideoNotificationWithoutRegeneration(t *testing.T) {
	schema := grokSchema(t, "video")
	store := NewMemoryStore(nil)
	task := &Task{ID: "mtask_recover", SchemaID: schema.ID, Schema: schema, Status: StatusSucceeded, CallbackState: CallbackExhausted, CallbackAttempts: 10, Parameters: map[string]any{"duration": float64(6)}, Response: []byte(`{"request_id":"existing-video"}`)}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}
	callback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["usage"].(map[string]any)["quantity"] != float64(6) {
			t.Error("recovered usage lost")
		}
		w.WriteHeader(200)
	}))
	defer callback.Close()
	tracker := NewTracker(store, func(string) (*Schema, bool) { return schema, true }, nil, nil)
	defer tracker.Stop()
	tracker.SetCallback(CallbackConfig{Enabled: true, URL: callback.URL, Secret: "cb"})
	mux := http.NewServeMux()
	NewManagementHandler(NewSchemaRegistry(nil), store, tracker, "token").Register(mux)
	r := httptest.NewRequest("POST", "/management/tasks/mtask_recover/recover", strings.NewReader(`{}`))
	r.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	deadline := time.Now().Add(time.Second)
	for {
		got, _ := store.Get(task.ID)
		if got.CallbackState == CallbackConfirmed && got.Usage != nil && got.Usage.Quantity == 6 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("not recovered", got)
		}
		time.Sleep(time.Millisecond)
	}
}
