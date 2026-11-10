package relay

import "net/http"

// Public errors use fixed messages; saved provider responses remain available for recovery.
func publicTaskError(task *Task) (int, string, string) {
	if task.Status != StatusFailed {
		return http.StatusBadGateway, "generation_unavailable", "Generation result is unavailable. Please contact support with the task ID before retrying."
	}
	switch task.ResponseStatus {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return task.ResponseStatus, "generation_failed", "Generation failed. Please check the request and try again."
	case http.StatusTooManyRequests:
		return http.StatusTooManyRequests, "generation_busy", "Generation service is busy. Please try again later."
	default:
		return http.StatusBadGateway, "generation_failed", "Generation failed. Please contact support with the task ID."
	}
}

func writeTaskError(w http.ResponseWriter, task *Task, format string) {
	status, code, message := publicTaskError(task)
	w.Header().Set("X-Task-Id", task.ID)
	err := map[string]any{"type": code, "message": message}
	if format == "gemini" {
		name := "INTERNAL"
		if status == http.StatusBadRequest || status == http.StatusRequestEntityTooLarge || status == http.StatusUnsupportedMediaType || status == http.StatusUnprocessableEntity {
			name = "INVALID_ARGUMENT"
		} else if status == http.StatusTooManyRequests {
			name = "RESOURCE_EXHAUSTED"
		}
		err = map[string]any{"code": status, "status": name, "message": message}
	}
	writeJSON(w, status, map[string]any{"task_id": task.ID, "error": err})
}
