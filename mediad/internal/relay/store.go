package relay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// Task 是异步任务在媒体服务侧的注册记录。
// 生命周期:创建(processing)→ 轮询至终态(succeeded/failed)
// → 回调 sub2api(confirmed)→ TTL 回收。
// interrupted 表示持久化恢复时缺少凭证、无法继续轮询的任务。
type Task struct {
	DeletedFileIDs    []string         `json:"deleted_file_ids,omitempty"`
	DownloadToken     string           `json:"download_token,omitempty"`
	Schema            *Schema          `json:"schema,omitempty"`
	Request           *UpstreamRequest `json:"request,omitempty"`
	Response          []byte           `json:"response,omitempty"`
	ResponseStatus    int              `json:"response_status,omitempty"`
	Parameters        map[string]any   `json:"parameters,omitempty"`
	Files             []ResultFile     `json:"files,omitempty"`
	FilesAcknowledged bool             `json:"files_acknowledged"`
	Fingerprint       string           `json:"fingerprint,omitempty"`
	PollStartedAt     time.Time        `json:"poll_started_at,omitempty"`
	ID                string           `json:"id"`
	UpstreamTaskID    string           `json:"upstream_task_id"`
	SchemaID          string           `json:"schema_id"`
	SchemaVersion     int              `json:"schema_version"`
	Model             string           `json:"model,omitempty"`
	UserID            string           `json:"user_id"`
	KeyID             string           `json:"key_id"`
	GroupID           string           `json:"group_id,omitempty"`
	UpstreamURL       string           `json:"upstream_url"`
	Credential        string           `json:"credential,omitempty"` // 仅内存/按配置持久化;永不写日志
	Status            string           `json:"status"`               // processing|succeeded|failed|interrupted
	Usage             *Usage           `json:"usage,omitempty"`
	Result            map[string]any   `json:"result,omitempty"`
	ErrorMessage      string           `json:"error_message,omitempty"`
	CreatedAt         time.Time        `json:"created_at"`
	CompletedAt       *time.Time       `json:"completed_at,omitempty"`
	// CallbackState: pending(待回调)| confirmed(已确认)| exhausted(重试耗尽)| disabled(未配置/被禁用)
	CallbackState     string     `json:"callback_state"`
	CallbackAttempts  int        `json:"callback_attempts"`
	LastCallbackAt    *time.Time `json:"last_callback_at,omitempty"`
	LastCallbackError string     `json:"last_callback_error,omitempty"`
	IdempotencyKey    string     `json:"idempotency_key,omitempty"`
}

// Usage 是一次成功生成的计费用量。价格换算在 sub2api 侧。
type Usage struct {
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
}

// 终态与回调状态常量。
const (
	StatusProcessing       = "processing"
	StatusQueued           = "queued"
	StatusSubmitting       = "submitting"
	StatusDownloading      = "downloading"
	StatusRecoveryRequired = "recovery_required"
	StatusSucceeded        = "succeeded"
	StatusFailed           = "failed"
	StatusInterrupted      = "interrupted"

	CallbackPending   = "pending"
	CallbackConfirmed = "confirmed"
	CallbackExhausted = "exhausted"
	CallbackDisabled  = "disabled"
)

// Store 是任务注册表接口。v1 提供内存实现 + 文件快照;
// 多实例部署时按同一接口替换为 Redis 实现。
type Store interface {
	Create(task *Task) error
	Get(id string) (*Task, bool)
	Update(task *Task) error
	FindByIdempotencyKey(key string) (*Task, bool)
	// ListPending 返回已到终态但回调未确认的任务(对账用)。
	ListPending() []*Task
	// ListProcessing 返回仍在轮询中的任务(重启恢复用)。
	ListProcessing() []*Task
	Delete(id string) error
}

// MemoryStore 是 Store 的内存实现。onDirty 在每次变更后触发,
// 供服务层做防抖快照持久化。
type MemoryStore struct {
	mu      sync.RWMutex
	tasks   map[string]*Task
	onDirty func()
	root    string
}

// NewMemoryStore 创建内存任务表。
func NewMemoryStore(onDirty func()) *MemoryStore {
	if onDirty == nil {
		onDirty = func() {}
	}
	return &MemoryStore{tasks: map[string]*Task{}, onDirty: onDirty}
}

func (s *MemoryStore) Create(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.tasks[task.ID]; exists {
		return fmt.Errorf("task already exists")
	}
	if err := s.persist(task); err != nil {
		return err
	}
	s.tasks[task.ID] = cloneTask(task)
	s.onDirty()
	return nil
}

func (s *MemoryStore) Get(id string) (*Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	return cloneTask(t), ok
}

func (s *MemoryStore) Update(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if prev := s.tasks[task.ID]; prev != nil && len(prev.DeletedFileIDs) > 0 {
		task.DeletedFileIDs = slices.Clone(prev.DeletedFileIDs)
		task.Files = slices.DeleteFunc(task.Files, func(f ResultFile) bool { return slices.Contains(task.DeletedFileIDs, f.ID) })
		task.Response, task.Result = nil, nil
	}
	if prev := s.tasks[task.ID]; prev != nil && prev.FilesAcknowledged {
		task.FilesAcknowledged = true
	}
	if err := s.persist(task); err != nil {
		return err
	}
	s.tasks[task.ID] = cloneTask(task)
	s.onDirty()
	return nil
}

func (s *MemoryStore) AcknowledgeFiles(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task := cloneTask(s.tasks[id])
	if task == nil {
		return os.ErrNotExist
	}
	task.FilesAcknowledged = true
	if err := s.persist(task); err != nil {
		return err
	}
	s.tasks[id] = task
	return nil
}

func (s *MemoryStore) FindByIdempotencyKey(key string) (*Task, bool) {
	if key == "" {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.tasks {
		if t.IdempotencyKey == key {
			return cloneTask(t), true
		}
	}
	return nil, false
}

func (s *MemoryStore) ListPending() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Task
	for _, t := range s.tasks {
		if (t.Status == StatusSucceeded || t.Status == StatusFailed) &&
			t.CallbackState != CallbackConfirmed {
			out = append(out, cloneTask(t))
		}
	}
	return out
}

func (s *MemoryStore) ListProcessing() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Task
	for _, t := range s.tasks {
		if t.Status == StatusProcessing || t.Status == StatusQueued || t.Status == StatusSubmitting || t.Status == StatusDownloading {
			out = append(out, cloneTask(t))
		}
	}
	return out
}

func (s *MemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root != "" {
		if err := os.Remove(filepath.Join(s.root, id+".json")); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	delete(s.tasks, id)
	s.onDirty()
	return nil
}

// GCFiles expires only terminal results already accepted by the main site.
func (s *MemoryStore) GCFiles(hours int, root string) (int, error) {
	if hours <= 0 {
		return 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for id, prev := range s.tasks {
		if prev.CallbackState != CallbackConfirmed || prev.FilesAcknowledged || prev.Status != StatusSucceeded || prev.CompletedAt == nil || time.Since(*prev.CompletedAt)/time.Hour < time.Duration(hours) {
			continue
		}
		// Never let a malformed persisted task turn cleanup into deletion outside the media directory.
		if root == "" || id == "" || id == "." || id == ".." || filepath.Base(id) != id {
			return removed, fmt.Errorf("invalid media cleanup path for task %q", id)
		}
		if err := os.RemoveAll(filepath.Join(root, id)); err != nil {
			return removed, err
		}
		task := cloneTask(prev)
		task.FilesAcknowledged = true
		// Raw upstream responses can also contain the expired image as base64.
		task.Response = nil
		task.Result = nil
		if err := s.persist(task); err != nil {
			return removed, err
		}
		s.tasks[id] = task
		removed++
	}
	return removed, nil
}

// GC 清理完成时间超过 ttl 的任务。由服务层定时调用。
func (s *MemoryStore) GC(ttl time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for id, t := range s.tasks {
		// 未交给主站的结果不能因超时被清理。
		if t.CallbackState != CallbackConfirmed || !t.FilesAcknowledged {
			continue
		}
		ref := t.CompletedAt
		if ref == nil {
			if t.Status == StatusProcessing || t.Status == StatusInterrupted {
				continue
			}
			ref = &t.CreatedAt
		}
		if time.Since(*ref) > ttl {
			if s.root != "" {
				if err := os.Remove(filepath.Join(s.root, id+".json")); err != nil && !os.IsNotExist(err) {
					continue
				}
			}
			delete(s.tasks, id)
			removed++
		}
	}
	if removed > 0 {
		s.onDirty()
	}
	return removed
}

func cloneTask(t *Task) *Task {
	if t == nil {
		return nil
	}
	buf, err := json.Marshal(t)
	if err != nil {
		panic(err)
	}
	var cp Task
	if err := json.Unmarshal(buf, &cp); err != nil {
		panic(err)
	}
	return &cp
}

// NewDiskStore 在确认任务受理前持久化，每个任务独立保存；损坏时拒绝启动。
func NewDiskStore(root string) (*MemoryStore, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	s := NewMemoryStore(nil)
	s.root = root
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		buf, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			return nil, err
		}
		var task Task
		if err := json.Unmarshal(buf, &task); err != nil {
			return nil, fmt.Errorf("read task %s: %w", e.Name(), err)
		}
		if task.ID+".json" != e.Name() {
			return nil, fmt.Errorf("invalid task filename %s", e.Name())
		}
		s.tasks[task.ID] = &task
	}
	return s, nil
}

func (s *MemoryStore) persist(t *Task) error {
	if s.root == "" {
		return nil
	}
	if filepath.Base(t.ID) != t.ID || t.ID == "." || t.ID == "" {
		return fmt.Errorf("invalid task id")
	}
	return WriteState(filepath.Join(s.root, t.ID+".json"), t)
}

func (s *MemoryStore) ListAll() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		out = append(out, cloneTask(t))
	}
	return out
}

// WriteState 写入、刷盘后原子替换；目录同步保证 Linux 重启后的目录项可恢复。
func WriteState(path string, v any) error {
	buf, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".state-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(buf); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	return SyncDirectory(filepath.Dir(path))
}

// SnapshotState 是可持久化的服务状态(任务表 + 由上层并入的配置)。
type SnapshotState struct {
	Tasks []*Task `json:"tasks"`
}

// SaveSnapshot 把任务表原子写入 path。stripCredential 为 true 时
// 落盘前剥掉凭证,对应恢复后的 interrupted 处理。
func SaveSnapshot(path string, store *MemoryStore, stripCredential bool) error {
	store.mu.RLock()
	tasks := make([]*Task, 0, len(store.tasks))
	for _, t := range store.tasks {
		cp := *t
		if stripCredential {
			cp.Credential = ""
		}
		tasks = append(tasks, &cp)
	}
	store.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	buf, err := json.Marshal(SnapshotState{Tasks: tasks})
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadSnapshot 从 path 恢复任务表。缺失凭证且仍在轮询的任务标记为
// interrupted,避免静默丢失;其冻结由 sub2api 侧预留 TTL 到期自然释放。
func LoadSnapshot(path string, store *MemoryStore) (int, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var state SnapshotState
	if err := json.Unmarshal(buf, &state); err != nil {
		return 0, err
	}
	n := 0
	for _, t := range state.Tasks {
		if t.Status == StatusProcessing && t.Credential == "" {
			t.Status = StatusRecoveryRequired
		}
		if _, ok := store.Get(t.ID); ok {
			continue
		}
		if err := store.Create(t); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
