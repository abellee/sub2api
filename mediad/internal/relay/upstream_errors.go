package relay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Private diagnostics are separate from task snapshots: callbacks, media cleanup and task GC must not erase them.
type upstreamErrorRecord struct {
	TaskID        string      `json:"task_id"`
	UserID        string      `json:"user_id"`
	GroupID       string      `json:"group_id,omitempty"`
	Model         string      `json:"model,omitempty"`
	Phase         string      `json:"phase"`
	RecordedAt    time.Time   `json:"recorded_at"`
	Method        string      `json:"method,omitempty"`
	URL           string      `json:"url,omitempty"`
	StatusCode    int         `json:"status_code"`
	Status        string      `json:"status"`
	Header        http.Header `json:"headers"`
	Trailer       http.Header `json:"trailers,omitempty"`
	ContentLength int64       `json:"content_length"`
	Protocol      string      `json:"protocol"`
	BodyFile      string      `json:"body_file"`
	BodyBytes     int64       `json:"body_bytes"`
	ReadError     string      `json:"read_error,omitempty"`
	StorageError  string      `json:"storage_error,omitempty"`
}

// Stream the complete error body to disk, including non-JSON and large responses, without keeping it in memory.
func (s *MemoryStore) recordUpstreamError(task *Task, phase string, resp *http.Response, body io.Reader, initialReadErr error) (string, error) {
	if s == nil || s.root == "" {
		return "", nil // In-memory stores are used by tests; production uses NewDiskStore.
	}
	if task.ID == "" || task.ID == "." || task.ID == ".." || filepath.Base(task.ID) != task.ID {
		return "", fmt.Errorf("invalid task id")
	}
	dir := filepath.Join(s.root, "upstream-errors", task.ID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "response-*.body")
	if err != nil {
		return "", err
	}
	name := f.Name()
	record := upstreamErrorRecord{TaskID: task.ID, UserID: task.UserID, GroupID: task.GroupID, Model: task.Model,
		Phase: phase, RecordedAt: time.Now().UTC(), StatusCode: resp.StatusCode, Status: resp.Status,
		Header: resp.Header.Clone(), ContentLength: resp.ContentLength, Protocol: resp.Proto, BodyFile: filepath.Base(name)}
	if resp.Request != nil {
		record.Method = resp.Request.Method
		if resp.Request.URL != nil {
			u := *resp.Request.URL
			u.User, u.RawQuery, u.Fragment, u.ForceQuery = nil, "", "", false
			record.URL = u.String()
		}
	}
	n, readErr := io.Copy(f, body)
	record.BodyBytes = n
	record.Trailer = resp.Trailer.Clone()
	readErr = errors.Join(initialReadErr, readErr)
	if readErr != nil {
		record.ReadError = readErr.Error()
	}
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(syncErr, closeErr); err != nil {
		record.StorageError = err.Error()
	}
	if err := WriteState(name+".json", record); err != nil {
		return name, err
	}
	for _, err := range []error{readErr, syncErr, closeErr} {
		if err != nil {
			return name, err
		}
	}
	return name, nil
}

func (t *Tracker) readUpstreamResponse(task *Task, phase string, resp *http.Response, limit int64) ([]byte, error) {
	body := io.Reader(resp.Body)
	httpError := resp.StatusCode < 200 || resp.StatusCode >= 300
	if httpError {
		path, err := t.store.recordUpstreamError(task, phase, resp, body, nil)
		if err != nil {
			t.logger.Printf("[mediad] persist upstream error %s (%s): %v", task.ID, phase, err)
			if path != "" {
				return nil, err
			}
		}
		if path != "" {
			f, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			body = f
		}
	}
	raw, readErr := io.ReadAll(io.LimitReader(body, limit+1))
	tooLarge := int64(len(raw)) > limit
	archive := readErr != nil || tooLarge
	if schema, ok := t.taskSchema(task); ok && !httpError && !archive {
		archive = !json.Valid(raw) || ExtractErrorMessage(schema, raw) != ""
		var envelope struct {
			Error json.RawMessage `json:"error"`
		}
		if json.Unmarshal(raw, &envelope) == nil && len(envelope.Error) > 0 && string(envelope.Error) != "null" {
			archive = true
		}
		if phase == "query" {
			status, _, err := ExtractStatus(schema, raw)
			archive = archive || err != nil || ClassifyStatus(schema, status) == "failed"
		} else if schema.Mode != "sync" {
			_, err := ExtractTaskID(schema, raw)
			archive = archive || err != nil
		}
	}
	if !httpError && archive {
		// Continue writing past the parser's limit without accumulating a large response in memory.
		if _, err := t.store.recordUpstreamError(task, phase, resp, io.MultiReader(bytes.NewReader(raw), body), readErr); err != nil {
			t.logger.Printf("[mediad] persist upstream error %s (%s): %v", task.ID, phase, err)
		}
	}
	if readErr == nil && tooLarge {
		readErr = fmt.Errorf("upstream response exceeds %d bytes", limit)
	}
	return raw, readErr
}
