package relay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type FileDeletionEvent struct {
	Event     string    `json:"event"`
	EventID   string    `json:"event_id"`
	TaskID    string    `json:"task_id"`
	UserID    string    `json:"user_id"`
	KeyID     string    `json:"key_id"`
	FileIDs   []string  `json:"file_ids"`
	DeletedAt time.Time `json:"deleted_at"`
	Reason    string    `json:"reason"`
}

type deletionNotice struct {
	Event         FileDeletionEvent `json:"event"`
	Ready         bool              `json:"ready"`
	NextAttemptAt time.Time         `json:"next_attempt_at,omitempty"`
}

// Deletion notices outlive task GC and are removed only after the main site's explicit acknowledgement.
type deletionQueue struct {
	mu    sync.Mutex
	root  string
	items map[string]deletionNotice
}

func newDeletionEvent(task *Task, ids []string, at time.Time, reason string) FileDeletionEvent {
	ids = append([]string(nil), ids...)
	sort.Strings(ids)
	sum := sha256.Sum256([]byte(task.ID + "\n" + task.UserID + "\n" + task.KeyID + "\n" + strings.Join(ids, ",")))
	return FileDeletionEvent{Event: "files.deleted", EventID: "mfd_" + hex.EncodeToString(sum[:16]), TaskID: task.ID, UserID: task.UserID, KeyID: task.KeyID, FileIDs: ids, DeletedAt: at.UTC(), Reason: reason}
}

func (q *deletionQueue) put(ev FileDeletionEvent, ready bool) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	n, exists := q.items[ev.EventID]
	if exists && (n.Ready || !ready) {
		return nil
	}
	n = deletionNotice{Event: ev, Ready: ready}
	if err := q.persist(n); err != nil {
		return err
	}
	q.items[ev.EventID] = n
	return nil
}

func (q *deletionQueue) persist(n deletionNotice) error {
	if q.root == "" {
		return nil
	}
	return WriteState(filepath.Join(q.root, n.Event.EventID+".json"), n)
}

func (q *deletionQueue) pending(now time.Time) []deletionNotice {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []deletionNotice
	for _, n := range q.items {
		if n.Ready && !n.NextAttemptAt.After(now) {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Event.EventID < out[j].Event.EventID })
	if len(out) > 100 {
		out = out[:100]
	}
	return out
}

func (q *deletionQueue) hasRetentionTask(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, n := range q.items {
		if n.Event.TaskID == id && n.Event.Reason == "retention_expired" {
			return true
		}
	}
	return false
}

func (q *deletionQueue) finish(id string, acknowledged bool) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	n, ok := q.items[id]
	if !ok {
		return nil
	}
	if !acknowledged {
		n.NextAttemptAt = time.Now().Add(time.Minute)
		if err := q.persist(n); err != nil {
			return err
		}
		q.items[id] = n
		return nil
	}
	if q.root != "" {
		if err := os.Remove(filepath.Join(q.root, id+".json")); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	delete(q.items, id)
	return nil
}

func (q *deletionQueue) load(root string) error {
	q.root = root
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			return err
		}
		var n deletionNotice
		if err := json.Unmarshal(raw, &n); err != nil {
			return err
		}
		if !strings.HasPrefix(n.Event.EventID, "mfd_") || n.Event.EventID+".json" != e.Name() || filepath.Base(n.Event.EventID) != n.Event.EventID || (n.Ready && n.Event.DeletedAt.IsZero()) {
			return fmt.Errorf("invalid file deletion notice %s", e.Name())
		}
		q.items[n.Event.EventID] = n
	}
	return nil
}

func (t *Tracker) StartFileDeletionNotifications() {
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			t.DeliverFileDeletions(t.ctx)
			select {
			case <-t.ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// A legacy task-completion ACK must never acknowledge a file deletion.
func (t *Tracker) DeliverFileDeletions(parent context.Context) int {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	delivered := 0
	for _, n := range t.store.deletions.pending(time.Now()) {
		if ctx.Err() != nil {
			break
		}
		payload, err := json.Marshal(n.Event)
		if err != nil {
			continue
		}
		attempt, done := context.WithTimeout(ctx, 10*time.Second)
		_, raw, err := t.deliverRaw(attempt, "files.deleted", payload)
		done()
		var ack struct {
			Deleted bool   `json:"files_deleted"`
			EventID string `json:"event_id"`
		}
		ok := err == nil && json.Unmarshal(raw, &ack) == nil && ack.Deleted && ack.EventID == n.Event.EventID
		if err := t.store.deletions.finish(n.Event.EventID, ok); err != nil {
			t.logger.Printf("[mediad] persist file deletion acknowledgement: %v", err)
			continue
		}
		if ok {
			delivered++
		} else {
			t.logger.Printf("[mediad] file deletion notification pending task=%s", n.Event.TaskID)
		}
	}
	return delivered
}

type reconcileFileRecord struct {
	TaskID  string   `json:"task_id"`
	UserID  string   `json:"user_id"`
	KeyID   string   `json:"key_id"`
	FileIDs []string `json:"file_ids"`
}

// Reconciliation checks the media filesystem, rather than treating an HTTP error as deletion.
func (h *ManagementHandler) handleReconcileFiles(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Records []reconcileFileRecord `json:"records"`
	}
	if err := decodeJSONBody(w, r, &input, 1<<20); err != nil || len(input.Records) == 0 || len(input.Records) > 200 {
		writeError(w, 400, "bad_request", "expected 1 to 200 file records")
		return
	}
	if h.tracker.stager == nil {
		writeError(w, 503, "storage_unavailable", "media storage is unavailable")
		return
	}
	root := h.tracker.stager.Root
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		writeError(w, 503, "storage_unavailable", "media storage root is unavailable")
		return
	}
	queued := 0
	for _, record := range input.Records {
		user, userErr := strconv.ParseInt(record.UserID, 10, 64)
		key, keyErr := strconv.ParseInt(record.KeyID, 10, 64)
		if !validTaskID(record.TaskID) || userErr != nil || keyErr != nil || user <= 0 || key <= 0 || len(record.FileIDs) == 0 || len(record.FileIDs) > 256 {
			writeError(w, 400, "bad_request", "invalid file record")
			return
		}
		task, exists := h.store.PublicTask(record.TaskID)
		if exists && (task.UserID != record.UserID || task.KeyID != record.KeyID) {
			writeError(w, 409, "owner_mismatch", "file owner mismatch")
			return
		}
		if exists && (task.Status != StatusSucceeded || task.CallbackState != CallbackConfirmed) {
			continue
		}
		var missing []string
		for _, id := range record.FileIDs {
			if !validFileID(id) {
				writeError(w, 400, "bad_request", "invalid file ID")
				return
			}
			if exists {
				found := false
				for _, f := range task.Files {
					if f.ID == id {
						found = true
						break
					}
				}
				if !found {
					writeError(w, 409, "file_mismatch", "file does not belong to task")
					return
				}
			}
			if _, err := os.Stat(filepath.Join(root, record.TaskID, id)); os.IsNotExist(err) {
				missing = append(missing, id)
			} else if err != nil {
				writeError(w, 503, "storage_unavailable", "cannot inspect media file")
				return
			}
		}
		if len(missing) == 0 {
			continue
		}
		reason, at := "files_missing", time.Now().UTC()
		if exists && task.FilesDeletedAt != nil {
			reason, at = "retention_expired", *task.FilesDeletedAt
		}
		ev := newDeletionEvent(&Task{ID: record.TaskID, UserID: record.UserID, KeyID: record.KeyID}, missing, at, reason)
		if err := h.store.deletions.put(ev, true); err != nil {
			writeError(w, 503, "storage_unavailable", "cannot save deletion notification")
			return
		}
		queued++
	}
	delivered := h.tracker.DeliverFileDeletions(r.Context())
	writeJSON(w, 200, map[string]any{"checked": len(input.Records), "queued_events": queued, "delivered_events": delivered})
}

func validTaskID(id string) bool {
	if !strings.HasPrefix(id, "mtask_") || len(id) > 128 || len(id) <= 6 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func validFileID(id string) bool {
	if len(id) < 5 || len(id) > 9 || id[0] != 'f' {
		return false
	}
	for _, c := range id[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
