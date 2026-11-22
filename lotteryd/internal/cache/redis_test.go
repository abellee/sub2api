package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

func testCache(t *testing.T) (*Cache, *miniredis.Miniredis) {
	t.Helper()
	r := miniredis.RunT(t)
	c, err := New("redis://"+r.Addr(), t.Name())
	require.NoError(t, err)
	t.Cleanup(func() { c.Close() })
	return c, r
}

func TestLoadCoalescesMissesAndReturnsIndependentCopies(t *testing.T) {
	c, r := testCache(t)
	var calls atomic.Int64
	load := func() ([]string, error) {
		calls.Add(1)
		time.Sleep(10 * time.Millisecond)
		return []string{"original"}, nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := Load(c, context.Background(), "shared", time.Minute, load)
			require.NoError(t, err)
			v[0] = "changed"
		}()
	}
	wg.Wait()
	require.Equal(t, int64(1), calls.Load())
	v, err := Load(c, context.Background(), "shared", time.Minute, load)
	require.NoError(t, err)
	require.Equal(t, []string{"original"}, v)
	r.FastForward(2 * time.Minute)
	_, err = Load(c, context.Background(), "shared", time.Minute, load)
	require.NoError(t, err)
	require.Equal(t, int64(2), calls.Load())
}

func TestCommittedWriteWinsOverLateFill(t *testing.T) {
	c, _ := testCache(t)
	started, finish, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		_, err := Load(c, context.Background(), "activity", time.Minute, func() (string, error) { close(started); <-finish; return "old", nil })
		require.NoError(t, err)
	}()
	<-started
	require.True(t, c.Invalidate())
	value, err := Load(c, context.Background(), "activity", time.Minute, func() (string, error) { return "new", nil })
	require.NoError(t, err)
	require.Equal(t, "new", value)
	close(finish)
	<-done
	value, err = Load(c, context.Background(), "activity", time.Minute, func() (string, error) { t.Fatal("new cache should already be populated"); return "", nil })
	require.NoError(t, err)
	require.Equal(t, "new", value)
}

func TestOutageRecoveryDiscardsStaleValuesAndErrors(t *testing.T) {
	c, r := testCache(t)
	source := "old"
	load := func() (string, error) { return source, nil }
	_, err := Load(c, context.Background(), "activity", time.Minute, load)
	require.NoError(t, err)
	r.SetError("unavailable")
	source = "new"
	require.False(t, c.Invalidate())
	value, err := Load(c, context.Background(), "activity", time.Minute, load)
	require.NoError(t, err)
	require.Equal(t, "new", value)
	r.SetError("")
	c.mu.Lock()
	c.retryAt = time.Time{}
	c.mu.Unlock()
	value, err = Load(c, context.Background(), "activity", time.Minute, load)
	require.NoError(t, err)
	require.Equal(t, "new", value)
	_, err = Load(c, context.Background(), "failure", time.Minute, func() (string, error) { return "", errors.New("upstream") })
	require.Error(t, err)
	value, err = Load(c, context.Background(), "failure", time.Minute, load)
	require.NoError(t, err)
	require.Equal(t, "new", value)
}

func TestNamespacesAndMissingGenerationCannotResurrectOldData(t *testing.T) {
	c, r := testCache(t)
	other, err := New("redis://"+r.Addr(), "other-database")
	require.NoError(t, err)
	defer other.Close()
	_, err = Load(c, context.Background(), "same", time.Minute, func() (string, error) { return "first", nil })
	require.NoError(t, err)
	v, err := Load(other, context.Background(), "same", time.Minute, func() (string, error) { return "second", nil })
	require.NoError(t, err)
	require.Equal(t, "second", v)
	r.Del(c.prefix + "generation")
	v, err = Load(c, context.Background(), "same", time.Minute, func() (string, error) { return "fresh", nil })
	require.NoError(t, err)
	require.Equal(t, "fresh", v)
}
