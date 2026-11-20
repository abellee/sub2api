package relay

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Reserve for decoding and temporary copies as well as the input itself.
const workerMemory = int64(768 << 20)

type memoryBudget struct {
	mu        sync.Mutex
	reserved  int64
	available func() int64
}

var mediaMemory = memoryBudget{available: availableMemory}

func availableMemory() int64 {
	var available int64
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) >= 2 && f[0] == "MemAvailable:" {
				n, _ := strconv.ParseInt(f[1], 10, 64)
				available = n * 1024
			}
		}
	} else {
		available = platformAvailableMemory()
	}
	// Docker's memory allowance can be smaller than the host's available RAM.
	for _, pair := range [][2]string{{"/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory.current"}, {"/sys/fs/cgroup/memory/memory.limit_in_bytes", "/sys/fs/cgroup/memory/memory.usage_in_bytes"}} {
		limit, e1 := os.ReadFile(pair[0])
		used, e2 := os.ReadFile(pair[1])
		if e1 != nil || e2 != nil {
			continue
		}
		max, e1 := strconv.ParseInt(strings.TrimSpace(string(limit)), 10, 64)
		current, e2 := strconv.ParseInt(strings.TrimSpace(string(used)), 10, 64)
		if e1 == nil && e2 == nil {
			stats, _ := os.ReadFile(filepath.Join(filepath.Dir(pair[0]), "memory.stat"))
			remaining := cgroupAvailable(max, current, string(stats))
			if remaining < available {
				available = remaining
			}
		}
	}
	return available
}
func (b *memoryBudget) try(n int64) (func(), bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// Leave memory for SSH, the OS, task metadata and other services. Subtract
	// reservations too: admitted readers may not have allocated their buffers yet.
	if n < 0 || n > b.available()-(512<<20)-b.reserved {
		return nil, false
	}
	b.reserved += n
	var once sync.Once
	return func() { once.Do(func() { b.mu.Lock(); b.reserved -= n; b.mu.Unlock() }) }, true
}
func (b *memoryBudget) wait(ctx context.Context, n int64) (func(), error) {
	for {
		if release, ok := b.try(n); ok {
			return release, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}
func reserveRequest(size, limit int64) (func(), error) {
	if size < 0 {
		size = limit
	}
	if size > limit {
		return nil, fmt.Errorf("request exceeds maximum size")
	}
	release, ok := mediaMemory.try(size*8 + (32 << 20))
	if !ok {
		return nil, fmt.Errorf("not enough available memory; retry later")
	}
	return release, nil
}

// Inactive file cache is reclaimable; counting it as live payload memory can
// stall every worker after the initial disk migration warms the page cache.
func cgroupAvailable(limit, current int64, stats string) int64 {
	var cache int64
	for _, line := range strings.Split(stats, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && (fields[0] == "inactive_file" || fields[0] == "total_inactive_file") {
			n, err := strconv.ParseInt(fields[1], 10, 64)
			if err == nil && n > cache {
				cache = n
			}
		}
	}
	cache = min(max(cache, 0), current)
	return max(0, limit-current+cache)
}
