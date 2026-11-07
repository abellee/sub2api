package mediahub

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Binding 把 sub2api 分组绑定到 mediad 的一个 schema。
// 上游账号(地址/凭证)沿用 sub2api 既有的 OpenAI 平台账号体系;
// 绑定只回答"这个分组的媒体请求走哪套组装规则"。
type Binding struct {
	GroupID  int64  `json:"group_id"`
	SchemaID string `json:"schema_id"`
	Enabled  bool   `json:"enabled"`
	// EstimateKind 是冻结估算口径:"image"(默认,按张)| "video"(按秒,
	// 从请求 body 的 duration 字段取时长)| "token"。
	EstimateKind string `json:"estimate_kind,omitempty"`
}

// bindingFile 是持久化文件结构。
type bindingFile struct {
	Bindings []Binding `json:"bindings"`
}

// BindingStore 是绑定的 JSON 文件存储(原子写,进程内互斥)。
type BindingStore struct {
	mu   sync.RWMutex
	path string
	data bindingFile
}

// NewBindingStore 打开(或初始化)绑定存储。
func NewBindingStore(path string) (*BindingStore, error) {
	s := &BindingStore{path: path}
	buf, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(buf, &s.data); err != nil {
			return nil, err
		}
	case os.IsNotExist(err):
		// 首次使用
	default:
		return nil, err
	}
	return s, nil
}

// List 返回全部绑定(拷贝)。
func (s *BindingStore) List() []Binding {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Binding, len(s.data.Bindings))
	copy(out, s.data.Bindings)
	return out
}

// Get 返回分组绑定(不存在时 enabled=false 的零值 + false)。
func (s *BindingStore) Get(groupID int64) (Binding, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, b := range s.data.Bindings {
		if b.GroupID == groupID {
			return b, true
		}
	}
	return Binding{}, false
}

// Put 写入或更新绑定(group_id 为主键),并持久化。
func (s *BindingStore) Put(b Binding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := append([]Binding(nil), s.data.Bindings...)
	replaced := false
	for i := range s.data.Bindings {
		if s.data.Bindings[i].GroupID == b.GroupID {
			s.data.Bindings[i] = b
			replaced = true
			break
		}
	}
	if !replaced {
		s.data.Bindings = append(s.data.Bindings, b)
	}
	if err := s.saveLocked(); err != nil {
		s.data.Bindings = previous
		return err
	}
	return nil
}

// Delete 移除分组绑定,并持久化。
func (s *BindingStore) Delete(groupID int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Bindings {
		if s.data.Bindings[i].GroupID == groupID {
			previous := append([]Binding(nil), s.data.Bindings...)
			s.data.Bindings = append(s.data.Bindings[:i], s.data.Bindings[i+1:]...)
			if err := s.saveLocked(); err != nil {
				s.data.Bindings = previous
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

func (s *BindingStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
