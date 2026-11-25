package relay

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// GCImagesForDisk removes the oldest completed images until disk usage is at or below the threshold.
func (s *MemoryStore) GCImagesForDisk(threshold int, root string) (int, error) {
	return s.gcImagesForDisk(threshold, root, diskUsedPercent)
}

func (s *MemoryStore) gcImagesForDisk(threshold int, root string, usage func(string) (float64, error)) (int, error) {
	if threshold < 0 || threshold > 100 {
		return 0, fmt.Errorf("invalid disk usage threshold")
	}
	removed := 0
	// Finish durable deletion intents even if the setting changed or the service restarted.
	s.deletions.mu.Lock()
	var intents []FileDeletionEvent
	for _, notice := range s.deletions.items {
		if notice.Event.Reason == "disk_pressure" && !notice.Ready {
			intents = append(intents, notice.Event)
		}
	}
	s.deletions.mu.Unlock()
	for _, event := range intents {
		if err := s.deleteDiskImage(root, event); err != nil {
			return removed, err
		}
		removed++
	}
	if threshold == 0 {
		return removed, nil
	}
	overLimit := func() (bool, error) {
		used, err := usage(root)
		if err != nil {
			return false, err
		}
		if math.IsNaN(used) || math.IsInf(used, 0) || used < 0 || used > 100 {
			return false, fmt.Errorf("invalid measured disk usage")
		}
		return used > float64(threshold), nil
	}
	over, err := overLimit()
	if err != nil || !over {
		return removed, err
	}
	type candidate struct {
		id        string
		completed time.Time
	}
	var candidates []candidate
	s.mu.RLock()
	for id, task := range s.tasks {
		if diskCleanupEligible(task) && slices.ContainsFunc(task.Files, imageFile) {
			candidates = append(candidates, candidate{id, *task.CompletedAt})
		}
	}
	s.mu.RUnlock()
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].completed.Equal(candidates[j].completed) {
			return candidates[i].id < candidates[j].id
		}
		return candidates[i].completed.Before(candidates[j].completed)
	})
	for _, candidate := range candidates {
		for {
			// Recheck the actual filesystem after every image; never estimate space from file sizes.
			over, err := overLimit()
			if err != nil || !over {
				return removed, err
			}
			s.mu.RLock()
			task := s.tasks[candidate.id]
			var event FileDeletionEvent
			if diskCleanupEligible(task) {
				for _, file := range task.Files {
					if imageFile(file) {
						event = newDeletionEvent(task, []string{file.ID}, time.Now(), "disk_pressure")
						break
					}
				}
			}
			s.mu.RUnlock()
			if event.EventID == "" {
				break
			}
			if err := s.deleteDiskImage(root, event); err != nil {
				return removed, err
			}
			removed++
		}
	}
	return removed, nil
}

func diskCleanupEligible(task *Task) bool {
	return task != nil && task.Status == StatusSucceeded && task.CallbackState == CallbackConfirmed && task.CompletedAt != nil && !task.FilesAcknowledged && task.FilesDeletedAt == nil
}

func imageFile(file ResultFile) bool {
	return strings.HasPrefix(strings.ToLower(file.ContentType), "image/")
}

func (s *MemoryStore) deleteDiskImage(root string, event FileDeletionEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if root == "" || !validTaskID(event.TaskID) || len(event.FileIDs) != 1 || !validFileID(event.FileIDs[0]) {
		return fmt.Errorf("invalid disk cleanup path")
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("media storage root is unavailable")
	}
	dir := filepath.Join(root, event.TaskID)
	if info, err := os.Lstat(dir); err != nil && !os.IsNotExist(err) {
		return err
	} else if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return fmt.Errorf("invalid media task directory")
	}
	id := event.FileIDs[0]
	prev := s.tasks[event.TaskID]
	if prev == nil {
		// Task GC may run after metadata was saved but before the deletion notice was marked ready.
		if _, err := os.Lstat(filepath.Join(dir, id)); !os.IsNotExist(err) {
			return fmt.Errorf("missing task still has a media file")
		}
		return s.deletions.put(event, true)
	}
	if prev.Status != StatusSucceeded || prev.CallbackState != CallbackConfirmed || prev.UserID != event.UserID || prev.KeyID != event.KeyID {
		return fmt.Errorf("disk cleanup task eligibility changed")
	}
	index := slices.IndexFunc(prev.Files, func(file ResultFile) bool { return file.ID == id && imageFile(file) })
	if index < 0 && !slices.Contains(prev.DeletedFileIDs, id) {
		return fmt.Errorf("disk cleanup file is not an image")
	}
	if err := s.deletions.put(event, false); err != nil {
		return err
	}
	for _, name := range []string{id, id + ".json"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	task := cloneTask(prev)
	if index >= 0 {
		task.Files = slices.Delete(task.Files, index, index+1)
	}
	if !slices.Contains(task.DeletedFileIDs, id) {
		task.DeletedFileIDs = append(task.DeletedFileIDs, id)
	}
	task.Response, task.Result, task.ResponseData, task.ResultData, task.ResponseSummary = nil, nil, nil, nil, nil
	if len(task.Files) == 0 {
		task.FilesAcknowledged = true
		task.FilesDeletedAt = &event.DeletedAt
	}
	if err := s.persist(task); err != nil {
		return err
	}
	s.tasks[task.ID] = task
	s.onDirty()
	if len(task.Files) == 0 {
		_ = os.Remove(dir)
	}
	return s.deletions.put(event, true)
}
