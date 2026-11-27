package relay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"
)

// This wire shape mirrors lotteryd/api/events without adding a dependency to mediad.
type gameplayEvent struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Source     string            `json:"source"`
	UserID     int64             `json:"user_id"`
	Reference  string            `json:"reference"`
	OccurredAt time.Time         `json:"occurred_at"`
	Data       map[string]string `json:"data"`
}

func taskGameplayEvent(kind string, task *Task, reference string) gameplayEvent {
	uid, _ := strconv.ParseInt(task.UserID, 10, 64)
	if reference == "" {
		reference = task.ID
	}
	sum := sha256.Sum256([]byte("mediad/" + kind + "/" + reference))
	e := gameplayEvent{ID: "mediad:" + hex.EncodeToString(sum[:]), Type: kind, Source: "mediad", UserID: uid, Reference: reference, OccurredAt: time.Now().UTC(), Data: map[string]string{"source_id": task.ID, "api_key_id": task.KeyID, "group_id": task.GroupID, "model": task.Model, "status": task.Status, "image_count": strconv.Itoa(len(task.Files))}}
	if task.CompletedAt != nil {
		e.Data["duration_ms"] = strconv.FormatInt(task.CompletedAt.Sub(task.CreatedAt).Milliseconds(), 10)
	}
	if task.Usage != nil {
		e.Data["quantity"] = strconv.FormatFloat(task.Usage.Quantity, 'f', -1, 64)
	}
	return e
}

// Called only after a successful persistence operation; returning a snapshot cannot fail the operation.
func taskGameplayChanges(previous, task *Task) []gameplayEvent {
	if task == nil {
		return nil
	}
	var out []gameplayEvent
	add := func(kind string) { out = append(out, taskGameplayEvent(kind, task, "")) }
	if previous == nil {
		add("media.created")
	}
	if previous == nil || previous.Status != task.Status {
		switch task.Status {
		case StatusSubmitting:
			add("media.started")
		case StatusProcessing:
			if previous == nil || previous.Status == StatusQueued {
				add("media.started")
			}
		case StatusSucceeded:
			add("media.succeeded")
			if len(task.Files) > 0 {
				add("media.files_saved")
			}
		case StatusFailed:
			add("media.failed")
			if task.stagingFailed() {
				add("media.files_failed")
			}
		case StatusRecoveryRequired, StatusInterrupted:
			add("media.recovery_required")
		}
	}
	return out
}

func (t *Tracker) queueGameplay(e gameplayEvent) {
	if e.UserID <= 0 || !t.gameplayStarted.Load() || !t.gameplayConfigured.Load() {
		return
	}
	select {
	case <-t.ctx.Done():
		return
	default:
	}
	select {
	case t.gameplay <- e:
	default:
		t.gameplayDropped.Add(1)
	}
}

// StartGameplayEvents is paired with the server's other background delivery loops.
func (t *Tracker) StartGameplayEvents() {
	t.activeMu.Lock()
	defer t.activeMu.Unlock()
	if t.ctx.Err() != nil || !t.gameplayStarted.CompareAndSwap(false, true) {
		return
	}
	for i := 0; i < 2; i++ {
		t.wg.Add(1)
		go t.runGameplay()
	}
}

func (t *Tracker) runGameplay() {
	defer t.wg.Done()
	for {
		select {
		case <-t.ctx.Done():
			return
		case e := <-t.gameplay:
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.logger.Printf("[mediad] gameplay sender panic: %v", r)
					}
				}()
				raw, err := json.Marshal(e)
				if err != nil {
					return
				}
				for attempt := 0; attempt < 3; attempt++ {
					ctx, cancel := context.WithTimeout(t.ctx, 2*time.Second)
					_, _, err = t.deliverRaw(ctx, "gameplay.event", raw)
					cancel()
					if err == nil {
						return
					}
					if t.ctx.Err() != nil {
						return
					}
				}
				if n := t.gameplayFailed.Add(1); n == 1 || n%100 == 0 {
					t.logger.Printf("[mediad] gameplay delivery failed total=%d", n)
				}
			}()
		}
	}
}

type gameplayDownloadWriter struct {
	http.ResponseWriter
	status int
	failed bool
}

func (w *gameplayDownloadWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *gameplayDownloadWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	if err != nil {
		w.failed = true
	}
	return n, err
}

func (w *gameplayDownloadWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Keep net/http's optimized file-copy path while observing only its completion.
func (w *gameplayDownloadWriter) ReadFrom(r io.Reader) (int64, error) {
	if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		if w.status == 0 {
			w.status = http.StatusOK
		}
		n, err := readerFrom.ReadFrom(r)
		if err != nil {
			w.failed = true
		}
		return n, err
	}
	return io.Copy(struct{ io.Writer }{w}, r)
}
