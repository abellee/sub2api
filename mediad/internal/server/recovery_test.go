package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"mediad/internal/config"
	"mediad/internal/relay"
	"mediad/internal/signature"
)

func relayRequest(method, path string, body []byte) *http.Request {
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	ts := fmt.Sprint(time.Now().Unix())
	r.Header.Set(relay.HeaderTimestamp, ts)
	r.Header.Set(relay.HeaderSignature, signature.Sign("relay-secret", ts, method, path, body))
	r.Header.Set(relay.HeaderUserID, "42")
	r.Header.Set(relay.HeaderKeyID, "7")
	r.Header.Set(relay.HeaderCredential, "upstream-key")
	return r
}

func management(t *testing.T, s *Server, method, path string, value any) {
	t.Helper()
	body, _ := json.Marshal(value)
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer admin")
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("management %s: %d %s", path, w.Code, w.Body.String())
	}
}

func TestRestartFindsResultCompletedWhileServiceWasDown(t *testing.T) {
	var done atomic.Bool
	var creates atomic.Int32
	queried := make(chan struct{}, 1)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			creates.Add(1)
			io.WriteString(w, `{"id":"provider-1"}`)
			return
		}
		select {
		case queried <- struct{}{}:
		default:
		}
		if done.Load() {
			io.WriteString(w, `{"status":"done","images":["data:image/png;base64,aGVsbG8="],"duration":8}`)
		} else {
			io.WriteString(w, `{"status":"pending"}`)
		}
	}))
	defer up.Close()
	callbacks := make(chan map[string]any, 4)
	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := signature.Verify("callback-secret", r.Header.Get(relay.HeaderTimestamp), r.Method, r.URL.Path, body, r.Header.Get(relay.HeaderSignature), time.Now()); err != nil {
			t.Error(err)
			w.WriteHeader(401)
			return
		}
		var event map[string]any
		json.Unmarshal(body, &event)
		callbacks <- event
		w.WriteHeader(200)
	}))
	defer cb.Close()
	cfg := config.Config{DataDir: t.TempDir(), PersistCredentials: true, RelaySecret: "relay-secret", ManagementToken: "admin", MaxFileBytes: 1 << 20}
	s1, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s1.tracker.Stop()
	schema := &relay.Schema{ID: "pictures", Version: 1, Mode: "async", Upstream: relay.UpstreamSpec{Create: relay.CallSpec{Method: "POST", Path: "/create"}, Query: &relay.CallSpec{Method: "GET", Path: "/tasks/{task_id}", PollIntervalSeconds: 1, MaxPollSeconds: 60}}, Request: relay.RequestSpec{BodyMode: "passthrough"}, Response: relay.ResponseSpec{TaskID: "id", Status: "status", Result: map[string]string{"images": "images"}, Usage: relay.UsageSpec{Quantity: 1, Unit: "image"}}, TaskStates: relay.TaskStatesSpec{Success: []string{"done"}}}
	management(t, s1, "PUT", "/management/schemas/pictures", schema)
	management(t, s1, "PUT", "/management/callback", relay.CallbackConfig{Enabled: true, URL: cb.URL + "/result", Secret: "callback-secret"})
	r := relayRequest("POST", "/relay/pictures", []byte(`{"prompt":"test"}`))
	r.Header.Set(relay.HeaderUpstreamURL, up.URL)
	ctx, cancel := context.WithCancel(context.Background())
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	s1.httpServer.Handler.ServeHTTP(w, r)
	cancel()
	if w.Code != 202 {
		t.Fatalf("accept=%d %s", w.Code, w.Body.String())
	}
	var accepted map[string]any
	json.Unmarshal(w.Body.Bytes(), &accepted)
	id := accepted["task_id"].(string)
	select {
	case <-queried:
	case <-time.After(5 * time.Second):
		t.Fatal("client disconnect stopped generation")
	}
	s1.tracker.Stop()
	stored, _ := s1.store.Get(id)
	stored.PollStartedAt = time.Now().Add(-24 * time.Hour)
	if err := s1.store.Update(stored); err != nil {
		t.Fatal(err)
	}
	// 上游在微服务停机期间完成。
	done.Store(true)
	s2, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.tracker.Stop()
	s2.tracker.ResumeAll()
	select {
	case ev := <-callbacks:
		if ev["status"] != "succeeded" || len(ev["files"].([]any)) != 1 {
			t.Fatalf("event=%v", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("restart did not resume query/staging/callback")
	}
	path := filepath.Join(cfg.DataDir, "results", id, "f0000")
	buf, err := os.ReadFile(path)
	if err != nil || string(buf) != "hello" {
		t.Fatalf("staged file: %q %v", buf, err)
	}
	if creates.Load() != 1 {
		t.Fatalf("restart created another paid task: %d", creates.Load())
	}
	// 通知成功仍保留文件，越权不能读取，旧版确认请求也不能删除。
	bad := relayRequest("GET", "/relay/tasks/"+id+"/files/f0000", nil)
	bad.Header.Set(relay.HeaderKeyID, "999")
	badW := httptest.NewRecorder()
	s2.httpServer.Handler.ServeHTTP(badW, bad)
	if badW.Code != 404 {
		t.Fatalf("cross-key file=%d", badW.Code)
	}
	ack := relayRequest("POST", "/relay/tasks/"+id+"/files/ack", nil)
	ackW := httptest.NewRecorder()
	s2.httpServer.Handler.ServeHTTP(ackW, ack)
	if ackW.Code != 410 {
		t.Fatalf("ack=%d %s", ackW.Code, ackW.Body.String())
	}
	if buf, err := os.ReadFile(path); err != nil || string(buf) != "hello" {
		t.Fatalf("retained media file was removed or changed: %v", err)
	}
}

func TestCorruptTaskPreventsEmptyStartup(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "tasks"), 0700)
	os.WriteFile(filepath.Join(dir, "tasks", "bad.json"), []byte("broken"), 0600)
	if _, err := New(config.Config{DataDir: dir, PersistCredentials: true}, nil); err == nil {
		t.Fatal("corruption must not silently start with empty task state")
	}
}
