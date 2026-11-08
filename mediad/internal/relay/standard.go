package relay

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

// Standard results become readable only after the main site confirms billing.
func (h *RelayHandler) handleStandardResult(w http.ResponseWriter, r *http.Request, task *Task, format string) {
	if format == "state" {
		writeJSON(w, 200, publicTaskView(task))
		return
	}
	if format != "openai_url" && format != "openai_b64_json" && format != "gemini" {
		writeError(w, 400, "invalid_format", "unsupported result format")
		return
	}
	if task.Status == StatusSucceeded && task.FilesAcknowledged {
		writeError(w, http.StatusGone, "result_expired", "media files have expired")
		return
	}
	if task.Status == StatusFailed {
		if (task.ResponseStatus >= 400 || (format == "gemini" && task.ResponseStatus == 200)) && json.Valid(task.Response) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(task.ResponseStatus)
			_, _ = w.Write(task.Response)
			return
		}
		writeError(w, 502, "generation_failed", task.ErrorMessage)
		return
	}
	if task.Status != StatusSucceeded || task.CallbackState != CallbackConfirmed || len(task.Files) == 0 {
		writeError(w, 409, "result_pending", "result or settlement is pending")
		return
	}
	var out map[string]any
	_ = json.Unmarshal(task.Response, &out)
	if out == nil {
		out = map[string]any{}
	}
	if format == "gemini" {
		if _, ok := out["candidates"]; !ok {
			writeError(w, 502, "invalid_result", "Gemini candidates missing")
			return
		}
		urls := make([]string, 0, len(task.Files))
		for _, file := range task.Files {
			urls = append(urls, formattedDownloadURL(file))
		}
		out["download_urls"] = urls
	} else {
		data := make([]map[string]any, 0, len(task.Files))
		original, _ := out["data"].([]any)
		for i, file := range task.Files {
			item := map[string]any{}
			if i < len(original) {
				if fields, ok := original[i].(map[string]any); ok {
					for k, v := range fields {
						if k != "url" && k != "b64_json" {
							item[k] = v
						}
					}
				}
			}
			item["url"] = formattedDownloadURL(file)
			if format == "openai_b64_json" {
				if h.tracker.stager == nil {
					writeError(w, 503, "storage_unavailable", "result storage unavailable")
					return
				}
				raw, err := os.ReadFile(filepath.Join(h.tracker.stager.Root, task.ID, file.ID))
				if err != nil {
					writeError(w, 503, "file_unavailable", "saved result unavailable")
					return
				}
				delete(item, "url")
				item["b64_json"] = base64.StdEncoding.EncodeToString(raw)
				item["download_url"] = formattedDownloadURL(file)
			}
			data = append(data, item)
		}
		out["data"] = data
		if _, ok := out["created"]; !ok {
			out["created"] = task.CreatedAt.Unix()
		}
	}
	writeJSON(w, 200, out)
}
