package relay

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"mediad/internal/signature"
)

const (
	HeaderTimestamp      = "X-Media-Timestamp"
	HeaderSignature      = "X-Media-Signature"
	HeaderCredential     = "X-Media-Credential"
	HeaderUpstreamURL    = "X-Media-Upstream-URL"
	HeaderSchemaVersion  = "X-Media-Schema-Version"
	HeaderIdempotencyKey = "X-Media-Idempotency-Key"
	HeaderModel          = "X-Media-Model"
	HeaderUserID         = "X-Sub2API-User-Id"
	HeaderKeyID          = "X-Sub2API-Key-Id"
	HeaderGroupID        = "X-Sub2API-Group-Id"
	HeaderUsage          = "X-Media-Usage"
	HeaderUsageError     = "X-Media-Usage-Error"
)

const MaxRelayBody = 128 << 20

type RelayHandler struct {
	registry *SchemaRegistry
	store    *MemoryStore
	tracker  *Tracker
	secret   string
	MaxSync  time.Duration
	now      func() time.Time
	createMu sync.Mutex
}

func NewRelayHandler(registry *SchemaRegistry, store *MemoryStore, tracker *Tracker, secret string, logger *log.Logger) *RelayHandler {
	return &RelayHandler{registry: registry, store: store, tracker: tracker, secret: secret, MaxSync: 10 * time.Minute, now: time.Now}
}

func (h *RelayHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /downloads/{taskID}/{fileID}", h.handleDownload)
	mux.HandleFunc("POST /relay/{schemaID}", h.handleCreate)
	mux.HandleFunc("POST /relay/{schemaID}/{path...}", h.handleCreate)
	mux.HandleFunc("GET /relay/tasks/{taskID}", h.handleTaskStatus)
	mux.HandleFunc("GET /relay/tasks/{taskID}/files/{fileID}", h.handleFile)
	mux.HandleFunc("POST /relay/tasks/{taskID}/files/ack", h.handleFilesAck)
}

// 图片、视频统一先持久化受理，实际生成与客户端连接断开无关。
func (h *RelayHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	if h.secret == "" {
		writeError(w, 503, "relay_disabled", "relay secret is not configured")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxRelayBody))
	if err != nil {
		writeError(w, 413, "body_too_large", err.Error())
		return
	}
	if err := h.verify(r, body); err != nil {
		writeError(w, 401, "invalid_signature", err.Error())
		return
	}
	schema, ok := h.registry.Get(r.PathValue("schemaID"))
	if !ok {
		writeError(w, 400, "unknown_schema", "schema is not registered")
		return
	}
	if v := r.Header.Get(HeaderSchemaVersion); v != "" {
		if want, _ := strconv.Atoi(v); want != schema.Version {
			writeError(w, 409, "schema_version_mismatch", "schema version mismatch")
			return
		}
	}
	in := AssembleInput{Schema: schema, UpstreamURL: r.Header.Get(HeaderUpstreamURL), Credential: r.Header.Get(HeaderCredential), Method: schema.Upstream.Create.Method, ClientPath: "/" + r.PathValue("path"), Body: body, ContentType: r.Header.Get("Content-Type"), UserID: r.Header.Get(HeaderUserID), KeyID: r.Header.Get(HeaderKeyID), GroupID: r.Header.Get(HeaderGroupID), Model: r.Header.Get(HeaderModel)}
	if in.UpstreamURL == "" || in.Credential == "" || in.UserID == "" || in.KeyID == "" {
		writeError(w, 400, "missing_required_headers", "upstream and identity headers are required")
		return
	}
	assembled, err := Assemble(in)
	if err != nil {
		writeError(w, 400, "assemble_failed", err.Error())
		return
	}
	task := &Task{ID: newTaskID(h.now()), Schema: schema, SchemaID: schema.ID, SchemaVersion: schema.Version, Request: assembled, Model: in.Model, UserID: in.UserID, KeyID: in.KeyID, GroupID: in.GroupID, UpstreamURL: in.UpstreamURL, Credential: in.Credential, Status: StatusQueued, CreatedAt: h.now(), CallbackState: CallbackPending}
	var params map[string]any
	_ = json.Unmarshal(body, &params)
	task.Parameters = map[string]any{}
	for _, k := range []string{"duration", "resolution", "size", "n"} {
		if v, ok := params[k]; ok {
			task.Parameters[k] = v
		}
	}
	fingerprint := sha256.Sum256(append([]byte(schema.ID+"\x00"+in.Model+"\x00"+in.UpstreamURL+"\x00"), body...))
	task.Fingerprint = hex.EncodeToString(fingerprint[:])
	if key := r.Header.Get(HeaderIdempotencyKey); key != "" {
		scope, _ := json.Marshal([]string{in.UserID, in.KeyID, in.GroupID, schema.ID, key})
		sum := sha256.Sum256(scope)
		task.IdempotencyKey = hex.EncodeToString(sum[:])
	}
	// 只串行短暂的防重检查/写盘，不串行上游调用。
	h.tracker.settingsMu.Lock()
	defer h.tracker.settingsMu.Unlock()
	if h.tracker.runtimeSettings != nil && h.tracker.runtimeSettings.StorageChangePending() {
		writeError(w, 503, "storage_restart_pending", "storage directory change pending; restart after migrating service state")
		return
	}
	h.createMu.Lock()
	if task.IdempotencyKey != "" {
		if prev, ok := h.store.FindByIdempotencyKey(task.IdempotencyKey); ok {
			h.createMu.Unlock()
			if prev.Fingerprint != task.Fingerprint {
				writeError(w, 409, "idempotency_conflict", "same key used with a different request")
				return
			}
			writeAccepted(w, prev, true)
			return
		}
	}
	err = h.store.Create(task)
	h.createMu.Unlock()
	if err != nil {
		writeError(w, 503, "task_store_failed", err.Error())
		return
	}
	h.tracker.Track(task)
	writeAccepted(w, task, false)
}

func writeAccepted(w http.ResponseWriter, task *Task, duplicate bool) {
	w.Header().Set("Location", "/relay/tasks/"+task.ID)
	writeJSON(w, http.StatusAccepted, map[string]any{"task_id": task.ID, "upstream_task_id": task.UpstreamTaskID, "status": task.Status, "poll_url": "/relay/tasks/" + task.ID, "duplicated": duplicate})
}

func (h *RelayHandler) handleTaskStatus(w http.ResponseWriter, r *http.Request) {
	if h.secret == "" {
		writeError(w, 503, "relay_disabled", "relay secret is not configured")
		return
	}
	if err := h.verify(r, nil); err != nil {
		writeError(w, 401, "invalid_signature", err.Error())
		return
	}
	task, ok := h.ownedTask(r)
	if !ok {
		writeError(w, 404, "task_not_found", "task not found")
		return
	}
	out := publicTaskView(task)
	if task.Result != nil {
		out["result"] = task.Result
	}
	out["files"] = task.Files
	out["files_acknowledged"] = task.FilesAcknowledged
	writeJSON(w, 200, out)
}

func (h *RelayHandler) verify(r *http.Request, body []byte) error {
	return signature.Verify(h.secret, r.Header.Get(HeaderTimestamp), r.Method, r.URL.Path, body, r.Header.Get(HeaderSignature), h.now())
}

func newTaskID(now time.Time) string {
	var rnd [16]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		panic(fmt.Errorf("task ID randomness: %w", err))
	}
	return "mtask_" + hex.EncodeToString(rnd[:])
}
func marshalUsage(quantity float64, unit string) string {
	b, _ := json.Marshal(map[string]any{"quantity": quantity, "unit": unit})
	return string(b)
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, typ, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"type": typ, "message": msg}})
}
