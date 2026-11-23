// Package querycache reuses short-lived read-only query results.
package querycache

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type entry struct {
	data  []byte
	until time.Time
}

// Cache has a usable zero value. JSON gives each caller its own result, so
// attaching dashboard health or redacting public fields cannot mutate the cache.
type Cache struct {
	mu     sync.Mutex
	items  map[string]entry
	flight singleflight.Group
}

func (c *Cache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[key]
	return e.data, ok && time.Now().Before(e.until)
}

func Load[T any](ctx context.Context, c *Cache, key string, ttl time.Duration, load func(context.Context) (T, error)) (T, error) {
	var result T
	if err := ctx.Err(); err != nil {
		return result, err
	}
	data, ok := c.get(key)
	if !ok {
		ch := c.flight.DoChan(key, func() (any, error) {
			if data, ok := c.get(key); ok {
				return data, nil
			}
			// One disconnected reader must not cancel the work shared by others.
			loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()
			value, err := load(loadCtx)
			if err != nil {
				return nil, err
			}
			data, err := json.Marshal(value)
			if err != nil {
				return nil, err
			}
			c.mu.Lock()
			if c.items == nil {
				c.items = make(map[string]entry)
			}
			now := time.Now()
			for k, e := range c.items {
				if !now.Before(e.until) {
					delete(c.items, k)
				}
			}
			if len(c.items) >= 64 {
				var oldest string
				var until time.Time
				for k, e := range c.items {
					if until.IsZero() || e.until.Before(until) {
						oldest, until = k, e.until
					}
				}
				delete(c.items, oldest)
			}
			// Oversized results can still be shared with concurrent readers.
			if len(data) <= 2<<20 {
				c.items[key] = entry{data: data, until: now.Add(ttl)}
			}
			c.mu.Unlock()
			return data, nil
		})
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case loaded := <-ch:
			if loaded.Err != nil {
				return result, loaded.Err
			}
			data, ok = loaded.Val.([]byte)
			if !ok {
				return result, errors.New("unexpected query cache result type")
			}
		}
	}
	err := json.Unmarshal(data, &result)
	return result, err
}
