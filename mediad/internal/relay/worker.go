package relay

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

const resultStagingTimeout = 10 * time.Minute

func (t *Tracker) taskSchema(task *Task) (*Schema, bool) {
	if task.Schema != nil {
		return task.Schema, true
	}
	return t.schemas(task.SchemaID)
}

// Drop the URL (it may contain provider credentials), retain the transport failure.
func upstreamCause(err error) string {
	var u *url.Error
	if errors.As(err, &u) {
		err = u.Err
	}
	return err.Error()
}

func (t *Tracker) recovery(task *Task, reason string) {
	task.Status = StatusRecoveryRequired
	if task.RecoveryAt == nil {
		now := t.now()
		task.RecoveryAt = &now
	}
	task.ErrorMessage = reason
	if err := t.store.Update(task); err != nil {
		t.logger.Printf("[mediad] persist recovery status %s: %v", task.ID, err)
	}
}

// 上游结果已返回时，短暂写盘失败仍留在工作协程重试，避免仅记录日志后丢掉响应。
func (t *Tracker) saveResult(task *Task) bool {
	logged := false
	for {
		if err := t.store.Update(task); err == nil {
			return true
		} else if errors.Is(err, errTaskExpired) {
			return false
		} else if !logged {
			t.logger.Printf("[mediad] waiting for result storage %s: %v", task.ID, err)
			logged = true
		}
		select {
		case <-t.ctx.Done():
			return false
		case <-time.After(time.Second):
		}
	}
}

func (t *Tracker) run(id string) {
	task, ok := t.store.Get(id)
	if !ok {
		return
	}
	if task.Status == StatusFailed || task.Status == StatusSucceeded {
		if task.CallbackState != CallbackConfirmed {
			t.deliver(task)
		}
		return
	}
	schema, ok := t.taskSchema(task)
	if !ok {
		t.recovery(task, "schema_missing")
		return
	}
	switch task.Status {
	case StatusQueued:
		t.submit(task, schema)
	case StatusSubmitting:
		if len(task.Response) > 0 || task.ResponseData != nil {
			t.consumeResponse(task, schema)
			return
		}
		if schema.Request.IdempotencyHeader != "" {
			t.submit(task, schema)
		} else {
			t.recovery(task, "upstream response lost; attach upstream task ID or response; unsafe to submit again")
		}
	case StatusProcessing:
		t.trackLoop(id)
	case StatusDownloading:
		t.finishFiles(task)
	case StatusSucceeded, StatusFailed:
		if task.CallbackState != CallbackConfirmed {
			t.deliver(task)
		}
	}
}

func (t *Tracker) submit(task *Task, schema *Schema) {
	if task.Request == nil {
		t.recovery(task, "missing saved upstream request")
		return
	}
	task.ErrorArchive, task.PrivateCause, task.ResponseStatus = "-", "", 0
	task.Status = StatusSubmitting
	if err := t.store.Update(task); err != nil {
		t.logger.Printf("[mediad] persist before submit %s: %v", task.ID, err)
		return
	}
	timeout := time.Duration(schema.Upstream.Create.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = t.createTimeout
	}
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(t.ctx, timeout)
	defer cancel()
	bodyReader, bodySize, err := t.store.requestReader(task)
	if err != nil {
		t.recovery(task, "saved request body unavailable")
		return
	}
	defer bodyReader.Close()
	r, err := http.NewRequestWithContext(ctx, task.Request.Method, task.Request.URL, bodyReader)
	if err != nil {
		task.PrivateCause = "invalid saved upstream request: " + upstreamCause(err)
		t.recovery(task, "invalid saved request")
		return
	}
	r.ContentLength = bodySize
	r.Header = task.Request.Header.Clone()
	if name := schema.Request.IdempotencyHeader; name != "" {
		r.Header.Set(name, task.ID)
	}
	resp, err := t.client.Do(r)
	if err != nil {
		// 不把断线当成生成失败：上游可能已接受并产生费用。
		if t.ctx.Err() == nil {
			task.PrivateCause = "submitting upstream request: " + upstreamCause(err)
			t.recovery(task, "upstream response unavailable; verify upstream before retry")
		}
		return
	}
	defer resp.Body.Close()
	body, err := t.readUpstreamResponse(task, "create", resp, 64<<20)
	if err != nil || len(body) > 64<<20 {
		if t.ctx.Err() == nil {
			if err != nil {
				task.PrivateCause = "reading upstream response: " + upstreamCause(err)
			}
			t.recovery(task, "upstream response incomplete; verify upstream before retry")
		}
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		task.ResponseStatus = resp.StatusCode
		task.Response = body
		if resp.StatusCode >= 500 || resp.StatusCode == 408 {
			t.recovery(task, fmt.Sprintf("upstream HTTP %d; acceptance unknown", resp.StatusCode))
		} else {
			t.complete(task, StatusFailed, nil, nil, fmt.Sprintf("upstream HTTP %d", resp.StatusCode))
		}
		return
	}
	// 先保存原响应，提取/下载期间崩溃无需再次生成。
	task.Response = body
	if !t.saveResult(task) {
		return
	}
	t.consumeResponse(task, schema)
}

func (t *Tracker) consumeResponse(task *Task, schema *Schema) {
	if err := t.loadSavedResults(t.ctx, task, true, false); err != nil {
		t.recovery(task, "saved response unavailable")
		return
	}
	if schema.Mode == "sync" {
		quantity, unit, err := ExtractUsage(schema, task.Response)
		if err == nil && quantity == 0 && schema.Response.Usage.Quantity == "$result_files" {
			task.ResponseStatus = 200
			t.complete(task, StatusFailed, nil, nil, "upstream returned no images")
			return
		}
		var usage *Usage
		if err == nil {
			usage = &Usage{Quantity: quantity, Unit: unit}
		}
		t.complete(task, StatusSucceeded, usage, ExtractResult(schema, task.Response), "")
		return
	}
	id, err := ExtractTaskID(schema, task.Response)
	if err != nil {
		t.recovery(task, "cannot extract upstream task ID; saved response needs recovery")
		return
	}
	task.UpstreamTaskID = id
	task.Status = StatusProcessing
	task.PollStartedAt = time.Now()
	task.Request = nil
	task.RequestData = nil
	if !t.saveResult(task) {
		return
	}
	t.trackLoop(task.ID)
}

func (t *Tracker) finishFiles(task *Task) {
	if t.stager == nil {
		return
	}
	// CompletedAt records the upstream result time, so restart cannot reset this limit.
	if task.CompletedAt == nil {
		now := t.now()
		task.CompletedAt = &now
		if !t.saveResult(task) {
			return
		}
	}
	ctx, cancel := context.WithDeadline(t.ctx, task.CompletedAt.Add(resultStagingTimeout))
	defer cancel()
	attempts := t.downloadAttempts
	if attempts <= 0 {
		attempts = 3
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if ctx.Err() != nil {
			break
		}
		task.ErrorArchive = "-"
		if task.ResponseData != nil {
			task.Response = nil
		}
		if err := t.loadSavedResults(ctx, task, false, true); err != nil {
			t.recovery(task, "saved result unavailable")
			return
		}
		files, err := t.stager.Stage(ctx, task)
		if err != nil {
			task.PrivateCause = "retrieving result files: " + upstreamCause(err)
		}
		if t.ctx.Err() != nil {
			return
		}
		if ctx.Err() != nil {
			break
		}
		if err == nil {
			task.Files = files
			task.Status = StatusSucceeded
			task.ErrorMessage = ""
			if !t.saveResult(task) {
				return
			}
			t.deliver(task)
			return
		}
		task.ErrorMessage = "result staging: " + err.Error()
		if err := t.store.Update(task); err != nil {
			return
		}
		// 可查询的上游重新读取结果，刷新可能过期的下载地址。
		if schema, ok := t.taskSchema(task); ok && task.UpstreamTaskID != "" && attempt+1 < attempts {
			archive := task.ErrorArchive
			status, body, qerr := t.pollOnceContext(ctx, task, schema)
			task.ErrorArchive = archive
			if qerr == nil && schema.IsTerminalSuccess(status) {
				task.Result = ExtractResult(schema, body)
				if err := t.store.Update(task); err != nil {
					return
				}
			}
		}
		if attempt+1 == attempts {
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
	}
	if t.ctx.Err() != nil {
		return
	}
	task.ErrorCode = "result_staging_failed"
	message := "Media result could not be retrieved after retries. Stop polling and contact support with the task ID."
	if ctx.Err() != nil {
		task.ErrorCode = "result_staging_timeout"
		message = "Media result retrieval exceeded 10 minutes. Stop polling and contact support with the task ID."
	}
	t.complete(task, StatusFailed, nil, nil, message)
}

func (t *Tracker) loadSavedResults(ctx context.Context, task *Task, response, result bool) error {
	logged := false
	for {
		err := t.store.loadPayloadFields(task, response, result)
		if err == nil {
			return nil
		}
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			return err
		}
		if !logged {
			t.logger.Printf("[mediad] waiting for saved payload task=%s: %v", task.ID, err)
			logged = true
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
