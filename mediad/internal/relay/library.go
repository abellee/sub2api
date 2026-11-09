package relay

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"
)

type libraryRef struct {
	TaskID string `json:"task_id"`
	FileID string `json:"file_id"`
}

// These endpoints are called by the main site after checking its user records.
func (h *ManagementHandler) handleLibrary(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID int64        `json:"user_id"`
		Files  []libraryRef `json:"files"`
	}
	if err := decodeJSONBody(w, r, &in, 16384); err != nil || in.UserID <= 0 || len(in.Files) == 0 || len(in.Files) > 50 {
		writeError(w, 400, "bad_request", "provide an owner and 1 to 50 files")
		return
	}
	if h.tracker.stager == nil {
		writeError(w, 503, "storage_unavailable", "media storage unavailable")
		return
	}
	owner := strconv.FormatInt(in.UserID, 10)
	out := make([]map[string]any, 0, len(in.Files))
	for _, ref := range in.Files {
		item := map[string]any{"task_id": ref.TaskID, "file_id": ref.FileID, "available": false}
		task, ok := h.store.Get(ref.TaskID)
		if !ok {
			// An already collected task is also an idempotent deletion success.
			item["deleted"] = true
		} else if task.UserID != owner {
			item["deleted"] = false
		} else if r.URL.Path == "/management/files/delete" {
			err := h.store.deleteResultFile(ref.TaskID, ref.FileID, owner, h.tracker.stager.Root)
			item["deleted"] = err == nil
		} else if task.Status == StatusSucceeded && task.CallbackState == CallbackConfirmed && !task.FilesAcknowledged {
			for _, file := range task.Files {
				if file.ID == ref.FileID {
					_, err := os.Stat(filepath.Join(h.tracker.stager.Root, task.ID, file.ID))
					item["available"] = err == nil
					if h.Settings != nil && task.CompletedAt != nil {
						hours := h.Settings.FileRetentionHours()
						if hours > 0 && int64(hours) <= int64((1<<63-1)/time.Hour) {
							item["expires_at"] = task.CompletedAt.Add(time.Duration(hours) * time.Hour)
						}
					}
					break
				}
			}
		}
		out = append(out, item)
	}
	writeJSON(w, 200, map[string]any{"files": out})
}

func (s *MemoryStore) deleteResultFile(taskID, fileID, owner, root string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.tasks[taskID]
	if prev == nil || prev.UserID != owner || prev.Status != StatusSucceeded || prev.CallbackState != CallbackConfirmed {
		return fmt.Errorf("file is not ready or belongs to another user")
	}
	if root == "" || taskID == "." || taskID == ".." || filepath.Base(taskID) != taskID {
		return fmt.Errorf("invalid media path")
	}
	task := cloneTask(prev)
	index := slices.IndexFunc(task.Files, func(f ResultFile) bool { return f.ID == fileID })
	if index < 0 && !slices.Contains(task.DeletedFileIDs, fileID) && !task.FilesAcknowledged {
		return fmt.Errorf("unknown file")
	}
	if fileID == "" || fileID == "." || fileID == ".." || filepath.Base(fileID) != fileID {
		return fmt.Errorf("invalid media path")
	}
	for _, name := range []string{fileID, fileID + ".json"} {
		if err := os.Remove(filepath.Join(root, taskID, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if index >= 0 {
		task.Files = slices.Delete(task.Files, index, index+1)
	}
	if !slices.Contains(task.DeletedFileIDs, fileID) {
		task.DeletedFileIDs = append(task.DeletedFileIDs, fileID)
	}
	task.Response, task.Result = nil, nil
	if len(task.Files) == 0 {
		task.FilesAcknowledged = true
	}
	if err := s.persist(task); err != nil {
		return err
	}
	s.tasks[taskID] = task
	s.onDirty()
	return nil
}
