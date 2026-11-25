package relay

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Receive through a small buffer first. Provider latency and network transfer
// must not hold an image-sized reservation or require trusting Content-Length.
func (t *Tracker) bufferedResponse(ctx context.Context, task *Task, phase string, resp *http.Response, limit int64) ([]byte, func(), error) {
	noop := func() {}
	f, err := os.CreateTemp(t.store.root, ".response-*")
	if err != nil {
		return nil, noop, err
	}
	defer func() { f.Close(); os.Remove(f.Name()) }()
	n, readErr := io.Copy(f, io.LimitReader(resp.Body, limit+1))
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, noop, err
	}
	if readErr != nil || n > limit {
		_, archiveErr := t.store.recordUpstreamError(task, phase, resp, io.MultiReader(f, resp.Body), readErr)
		if archiveErr != nil {
			t.logger.Printf("[mediad] persist upstream error %s (%s): %v", task.ID, phase, archiveErr)
		}
		if readErr != nil {
			return nil, noop, readErr
		}
		return nil, noop, fmt.Errorf("upstream response exceeds %d bytes", limit)
	}
	// Close the network body before waiting for RAM; the complete result is on disk.
	resp.Body.Close()
	if phase == "create" && resp.StatusCode >= 200 && resp.StatusCode < 300 && t.store.root != "" {
		ref, err := t.store.savePayload(task.ID, "response", func(w io.Writer) error {
			if _, err := io.WriteString(w, "\""); err != nil {
				return err
			}
			enc := base64.NewEncoder(base64.StdEncoding, w)
			if _, err := io.Copy(enc, f); err != nil {
				return err
			}
			if err := enc.Close(); err != nil {
				return err
			}
			_, err := io.WriteString(w, "\"")
			return err
		})
		if err != nil {
			return nil, noop, err
		}
		task.ResponseData = ref
		task.ResponseStatus = resp.StatusCode
		if !t.saveResult(task) {
			return nil, noop, ctx.Err()
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, noop, err
		}
	}
	release, err := mediaMemory.wait(ctx, n*8+(32<<20))
	if err != nil {
		return nil, noop, err
	}
	copy := *resp
	copy.Body = io.NopCloser(f)
	body, err := t.readUpstreamResponse(task, phase, &copy, limit)
	return body, release, err
}

func reserveSavedResults(ctx context.Context, task *Task, response bool) (func(), error) {
	var n int64
	if response {
		if task.ResponseData != nil {
			n = task.ResponseData.Size
		} else {
			n = int64(len(task.Response))
		}
	} else if task.ResultData != nil {
		n = task.ResultData.Size
	} else if len(task.Result) > 0 {
		// In-memory stores are test-only; production results always have a disk reference.
		n = 64 << 20
	}
	if n > 128<<20 {
		return nil, fmt.Errorf("saved response exceeds memory-safe decode limit")
	}
	return mediaMemory.wait(ctx, n*8+(32<<20))
}
