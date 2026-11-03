package costguard

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestHistoryLatestUsageAndIsolation(t *testing.T) {
	h := NewHistory()
	k := HistoryKey{APIKeyID: 1, GroupID: 2, Model: "model"}
	p := TokenPrices{Input: 10, CacheRead: 1, CacheCreation: 20}
	if got := h.InputPrice(k, p); got != 10 {
		t.Fatal(got)
	}
	h.Observe(k, 20, 70, 10)
	if got := h.InputPrice(k, p); got < 4.699999 || got > 4.700001 {
		t.Fatal(got)
	}
	for _, other := range []HistoryKey{{2, 2, "model"}, {1, 3, "model"}, {1, 2, "other"}} {
		if got := h.InputPrice(other, p); got != 10 {
			t.Fatal("history crossed key/group/model", other, got)
		}
	}
	h.Observe(k, 100, 0, 0)
	if got := h.InputPrice(k, p); got != 10 {
		t.Fatal("last request must replace prior cache usage", got)
	}
	h.Observe(k, 0, 0, 100)
	if got := h.InputPrice(k, p); got != 20 {
		t.Fatal("cache creation must use its actual price", got)
	}
	h.Observe(k, 0, 0, 0)
	if got := h.InputPrice(k, p); got != 10 {
		t.Fatal("zero input should clear history", got)
	}
}

func TestHistoryExpiryContentionAndNil(t *testing.T) {
	h := NewHistory()
	k := HistoryKey{APIKeyID: 1, Model: "m"}
	p := TokenPrices{Input: 10, CacheRead: 1}
	h.Observe(k, 0, 100, 0)
	s := &h.shards[1]
	u := s.entries[k]
	u.expires = time.Now().Add(-time.Second)
	s.entries[k] = u
	if got := h.InputPrice(k, p); got != 10 {
		t.Fatal("expired history used", got)
	}
	s.mu.Lock()
	h.Observe(k, 0, 100, 0)
	if got := h.InputPrice(k, p); got != 10 {
		t.Fatal("busy history must be skipped", got)
	}
	s.mu.Unlock()
	var empty *History
	empty.Observe(k, 0, 100, 0)
	if got := empty.InputPrice(k, p); got != 10 {
		t.Fatal(got)
	}
}

func TestHistoryBoundedAndConcurrent(t *testing.T) {
	h := NewHistory()
	for i := 0; i < 3000; i++ {
		h.Observe(HistoryKey{APIKeyID: 1, Model: fmt.Sprint(i)}, 1, 1, 0)
	}
	if len(h.shards[1].entries) > 2048 {
		t.Fatal("history exceeds capacity")
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			k := HistoryKey{APIKeyID: int64(i + 1), Model: "m"}
			for j := 0; j < 1000; j++ {
				h.Observe(k, 20, 80, 0)
				got := h.InputPrice(k, TokenPrices{Input: 10, CacheRead: 1})
				if got < 1 || got > 10 {
					t.Errorf("invalid estimate %v", got)
					return
				}
			}
		}(i)
	}
	wg.Wait()
}
