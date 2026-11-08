package relay

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"testing"
)

func testSchema() *Schema {
	return &Schema{
		ID:      "mj-proxy-v1",
		Version: 3,
		Mode:    "async",
		Upstream: UpstreamSpec{
			Create: CallSpec{Method: "POST", Path: "/v1/midjourney/generations", TimeoutSeconds: 30},
			Query: &CallSpec{
				Method: "GET", Path: "/mj/task/{task_id}/fetch",
				PollIntervalSeconds: 1, MaxPollSeconds: 60,
			},
		},
		Request: RequestSpec{
			Credential:   map[string]string{"Authorization": "Bearer {credential}"},
			HeaderParams: map[string]string{"X-Trace-User": "{user_id}"},
			BodyMode:     "mapped",
			FieldMap: map[string]FieldRule{
				"model":   {Transform: "passthrough"},
				"prompt":  {Transform: "passthrough"},
				"size":    {Transform: "pixel_to_ratio", Args: map[string]any{"ratios": []any{"1:1", "16:9"}}},
				"quality": {Transform: "enum_map", Args: map[string]any{"map": map[string]any{"high": "2", "low": "0.5"}, "default": "1"}},
				"n":       {Drop: true},
				"raw":     {Transform: "bool_strict"},
			},
			StaticFields: map[string]any{"process_mode": "relaxed"},
		},
		Response: ResponseSpec{
			TaskID:       "data.task_id",
			Status:       "data.status",
			Usage:        UsageSpec{Quantity: float64(4), Unit: UnitImage},
			Result:       map[string]string{"image_urls": "data.result.data.image_urls"},
			ErrorMessage: "data.error.message",
		},
		TaskStates: TaskStatesSpec{
			Success: []string{"completed", "succeeded", "success"},
			Failure: []string{"failed", "cancelled"},
		},
	}
}

func TestAssembleMappedBody(t *testing.T) {
	schema := testSchema()
	clientBody := `{"model":"mj-v8.2","prompt":"a lighthouse","size":"1024x1024","quality":"high","n":1,"raw":"true"}`
	up, err := Assemble(AssembleInput{
		Schema:      schema,
		UpstreamURL: "https://relay.example.com/",
		Credential:  "sk-upstream-key",
		Method:      "POST",
		Body:        []byte(clientBody),
		ContentType: "application/json",
		UserID:      "42",
		KeyID:       "7",
		GroupID:     "3",
		Model:       "mj-v8.2",
	})
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if up.URL != "https://relay.example.com/v1/midjourney/generations" {
		t.Fatalf("url = %q", up.URL)
	}
	if got := up.Header.Get("Authorization"); got != "Bearer sk-upstream-key" {
		t.Fatalf("authorization = %q", got)
	}
	if got := up.Header.Get("X-Trace-User"); got != "42" {
		t.Fatalf("trace user = %q", got)
	}
	var payload map[string]any
	if err := json.Unmarshal(up.Body, &payload); err != nil {
		t.Fatalf("payload not json: %v", err)
	}
	if payload["size"] != "1:1" {
		t.Fatalf("size transform = %v", payload["size"])
	}
	if payload["quality"] != "2" {
		t.Fatalf("quality transform = %v", payload["quality"])
	}
	if payload["raw"] != true {
		t.Fatalf("raw bool = %v", payload["raw"])
	}
	if _, exists := payload["n"]; exists {
		t.Fatalf("n should be dropped")
	}
	if payload["process_mode"] != "relaxed" {
		t.Fatalf("static fields missing: %v", payload["process_mode"])
	}
}

func TestAssembleImageEdits(t *testing.T) {
	schema := &Schema{ID: "test", Version: 1, Mode: "sync", Upstream: UpstreamSpec{Create: CallSpec{Method: "POST", Path: "/v1/images/generations"}}, Request: RequestSpec{BodyMode: "passthrough", StaticFields: map[string]any{"stream": false, "response_format": "url"}}}
	up, err := Assemble(AssembleInput{Schema: schema, UpstreamURL: "https://upstream.example", ClientPath: "/v1/images/edits", Body: []byte(`{"model":"gpt-image-2.6","images":[{"image_url":"data:image/png;base64,aW1hZ2U="}],"mask":{"image_url":"https://example.com/mask.png"},"stream":true}`)})
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	_ = json.Unmarshal(up.Body, &body)
	if up.URL != "https://upstream.example/v1/images/edits" || body["stream"] != false || body["mask"] == nil || len(body["images"].([]any)) != 1 {
		t.Fatal("edit request not preserved", up.URL)
	}
}

func TestAssemblePassthroughPreservesBody(t *testing.T) {
	schema := testSchema()
	schema.Request.BodyMode = "passthrough"
	schema.Request.FieldMap = nil
	body := `{"model":"m","prompt":"p","n":4}`
	up, err := Assemble(AssembleInput{
		Schema: schema, UpstreamURL: "https://u.example", Credential: "k",
		Method: "POST", Body: []byte(body), ContentType: "application/json",
		UserID: "1", KeyID: "2",
	})
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	var payload map[string]any
	_ = json.Unmarshal(up.Body, &payload)
	if payload["n"] != float64(4) {
		t.Fatalf("passthrough body altered: %v", payload)
	}
}

func TestAssembleMultipartToDataURLs(t *testing.T) {
	schema := testSchema()
	schema.Request.BodyMode = "mapped"
	schema.Request.Multipart = &MultipartSpec{FilesField: "image", ToBodyField: "images", MaxFiles: 5}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("prompt", "a lighthouse")
	part := func(name, filename, ctype string, data []byte) {
		h := make(map[string][]string)
		h["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="%s"; filename="%s"`, name, filename)}
		h["Content-Type"] = []string{ctype}
		fw, _ := mw.CreatePart(h)
		_, _ = fw.Write(data)
	}
	part("image[]", "a.png", "image/png", []byte("png-bytes-1"))
	part("image[]", "b.png", "image/png", []byte("png-bytes-2"))
	mw.Close()

	up, err := Assemble(AssembleInput{
		Schema: schema, UpstreamURL: "https://u.example", Credential: "k",
		Method: "POST", Body: buf.Bytes(), ContentType: mw.FormDataContentType(),
		UserID: "1", KeyID: "2",
	})
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(up.Body, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	imgs, ok := payload["images"].([]any)
	if !ok || len(imgs) != 2 {
		t.Fatalf("images field = %v", payload["images"])
	}
	wantPrefix := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("png-bytes-1"))
	if imgs[0] != wantPrefix {
		t.Fatalf("data url mismatch: %v", imgs[0])
	}
	if payload["prompt"] != "a lighthouse" {
		t.Fatalf("form fields lost: %v", payload)
	}
}

func TestAssembleRejectsUnknownTransformAndBadInput(t *testing.T) {
	schema := testSchema()
	schema.Request.FieldMap = map[string]FieldRule{"x": {Transform: "nope"}}
	if _, err := Assemble(AssembleInput{Schema: schema, UpstreamURL: "https://u", Credential: "k",
		Body: []byte(`{"x":1}`), ContentType: "application/json", UserID: "1", KeyID: "2"}); err == nil {
		t.Fatal("unknown transform should error")
	}

	schema2 := testSchema()
	if _, err := Assemble(AssembleInput{Schema: schema2, UpstreamURL: "", Credential: "k",
		Body: []byte(`{}`), ContentType: "application/json", UserID: "1", KeyID: "2"}); err == nil {
		t.Fatal("empty upstream url should error")
	}

	schema3 := testSchema()
	if _, err := Assemble(AssembleInput{Schema: schema3, UpstreamURL: "https://u", Credential: "k",
		Body: []byte(`not-json`), ContentType: "application/json", UserID: "1", KeyID: "2"}); err == nil {
		t.Fatal("invalid json body should error")
	}
}

func TestSchemaValidate(t *testing.T) {
	s := testSchema()
	if err := s.Validate(); err != nil {
		t.Fatalf("valid schema rejected: %v", err)
	}
	asyncNoQuery := testSchema()
	asyncNoQuery.Upstream.Query = nil
	if err := asyncNoQuery.Validate(); err == nil {
		t.Fatal("async without query should fail")
	}
	badUnit := testSchema()
	badUnit.Response.Usage.Unit = "kg"
	if err := badUnit.Validate(); err == nil {
		t.Fatal("bad unit should fail")
	}
	syncOK := testSchema()
	syncOK.Mode = "sync"
	syncOK.Upstream.Query = nil
	if err := syncOK.Validate(); err != nil {
		t.Fatalf("sync schema rejected: %v", err)
	}
}

func TestRegistryVersionGuard(t *testing.T) {
	reg := NewSchemaRegistry(nil)
	s := testSchema()
	if err := reg.Put(s); err != nil {
		t.Fatalf("put: %v", err)
	}
	s.Version = 2 // 回退版本
	if err := reg.Put(s); err == nil {
		t.Fatal("version downgrade should be rejected")
	}
}

func TestResolvePathAndExtraction(t *testing.T) {
	data := map[string]any{
		"data": map[string]any{
			"result": map[string]any{
				"data": map[string]any{"image_urls": []any{"a", "b", "c", "d"}},
			},
			"usage": map[string]any{"completion_tokens": float64(123)},
			"error": map[string]any{"message": "boom"},
		},
	}
	if v, ok := ResolvePath(data, "data.result.data.image_urls.1"); !ok || v != "b" {
		t.Fatalf("array index resolve = %v, %v", v, ok)
	}
	if _, ok := ResolvePath(data, "data.missing"); ok {
		t.Fatal("missing path should not resolve")
	}

	schema := testSchema()
	schema.Response.Usage = UsageSpec{Quantity: "data.usage.completion_tokens", Unit: UnitToken}
	q, u, err := ExtractUsage(schema, mustJSON(data))
	if err != nil || q != 123 || u != UnitToken {
		t.Fatalf("usage from path = %v %v %v", q, u, err)
	}

	schema.Response.Usage = UsageSpec{Quantity: "data.result.data.image_urls", Unit: UnitImage}
	q, _, _ = ExtractUsage(schema, mustJSON(data))
	if q != 4 {
		t.Fatalf("array usage should be length, got %v", q)
	}

	if msg := ExtractErrorMessage(schema, mustJSON(data)); msg != "boom" {
		t.Fatalf("error message = %q", msg)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestBuildQueryURL(t *testing.T) {
	got := BuildQueryURL("https://u.example/", "/mj/task/{task_id}/fetch", "t 1")
	if got != "https://u.example/mj/task/t%201/fetch" {
		t.Fatalf("query url = %q", got)
	}
}

func TestStatusClassification(t *testing.T) {
	s := testSchema()
	if ClassifyStatus(s, "Completed") != "succeeded" {
		t.Fatal("completed should classify succeeded (case-insensitive)")
	}
	if ClassifyStatus(s, "CANCELLED") != "failed" {
		t.Fatal("cancelled should classify failed")
	}
	if ClassifyStatus(s, "running") != "processing" {
		t.Fatal("running should classify processing")
	}
}

func TestExtractorTaskIDMissing(t *testing.T) {
	s := testSchema()
	if _, err := ExtractTaskID(s, []byte(`{"data":{}}`)); err == nil {
		t.Fatal("missing task_id should error")
	}
	if _, err := ExtractTaskID(s, []byte(`{"data":{"task_id":""}}`)); err == nil {
		t.Fatal("empty task_id should error")
	}
}
