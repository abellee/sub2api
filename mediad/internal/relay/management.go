package relay

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mediad/internal/config"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ManagementHandler 是管理面:下发 schema、注册回调、试运行与对账。
// 全部端点要求 Authorization: Bearer <management token>;
// 未配置 token 时整体返回 503(fail-closed)。
// 管理端点永不返回凭证。
type ManagementHandler struct {
	registry *SchemaRegistry
	store    *MemoryStore
	tracker  *Tracker
	token    string
	now      func() time.Time
	Settings *config.SettingsStore
}

// NewManagementHandler 创建管理处理器。
func NewManagementHandler(registry *SchemaRegistry, store *MemoryStore, tracker *Tracker, token string) *ManagementHandler {
	return &ManagementHandler{registry: registry, store: store, tracker: tracker, token: token, now: time.Now}
}

// Register 把管理端点挂到 mux。
func (h *ManagementHandler) Register(mux *http.ServeMux) {
	if h.Settings != nil {
		h.tracker.runtimeSettings = h.Settings
		mux.Handle("GET /management/config", h.auth(h.handleConfig))
		mux.Handle("PUT /management/config", h.auth(h.handleConfig))
	}
	mux.HandleFunc("GET /capabilities", h.handleCapabilities)
	mux.Handle("PUT /management/schemas/{schemaID}", h.auth(h.handleSchemaPut))
	mux.Handle("GET /management/schemas", h.auth(h.handleSchemaList))
	mux.Handle("GET /management/schemas/{schemaID}", h.auth(h.handleSchemaGet))
	mux.Handle("DELETE /management/schemas/{schemaID}", h.auth(h.handleSchemaDelete))
	mux.Handle("PUT /management/callback", h.auth(h.handleCallbackPut))
	mux.Handle("GET /management/callback", h.auth(h.handleCallbackGet))
	mux.Handle("POST /management/callback/test", h.auth(h.handleCallbackTest))
	mux.Handle("POST /management/dry-run", h.auth(h.handleDryRun))
	mux.Handle("GET /management/settlements/pending", h.auth(h.handlePending))
	mux.Handle("GET /management/tasks/pending", h.auth(h.handlePending))
	mux.Handle("POST /management/tasks/{taskID}/recover", h.auth(h.handleRecover))
	mux.Handle("POST /management/files/reconcile", h.auth(h.handleReconcileFiles))
}

func (h *ManagementHandler) handleConfig(w http.ResponseWriter, r *http.Request) {
	h.tracker.settingsMu.Lock()
	defer h.tracker.settingsMu.Unlock()
	if r.Method == http.MethodPut {
		// Decode over desired values so PATCH-like clients cannot accidentally reset hidden keys.
		var next config.Settings
		view := h.Settings.View()
		buf, _ := json.Marshal(view["settings"])
		_ = json.Unmarshal(buf, &next)
		if err := decodeJSONBody(w, r, &next, 1<<20); err != nil {
			writeError(w, 400, "bad_request", err.Error())
			return
		}
		active := h.Settings.Active()
		if next.DataDir != active.DataDir && len(h.store.ListAll()) > 0 {
			writeError(w, 409, "tasks_exist", "data_dir cannot change while task records exist; migrate storage while stopped")
			return
		}
		if next.ResultDir != active.ResultDir {
			for _, task := range h.store.ListAll() {
				if !task.FilesAcknowledged {
					writeError(w, 409, "tasks_exist", "result_dir cannot change while retained tasks exist; stop the service and migrate files and task records")
					return
				}
			}
		}
		if err := h.Settings.Update(next); err != nil {
			writeError(w, 400, "config_rejected", err.Error())
			return
		}
	}
	writeJSON(w, 200, h.Settings.View())
}

func (h *ManagementHandler) auth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(h.token) == "" {
			writeError(w, http.StatusServiceUnavailable, "management_disabled",
				"management token is not configured; management API is disabled")
			return
		}
		auth := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(auth, prefix) || strings.TrimSpace(strings.TrimPrefix(auth, prefix)) != h.token {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid management bearer token required")
			return
		}
		next(w, r)
	})
}

// handleCapabilities 公开只读的能力自描述,不含任何秘密。
func (h *ManagementHandler) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service":          "mediad",
		"role":             "assembly-relay",
		"contract_version": ContractVersion,
		"usage_units":      []string{UnitImage, UnitSecond, UnitToken, UnitAction},
		"transforms":       TransformIDs(),
		"schema_count":     h.registry.Count(),
		"callback": map[string]any{
			"configured": h.tracker.CallbackSnapshot().URL != "",
			"enabled":    h.tracker.CallbackSnapshot().Enabled,
		},
	})
}

func (h *ManagementHandler) handleSchemaPut(w http.ResponseWriter, r *http.Request) {
	var schema Schema
	if err := decodeJSONBody(w, r, &schema, 4<<20); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if id := r.PathValue("schemaID"); id != schema.ID {
		schema.ID = id // 以 URL 为准,避免路径与 body 不一致
	}
	if err := h.registry.Put(&schema); err != nil {
		writeError(w, http.StatusConflict, "schema_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": schema.ID, "version": schema.Version, "stored": true})
}

func (h *ManagementHandler) handleSchemaGet(w http.ResponseWriter, r *http.Request) {
	schema, ok := h.registry.Get(r.PathValue("schemaID"))
	if !ok {
		writeError(w, http.StatusNotFound, "schema_not_found", "schema not found")
		return
	}
	writeJSON(w, http.StatusOK, schema)
}

func (h *ManagementHandler) handleSchemaList(w http.ResponseWriter, r *http.Request) {
	schemas := h.registry.List()
	out := make([]map[string]any, 0, len(schemas))
	for _, s := range schemas {
		out = append(out, map[string]any{
			"id":          s.ID,
			"version":     s.Version,
			"mode":        s.Mode,
			"description": s.Description,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"schemas": out})
}

func (h *ManagementHandler) handleSchemaDelete(w http.ResponseWriter, r *http.Request) {
	deleted, err := h.registry.Delete(r.PathValue("schemaID"))
	if err != nil {
		writeError(w, 503, "store_failed", err.Error())
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "schema_not_found", "schema not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (h *ManagementHandler) handleCallbackPut(w http.ResponseWriter, r *http.Request) {
	var cfg CallbackConfig
	if err := decodeJSONBody(w, r, &cfg, 1<<20); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if cfg.Secret == "" {
		h.tracker.cbMu.RLock()
		cfg.Secret = h.tracker.cb.Secret
		h.tracker.cbMu.RUnlock()
	}
	if cfg.MaxAttempts < 0 || cfg.MaxAttempts > 100 || cfg.BackoffBase < 0 || cfg.BackoffBase > 60 {
		writeError(w, 400, "bad_request", "max_attempts must be 0–100 and backoff_base_seconds 0–60")
		return
	}
	if cfg.Enabled {
		if strings.TrimSpace(cfg.URL) == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "callback_url is required when enabled")
			return
		}
		u, err := url.Parse(cfg.URL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
			writeError(w, http.StatusBadRequest, "bad_request", "callback_url must be http(s)")
			return
		}
		if strings.TrimSpace(cfg.Secret) == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "callback_secret is required when enabled")
			return
		}
	}
	if err := h.tracker.SetCallback(cfg); err != nil {
		writeError(w, 503, "store_failed", err.Error())
		return
	}
	h.tracker.ResumeAll()
	writeJSON(w, http.StatusOK, map[string]any{"stored": true, "enabled": cfg.Enabled})
}

func (h *ManagementHandler) handleCallbackGet(w http.ResponseWriter, r *http.Request) {
	snap := h.tracker.CallbackSnapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"callback_url":         snap.URL,
		"enabled":              snap.Enabled,
		"max_attempts":         snap.MaxAttempts,
		"backoff_base_seconds": snap.BackoffBase,
		// secret 永不回读
	})
}

// handleCallbackTest 向注册的回调地址发送一条签名测试事件。
func (h *ManagementHandler) handleCallbackTest(w http.ResponseWriter, r *http.Request) {
	snap := h.tracker.CallbackSnapshot()
	if !snap.Enabled || snap.URL == "" {
		writeError(w, http.StatusBadRequest, "callback_not_configured", "register and enable callback first")
		return
	}
	body, _ := json.Marshal(map[string]any{
		"event":    "callback.test",
		"sent_at":  h.now().UTC().Format(time.RFC3339),
		"contract": ContractVersion,
	})
	status, err := h.tracker.DeliverRaw("callback.test", body)
	result := map[string]any{"delivered": err == nil, "status": status}
	if err != nil {
		result["error"] = err.Error()
		writeJSON(w, http.StatusOK, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleDryRun 组装上游请求但不发送;带 upstream_response 时同时演示提取。
func (h *ManagementHandler) handleDryRun(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SchemaID         string            `json:"schema_id"`
		Method           string            `json:"method"`
		Path             string            `json:"path"`
		ContentType      string            `json:"content_type"`
		Credential       string            `json:"credential"`
		Headers          map[string]string `json:"headers"`
		Body             any               `json:"body"`
		UpstreamResponse any               `json:"upstream_response"`
	}
	if err := decodeJSONBody(w, r, &in, 32<<20); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	schema, ok := h.registry.Get(in.SchemaID)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown_schema", "schema "+in.SchemaID+" is not registered")
		return
	}

	body, err := decodeDryRunBody(in.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	get := func(k, def string) string {
		if v, ok := in.Headers[k]; ok && v != "" {
			return v
		}
		return def
	}
	assembled, err := Assemble(AssembleInput{
		Schema:      schema,
		UpstreamURL: "https://upstream.example.com", // dry-run 展示拼接规则
		Credential:  in.Credential,
		Method:      firstNonEmpty(in.Method, schema.Upstream.Create.Method),
		ClientPath:  in.Path,
		Body:        body,
		ContentType: firstNonEmpty(in.ContentType, "application/json"),
		UserID:      get(HeaderUserID, "42"),
		KeyID:       get(HeaderKeyID, "7"),
		GroupID:     get(HeaderGroupID, "3"),
		Model:       get(HeaderModel, "demo-model"),
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "assemble_failed", err.Error())
		return
	}

	headers := map[string]string{}
	for k, vs := range assembled.Header {
		headers[k] = strings.Join(vs, ", ")
	}
	var payload any
	if len(assembled.Body) > 0 {
		_ = json.Unmarshal(assembled.Body, &payload)
	}
	out := map[string]any{
		"upstream_request": map[string]any{
			"method":  assembled.Method,
			"url":     assembled.URL,
			"headers": headers,
			"payload": payload,
		},
	}

	if in.UpstreamResponse != nil {
		respBody, _ := json.Marshal(in.UpstreamResponse)
		extraction := map[string]any{}
		if schema.Mode == "async" {
			if tid, err := ExtractTaskID(schema, respBody); err == nil {
				extraction["task_id"] = tid
			} else {
				extraction["task_id_error"] = err.Error()
			}
			if status, ok, err := ExtractStatus(schema, respBody); ok && err == nil {
				extraction["upstream_status"] = status
				extraction["status_class"] = ClassifyStatus(schema, status)
			}
		}
		if usage, unit, err := ExtractUsage(schema, respBody); err == nil {
			extraction["usage"] = map[string]any{"quantity": usage, "unit": unit}
		} else {
			extraction["usage_error"] = err.Error()
		}
		if res := ExtractResult(schema, respBody); len(res) > 0 {
			extraction["result"] = res
		}
		out["extraction"] = extraction
	}
	writeJSON(w, http.StatusOK, out)
}

// handlePending 列出终态但回调未确认的任务(对账),凭证脱敏。
func (h *ManagementHandler) handlePending(w http.ResponseWriter, r *http.Request) {
	tasks := h.store.ListAll()
	out := make([]map[string]any, 0, len(tasks))
	for _, t := range tasks {
		if t.CallbackState == CallbackConfirmed {
			continue
		}
		out = append(out, publicTaskView(t))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": out, "count": len(out)})
}

// 不自动重发不确定请求；管理员可补录上游任务号/原响应，或重试文件与通知交接。
func (h *ManagementHandler) handleRecover(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UpstreamTaskID string          `json:"upstream_task_id"`
		Response       json.RawMessage `json:"response"`
	}
	if err := decodeJSONBody(w, r, &in, 64<<20); err != nil {
		writeError(w, 400, "bad_request", err.Error())
		return
	}
	id := r.PathValue("taskID")
	h.tracker.activeMu.Lock()
	defer h.tracker.activeMu.Unlock()
	if h.tracker.active[id] {
		writeError(w, 409, "task_active", "task is still running")
		return
	}
	task, ok := h.store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if task.CallbackState == CallbackConfirmed {
		writeError(w, 409, "already_delivered", "task already delivered")
		return
	}
	schema, ok := h.tracker.taskSchema(task)
	if !ok {
		writeError(w, 409, "schema_missing", "schema missing")
		return
	}
	if in.UpstreamTaskID != "" && len(in.Response) > 0 {
		writeError(w, 400, "bad_request", "provide task ID or response, not both")
		return
	}
	if in.UpstreamTaskID != "" {
		if schema.Mode != "async" {
			writeError(w, 400, "bad_request", "sync upstream has no query endpoint")
			return
		}
		task.UpstreamTaskID = in.UpstreamTaskID
		task.Status = StatusProcessing
		task.PollStartedAt = time.Now()
	} else if len(in.Response) > 0 {
		task.Response = append([]byte(nil), in.Response...)
		task.Status = StatusSubmitting
	} else if task.Status == StatusRecoveryRequired {
		if task.UpstreamTaskID != "" {
			task.Status = StatusProcessing
			task.PollStartedAt = time.Now()
		} else if schema.Request.IdempotencyHeader != "" {
			task.Status = StatusQueued
		} else {
			writeError(w, 409, "recovery_required", "attach upstream task ID or response; unsafe to resubmit")
			return
		}
	}
	task.CallbackAttempts = 0
	task.CallbackState = CallbackPending
	task.ErrorMessage = ""
	if err := h.store.Update(task); err != nil {
		writeError(w, 503, "store_failed", err.Error())
		return
	}
	// Track 自行加锁，释放管理操作的锁后再启动。
	h.tracker.activeMu.Unlock()
	h.tracker.Track(task)
	h.tracker.activeMu.Lock()
	writeJSON(w, 202, map[string]any{"task_id": task.ID, "status": task.Status})
}

// PublicTaskView 供内部复用:输出脱敏任务视图。
func PublicTaskView(t *Task) map[string]any { return publicTaskView(t) }

func publicTaskView(t *Task) map[string]any {
	out := map[string]any{
		"task_id":            t.ID,
		"upstream_task_id":   t.UpstreamTaskID,
		"schema_id":          t.SchemaID,
		"model":              t.Model,
		"user_id":            t.UserID,
		"key_id":             t.KeyID,
		"group_id":           t.GroupID,
		"status":             t.Status,
		"callback_state":     t.CallbackState,
		"callback_attempts":  t.CallbackAttempts,
		"created_at":         t.CreatedAt.UTC().Format(time.RFC3339),
		"files_acknowledged": t.FilesAcknowledged,
	}
	if t.FilesDeletedAt != nil {
		out["files_deleted_at"] = t.FilesDeletedAt
	}
	if t.Usage != nil {
		out["usage"] = map[string]any{"quantity": t.Usage.Quantity, "unit": t.Usage.Unit}
	}
	if t.ErrorMessage != "" {
		out["error_message"] = t.ErrorMessage
	}
	if t.LastCallbackError != "" {
		out["last_callback_error"] = t.LastCallbackError
	}
	return out
}

// decodeDryRunBody 支持 JSON 对象或 "base64:..." 字符串(multipart 演示)。
func decodeDryRunBody(v any) ([]byte, error) {
	switch b := v.(type) {
	case nil:
		return nil, nil
	case string:
		if s, ok := strings.CutPrefix(b, "base64:"); ok {
			return base64.StdEncoding.DecodeString(s)
		}
		return []byte(b), nil
	default:
		return json.Marshal(b)
	}
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any, limit int64) error {
	buf, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	if len(buf) == 0 {
		return fmt.Errorf("request body is empty")
	}
	dec := json.NewDecoder(strings.NewReader(string(buf)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("decode body: %w", err)
	}
	return nil
}
