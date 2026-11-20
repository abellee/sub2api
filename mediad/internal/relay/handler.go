package relay

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
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
	HeaderProtocol       = "X-Media-Protocol"
	HeaderModel          = "X-Media-Model"
	HeaderUserID         = "X-Sub2API-User-Id"
	HeaderKeyID          = "X-Sub2API-Key-Id"
	HeaderGroupID        = "X-Sub2API-Group-Id"
	HeaderUsage          = "X-Media-Usage"
	HeaderUsageError     = "X-Media-Usage-Error"
)

const MaxRelayBody = 128 << 20

type RelayHandler struct {
	registry    *SchemaRegistry
	store       *MemoryStore
	tracker     *Tracker
	secret      string
	MaxSync     time.Duration
	now         func() time.Time
	createMu    sync.Mutex
	queryLimits taskQueryLimits
}

func NewRelayHandler(registry *SchemaRegistry, store *MemoryStore, tracker *Tracker, secret string, logger *log.Logger) *RelayHandler {
	return &RelayHandler{registry: registry, store: store, tracker: tracker, secret: secret, MaxSync: 10 * time.Minute, now: time.Now}
}

func (h *RelayHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /downloads/{taskID}/result", h.handlePublicResult)
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
	schema, ok := h.registry.Get(r.PathValue("schemaID"))
	if !ok {
		writeError(w, 400, "unknown_schema", "schema is not registered")
		return
	}
	streamed := canStreamRequest(schema, r, h.store)
	need := r.ContentLength
	limit := int64(MaxRelayBody)
	if streamed {
		need = 0
	}
	release, err := reserveRequest(need, limit)
	if err != nil {
		w.Header().Set("Retry-After", "3")
		writeError(w, 503, "memory_busy", err.Error())
		return
	}
	defer release()
	var body []byte
	if !streamed {
		readLimit := int64(MaxRelayBody)
		if r.ContentLength >= 0 && r.ContentLength < readLimit {
			readLimit = r.ContentLength
		}
		body, err = io.ReadAll(http.MaxBytesReader(w, r.Body, readLimit))
		if err != nil {
			writeError(w, 413, "body_too_large", err.Error())
			return
		}
		if err = h.verify(r, body); err != nil {
			writeError(w, 401, "invalid_signature", err.Error())
			return
		}
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
	id := newTaskID(h.now())
	var assembled *UpstreamRequest
	var ref *payloadRef
	var params map[string]any
	var streamedFingerprint string
	if streamed {
		assembled, ref, params, streamedFingerprint, err = h.streamRequest(w, r, in, id)
		if err != nil {
			code := 400
			if errors.Is(err, signature.ErrMismatch) || errors.Is(err, signature.ErrExpired) || errors.Is(err, signature.ErrBadFormat) {
				code = 401
			}
			writeError(w, code, "invalid_request_error", err.Error())
			return
		}
	} else {
		if r.Header.Get(HeaderProtocol) != "" {
			in.Body, in.ContentType, err = normalizeStandardRequest(in, r.Header.Get(HeaderProtocol))
			if err != nil {
				writeError(w, 400, "invalid_request_error", err.Error())
				return
			}
		}
		assembled, err = Assemble(in)
		if err != nil {
			writeError(w, 400, "assemble_failed", err.Error())
			return
		}
		_ = json.Unmarshal(compactRequestParameters(in.Body), &params)
	}
	committed := false
	defer func() {
		if streamed && !committed {
			_ = os.RemoveAll(h.store.payloadDir(id))
		}
	}()
	task := &Task{ID: id, Schema: schema, SchemaID: schema.ID, SchemaVersion: schema.Version, Request: assembled, RequestData: ref, Model: in.Model, UserID: in.UserID, KeyID: in.KeyID, GroupID: in.GroupID, UpstreamURL: in.UpstreamURL, Credential: in.Credential, Status: StatusQueued, CreatedAt: h.now(), CallbackState: CallbackPending}
	task.InboundEndpoint = endpointPath(firstNonEmpty(r.Header.Get("X-Media-Inbound-Endpoint"), in.ClientPath))
	task.UpstreamEndpoint = endpointPath(assembled.URL)
	task.Parameters = map[string]any{}
	for _, k := range []string{"duration", "resolution", "size", "n"} {
		if v, ok := params[k]; ok {
			task.Parameters[k] = v
		}
	}
	if generation, ok := params["generationConfig"].(map[string]any); ok {
		if image, ok := generation["imageConfig"].(map[string]any); ok {
			task.Parameters["size"] = image["imageSize"]
		}
	}
	fingerprint := sha256.Sum256(append([]byte(schema.ID+"\x00"+in.Model+"\x00"+in.UpstreamURL+"\x00"), body...))
	task.Fingerprint = hex.EncodeToString(fingerprint[:])
	if streamed {
		task.Fingerprint = streamedFingerprint
	}
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
			h.writeAccepted(w, prev, true)
			return
		}
	}
	err = h.store.Create(task)
	h.createMu.Unlock()
	if err != nil {
		writeError(w, 503, "task_store_failed", err.Error())
		return
	}
	committed = true
	h.tracker.Track(task)
	h.writeAccepted(w, task, false)
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
	if query := r.Header.Get(HeaderClientQuery); query != "" && !h.allowTaskQuery(w, task, query == "content") {
		return
	}
	if format := r.URL.Query().Get("response_format"); format != "" {
		if r.URL.Query().Get("wait") == "true" {
			h.waitStandardResult(w, r, task, format)
			return
		}
		h.handleStandardResult(w, r, task, format)
		return
	}
	for i := range task.Files {
		task.Files[i].DownloadURL = formattedDownloadURL(task.Files[i])
	}
	out := publicTaskView(task)
	if task.Result != nil {
		out["result"] = task.Result
	}
	out["files"] = task.Files
	out["files_acknowledged"] = task.FilesAcknowledged
	if task.FilesDeletedAt != nil {
		out["files_deleted_at"] = task.FilesDeletedAt
	}
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

// Store only the path; provider URLs can contain credentials or signed queries.
func endpointPath(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.EscapedPath()
}
