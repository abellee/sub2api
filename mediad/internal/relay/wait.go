package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (h *RelayHandler) waitStandardResult(w http.ResponseWriter, r *http.Request, task *Task, format string) {
	stream := format == "openai_sse" || format == "openai_edit_sse"
	if !stream && format != "openai_url" && format != "openai_b64_json" && format != "gemini" {
		writeError(w, 400, "invalid_format", "unsupported result format")
		return
	}
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		_, _ = fmt.Fprint(w, ": keep-alive\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	lastHeartbeat := time.Now()
	for {
		if task.Status == StatusFailed || (task.Status == StatusSucceeded && task.CallbackState == CallbackConfirmed) {
			var ok bool
			task, ok = h.store.Get(task.ID)
			if !ok {
				return
			}
			if stream {
				recorder := newResultRecorder()
				h.handleStandardResult(recorder, r, task, "openai_b64_json")
				event := "image_generation.completed"
				if format == "openai_edit_sse" {
					event = "image_edit.completed"
				}
				mediaStreamResult(w, r, recorder.status, recorder.body.Bytes(), event)
			} else {
				h.handleStandardResult(w, r, task, format)
			}
			return
		}
		if stream && time.Since(lastHeartbeat) >= 10*time.Second {
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			lastHeartbeat = time.Now()
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
		var ok bool
		task, ok = h.store.PublicTask(task.ID)
		if !ok {
			return
		}
	}
}

type resultRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newResultRecorder() *resultRecorder {
	return &resultRecorder{header: make(http.Header), status: 200}
}
func (w *resultRecorder) Header() http.Header         { return w.header }
func (w *resultRecorder) WriteHeader(status int)      { w.status = status }
func (w *resultRecorder) Write(p []byte) (int, error) { return w.body.Write(p) }
func mediaStreamEvent(w http.ResponseWriter, r *http.Request, event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	controller := http.NewResponseController(w)
	_ = controller.SetWriteDeadline(time.Now().Add(120 * time.Second))
	defer func() { _ = controller.SetWriteDeadline(time.Time{}) }()
	if _, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

func mediaStreamResult(w http.ResponseWriter, r *http.Request, status int, body []byte, event string) {
	if status >= 400 {
		var payload map[string]any
		if json.Unmarshal(body, &payload) != nil {
			payload = map[string]any{"error": map[string]any{"message": "image generation failed"}}
		}
		payload["type"] = "error"
		_ = mediaStreamEvent(w, r, "error", payload)
		return
	}
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil {
		_ = mediaStreamEvent(w, r, "error", map[string]any{"type": "error", "error": map[string]any{"message": "invalid image result"}})
		return
	}
	items, _ := payload["data"].([]any)
	if len(items) == 0 {
		_ = mediaStreamEvent(w, r, "error", map[string]any{"type": "error", "error": map[string]any{"message": "upstream returned no images"}})
		return
	}
	delete(payload, "data")
	payload["type"] = event
	payload["created_at"] = payload["created"]
	delete(payload, "created")
	for _, item := range items {
		out := make(map[string]any, len(payload)+3)
		for k, v := range payload {
			out[k] = v
		}
		if fields, ok := item.(map[string]any); ok {
			for k, v := range fields {
				out[k] = v
			}
		}
		if err := mediaStreamEvent(w, r, event, out); err != nil {
			return
		}
	}
}
