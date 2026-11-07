package relay

import (
	"encoding/json"
	"os"
	"sync"
)

// SchemaRegistry 是 schema 的内存注册表,由管理接口写入、
// relay 与轮询器读取。onDirty 供快照持久化挂钩。
type SchemaRegistry struct {
	mu      sync.RWMutex
	items   map[string]*Schema
	onDirty func()
	path    string
}

// NewSchemaRegistry 创建注册表。
func NewSchemaRegistry(onDirty func()) *SchemaRegistry {
	if onDirty == nil {
		onDirty = func() {}
	}
	return &SchemaRegistry{items: map[string]*Schema{}, onDirty: onDirty}
}

// Put 校验并写入 schema。已存在且版本不递增时拒绝,防止配置回退。
func (r *SchemaRegistry) Put(s *Schema) error {
	if err := s.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if prev, ok := r.items[s.ID]; ok && s.Version <= prev.Version {
		return ErrSchemaVersionNotIncreasing
	}
	cp := *s
	prev, hadPrev := r.items[s.ID]
	r.items[s.ID] = &cp
	if err := r.saveLocked(); err != nil {
		if hadPrev {
			r.items[s.ID] = prev
		} else {
			delete(r.items, s.ID)
		}
		return err
	}
	r.onDirty()
	return nil
}

// ErrSchemaVersionNotIncreasing 在写入旧版本时返回。
var ErrSchemaVersionNotIncreasing = &schemaVersionError{}

type schemaVersionError struct{}

func (*schemaVersionError) Error() string {
	return "schema version must be greater than the stored version"
}

// Get 读取 schema。
func (r *SchemaRegistry) Get(id string) (*Schema, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.items[id]
	return s, ok
}

// Delete 移除 schema。
func (r *SchemaRegistry) Delete(id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[id]; !ok {
		return false, nil
	}
	prev := r.items[id]
	delete(r.items, id)
	if err := r.saveLocked(); err != nil {
		r.items[id] = prev
		return false, err
	}
	r.onDirty()
	return true, nil
}

func (r *SchemaRegistry) Load(path string) error {
	r.path = path
	buf, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var items map[string]*Schema
	if err := json.Unmarshal(buf, &items); err != nil {
		return err
	}
	for _, s := range items {
		if err := s.Validate(); err != nil {
			return err
		}
	}
	r.items = items
	return nil
}
func (r *SchemaRegistry) saveLocked() error {
	if r.path == "" {
		return nil
	}
	return WriteState(r.path, r.items)
}

// List 返回全部 schema(浅拷贝)。
func (r *SchemaRegistry) List() []*Schema {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Schema, 0, len(r.items))
	for _, s := range r.items {
		cp := *s
		out = append(out, &cp)
	}
	return out
}

// Count 返回 schema 数量。
func (r *SchemaRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}
