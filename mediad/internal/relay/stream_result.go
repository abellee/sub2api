package relay

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Every result image goes from the saved file to the client through a small buffer.
func (h *RelayHandler) streamImageResult(w http.ResponseWriter, r *http.Request, task *Task, format string, out map[string]any) {
	if h.tracker.stager == nil {
		writeError(w, 503, "storage_unavailable", "result storage unavailable")
		return
	}
	files := make([]*os.File, 0, len(task.Files))
	defer func() {
		for _, f := range files {
			f.Close()
		}
	}()
	for _, meta := range task.Files {
		f, err := os.Open(filepath.Join(h.tracker.stager.Root, task.ID, meta.ID))
		if err != nil {
			writeError(w, 503, "file_unavailable", "saved result unavailable")
			return
		}
		files = append(files, f)
	}
	items, _ := out["data"].([]map[string]any)
	stream := format == "openai_sse" || format == "openai_edit_sse"
	controller := http.NewResponseController(w)
	_ = controller.SetWriteDeadline(time.Now().Add(120 * time.Second))
	defer controller.SetWriteDeadline(time.Time{})
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		event := "image_generation.completed"
		if format == "openai_edit_sse" {
			event = "image_edit.completed"
		}
		for i, item := range items {
			payload := make(map[string]any)
			for k, v := range out {
				if k != "data" && k != "created" {
					payload[k] = v
				}
			}
			for k, v := range item {
				payload[k] = v
			}
			payload["type"] = event
			payload["created_at"] = out["created"]
			if _, err := fmt.Fprintf(w, "event: %s\ndata: ", event); err != nil {
				return
			}
			if err := writeImageObject(w, payload, files[i]); err != nil {
				return
			}
			if _, err := io.WriteString(w, "\n\n"); err != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := io.WriteString(w, "{"); err != nil {
		return
	}
	keys := make([]string, 0, len(out))
	for k := range out {
		if k != "data" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		key, _ := json.Marshal(k)
		value, err := json.Marshal(out[k])
		if err != nil {
			return
		}
		if _, err = fmt.Fprintf(w, "%s:%s,", key, value); err != nil {
			return
		}
	}
	if _, err := io.WriteString(w, "\"data\":["); err != nil {
		return
	}
	for i, item := range items {
		if i > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return
			}
		}
		if err := writeImageObject(w, item, files[i]); err != nil {
			return
		}
	}
	_, _ = io.WriteString(w, "]}")
}
func writeImageObject(w io.Writer, item map[string]any, file io.Reader) error {
	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}
	if _, err = w.Write(raw[:len(raw)-1]); err != nil {
		return err
	}
	if len(item) > 0 {
		if _, err = io.WriteString(w, ","); err != nil {
			return err
		}
	}
	if _, err = io.WriteString(w, "\"b64_json\":\""); err != nil {
		return err
	}
	encoder := base64.NewEncoder(base64.StdEncoding, w)
	if _, err = io.Copy(encoder, file); err != nil {
		return err
	}
	if err = encoder.Close(); err != nil {
		return err
	}
	_, err = io.WriteString(w, "\"}")
	return err
}
