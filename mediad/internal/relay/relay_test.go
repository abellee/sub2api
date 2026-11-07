package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"mediad/internal/signature"
)

const relaySecret = "relay-shared-secret"

// signedRequest 以契约规范串为 sub2api 侧请求签名。
func signedRequest(t *testing.T, method, target string, body []byte, extra map[string]string) *http.Request {
	t.Helper()
	ts := fmt.Sprintf("%d", time.Now().Unix())
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	req.Header.Set(HeaderTimestamp, ts)
	req.Header.Set(HeaderSignature, signature.Sign(relaySecret, ts, method, req.URL.Path, body))
	req.Header.Set(HeaderUserID, "42")
	req.Header.Set(HeaderKeyID, "7")
	req.Header.Set(HeaderGroupID, "3")
	req.Header.Set(HeaderCredential, "sk-upstream-key")
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	return req
}

type upstreamStub struct {
	mu               sync.Mutex
	createCalls      int
	queryCalls       int
	queryScript      []string // 依次消耗的响应体;耗尽后按 alwaysProcessing 或默认 completed
	alwaysProcessing bool
}

func (s *upstreamStub) nextQuery() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.queryCalls-1 < len(s.queryScript) {
		return s.queryScript[s.queryCalls-1]
	}
	if s.alwaysProcessing {
		return `{"data":{"status":"processing"}}`
	}
	return `{"data":{"status":"completed","result":{"data":{"image_urls":["u1","u2","u3","u4"]}}}}`
}

// fixture 汇集端到端测试所需的全部组件。
type fixture struct {
	handler     *RelayHandler
	upstreamURL string
	registry    *SchemaRegistry
	mux         *http.ServeMux
	stub        *upstreamStub

	cbMu       sync.Mutex
	callbacks  []map[string]any
	callbackCh chan struct{}
}

func (fx *fixture) waitCallback(t *testing.T, timeout time.Duration) map[string]any {
	t.Helper()
	select {
	case <-fx.callbackCh:
	case <-time.After(timeout):
		t.Fatalf("callback not delivered in time; queryCalls=%d", fx.stub.queryCalls)
	}
	fx.cbMu.Lock()
	defer fx.cbMu.Unlock()
	if len(fx.callbacks) == 0 {
		t.Fatal("callback channel closed but no body recorded")
	}
	return fx.callbacks[0]
}

// newFixture 搭建:上游假服务 + 回调接收器 + relay/management 处理器 + 注册好的异步 schema。
func newFixture(t *testing.T) *fixture {
	t.Helper()
	stub := &upstreamStub{}
	fx := &fixture{stub: stub, callbackCh: make(chan struct{})}

	var once sync.Once
	callbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		if err := signature.Verify("cb-secret", r.Header.Get(HeaderTimestamp),
			r.Method, r.URL.Path, body, r.Header.Get(HeaderSignature), time.Now()); err != nil {
			t.Errorf("callback signature invalid: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fx.cbMu.Lock()
		fx.callbacks = append(fx.callbacks, m)
		fx.cbMu.Unlock()
		once.Do(func() { close(fx.callbackCh) })
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(callbackServer.Close)

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/midjourney/generations"):
			stub.mu.Lock()
			stub.createCalls++
			stub.mu.Unlock()
			if got := r.Header.Get("Authorization"); got != "Bearer sk-upstream-key" {
				t.Errorf("upstream credential header = %q", got)
			}
			_, _ = w.Write([]byte(`{"data":{"task_id":"up-123","status":"submitted"}}`))
		case strings.Contains(r.URL.Path, "/mj/task/"):
			stub.mu.Lock()
			stub.queryCalls++
			stub.mu.Unlock()
			_, _ = w.Write([]byte(stub.nextQuery()))
		default:
			t.Errorf("unexpected upstream path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(upstreamServer.Close)
	fx.upstreamURL = upstreamServer.URL

	fx.registry = NewSchemaRegistry(nil)
	if err := fx.registry.Put(testSchema()); err != nil {
		t.Fatalf("put schema: %v", err)
	}
	store := NewMemoryStore(nil)
	tracker := NewTracker(store, func(id string) (*Schema, bool) { return fx.registry.Get(id) },
		&http.Client{Timeout: 5 * time.Second}, nil)
	tracker.SetCallback(CallbackConfig{
		URL: callbackServer.URL + "/internal/media/settle", Secret: "cb-secret", Enabled: true,
	})
	t.Cleanup(tracker.Stop)

	fx.handler = NewRelayHandler(fx.registry, store, tracker, relaySecret, nil)
	fx.handler.MaxSync = 5 * time.Second

	fx.mux = http.NewServeMux()
	fx.handler.Register(fx.mux)
	NewManagementHandler(fx.registry, store, tracker, "mgmt-token").Register(fx.mux)
	return fx
}

func TestAsyncEndToEnd(t *testing.T) {
	fx := newFixture(t)
	fx.stub.mu.Lock()
	fx.stub.queryScript = []string{`{"data":{"status":"processing"}}`}
	fx.stub.mu.Unlock()

	body := `{"model":"mj-v8.2","prompt":"a lighthouse","size":"1024x1024"}`
	req := signedRequest(t, "POST", "/relay/mj-proxy-v1/v1/images/generations", []byte(body), map[string]string{
		HeaderUpstreamURL:    fx.upstreamURL,
		HeaderModel:          "mj-v8.2",
		HeaderIdempotencyKey: "idem-1",
	})
	rec := httptest.NewRecorder()
	fx.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	taskID, _ := created["task_id"].(string)
	if taskID == "" {
		t.Fatalf("no task_id in create response: %s", rec.Body.String())
	}

	cb := fx.waitCallback(t, 15*time.Second)
	if cb["status"] != "succeeded" {
		t.Fatalf("callback status = %v", cb["status"])
	}
	if cb["user_id"] != "42" || cb["key_id"] != "7" {
		t.Fatalf("callback identity = %v", cb)
	}
	if cb["event"] != CallbackEventTaskCompleted {
		t.Fatalf("callback event = %v", cb["event"])
	}
	usage, ok := cb["usage"].(map[string]any)
	if !ok || usage["unit"] != "image" || usage["quantity"] != float64(4) {
		t.Fatalf("callback usage = %v", cb["usage"])
	}

	// 任务状态查询:身份匹配。回调确认与 store 更新存在毫秒级时序差,轮询等待。
	var st map[string]any
	deadline := time.Now().Add(5 * time.Second)
	for {
		stReq := signedRequest(t, "GET", "/relay/tasks/"+taskID, nil, nil)
		stRec := httptest.NewRecorder()
		fx.mux.ServeHTTP(stRec, stReq)
		if stRec.Code != 200 {
			t.Fatalf("task status = %d, %s", stRec.Code, stRec.Body.String())
		}
		st = nil
		_ = json.Unmarshal(stRec.Body.Bytes(), &st)
		if st["status"] == "succeeded" && st["callback_state"] == "confirmed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("task state never confirmed: %v", st)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// 越权查询(不同 user)→ 404。
	other := signedRequest(t, "GET", "/relay/tasks/"+taskID, nil, nil)
	other.Header.Set(HeaderUserID, "999")
	otherRec := httptest.NewRecorder()
	fx.mux.ServeHTTP(otherRec, other)
	if otherRec.Code != http.StatusNotFound {
		t.Fatalf("cross-user query should be 404, got %d", otherRec.Code)
	}

	// 幂等重放:同键再次创建返回同一任务,上游不再新增 create 调用。
	creates := fx.stub.createCalls
	dup := signedRequest(t, "POST", "/relay/mj-proxy-v1/v1/images/generations", []byte(body), map[string]string{
		HeaderUpstreamURL:    fx.upstreamURL,
		HeaderModel:          "mj-v8.2",
		HeaderIdempotencyKey: "idem-1",
	})
	dupRec := httptest.NewRecorder()
	fx.mux.ServeHTTP(dupRec, dup)
	if dupRec.Code != http.StatusAccepted {
		t.Fatalf("duplicate status = %d", dupRec.Code)
	}
	var dupBody map[string]any
	_ = json.Unmarshal(dupRec.Body.Bytes(), &dupBody)
	if dupBody["duplicated"] != true || dupBody["task_id"] != taskID {
		t.Fatalf("duplicate response = %v", dupBody)
	}
	if fx.stub.createCalls != creates {
		t.Fatalf("duplicate create hit upstream: %d -> %d", creates, fx.stub.createCalls)
	}
}

func TestSyncUpstreamImageIsAcceptedAsAsyncTask(t *testing.T) {
	fx := newFixture(t)
	syncSchema := testSchema()
	syncSchema.ID = "img-sync"
	syncSchema.Mode = "sync"
	syncSchema.Upstream.Query = nil
	if err := fx.registry.Put(syncSchema); err != nil {
		t.Fatalf("put sync schema: %v", err)
	}

	syncUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"created":1,"data":[{"url":"https://cdn/x.png"},{"url":"https://cdn/y.png"}]}`))
	}))
	t.Cleanup(syncUpstream.Close)

	req := signedRequest(t, "POST", "/relay/img-sync/v1/images/generations", []byte(`{"model":"gpt-image-2","prompt":"x"}`), map[string]string{
		HeaderUpstreamURL: syncUpstream.URL,
	})
	rec := httptest.NewRecorder()
	fx.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("sync status = %d, %s", rec.Code, rec.Body.String())
	}
	cb := fx.waitCallback(t, 5*time.Second)
	if cb["status"] != "succeeded" {
		t.Fatalf("image task did not finish: %v", cb)
	}
	if usage, ok := cb["usage"].(map[string]any); !ok || usage["unit"] != "image" || usage["quantity"] != float64(4) {
		t.Fatalf("image metadata=%v", cb)
	}
}

func TestBadSignatureRejected(t *testing.T) {
	fx := newFixture(t)
	req := signedRequest(t, "POST", "/relay/mj-proxy-v1/x", []byte(`{}`), map[string]string{
		HeaderUpstreamURL: fx.upstreamURL,
	})
	req.Header.Set(HeaderSignature, "sha256=deadbeef")
	rec := httptest.NewRecorder()
	fx.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad signature status = %d", rec.Code)
	}
}

func TestMissingRequiredHeaders(t *testing.T) {
	fx := newFixture(t)
	req := signedRequest(t, "POST", "/relay/mj-proxy-v1/x", []byte(`{}`), nil) // 无 Upstream-URL
	rec := httptest.NewRecorder()
	fx.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing headers status = %d", rec.Code)
	}
}

func TestUnknownSchemaRejected(t *testing.T) {
	fx := newFixture(t)
	req := signedRequest(t, "POST", "/relay/ghost/x", []byte(`{}`), map[string]string{
		HeaderUpstreamURL: fx.upstreamURL,
	})
	rec := httptest.NewRecorder()
	fx.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown schema status = %d", rec.Code)
	}
}

func TestPollTimeoutPreservesUnknownResultForRecovery(t *testing.T) {
	fx := newFixture(t)
	fx.stub.mu.Lock()
	fx.stub.alwaysProcessing = true
	fx.stub.mu.Unlock()

	// schema v4:1 秒轮询上限。
	s := testSchema()
	s.Version = 4
	s.Upstream.Query.MaxPollSeconds = 1
	s.Upstream.Query.PollIntervalSeconds = 1
	if err := fx.registry.Put(s); err != nil {
		t.Fatalf("put v4: %v", err)
	}

	req := signedRequest(t, "POST", "/relay/mj-proxy-v1/x", []byte(`{"prompt":"p"}`), map[string]string{
		HeaderUpstreamURL: fx.upstreamURL,
	})
	rec := httptest.NewRecorder()
	fx.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create = %d, %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	taskID := created["task_id"].(string)

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		stReq := signedRequest(t, "GET", "/relay/tasks/"+taskID, nil, nil)
		stRec := httptest.NewRecorder()
		fx.mux.ServeHTTP(stRec, stReq)
		var st map[string]any
		_ = json.Unmarshal(stRec.Body.Bytes(), &st)
		if st["status"] == StatusRecoveryRequired {
			if !strings.Contains(fmt.Sprint(st["error_message"]), "poll_timeout") {
				t.Fatalf("expected poll_timeout, got %v", st["error_message"])
			}
			if st["upstream_task_id"] != "up-123" {
				t.Fatalf("lost upstream task ID: %v", st)
			}
			fx.cbMu.Lock()
			n := len(fx.callbacks)
			fx.cbMu.Unlock()
			if n != 0 {
				t.Fatal("unknown result must not be reported as failed")
			}
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("task did not reach poll_timeout in time")
}

func TestManagementEndpoints(t *testing.T) {
	fx := newFixture(t)

	// 无 token → 401。
	rec := httptest.NewRecorder()
	fx.mux.ServeHTTP(rec, httptest.NewRequest("GET", "/management/schemas", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthed management = %d", rec.Code)
	}

	// capabilities 无需 token。
	rec = httptest.NewRecorder()
	fx.mux.ServeHTTP(rec, httptest.NewRequest("GET", "/capabilities", nil))
	if rec.Code != 200 {
		t.Fatalf("capabilities = %d", rec.Code)
	}

	// dry-run:组装 + 提取演示。
	dryBody := map[string]any{
		"schema_id":  "mj-proxy-v1",
		"method":     "POST",
		"path":       "/v1/images/generations",
		"credential": "sk-demo",
		"headers":    map[string]string{HeaderUserID: "42", HeaderKeyID: "7"},
		"body": map[string]any{
			"model": "mj-v8.2", "prompt": "p", "size": "1024x1024", "n": 2.0,
		},
		"upstream_response": map[string]any{
			"data": map[string]any{
				"task_id": "up-9",
				"status":  "completed",
				"result":  map[string]any{"data": map[string]any{"image_urls": []any{"a", "b", "c", "d"}}},
			},
		},
	}
	buf, _ := json.Marshal(dryBody)
	req := httptest.NewRequest("POST", "/management/dry-run", bytes.NewReader(buf))
	req.Header.Set("Authorization", "Bearer mgmt-token")
	rec = httptest.NewRecorder()
	fx.mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("dry-run = %d, %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	ur := out["upstream_request"].(map[string]any)
	if ur["url"] != "https://upstream.example.com/v1/midjourney/generations" {
		t.Fatalf("dry-run url = %v", ur["url"])
	}
	pl := ur["payload"].(map[string]any)
	if pl["size"] != "1:1" {
		t.Fatalf("dry-run payload size = %v", pl["size"])
	}
	if _, exists := pl["n"]; exists {
		t.Fatal("dry-run should drop n")
	}
	ex := out["extraction"].(map[string]any)
	if ex["status_class"] != "succeeded" || ex["task_id"] != "up-9" {
		t.Fatalf("dry-run extraction = %v", ex)
	}

	// callback/test:向注册地址发签名测试事件。
	req = httptest.NewRequest("POST", "/management/callback/test", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer mgmt-token")
	rec = httptest.NewRecorder()
	fx.mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("callback test = %d, %s", rec.Code, rec.Body.String())
	}
	var cbOut map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &cbOut)
	if cbOut["delivered"] != true {
		t.Fatalf("callback test should deliver: %v", cbOut)
	}
}

func TestPendingSettlements(t *testing.T) {
	fx := newFixture(t)
	task := &Task{
		ID: "mtask_x", UpstreamTaskID: "u", SchemaID: "mj-proxy-v1",
		UserID: "42", KeyID: "7", Status: StatusSucceeded,
		Usage:         &Usage{Quantity: 4, Unit: UnitImage},
		CallbackState: CallbackExhausted, CreatedAt: time.Now(),
	}
	_ = fx.handler.store.Create(task)

	req := httptest.NewRequest("GET", "/management/settlements/pending", nil)
	req.Header.Set("Authorization", "Bearer mgmt-token")
	rec := httptest.NewRecorder()
	fx.mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("pending = %d", rec.Code)
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["count"] != float64(1) {
		t.Fatalf("pending = %v", out)
	}
	if strings.Contains(rec.Body.String(), "credential") {
		t.Fatal("pending view must not leak credential")
	}
}
