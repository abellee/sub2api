package mediahub

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSignatureMirrorRoundTrip(t *testing.T) {
	// 与 mediad/internal/signature 相同算法:同一输入必须产生同一签名。
	secret, ts, body := "shared", "1700000000", []byte(`{"a":1}`)
	sig := Sign(secret, ts, "POST", "/relay/mj", body)
	if err := Verify(secret, ts, "POST", "/relay/mj", body, sig, time.Unix(1700000000, 0)); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestForwardSignsAndParsesUsage(t *testing.T) {
	var gotSig, gotTS, gotUser, gotCred, gotUpstream string
	var gotPath string
	var gotBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get(HeaderSignature)
		gotTS = r.Header.Get(HeaderTimestamp)
		gotUser = r.Header.Get(HeaderUserID)
		gotCred = r.Header.Get(HeaderCredential)
		gotUpstream = r.Header.Get(HeaderUpstreamURL)
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set(HeaderUsage, `{"quantity":4,"unit":"image"}`)
		_, _ = w.Write([]byte(`{"data":[{"url":"x"}]}`))
	}))
	t.Cleanup(upstream.Close)

	resp, err := Forward(context.Background(), nil, ForwardInput{
		MediadBaseURL: upstream.URL,
		RelaySecret:   "shared",
		SchemaID:      "mj-proxy-v1",
		UpstreamURL:   "https://relay.example.com",
		Credential:    "sk-acc",
		ClientPath:    "v1/images/generations",
		Body:          []byte(`{"model":"mj-v8.2","prompt":"p"}`),
		ContentType:   "application/json",
		UpstreamModel: "mj-v8.2",
		UserID:        42,
		KeyID:         7,
		GroupID:       3,
	})
	if err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if gotPath != "/relay/mj-proxy-v1/v1/images/generations" {
		t.Fatalf("path = %q", gotPath)
	}
	if err := Verify("shared", gotTS, "POST", gotPath, gotBody, gotSig, time.Now()); err != nil {
		t.Fatalf("mediad-side signature verification failed: %v", err)
	}
	if gotUser != "42" || gotCred != "sk-acc" || gotUpstream != "https://relay.example.com" {
		t.Fatalf("identity headers = %q %q %q", gotUser, gotCred, gotUpstream)
	}
	if resp.Usage == nil || resp.Usage.Quantity != 4 || resp.Usage.Unit != UnitImage {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}

func TestSettleServerVerifiesAndDedups(t *testing.T) {
	var settled []SettleRequest
	sink := sinkFunc(func(req SettleRequest) error {
		settled = append(settled, req)
		return nil
	})
	srv := NewSettleServer(func() string { return "cb-secret" }, sink, time.Minute)

	callbackBody := []byte(`{"event":"task.completed","task_id":"mtask_1","user_id":"42","key_id":"7","group_id":"3","model":"mj-v8.2","status":"succeeded","created_at":"2026-11-08T10:00:00.125Z","completed_at":"2026-11-08T10:00:12.875Z","usage":{"quantity":4,"unit":"image"}}`)
	post := func(sig string) *httptest.ResponseRecorder {
		ts := fmt.Sprintf("%d", time.Now().Unix())
		req := httptest.NewRequest("POST", "/internal/media/settle", strings.NewReader(string(callbackBody)))
		req.Header.Set(HeaderTimestamp, ts)
		req.Header.Set(HeaderSignature, sig)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec
	}
	goodSig := Sign("cb-secret", fmt.Sprintf("%d", time.Now().Unix()), "POST", "/internal/media/settle", callbackBody)

	rec := post(goodSig)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"settled":true`) {
		t.Fatalf("first callback = %d %s", rec.Code, rec.Body.String())
	}
	if len(settled) != 1 || settled[0].RequestID != "media_task_settle:mtask_1" || settled[0].UserID != 42 {
		t.Fatalf("settled = %+v", settled)
	}
	if settled[0].Duration != 12750*time.Millisecond {
		t.Fatalf("callback duration = %v", settled[0].Duration)
	}

	// 重复回调:不重复结算,返回 already_settled。
	rec = post(goodSig)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"already_settled":true`) {
		t.Fatalf("dup callback = %d %s", rec.Code, rec.Body.String())
	}
	if len(settled) != 1 {
		t.Fatalf("dup settle executed: %+v", settled)
	}

	// 坏签名 → 401。
	rec = post("sha256=bad")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad signature = %d", rec.Code)
	}
}

func TestSettleServerDisabledWithoutSecret(t *testing.T) {
	srv := NewSettleServer(func() string { return "" }, NoopSink{}, 0)
	req := httptest.NewRequest("POST", "/internal/media/settle", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled = %d", rec.Code)
	}
}

func TestBindingStorePersistence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/bindings.json"

	s, err := NewBindingStore(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := s.Put(Binding{GroupID: 3, SchemaID: "mj-proxy-v1", Enabled: true}); err != nil {
		t.Fatalf("put: %v", err)
	}

	// 重新打开验证持久化。
	s2, err := NewBindingStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	b, ok := s2.Get(3)
	if !ok || !b.Enabled || b.SchemaID != "mj-proxy-v1" {
		t.Fatalf("binding after reopen = %+v ok=%v", b, ok)
	}
	if _, err := s2.Delete(3); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if deleted, _ := s2.Delete(3); deleted {
		t.Fatal("second delete should report false")
	}
	if _, ok := s2.Get(3); ok {
		t.Fatal("binding should be gone")
	}
}

type sinkFunc func(SettleRequest) error

func (f sinkFunc) Settle(req SettleRequest) error { return f(req) }
