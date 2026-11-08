package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"mediad/internal/config"
	"mediad/internal/signature"
)

// CallbackConfig 是回调注册配置(经管理接口写入)。
type CallbackConfig struct {
	URL         string `json:"callback_url"`
	Secret      string `json:"callback_secret"`
	Enabled     bool   `json:"enabled"`
	MaxAttempts int    `json:"max_attempts,omitempty"`         // 0 = 默认 10
	BackoffBase int    `json:"backoff_base_seconds,omitempty"` // 0 = 默认 2s,指数退避,上限 60s
}

// Tracker 负责异步任务的轮询与回调投递。
// 每个任务一个 goroutine;服务重启后由 ResumeAll 恢复仍在
// processing 的任务,保证"弃单也会被轮询到终态并回调"。
type Tracker struct {
	store            *MemoryStore
	schemas          func(id string) (*Schema, bool)
	client           *http.Client
	cbMu             sync.RWMutex
	cb               CallbackConfig
	ctx              context.Context
	cancel           context.CancelFunc
	now              func() time.Time
	logger           *log.Logger
	maxPoll          time.Duration // 全局轮询上限兜底
	interval         time.Duration // 全局轮询间隔兜底
	stager           *ResultStager
	configPath       string
	activeMu         sync.Mutex
	active           map[string]bool
	slots            chan struct{}
	wg               sync.WaitGroup
	createTimeout    time.Duration
	downloadAttempts int
	runtimeSettings  *config.SettingsStore
	settingsMu       sync.Mutex
	downloadBase     string
}

// ConfigureRuntime is called before serving requests or resuming tasks.
func (t *Tracker) ConfigureRuntime(concurrent, pollSeconds, maxPollSeconds, downloadSeconds, attempts int) {
	t.slots = make(chan struct{}, concurrent)
	t.interval = time.Duration(pollSeconds) * time.Second
	t.maxPoll = time.Duration(maxPollSeconds) * time.Second
	t.downloadAttempts = attempts
	if t.stager != nil {
		t.stager.Client.Timeout = time.Duration(downloadSeconds) * time.Second
	}
}

// NewTracker 创建轮询器。
func NewTracker(store *MemoryStore, schemas func(id string) (*Schema, bool), client *http.Client, logger *log.Logger) *Tracker {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if logger == nil {
		logger = log.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Tracker{
		store:    store,
		schemas:  schemas,
		client:   client,
		ctx:      ctx,
		cancel:   cancel,
		now:      time.Now,
		logger:   logger,
		maxPoll:  30 * time.Minute,
		interval: 5 * time.Second,
		active:   map[string]bool{}, slots: make(chan struct{}, 8),
	}
}

// Stop 停止全部轮询(进程退出时调用)。
func (t *Tracker) Stop() { t.activeMu.Lock(); t.cancel(); t.activeMu.Unlock(); t.wg.Wait() }

func (t *Tracker) SetCreateTimeout(timeout time.Duration) { t.createTimeout = timeout }

func (t *Tracker) ConfigureDownloads(base string) { t.downloadBase = base }

// SetCallback 更新回调注册配置。
func (t *Tracker) SetCallback(cfg CallbackConfig) error {
	t.cbMu.Lock()
	defer t.cbMu.Unlock()
	if t.configPath != "" {
		if err := WriteState(t.configPath, cfg); err != nil {
			return err
		}
	}
	t.cb = cfg
	return nil
}

func (t *Tracker) ConfigureFiles(root string, maxBytes int64) {
	t.stager = &ResultStager{Root: root, MaxBytes: maxBytes, Client: &http.Client{Timeout: 120 * time.Second}}
}

func (t *Tracker) LoadCallback(path string) error {
	t.configPath = path
	buf, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(buf, &t.cb)
}

// CallbackSnapshot 返回当前回调配置(脱敏:不含 secret)。
func (t *Tracker) CallbackSnapshot() CallbackConfig {
	t.cbMu.RLock()
	defer t.cbMu.RUnlock()
	cfg := t.cb
	cfg.Secret = ""
	return cfg
}

// Track 启动一个任务的轮询 goroutine。
func (t *Tracker) Track(task *Task) {
	t.activeMu.Lock()
	defer t.activeMu.Unlock()
	if t.ctx.Err() != nil || t.active[task.ID] {
		return
	}
	t.active[task.ID] = true
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer func() { t.activeMu.Lock(); delete(t.active, task.ID); t.activeMu.Unlock() }()
		select {
		case t.slots <- struct{}{}:
		case <-t.ctx.Done():
			return
		}
		defer func() { <-t.slots }()
		t.run(task.ID)
	}()
}

// ResumeAll 恢复全部 processing 任务(启动时调用)。
func (t *Tracker) ResumeAll() int {
	n := 0
	for _, task := range t.store.ListAll() {
		if task.Status == StatusRecoveryRequired {
			continue
		}
		if task.CallbackState == CallbackConfirmed || task.CallbackState == CallbackExhausted {
			continue
		}
		t.Track(task)
		n++
	}
	return n
}

func (t *Tracker) trackLoop(taskID string) {
	deadlineDefault := t.maxPoll
	for {
		if t.ctx.Err() != nil {
			return
		}
		task, ok := t.store.Get(taskID)
		if !ok || task.Status != StatusProcessing {
			return
		}
		schema, ok := t.taskSchema(task)
		if !ok {
			t.recovery(task, "schema_missing: cannot resume task")
			return
		}

		interval := t.interval
		deadline := deadlineDefault
		if q := schema.Upstream.Query; q != nil {
			if q.PollIntervalSeconds > 0 {
				interval = time.Duration(q.PollIntervalSeconds) * time.Second
			}
			if q.MaxPollSeconds > 0 {
				deadline = time.Duration(q.MaxPollSeconds) * time.Second
			}
		}
		started := task.PollStartedAt
		if started.IsZero() {
			started = task.CreatedAt
		}
		status, body, err := t.pollOnce(task, schema)
		if err == nil {
			switch ClassifyStatus(schema, status) {
			case "succeeded":
				usage, _, uerr := extractTaskUsage(schema, task, body)
				var usageOut *Usage
				if uerr == nil {
					usageOut = &Usage{Quantity: usage, Unit: usageUnit(schema)}
				}
				task.Response = append([]byte(nil), body...)
				t.complete(task, StatusSucceeded, usageOut, ExtractResult(schema, body), "")
				return
			case "failed":
				t.complete(task, StatusFailed, nil, ExtractResult(schema, body),
					firstNonEmpty(ExtractErrorMessage(schema, body), "upstream task failed"))
				return
			default:
				// 仍在进行,继续轮询。
			}
		}
		// 停机可能超过原等待上限，恢复时先查一次，不能丢掉已完成的结果。
		if time.Since(started) > deadline {
			t.recovery(task, "poll_timeout: result is unknown; resume query after checking upstream")
			return
		}
		// 查询失败视为瞬时问题,持续重试直到 deadline,避免把上游抖动误判成失败。

		sleep := interval + time.Duration(rand.Int63n(int64(time.Second))) // 抖动,避免整点齐步
		select {
		case <-t.ctx.Done():
			return
		case <-time.After(sleep):
		}
	}
}

// pollOnce 查询一次上游任务状态,返回(状态串, 响应体, 错误)。
func (t *Tracker) pollOnce(task *Task, schema *Schema) (string, []byte, error) {
	q := schema.Upstream.Query
	if q == nil {
		return "", nil, fmt.Errorf("schema %q has no query spec", task.SchemaID)
	}
	endpoint := BuildQueryURL(task.UpstreamURL, q.Path, task.UpstreamTaskID)

	req, err := http.NewRequestWithContext(t.ctx, strings.ToUpper(firstNonEmpty(q.Method, http.MethodGet)), endpoint, nil)
	if err != nil {
		return "", nil, err
	}
	for name, tmpl := range schema.Request.Credential {
		req.Header.Set(name, renderTaskTemplate(tmpl, task))
	}
	for name, tmpl := range schema.Request.HeaderParams {
		req.Header.Set(name, renderTaskTemplate(tmpl, task))
	}
	timeout := 30 * time.Second
	if q.TimeoutSeconds > 0 {
		timeout = time.Duration(q.TimeoutSeconds) * time.Second
	}
	ctx, cancel := context.WithTimeout(t.ctx, timeout)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return "", nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", body, fmt.Errorf("upstream query status %d", resp.StatusCode)
	}
	status, _, err := ExtractStatus(schema, body)
	if err != nil {
		return "", body, err
	}
	return status, body, nil
}

// complete 落终态并投递回调。usage 传 nil 表示失败/不计费。
func (t *Tracker) complete(task *Task, status string, usage *Usage, result map[string]any, errMsg string) {
	if t.ctx.Err() != nil {
		return
	}
	now := t.now()
	task.Status = status
	task.Usage = usage
	if result != nil {
		task.Result = result
	}
	task.ErrorMessage = errMsg
	task.CompletedAt = &now
	task.CallbackState = CallbackPending
	if status == StatusFailed && len(task.Files) == 0 {
		task.FilesAcknowledged = true
	}
	if status == StatusSucceeded && t.stager != nil {
		task.Status = StatusDownloading
		if !t.saveResult(task) {
			return
		}
		t.finishFiles(task)
		return
	}
	if !t.saveResult(task) {
		return
	}
	t.deliver(task)
}

// deliver 按 CallbackConfig 投递回调,指数退避重试直到确认或耗尽。
// 任何 2xx 均视为 sub2api 已确认(含 already_settled)。
func (t *Tracker) deliver(task *Task) {
	// Older completed jobs can be repaired without submitting generation again.
	if task.Status == StatusSucceeded && task.Usage == nil {
		if schema, ok := t.taskSchema(task); ok {
			if quantity, unit, err := extractTaskUsage(schema, task, task.Response); err == nil && quantity > 0 {
				task.Usage = &Usage{Quantity: quantity, Unit: unit}
				if !t.saveResult(task) {
					return
				}
			}
		}
	}
	if task.Status == StatusSucceeded && len(task.Files) > 0 {
		if err := t.prepareDownloadLinks(task); err != nil {
			t.logger.Printf("prepare result download links: %v", err)
			return
		}
	}
	t.cbMu.RLock()
	cfg := t.cb
	t.cbMu.RUnlock()

	if !cfg.Enabled || cfg.URL == "" {
		task.CallbackState = CallbackDisabled
		task.LastCallbackError = "callback not configured or disabled"
		_ = t.store.Update(task)
		return
	}
	maxAttempts := cfg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 10
	}
	base := time.Duration(cfg.BackoffBase) * time.Second
	if base <= 0 {
		base = 2 * time.Second
	}

	payload, err := BuildCallbackPayload(task)
	if err != nil {
		t.logger.Printf("[mediad] build callback payload for %s: %v", task.ID, err)
		return
	}
	u, err := url.Parse(cfg.URL)
	if err != nil {
		t.logger.Printf("[mediad] bad callback url: %v", err)
		return
	}

	var lastErr string
	for attempt := task.CallbackAttempts + 1; attempt <= maxAttempts; attempt++ {
		if t.ctx.Err() != nil {
			return
		}
		ts := fmt.Sprintf("%d", t.now().Unix())
		sig := signature.Sign(cfg.Secret, ts, http.MethodPost, u.Path, payload)

		ctx, cancel := context.WithTimeout(t.ctx, 10*time.Minute)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(payload))
		if err != nil {
			cancel()
			lastErr = err.Error()
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Media-Timestamp", ts)
		req.Header.Set("X-Media-Signature", sig)
		req.Header.Set("X-Media-Event", "task.completed")

		resp, err := t.client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				now := t.now()
				task.CallbackState = CallbackConfirmed
				task.CallbackAttempts = attempt
				task.LastCallbackAt = &now
				task.LastCallbackError = ""
				if err := t.store.Update(task); err != nil {
					t.logger.Printf("[mediad] persist notification confirmation %s: %v", task.ID, err)
				}
				cancel()
				return
			}
			lastErr = fmt.Sprintf("callback status %d", resp.StatusCode)
		} else {
			lastErr = err.Error()
		}

		cancel()
		task.CallbackAttempts = attempt
		task.LastCallbackError = lastErr
		_ = t.store.Update(task)

		backoff := base
		for i := 1; i < attempt && backoff < 60*time.Second; i++ {
			backoff *= 2
		}
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}
		select {
		case <-t.ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
	now := t.now()
	task.CallbackState = CallbackExhausted
	task.LastCallbackAt = &now
	_ = t.store.Update(task)
	t.logger.Printf("[mediad] callback exhausted for task %s after %d attempts: %s", task.ID, maxAttempts, lastErr)
}

// DeliverRaw 向当前注册的回调地址投递一条签名 payload(测试回调用)。
// 返回上游 HTTP 状态码与错误;任何错误都不重试(测试语义)。
func (t *Tracker) DeliverRaw(event string, payload []byte) (int, error) {
	t.cbMu.RLock()
	cfg := t.cb
	t.cbMu.RUnlock()
	if !cfg.Enabled || cfg.URL == "" {
		return 0, fmt.Errorf("callback not configured or disabled")
	}
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return 0, fmt.Errorf("bad callback url: %w", err)
	}
	ts := fmt.Sprintf("%d", t.now().Unix())
	req, err := http.NewRequestWithContext(t.ctx, http.MethodPost, cfg.URL, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Media-Timestamp", ts)
	req.Header.Set("X-Media-Signature", signature.Sign(cfg.Secret, ts, http.MethodPost, u.Path, payload))
	req.Header.Set("X-Media-Event", event)

	resp, err := t.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("callback status %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

// CallbackEventTaskCompleted 是回调事件名。
const CallbackEventTaskCompleted = "task.completed"

// BuildCallbackPayload 组装终态回调 body(sub2api 侧据此结算)。
func BuildCallbackPayload(task *Task) ([]byte, error) {
	result := task.Result
	if len(task.Files) > 0 {
		// 图片本体已经暂存；通知不携带大段 base64 或临时下载地址。
		result = map[string]any{}
		for _, k := range []string{"duration", "resolution", "size"} {
			if v, ok := task.Result[k]; ok {
				result[k] = v
			}
		}
	}
	payload := map[string]any{
		"event":            CallbackEventTaskCompleted,
		"task_id":          task.ID,
		"upstream_task_id": task.UpstreamTaskID,
		"schema_id":        task.SchemaID,
		"model":            task.Model,
		"user_id":          task.UserID,
		"key_id":           task.KeyID,
		"group_id":         task.GroupID,
		"status":           task.Status,
		"files":            task.Files,
		"parameters":       task.Parameters,
		"result":           result,
		"error_message":    task.ErrorMessage,
		"created_at":       task.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	if task.CompletedAt != nil {
		payload["completed_at"] = task.CompletedAt.UTC().Format(time.RFC3339Nano)
	}
	if task.Usage != nil {
		payload["usage"] = map[string]any{"quantity": task.Usage.Quantity, "unit": task.Usage.Unit}
	} else {
		payload["usage"] = nil
	}
	return json.Marshal(payload)
}

// renderTaskTemplate 以任务上下文渲染凭证/头模板。
func renderTaskTemplate(tmpl string, task *Task) string {
	return renderTemplate(tmpl, AssembleInput{
		Credential: task.Credential,
		UserID:     task.UserID,
		KeyID:      task.KeyID,
		GroupID:    task.GroupID,
		Model:      task.Model,
	})
}

// usageUnit 返回 schema 的计费单位(缺省 image)。
func usageUnit(schema *Schema) string {
	if schema.Response.Usage.Unit == "" {
		return UnitImage
	}
	return schema.Response.Usage.Unit
}
