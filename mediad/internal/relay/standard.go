package relay

import (
	"encoding/json"
	"net/http"
	"os"
)

// Standard results become readable only after the main site confirms billing.
func (h *RelayHandler) handleStandardResult(w http.ResponseWriter, r *http.Request, task *Task, format string) {
	if format == "error_context" {
		h.writeErrorContext(w, task)
		return
	}
	if writeStagingFailure(w, task) {
		return
	}
	if format == "state" {
		writeJSON(w, 200, publicTaskView(task))
		return
	}
	if format != "openai_url" && format != "openai_b64_json" && format != "gemini" && format != "grok_video" && format != "openai_sse" && format != "openai_edit_sse" {
		writeError(w, 400, "invalid_format", "unsupported result format")
		return
	}
	if task.Status == StatusSucceeded && task.FilesAcknowledged {
		writeError(w, http.StatusGone, "result_expired", "media files have expired")
		return
	}
	if task.Status == StatusRecoveryRequired || task.Status == StatusInterrupted {
		writeError(w, 502, "generation_unavailable", "The media task needs verification before it can be retried.")
		return
	}
	if task.Status == StatusFailed {
		release, err := h.store.errorResponse(task)
		if err != nil {
			writeError(w, 503, "memory_busy", err.Error())
			return
		}
		defer release()
		if task.ResponseStatus >= 400 && json.Valid(task.Response) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(task.ResponseStatus)
			_, _ = w.Write(task.Response)
			return
		}
		writeError(w, 502, "generation_failed", task.ErrorMessage)
		return
	}
	if task.Status != StatusSucceeded || task.CallbackState != CallbackConfirmed || len(task.Files) == 0 {
		if format == "grok_video" {
			writeJSON(w, 200, map[string]any{"id": task.ID, "request_id": task.ID, "status": "pending"})
			return
		}
		writeError(w, 409, "result_pending", "result or settlement is pending")
		return
	}
	var files []*os.File
	if format == "openai_b64_json" || format == "openai_sse" || format == "openai_edit_sse" {
		if h.tracker.stager == nil {
			writeError(w, 503, "storage_unavailable", "result storage unavailable")
			return
		}
		var release func()
		var err error
		files, release, err = h.store.openDownloads(h.tracker.stager.Root, task, task.Files)
		if err != nil {
			writeError(w, 503, "file_unavailable", "saved result unavailable")
			return
		}
		defer release()
	}
	var out map[string]any
	_ = json.Unmarshal(h.store.responseMetadata(task), &out)
	if out == nil {
		out = map[string]any{}
	}
	if format == "grok_video" {
		video := map[string]any{"url": formattedDownloadURL(task.Files[0])}
		if task.Usage != nil {
			video["duration"] = task.Usage.Quantity
		}
		writeJSON(w, 200, map[string]any{"id": task.ID, "request_id": task.ID, "status": "done", "model": task.Model, "video": video, "video_url": video["url"], "url": video["url"]})
		return
	}
	if format == "gemini" {
		// Deleted images must not survive in the original inline response.
		if len(task.DeletedFileIDs) > 0 {
			parts := make([]any, 0, len(task.Files))
			for _, file := range task.Files {
				parts = append(parts, map[string]any{"fileData": map[string]any{"mimeType": file.ContentType, "fileUri": formattedDownloadURL(file)}})
			}
			out["candidates"] = []any{map[string]any{"content": map[string]any{"role": "model", "parts": parts}, "finishReason": "STOP"}}
		}
		if _, ok := out["candidates"]; !ok {
			writeError(w, 502, "invalid_result", "Gemini candidates missing")
			return
		}
		urls := make([]string, 0, len(task.Files))
		for _, file := range task.Files {
			urls = append(urls, formattedDownloadURL(file))
		}
		// Canvas and Gemini clients can read fileData without transferring image bodies through the gateway.
		fileIndex := 0
		candidates, _ := out["candidates"].([]any)
		for _, candidate := range candidates {
			fields, _ := candidate.(map[string]any)
			content, _ := fields["content"].(map[string]any)
			parts, _ := content["parts"].([]any)
			for _, part := range parts {
				fields, _ := part.(map[string]any)
				if fields == nil || fileIndex >= len(task.Files) {
					continue
				}
				_, inline := fields["inlineData"]
				_, snakeInline := fields["inline_data"]
				_, file := fields["fileData"]
				_, snakeFile := fields["file_data"]
				if !inline && !snakeInline && !file && !snakeFile {
					continue
				}
				delete(fields, "inlineData")
				delete(fields, "inline_data")
				delete(fields, "file_data")
				fields["fileData"] = map[string]any{"mimeType": task.Files[fileIndex].ContentType, "fileUri": urls[fileIndex]}
				fileIndex++
			}
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
			if format == "openai_b64_json" || format == "openai_sse" || format == "openai_edit_sse" {
				delete(item, "url")
				item["download_url"] = formattedDownloadURL(file)
			}
			data = append(data, item)
		}
		out["data"] = data
		if _, ok := out["created"]; !ok {
			out["created"] = task.CreatedAt.Unix()
		}
	}
	if format == "openai_b64_json" || format == "openai_sse" || format == "openai_edit_sse" {
		h.streamImageResult(w, r, files, format, out)
		return
	}
	writeJSON(w, 200, out)
}
