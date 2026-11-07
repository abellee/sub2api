package relay

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

func (t *Tracker) taskSchema(task *Task) (*Schema, bool) {
	if task.Schema != nil {
		return task.Schema, true
	}
	return t.schemas(task.SchemaID)
}

func (t *Tracker) recovery(task *Task, reason string) {
	task.Status = StatusRecoveryRequired
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
	schema, ok := t.taskSchema(task)
	if !ok {
		t.recovery(task, "schema_missing")
		return
	}
	switch task.Status {
	case StatusQueued:
		t.submit(task, schema)
	case StatusSubmitting:
		if len(task.Response) > 0 {
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
	r, err := http.NewRequestWithContext(ctx, task.Request.Method, task.Request.URL, bytes.NewReader(task.Request.Body))
	if err != nil {
		t.recovery(task, "invalid saved request")
		return
	}
	r.Header = task.Request.Header.Clone()
	if name := schema.Request.IdempotencyHeader; name != "" {
		r.Header.Set(name, task.ID)
	}
	resp, err := t.client.Do(r)
	if err != nil {
		// 不把断线当成生成失败：上游可能已接受并产生费用。
		if t.ctx.Err() == nil {
			t.recovery(task, "upstream response unavailable; verify upstream before retry")
		}
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20+1))
	if err != nil || len(body) > 64<<20 {
		if t.ctx.Err() == nil {
			t.recovery(task, "upstream response incomplete; verify upstream before retry")
		}
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
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
	if schema.Mode == "sync" {
		quantity, unit, err := ExtractUsage(schema, task.Response)
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
	if !t.saveResult(task) {
		return
	}
	t.trackLoop(task.ID)
}

func (t *Tracker) finishFiles(task *Task) {
	if t.stager == nil {
		return
	}
	attempts := t.downloadAttempts
	if attempts <= 0 {
		attempts = 3
	}
	for attempt := 0; attempt < attempts; attempt++ {
		files, err := t.stager.Stage(t.ctx, task)
		if t.ctx.Err() != nil {
			return
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
		if schema, ok := t.taskSchema(task); ok && task.UpstreamTaskID != "" {
			if status, body, qerr := t.pollOnce(task, schema); qerr == nil && schema.IsTerminalSuccess(status) {
				task.Result = ExtractResult(schema, body)
				if err := t.store.Update(task); err != nil {
					return
				}
			}
		}
		select {
		case <-t.ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
	// 保持 downloading，管理端重试或重启可继续；不宣告成功、不丢文件。
}
