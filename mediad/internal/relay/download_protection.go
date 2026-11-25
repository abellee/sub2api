package relay

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const downloadRetryGrace = 2 * time.Minute

var errDownloadProtected = errors.New("media file is downloading or in its retry grace period")

type downloadProtection struct {
	active int
	until  time.Time
}

// Open and register under the deletion lock, before cleanup can unlink any file.
func (s *MemoryStore) openDownloads(root string, task *Task, metas []ResultFile) ([]*os.File, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.tasks[task.ID]
	if root == "" || !safeID(task.ID) || current == nil || current.Status != StatusSucceeded || current.CallbackState != CallbackConfirmed || current.FilesAcknowledged || current.UserID != task.UserID || current.KeyID != task.KeyID || current.DownloadToken != task.DownloadToken {
		return nil, nil, os.ErrNotExist
	}
	files := make([]*os.File, 0, len(metas))
	for _, meta := range metas {
		if !validFileID(meta.ID) || !slices.ContainsFunc(current.Files, func(f ResultFile) bool { return f.ID == meta.ID }) {
			for _, f := range files {
				f.Close()
			}
			return nil, nil, os.ErrNotExist
		}
		f, err := os.Open(filepath.Join(root, task.ID, meta.ID))
		if err != nil {
			for _, opened := range files {
				opened.Close()
			}
			return nil, nil, err
		}
		files = append(files, f)
	}
	if s.downloads[task.ID] == nil {
		s.downloads[task.ID] = make(map[string]*downloadProtection)
	}
	for _, meta := range metas {
		p := s.downloads[task.ID][meta.ID]
		if p == nil {
			p = &downloadProtection{}
			s.downloads[task.ID][meta.ID] = p
		}
		p.active++
	}
	return files, func() {
		for _, f := range files {
			f.Close()
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, meta := range metas {
			p := s.downloads[task.ID][meta.ID]
			p.active--
			if p.active == 0 {
				p.until = time.Now().Add(downloadRetryGrace)
			}
		}
	}, nil
}

// shortcut: retry grace is process-local; persist it if it must survive restarts.
// All protection helpers below require s.mu.
func (s *MemoryStore) fileDownloadProtected(taskID, fileID string) bool {
	p := s.downloads[taskID][fileID]
	return p != nil && (p.active > 0 || time.Now().Before(p.until))
}

func (s *MemoryStore) taskDownloadProtected(taskID string) bool {
	for fileID := range s.downloads[taskID] {
		if s.fileDownloadProtected(taskID, fileID) {
			return true
		}
	}
	return false
}

func (s *MemoryStore) pruneDownloads() {
	for taskID, files := range s.downloads {
		for fileID := range files {
			if !s.fileDownloadProtected(taskID, fileID) {
				delete(files, fileID)
			}
		}
		if len(files) == 0 {
			delete(s.downloads, taskID)
		}
	}
}
