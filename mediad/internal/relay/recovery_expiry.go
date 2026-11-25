package relay

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const recoveryLifetime = 30 * time.Minute

var errTaskExpired = errors.New("task verification window expired")

func (s *MemoryStore) ExpireRecovery(now time.Time, resultRoot string, logger *log.Logger) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var expired []string
	for id, prev := range s.tasks {
		if prev.ErrorCode == "recovery_expired" {
			if err := s.removeExpiredData(id, resultRoot); err != nil {
				return expired, err
			}
			continue
		}
		if prev.Status != StatusRecoveryRequired {
			continue
		}
		since := prev.CreatedAt
		if prev.RecoveryAt != nil {
			since = *prev.RecoveryAt
		}
		if now.Before(since.Add(recoveryLifetime)) {
			continue
		}
		record := map[string]any{"task_id": id, "user_id": prev.UserID, "key_id": prev.KeyID, "group_id": prev.GroupID, "model": prev.Model, "schema_id": prev.SchemaID, "previous_status": prev.Status, "reason": "verification exceeded 30 minutes", "previous_reason": expiryReason(prev, prev.ErrorMessage), "private_cause": expiryReason(prev, prev.PrivateCause), "upstream_status": prev.ResponseStatus, "inbound_endpoint": prev.InboundEndpoint, "upstream_endpoint": prev.UpstreamEndpoint, "created_at": prev.CreatedAt, "recovery_at": since, "expired_at": now}

		if s.root != "" {
			dir := filepath.Join(s.root, "expired-tasks")
			if err := os.MkdirAll(dir, 0700); err != nil {
				return expired, err
			}
			if err := WriteState(filepath.Join(dir, id+".json"), record); err != nil {
				return expired, err
			}
		}
		task := cloneTask(prev)
		task.Status = StatusFailed
		task.ErrorCode = "recovery_expired"
		task.ErrorMessage = "Task verification exceeded 30 minutes; saved media data has been removed."
		task.CompletedAt = &now
		task.Request = nil
		task.RequestData = nil
		task.Response = nil
		task.ResponseData = nil
		task.ResponseSummary = nil
		task.Result = nil
		task.ResultData = nil
		task.Files = nil
		task.FilesAcknowledged = true
		task.FilesDeletedAt = &now
		task.Usage = nil
		task.Credential = ""
		task.Schema = nil
		task.Parameters = nil
		task.DownloadToken = ""
		task.UpstreamURL = ""
		task.PrivateCause = ""
		task.ErrorArchive = "-"
		task.CallbackState = CallbackPending
		task.CallbackAttempts = 0
		task.LastCallbackError = ""
		if err := s.persist(task); err != nil {
			return expired, err
		}
		s.tasks[id] = task
		expired = append(expired, id)
		if logger != nil {
			logger.Printf("[mediad] recovery expired task=%s user=%s group=%s model=%s upstream_status=%d recovery_since=%s", id, prev.UserID, prev.GroupID, prev.Model, prev.ResponseStatus, since.UTC().Format(time.RFC3339))
		}
		if err := s.removeExpiredData(id, resultRoot); err != nil {
			return expired, err
		}
	}
	return expired, nil
}
func (s *MemoryStore) removeExpiredData(id, resultRoot string) error {
	if !safeID(id) {
		return fmt.Errorf("invalid task id")
	}
	var dirs []string
	if s.root != "" {
		dirs = append(dirs, s.payloadDir(id), filepath.Join(s.root, "upstream-errors", id))
	}
	if resultRoot != "" {
		dirs = append(dirs, filepath.Join(resultRoot, id))
	}
	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	return nil
}
func (t *Tracker) ExpireRecovery() {
	root := ""
	if t.stager != nil {
		root = t.stager.Root
	}
	t.activeMu.Lock()
	ids, err := t.store.ExpireRecovery(t.now(), root, t.logger)
	t.activeMu.Unlock()
	if err != nil {
		t.logger.Printf("[mediad] expire recovery: %v", err)
	}
	for _, id := range ids {
		if task, ok := t.store.Get(id); ok {
			t.Track(task)
		}
	}
}

func expiryReason(task *Task, v string) string {
	for _, secret := range []string{task.Credential, task.UpstreamURL} {
		if secret != "" {
			v = strings.ReplaceAll(v, secret, "[redacted]")
		}
	}
	if len(v) > 512 {
		v = v[:512]
	}
	return v
}
