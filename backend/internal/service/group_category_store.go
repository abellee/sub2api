package service

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	groupCategoryFileName       = "group_categories.db"
	groupCategoryMaxDescription = 200
)

// GroupCategory is an admin-defined label for grouping API key groups.
type GroupCategory struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GroupCategorySnapshot is the category configuration stored in SQLite.
type GroupCategorySnapshot struct {
	Categories  []GroupCategory   `json:"categories"`
	Assignments map[string]string `json:"assignments"`
	// GroupCaps 按分组 ID(十进制字符串)记录上限倍率;与分类同文件持久化。
	GroupCaps map[string]float64 `json:"group_caps,omitempty"`
}

// GroupCategoryStore shares a SQLite database with user menu configuration.
type GroupCategoryStore struct {
	mu       *sync.Mutex
	filePath string
}

var groupCategoryLocks sync.Map

func NewGroupCategoryStore(filePath string) *GroupCategoryStore {
	if strings.TrimSpace(filePath) == "" {
		filePath = groupCategoryFileName
	}
	// Accept existing callers passing the former JSON path, without replacing it.
	if strings.EqualFold(filepath.Ext(filePath), ".json") {
		filePath = strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".db"
	}
	filePath, _ = filepath.Abs(filePath)
	lock, _ := groupCategoryLocks.LoadOrStore(filePath, &sync.Mutex{})
	return &GroupCategoryStore{filePath: filePath, mu: lock.(*sync.Mutex)}
}

func DefaultGroupCategoryStore() *GroupCategoryStore {
	return NewGroupCategoryStore(groupCategoryFileName)
}

func (s *GroupCategoryStore) resolvedPath() string {
	if strings.TrimSpace(s.filePath) != "" {
		return s.filePath
	}
	return groupCategoryFileName
}

func (s *GroupCategoryStore) List() (*GroupCategorySnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	normalizeCategoryOrder(snap.Categories)
	return snap, nil
}

func normalizeCategoryDescription(description string) (string, error) {
	description = strings.TrimSpace(description)
	if len([]rune(description)) > groupCategoryMaxDescription {
		return "", fmt.Errorf("category description is too long")
	}
	return description, nil
}

func (s *GroupCategoryStore) Create(name, description string, recommended bool) (*GroupCategory, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("category name is required")
	}
	description, err := normalizeCategoryDescription(description)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	snap, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	cat := GroupCategory{
		ID:          uuid.NewString(),
		Name:        name,
		Description: description,
		SortOrder:   nextCategorySortOrder(snap.Categories),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_ = recommended
	snap.Categories = append(snap.Categories, cat)
	if err := s.saveLocked(snap); err != nil {
		return nil, err
	}
	return &cat, nil
}

func (s *GroupCategoryStore) Update(id, name string, description *string, recommended *bool) (*GroupCategory, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("category id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	snap, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	idx := indexOfCategory(snap.Categories, id)
	if idx < 0 {
		return nil, fmt.Errorf("category not found")
	}
	cat := snap.Categories[idx]
	if name = strings.TrimSpace(name); name != "" {
		cat.Name = name
	}
	if description != nil {
		normalized, err := normalizeCategoryDescription(*description)
		if err != nil {
			return nil, err
		}
		cat.Description = normalized
	}
	_ = recommended
	cat.UpdatedAt = time.Now().UTC()
	snap.Categories[idx] = cat
	if err := s.saveLocked(snap); err != nil {
		return nil, err
	}
	return &cat, nil
}

func (s *GroupCategoryStore) Delete(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("category id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	snap, err := s.loadLocked()
	if err != nil {
		return err
	}
	idx := indexOfCategory(snap.Categories, id)
	if idx < 0 {
		return fmt.Errorf("category not found")
	}
	snap.Categories = append(snap.Categories[:idx], snap.Categories[idx+1:]...)
	for groupID, catID := range snap.Assignments {
		if catID == id {
			delete(snap.Assignments, groupID)
		}
	}
	return s.saveLocked(snap)
}

func (s *GroupCategoryStore) Assign(groupID int64, categoryID string) error {
	if groupID <= 0 {
		return fmt.Errorf("invalid group id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	snap, err := s.loadLocked()
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%d", groupID)
	categoryID = strings.TrimSpace(categoryID)
	if categoryID == "" {
		delete(snap.Assignments, key)
		return s.saveLocked(snap)
	}
	if indexOfCategory(snap.Categories, categoryID) < 0 {
		return fmt.Errorf("category not found")
	}
	snap.Assignments[key] = categoryID
	return s.saveLocked(snap)
}

// SetGroupCap 设置/清除分组的上限倍率;capRate 为 nil 或 <=0 时清除。
func (s *GroupCategoryStore) SetGroupCap(groupID int64, capRate *float64) error {
	if groupID <= 0 {
		return fmt.Errorf("invalid group id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	snap, err := s.loadLocked()
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%d", groupID)
	if capRate == nil || *capRate <= 0 {
		delete(snap.GroupCaps, key)
		return s.saveLocked(snap)
	}
	if snap.GroupCaps == nil {
		snap.GroupCaps = map[string]float64{}
	}
	snap.GroupCaps[key] = *capRate
	return s.saveLocked(snap)
}

func (s *GroupCategoryStore) Reorder(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, err := s.loadLocked()
	if err != nil {
		return err
	}
	if len(ids) != len(snap.Categories) {
		return fmt.Errorf("category order does not match existing categories")
	}
	byID := make(map[string]GroupCategory, len(snap.Categories))
	for _, cat := range snap.Categories {
		byID[cat.ID] = cat
	}
	ordered := make([]GroupCategory, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	now := time.Now().UTC()
	n := len(ids)
	for i, id := range ids {
		id = strings.TrimSpace(id)
		cat, ok := byID[id]
		if !ok {
			return fmt.Errorf("category not found")
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("duplicate category id")
		}
		seen[id] = struct{}{}
		cat.SortOrder = n - i
		cat.UpdatedAt = now
		ordered = append(ordered, cat)
	}
	snap.Categories = ordered
	return s.saveLocked(snap)
}

func (s *GroupCategoryStore) loadLocked() (*GroupCategorySnapshot, error) {
	var snap GroupCategorySnapshot
	if err := s.readConfig(context.Background(), "group_categories", &snap); err != nil {
		return nil, err
	}
	if snap.Categories == nil {
		snap.Categories = []GroupCategory{}
	}
	if snap.Assignments == nil {
		snap.Assignments = map[string]string{}
	}
	if snap.GroupCaps == nil {
		snap.GroupCaps = map[string]float64{}
	}
	return &snap, nil
}

func (s *GroupCategoryStore) saveLocked(snap *GroupCategorySnapshot) error {
	return s.writeConfig(context.Background(), "group_categories", snap)
}

func nextCategorySortOrder(cats []GroupCategory) int {
	min := 0
	for _, cat := range cats {
		if min == 0 || cat.SortOrder < min {
			min = cat.SortOrder
		}
	}
	if min > 1 {
		return min - 1
	}
	return 0
}

func normalizeCategoryOrder(cats []GroupCategory) {
	if len(cats) == 0 {
		return
	}
	allZero := true
	for _, cat := range cats {
		if cat.SortOrder != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		n := len(cats)
		for i := range cats {
			cats[i].SortOrder = n - i
		}
	}
	sort.SliceStable(cats, func(i, j int) bool {
		if cats[i].SortOrder != cats[j].SortOrder {
			return cats[i].SortOrder > cats[j].SortOrder
		}
		return cats[i].Name < cats[j].Name
	})
}

func indexOfCategory(cats []GroupCategory, id string) int {
	for i := range cats {
		if cats[i].ID == id {
			return i
		}
	}
	return -1
}
