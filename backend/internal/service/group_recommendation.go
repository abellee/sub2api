package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// GroupRecommendation is intentionally stored outside the database so admins
// can curate a lightweight list without changing the group schema.
type GroupRecommendation struct {
	GroupID int64   `json:"group_id"`
	Reason  string  `json:"reason,omitempty"`
	Rating  float64 `json:"rating"`
}

type GroupRecommendationStore struct {
	mu       sync.Mutex
	filePath string
}

func ProvideGroupRecommendationStore(cfg *config.Config) *GroupRecommendationStore {
	dataDir := "./data"
	if cfg != nil && cfg.Pricing.DataDir != "" {
		dataDir = cfg.Pricing.DataDir
	}
	dir := filepath.Join(dataDir, "community")
	_ = os.MkdirAll(dir, 0o755)
	return &GroupRecommendationStore{filePath: filepath.Join(dir, "group-recommendations.json")}
}

func (s *GroupRecommendationStore) readLocked() ([]GroupRecommendation, error) {
	raw, err := os.ReadFile(s.filePath)
	if os.IsNotExist(err) {
		return []GroupRecommendation{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return []GroupRecommendation{}, nil
	}
	var records []GroupRecommendation
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, fmt.Errorf("decode group recommendations: %w", err)
	}
	return records, nil
}

func (s *GroupRecommendationStore) writeLocked(records []GroupRecommendation) error {
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return err
	}
	tmp := s.filePath + ".tmp"
	raw, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.filePath)
}

// List returns records and removes entries whose group IDs are not in validIDs.
func (s *GroupRecommendationStore) List(validIDs map[int64]struct{}) ([]GroupRecommendation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.readLocked()
	if err != nil {
		return nil, err
	}
	filtered := make([]GroupRecommendation, 0, len(records))
	seen := make(map[int64]struct{}, len(records))
	dirty := false
	for _, record := range records {
		if record.GroupID <= 0 {
			dirty = true
			continue
		}
		if _, ok := validIDs[record.GroupID]; !ok {
			dirty = true
			continue
		}
		if _, ok := seen[record.GroupID]; ok {
			dirty = true
			continue
		}
		seen[record.GroupID] = struct{}{}
		filtered = append(filtered, record)
	}
	if dirty {
		if err := s.writeLocked(filtered); err != nil {
			return nil, err
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Rating == filtered[j].Rating {
			return filtered[i].GroupID < filtered[j].GroupID
		}
		return filtered[i].Rating > filtered[j].Rating
	})
	return filtered, nil
}

func (s *GroupRecommendationStore) Set(record GroupRecommendation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.readLocked()
	if err != nil {
		return err
	}
	updated := false
	for i := range records {
		if records[i].GroupID == record.GroupID {
			records[i] = record
			updated = true
			break
		}
	}
	if !updated {
		records = append(records, record)
	}
	return s.writeLocked(records)
}

func (s *GroupRecommendationStore) Delete(groupID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.readLocked()
	if err != nil {
		return err
	}
	filtered := records[:0]
	for _, record := range records {
		if record.GroupID != groupID {
			filtered = append(filtered, record)
		}
	}
	return s.writeLocked(filtered)
}
