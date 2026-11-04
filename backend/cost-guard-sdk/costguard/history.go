package costguard

import (
	"strings"
	"sync"
	"time"
)

// HistoryKey separates the previous request for each key, group and model.
type HistoryKey struct {
	APIKeyID int64
	GroupID  int64
	Model    string
}

type previousUsage struct {
	read, create float64
	expires      time.Time
}

type historyShard struct {
	mu      sync.RWMutex
	entries map[HistoryKey]previousUsage
}

// History stores only the latest cache proportions. Busy shards are skipped so
// optional cache estimates never make a request wait for a writer.
type History struct{ shards [32]historyShard }

func NewHistory() *History {
	h := &History{}
	for i := range h.shards {
		h.shards[i].entries = make(map[HistoryKey]previousUsage)
	}
	return h
}

func historyKey(k HistoryKey) HistoryKey {
	k.Model = strings.ToLower(strings.TrimSpace(k.Model))
	return k
}

// Observe receives the host's already separated ordinary/read/write token counts.
func (h *History) Observe(k HistoryKey, input, read, create int) {
	if h == nil || k.APIKeyID <= 0 || input < 0 || read < 0 || create < 0 {
		return
	}
	k = historyKey(k)
	if k.Model == "" {
		return
	}
	s := &h.shards[uint64(k.APIKeyID)%uint64(len(h.shards))]
	if !s.mu.TryLock() {
		return
	}
	defer s.mu.Unlock()
	total := float64(input) + float64(read) + float64(create)
	if total <= 0 {
		delete(s.entries, k)
		return
	}
	if _, exists := s.entries[k]; !exists && len(s.entries) >= 2048 {
		// shortcut: bounded best-effort history; evict one entry when full.
		for old := range s.entries {
			delete(s.entries, old)
			break
		}
	}
	s.entries[k] = previousUsage{read: float64(read) / total, create: float64(create) / total, expires: time.Now().Add(10 * time.Minute)}
}

// InputPrice uses the previous request, or the full input price when history is
// missing, expired or busy. No history means no discount.
func (h *History) InputPrice(k HistoryKey, p TokenPrices) float64 {
	return h.EstimateInputPrice(k, p).Price
}

// InputEstimate reports the same history snapshot used to calculate Price.
type InputEstimate struct {
	Price                 float64
	CacheReadFraction     float64
	CacheCreationFraction float64
	HistoryUsed           bool
}

// EstimateInputPrice is safe on a nil receiver and never waits for history locks.
func (h *History) EstimateInputPrice(k HistoryKey, p TokenPrices) InputEstimate {
	fallback := InputEstimate{Price: p.Input}
	if h == nil || !validPrice(p.Input) {
		return fallback
	}
	k = historyKey(k)
	s := &h.shards[uint64(k.APIKeyID)%uint64(len(h.shards))]
	if !s.mu.TryRLock() {
		return fallback
	}
	u, ok := s.entries[k]
	s.mu.RUnlock()
	if !ok || !time.Now().Before(u.expires) {
		return fallback
	}
	readPrice, createPrice := p.CacheRead, p.CacheCreation
	if !validPrice(readPrice) {
		readPrice = p.Input
	}
	if !validPrice(createPrice) {
		createPrice = p.Input
	}
	price := p.Input*(1-u.read-u.create) + readPrice*u.read + createPrice*u.create
	if !validPrice(price) {
		return fallback
	}
	return InputEstimate{Price: price, CacheReadFraction: u.read, CacheCreationFraction: u.create, HistoryUsed: true}
}
