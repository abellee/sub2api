package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestAccountInputRoutingTransitionsAndIsolation(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewAccountInputRoutingStore(client)
	ctx := context.Background()
	for i, step := range []struct {
		input, output int
		restricted    bool
	}{
		{100001, 10001, false},
		{100001, 9999, false}, {99999, 10001, false},
		{100000, 10001, false}, {100001, 10000, false},
		{100001, 10001, false}, {20, 20, false}, {100001, 10001, true},
		{30, 30, true}, {40, 40, true}, {100000, 40, true},
		{50, 50, true}, {50, 10000, true},
		{60, 60, true}, {100001, 60, true},
		{60, 60, true}, {60, 10001, true},
		{70, 70, true}, {80, 80, true}, {90, 90, false},
		{100001, 10001, false}, {100001, 10001, false}, {100001, 10001, true},
	} {
		require.NoError(t, store.Observe(ctx, "v1", 2, 10, int64(i%2+1), fmt.Sprint(i), step.input, step.output, 100000, 10000))
		restricted, err := store.Restricted(ctx, "v1", 2, 10)
		require.NoError(t, err)
		require.Equal(t, step.restricted, restricted, "step %d", i)
		for _, pair := range [][2]int64{{3, 10}, {2, 11}} {
			other, err := store.Restricted(ctx, "v1", pair[0], pair[1])
			require.NoError(t, err)
			require.False(t, other)
		}
	}
}

func TestAccountInputRoutingDuplicatesConcurrencyAndRestart(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewAccountInputRoutingStore(client)
	ctx := context.Background()
	var wg sync.WaitGroup
	errors := make(chan error, 30)
	for range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errors <- store.Observe(ctx, "v1", 2, 10, 5, "same", 100001, 10001, 100000, 10000)
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	restricted, err := store.Restricted(ctx, "v1", 2, 10)
	require.NoError(t, err)
	require.False(t, restricted)
	require.Equal(t, "1", mr.HGet(inputRoutingStateKey("v1", 2, 10), "high"))
	// The same request ID on another API key is a distinct request, but shares the user's counter.
	require.NoError(t, store.Observe(ctx, "v1", 2, 10, 6, "same", 100001, 10001, 100000, 10000))
	require.NoError(t, store.Observe(ctx, "v1", 2, 10, 5, "third", 100001, 10001, 100000, 10000))
	mr.FastForward(8 * 24 * time.Hour)
	store = NewAccountInputRoutingStore(client)
	restricted, err = store.Restricted(ctx, "v1", 2, 10)
	require.NoError(t, err)
	require.True(t, restricted, "state must survive process restart and inactivity")
	restricted, err = store.Restricted(ctx, "v2", 2, 10)
	require.NoError(t, err)
	require.False(t, restricted, "new settings reset the effective state")
	require.NoError(t, store.Observe(ctx, "v2", 2, 10, 5, "new", 100001, 10001, 100000, 10000))
	require.Equal(t, "1", mr.HGet(inputRoutingStateKey("v2", 2, 10), "high"))
	require.NoError(t, store.Observe(ctx, "v1", 2, 10, 5, "late-old", 10, 10, 100000, 10000))
	require.NoError(t, store.Observe(ctx, "v2", 2, 10, 5, "second-new", 100001, 10001, 100000, 10000))
	require.NoError(t, store.Observe(ctx, "v2", 2, 10, 5, "third-new", 100001, 10001, 100000, 10000))
	restricted, err = store.Restricted(ctx, "v2", 2, 10)
	require.NoError(t, err)
	require.True(t, restricted, "old replicas must not overwrite new revision counters")
}

func TestAccountInputRoutingIgnoresLegacyInputOnlyCounters(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	mr.HSet("account_input_routing:{2:10}:v1:state", "revision", "v1", "restricted", "1", "high", "3")
	store := NewAccountInputRoutingStore(client)
	restricted, err := store.Restricted(context.Background(), "v1", 2, 10)
	require.NoError(t, err)
	require.False(t, restricted, "input-only history must not trigger the dual-threshold rule")
}
