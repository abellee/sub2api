package relay

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	HeaderClientQuery = "X-Media-Client-Query"
	taskQueryLifetime = time.Hour
	taskQueryInterval = 3 * time.Second
)

// Only public polling is limited; settlement, recovery and synchronous waits are internal.
type taskQueryLimits struct {
	mu        sync.Mutex
	next      map[string]time.Time
	cleanedAt time.Time
}

func (h *RelayHandler) allowTaskQuery(w http.ResponseWriter, task *Task, content bool) bool {
	now := h.now()
	expires := task.CreatedAt.Add(taskQueryLifetime)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Task-Query-Expires-At", expires.UTC().Format(time.RFC3339))
	if !now.Before(expires) {
		writeJSON(w, http.StatusGone, map[string]any{"id": task.ID, "task_id": task.ID, "status": "expired", "stop_polling": true, "retryable": false, "query_expires_at": expires.UTC().Format(time.RFC3339), "error": map[string]any{"type": "task_query_expired", "message": "Task query has expired. Stop polling; contact support with the task ID if the result is unknown."}})
		return false
	}
	// A status check followed immediately by downloading the completed video is valid.
	if content && task.Status == StatusSucceeded && task.CallbackState == CallbackConfirmed {
		return true
	}
	l := &h.queryLimits
	l.mu.Lock()
	if l.next == nil {
		l.next = make(map[string]time.Time)
	}
	if now.Sub(l.cleanedAt) >= time.Minute {
		for id, next := range l.next {
			if !now.Before(next) {
				delete(l.next, id)
			}
		}
		l.cleanedAt = now
	}
	next := l.next[task.ID]
	if !now.Before(next) {
		l.next[task.ID] = now.Add(taskQueryInterval)
	}
	l.mu.Unlock()
	if now.Before(next) {
		seconds := int(math.Ceil(next.Sub(now).Seconds()))
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"id": task.ID, "task_id": task.ID, "status": "rate_limited", "retryable": true, "retry_after_seconds": seconds, "error": map[string]any{"type": "task_query_rate_limited", "message": "Task queried too frequently. Wait for Retry-After before polling again."}})
		return false
	}
	w.Header().Set("Retry-After", "3")
	return true
}
