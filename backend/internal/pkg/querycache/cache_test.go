package querycache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCacheSharesWorkAndCopiesResults(t *testing.T) {
	var c Cache
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	load := func(context.Context) (map[string][]int, error) {
		calls.Add(1)
		close(started)
		<-release
		return map[string][]int{"items": {7}}, nil
	}
	cancelCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := Load(cancelCtx, &c, "key", time.Minute, load); done <- err }()
	<-started
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	var wg sync.WaitGroup
	results := make([]map[string][]int, 12)
	errs := make([]error, 12)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = Load(context.Background(), &c, "key", time.Minute, load)
		}()
	}
	close(release)
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, calls.Load())
	results[0]["items"][0] = 999
	require.Equal(t, 7, results[1]["items"][0])
	value, err := Load(context.Background(), &c, "key", time.Minute, load)
	require.NoError(t, err)
	require.Equal(t, 7, value["items"][0])
}

func TestCacheExpiryErrorsAndKeys(t *testing.T) {
	var c Cache
	ctx := context.Background()
	calls := 0
	load := func(context.Context) (int, error) { calls++; return calls, nil }
	v, err := Load(ctx, &c, "a", time.Minute, load)
	require.NoError(t, err)
	require.Equal(t, 1, v)
	v, err = Load(ctx, &c, "b", time.Minute, load)
	require.NoError(t, err)
	require.Equal(t, 2, v)
	c.mu.Lock()
	e := c.items["a"]
	e.until = time.Time{}
	c.items["a"] = e
	c.mu.Unlock()
	v, err = Load(ctx, &c, "a", time.Minute, load)
	require.NoError(t, err)
	require.Equal(t, 3, v)
	failure := errors.New("query failed")
	_, err = Load(ctx, &c, "failure", time.Minute, func(context.Context) (int, error) { return 0, failure })
	require.ErrorIs(t, err, failure)
	v, err = Load(ctx, &c, "failure", time.Minute, load)
	require.NoError(t, err)
	require.Equal(t, 4, v)
}
