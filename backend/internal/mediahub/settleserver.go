package mediahub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// SettleCallbackSecretProvider 返回用于校验 mediad 回调签名的密钥。
// 与注册到 mediad 的 callback_secret 一致;返回空表示回调功能关闭。
type SettleCallbackSecretProvider func() string

// SettlementSink 是核心计费系统的接缝:handler 包内的桥实现它,
// mediad 的回调与同步 usage 头都汇聚到这里。
type SettlementSink interface {
	Settle(req SettleRequest) error
}

// NoopSink 只记日志不扣费,供测试与未接线部署使用。
type NoopSink struct{}

// Settle 实现 SettlementSink(空操作)。
func (NoopSink) Settle(SettleRequest) error { return nil }

// settleEvent 是 mediad 终态回调的 body(契约见 mediad/README.md)。
type settleEvent struct {
	Event          string         `json:"event"`
	TaskID         string         `json:"task_id"`
	UpstreamTaskID string         `json:"upstream_task_id"`
	SchemaID       string         `json:"schema_id"`
	Model          string         `json:"model"`
	UserID         string         `json:"user_id"`
	KeyID          string         `json:"key_id"`
	GroupID        string         `json:"group_id"`
	Status         string         `json:"status"`
	Usage          *Usage         `json:"usage"`
	Result         map[string]any `json:"result"`
	ErrorMessage   string         `json:"error_message"`
	CreatedAt      string         `json:"created_at"`
	CompletedAt    string         `json:"completed_at"`
	Files          []ResultFile   `json:"files"`
	Parameters     map[string]any `json:"parameters"`
}

// SettleServer 处理 mediad 的终态回调:
// 验签(HMAC,与注册的 callback_secret 一致)→ 幂等 → 交给 Sink 结算。
type SettleServer struct {
	secret   SettleCallbackSecretProvider
	sink     SettlementSink
	dedupTTL time.Duration

	mu         sync.Mutex
	seen       map[string]time.Time
	processing map[string]bool
	login      func(format string, args ...any) // 可注入日志;nil 用静默
}

// NewSettleServer 创建回调处理服务。dedupTTL 为幂等记忆窗口(默认 24h)。
func NewSettleServer(secret SettleCallbackSecretProvider, sink SettlementSink, dedupTTL time.Duration) *SettleServer {
	if dedupTTL <= 0 {
		dedupTTL = 24 * time.Hour
	}
	return &SettleServer{
		secret:     secret,
		sink:       sink,
		dedupTTL:   dedupTTL,
		seen:       map[string]time.Time{},
		processing: map[string]bool{},
	}
}

func (s *SettleServer) logf(format string, args ...any) {
	if s.login != nil {
		s.login(format, args...)
	}
}

// ServeHTTP 实现 http.Handler。响应语义:任何 2xx 表示已确认
// (含 already_settled);非 2xx 触发 mediad 侧重试。
func (s *SettleServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	secret := s.secret()
	if strings.TrimSpace(secret) == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "settle callback disabled"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "read body failed"})
		return
	}
	if err := Verify(secret, r.Header.Get(HeaderTimestamp), r.Method, r.URL.Path, body,
		r.Header.Get(HeaderSignature), time.Now()); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid signature"})
		return
	}

	var ev settleEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid callback payload"})
		return
	}
	if ev.Event == "callback.test" {
		writeJSON(w, 200, map[string]any{"received": true})
		return
	}
	if strings.TrimSpace(ev.TaskID) == "" || (ev.Status != "succeeded" && ev.Status != "failed") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid callback payload"})
		return
	}

	if s.alreadyProcessed(ev.TaskID) {
		writeJSON(w, http.StatusOK, map[string]any{"already_settled": true})
		return
	}
	s.mu.Lock()
	if s.processing[ev.TaskID] {
		s.mu.Unlock()
		writeJSON(w, 409, map[string]any{"error": "task processing"})
		return
	}
	// 前一个请求可能刚结束，再查一次完成标记。
	if _, ok := s.seen[ev.TaskID]; ok {
		s.mu.Unlock()
		writeJSON(w, 200, map[string]any{"already_settled": true})
		return
	}
	s.processing[ev.TaskID] = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.processing, ev.TaskID); s.mu.Unlock() }()

	req := SettleRequest{
		TaskID:     ev.TaskID,
		RequestID:  "media_task_settle:" + ev.TaskID,
		UserID:     parseID(ev.UserID),
		APIKeyID:   parseID(ev.KeyID),
		GroupID:    parseID(ev.GroupID),
		Model:      ev.Model,
		Usage:      ev.Usage,
		Status:     ev.Status,
		Result:     ev.Result,
		Files:      ev.Files,
		Parameters: ev.Parameters,
		Ctx:        r.Context(),
	}
	if err := s.sink.Settle(req); err != nil {
		s.logf("[mediahub] settle failed task=%s: %v", ev.TaskID, err)
		// 5xx 触发 mediad 重试;已幂等,重试安全。
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "settle failed"})
		return
	}
	s.markProcessed(ev.TaskID)
	writeJSON(w, http.StatusOK, map[string]any{"settled": true})
}

func (s *SettleServer) alreadyProcessed(taskID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gcLocked()
	ts, ok := s.seen[taskID]
	return ok && time.Since(ts) <= s.dedupTTL
}

func (s *SettleServer) markProcessed(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen[taskID] = time.Now()
}

func (s *SettleServer) gcLocked() {
	for k, ts := range s.seen {
		if time.Since(ts) > s.dedupTTL {
			delete(s.seen, k)
		}
	}
}

func parseID(s string) int64 {
	var v int64
	_, _ = fmt.Sscanf(strings.TrimSpace(s), "%d", &v)
	return v
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
