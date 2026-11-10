package relay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

// The link grants access to one task's status and image links, never account credentials.
func (h *RelayHandler) resultToken(id string) string {
	m := hmac.New(sha256.New, []byte(h.secret))
	m.Write([]byte("task-result/" + id))
	return hex.EncodeToString(m.Sum(nil))
}

func (h *RelayHandler) writeAccepted(w http.ResponseWriter, task *Task, duplicate bool) {
	w.Header().Set("Location", "/relay/tasks/"+task.ID)
	out := map[string]any{"task_id": task.ID, "upstream_task_id": task.UpstreamTaskID, "status": task.Status, "poll_url": "/relay/tasks/" + task.ID, "duplicated": duplicate}
	if h.tracker.downloadBase != "" {
		out["result_url"] = strings.TrimRight(h.tracker.downloadBase, "/") + "/downloads/" + task.ID + "/result?token=" + h.resultToken(task.ID)
	}
	writeJSON(w, http.StatusAccepted, out)
}

func publicMediaHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Expose-Headers", "Retry-After")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func (h *RelayHandler) handlePublicResult(w http.ResponseWriter, r *http.Request) {
	publicMediaHeaders(w)
	id := r.PathValue("taskID")
	if h.secret == "" || !hmac.Equal([]byte(h.resultToken(id)), []byte(r.URL.Query().Get("token"))) {
		http.NotFound(w, r)
		return
	}
	task, ok := h.store.PublicTask(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if task.Status == StatusSucceeded && task.FilesAcknowledged {
		writeError(w, http.StatusGone, "result_expired", "media files have expired")
		return
	}
	out := map[string]any{"id": id, "task_id": id, "status": "processing"}
	switch {
	case task.Status == StatusFailed || task.Status == StatusRecoveryRequired:
		out["status"] = task.Status
		_, code, message := publicTaskError(task)
		out["error"] = map[string]any{"type": code, "message": message}
	case task.Status == StatusSucceeded && task.CallbackState == CallbackConfirmed && len(task.Files) > 0:
		out["status"] = "completed"
		items := make([]map[string]any, 0, len(task.Files))
		for _, f := range task.Files {
			items = append(items, map[string]any{"url": formattedDownloadURL(f), "mime_type": f.ContentType})
		}
		out["result"] = map[string]any{"created": task.CreatedAt.Unix(), "data": items}
	default:
		w.Header().Set("Retry-After", "3")
	}
	writeJSON(w, http.StatusOK, out)
}
