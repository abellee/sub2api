// Package cache provides Redis read-through caching for lottery read models.
package cache

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

type Cache struct {
	client     *redis.Client
	prefix     string
	flights    singleflight.Group
	mu         sync.Mutex
	retryAt    time.Time
	recovering bool
	hits       atomic.Uint64
	misses     atomic.Uint64
	fallbacks  atomic.Uint64
}

func New(rawURL, database string) (*Cache, error) {
	if rawURL == "" {
		return nil, nil
	}
	opts, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, errors.New("invalid lottery Redis URL")
	}
	opts.DialTimeout = 500 * time.Millisecond
	opts.ReadTimeout = 200 * time.Millisecond
	opts.WriteTimeout = 200 * time.Millisecond
	opts.MaxRetries = -1
	opts.ContextTimeoutEnabled = true
	return &Cache{client: redis.NewClient(opts), prefix: "lotteryd:v1:" + Hash(database) + ":"}, nil
}

func Hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (c *Cache) Close() error {
	if c == nil {
		return nil
	}
	return c.client.Close()
}

func (c *Cache) Stats() map[string]any {
	if c == nil {
		return map[string]any{"enabled": false}
	}
	return map[string]any{"enabled": true, "hits": c.hits.Load(), "misses": c.misses.Load(), "fallbacks": c.fallbacks.Load()}
}

func (c *Cache) failed() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.recovering {
		slog.Warn("lottery Redis unavailable; using database until cache recovery")
	}
	c.recovering = true
	c.retryAt = time.Now().Add(5 * time.Second)
	c.fallbacks.Add(1)
}

func (c *Cache) generation(ctx context.Context) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.retryAt) {
		return "", false
	}
	var token string
	var err error
	if c.recovering {
		// A database write may have succeeded while Redis was unreachable.
		token = rand.Text()
		err = c.client.Set(ctx, c.prefix+"generation", token, 0).Err()
	} else {
		token, err = c.client.Get(ctx, c.prefix+"generation").Result()
		if errors.Is(err, redis.Nil) {
			err = c.client.SetNX(ctx, c.prefix+"generation", rand.Text(), 0).Err()
			if err == nil {
				token, err = c.client.Get(ctx, c.prefix+"generation").Result()
			}
		}
	}
	if err != nil {
		if !c.recovering {
			slog.Warn("lottery Redis unavailable; using database until cache recovery")
		}
		c.recovering = true
		c.retryAt = time.Now().Add(5 * time.Second)
		c.fallbacks.Add(1)
		return "", false
	}
	c.recovering = false
	return token, true
}

// Invalidate advances the namespace after a successful commit. Late cache fills
// keep the old generation and cannot overwrite data published by a newer write.
func (c *Cache) Invalidate() bool {
	if c == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c.mu.Lock()
	if time.Now().Before(c.retryAt) {
		c.mu.Unlock()
		return false
	}
	err := c.client.Set(ctx, c.prefix+"generation", rand.Text(), 0).Err()
	if err == nil {
		c.recovering = false
	}
	c.mu.Unlock()
	if err != nil {
		c.failed()
		return false
	}
	return true
}

// Load returns a fresh decoded value for every caller, so callers cannot mutate
// each other's slices or maps. Errors are never cached.
func Load[T any](c *Cache, ctx context.Context, key string, ttl time.Duration, loader func() (T, error)) (T, error) {
	if c == nil {
		return loader()
	}
	redisCtx, cancel := context.WithTimeout(ctx, time.Second)
	generation, ok := c.generation(redisCtx)
	cancel()
	if !ok {
		return loader()
	}
	key = c.prefix + generation + ":" + key
	redisCtx, cancel = context.WithTimeout(ctx, time.Second)
	payload, err := c.client.Get(redisCtx, key).Bytes()
	cancel()
	var out T
	if err == nil && json.Unmarshal(payload, &out) == nil {
		c.hits.Add(1)
		return out, nil
	}
	if err != nil && !errors.Is(err, redis.Nil) {
		c.failed()
		return loader()
	}
	c.misses.Add(1)
	result, err, _ := c.flights.Do(key, func() (any, error) {
		// A second caller may have filled Redis before this flight started.
		rctx, done := context.WithTimeout(ctx, time.Second)
		data, readErr := c.client.Get(rctx, key).Bytes()
		done()
		var check T
		if readErr == nil && json.Unmarshal(data, &check) == nil {
			return data, nil
		}
		value, loadErr := loader()
		if loadErr != nil {
			return nil, loadErr
		}
		data, loadErr = json.Marshal(value)
		if loadErr != nil {
			return nil, loadErr
		}
		rctx, done = context.WithTimeout(ctx, time.Second)
		writeErr := c.client.Set(rctx, key, data, ttl).Err()
		done()
		if writeErr != nil {
			c.failed()
		}
		return data, nil
	})
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(result.([]byte), &out)
	return out, err
}
