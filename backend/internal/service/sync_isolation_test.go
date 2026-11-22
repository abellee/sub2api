package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSyncIsolationThresholdAndConsecutiveRequests(t *testing.T) {
	now := time.Unix(1000, 0)
	var w SyncIsolationWindow
	for i := range 10 {
		require.False(t, w.Observe(now.Add(time.Duration(i)*time.Second), false, true))
	}
	require.False(t, w.Isolated, "ten requests must stay in the original group")
	require.True(t, w.Observe(now.Add(10*time.Second), false, true))
	require.True(t, w.Isolated)
	require.Len(t, w.Events, 1, "the triggering request starts the isolated window")
	for range 9 {
		w.Observe(now.Add(11*time.Second), true, true)
	}
	require.True(t, w.Isolated, "exactly 90 percent must stay isolated")
	require.True(t, w.Observe(now.Add(12*time.Second), true, true))
	require.False(t, w.Isolated, "over 90 percent must return")

	for range 10 {
		w.Observe(now.Add(13*time.Second), false, true)
	}
	w.Observe(now.Add(14*time.Second), true, true)
	w.Observe(now.Add(15*time.Second), false, true)
	require.False(t, w.Isolated, "streaming interrupts a consecutive sync run")
	require.Len(t, w.Events, 1)
}

type syncIsolationBackgroundRepo struct {
	GroupRepository
	observe func(context.Context, int64, int64, int64, bool, time.Time) (int64, bool, error)
}

func (r *syncIsolationBackgroundRepo) ObserveSyncIsolation(ctx context.Context, keyID, userID, groupID int64, stream bool, at time.Time) (int64, bool, error) {
	return r.observe(ctx, keyID, userID, groupID, stream, at)
}

func syncIsolationTestKey() *APIKey {
	return &APIKey{ID: 1, UserID: 2, Group: &Group{ID: 3, SyncIsolationEnabled: true}}
}

func TestSyncIsolationBackgroundNeverWaitsForDatabaseAndPreservesOrder(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	observations := make(chan syncIsolationObservation, 2049)
	repo := &syncIsolationBackgroundRepo{observe: func(ctx context.Context, keyID, userID, groupID int64, stream bool, at time.Time) (int64, bool, error) {
		select {
		case <-entered:
		default:
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return 0, false, ctx.Err()
			}
		}
		observations <- syncIsolationObservation{keyID: keyID, userID: userID, groupID: groupID, stream: stream, at: at}
		return groupID, false, nil
	}}
	svc := &APIKeyService{groupRepo: repo}
	t.Cleanup(svc.StopSyncIsolation)
	key := syncIsolationTestKey()
	svc.QueueSyncIsolation(key, false)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("background worker did not start")
	}
	queued := make(chan struct{})
	go func() {
		for i := range 2048 {
			svc.QueueSyncIsolation(key, i%2 == 0)
		}
		close(queued)
	}()
	select {
	case <-queued:
	case <-time.After(time.Second):
		t.Fatal("new requests waited for the database")
	}
	// Mutating the request's auth object must not change already queued work.
	key.ID, key.UserID, key.Group.ID = 10, 20, 30
	releasedAt := time.Now()
	close(release)
	for i := range 2049 {
		select {
		case event := <-observations:
			require.Equal(t, int64(1), event.keyID)
			require.Equal(t, int64(2), event.userID)
			require.Equal(t, int64(3), event.groupID)
			require.Equal(t, i > 0 && (i-1)%2 == 0, event.stream)
			require.False(t, event.at.After(releasedAt), "windows must use arrival time, not worker execution time")
		case <-time.After(time.Second):
			t.Fatal("queued observations were lost")
		}
	}
}

func TestSyncIsolationBackgroundFailureDoesNotStopLaterRequests(t *testing.T) {
	finished := make(chan struct{})
	var calls atomic.Int32
	repo := &syncIsolationBackgroundRepo{observe: func(_ context.Context, _, _, groupID int64, _ bool, _ time.Time) (int64, bool, error) {
		switch calls.Add(1) {
		case 1:
			return 0, false, errors.New("database unavailable")
		case 2:
			panic("fixture failure")
		default:
			close(finished)
			return groupID, false, nil
		}
	}}
	svc := &APIKeyService{groupRepo: repo}
	t.Cleanup(svc.StopSyncIsolation)
	for range 3 {
		svc.QueueSyncIsolation(syncIsolationTestKey(), false)
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("a failed background operation stopped later observations")
	}
}

func TestSyncIsolationBackgroundStopCancelsDatabaseAndRejectsNewWork(t *testing.T) {
	entered := make(chan struct{})
	var calls atomic.Int32
	repo := &syncIsolationBackgroundRepo{observe: func(ctx context.Context, _, _, _ int64, _ bool, _ time.Time) (int64, bool, error) {
		calls.Add(1)
		close(entered)
		<-ctx.Done()
		return 0, false, ctx.Err()
	}}
	svc := &APIKeyService{groupRepo: repo}
	t.Cleanup(svc.StopSyncIsolation)
	svc.QueueSyncIsolation(syncIsolationTestKey(), false)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("background worker did not start")
	}
	svc.QueueSyncIsolation(syncIsolationTestKey(), true)
	stopped := make(chan struct{})
	go func() { svc.StopSyncIsolation(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel the database operation")
	}
	svc.QueueSyncIsolation(syncIsolationTestKey(), false)
	require.Equal(t, int32(1), calls.Load())
}

func TestSyncIsolationSlidingWindowAndDisable(t *testing.T) {
	now := time.Unix(1000, 0)
	var w SyncIsolationWindow
	for range 10 {
		w.Observe(now, false, true)
	}
	w.Observe(now.Add(time.Minute), false, true)
	require.False(t, w.Isolated, "events exactly 60 seconds old expire")
	require.Len(t, w.Events, 1)
	w = SyncIsolationWindow{Isolated: true, Events: []SyncIsolationEvent{{At: now.UnixMilli()}}}
	w.Observe(now.Add(time.Minute), true, true)
	require.False(t, w.Isolated, "the streaming ratio also uses a sliding window")
	w.Isolated = true
	require.True(t, w.Observe(now, false, false))
	require.False(t, w.Isolated, "disabling returns isolated users")
}

func TestSyncIsolationSurvivesAuthSnapshot(t *testing.T) {
	svc := &APIKeyService{}
	key := &APIKey{ID: 1, User: &User{ID: 2}, Group: &Group{ID: 4, SyncIsolationSourceGroupID: 3, SyncIsolationEnabled: true}}
	snapshot := svc.snapshotFromAPIKey(context.Background(), key)
	stored := svc.snapshotToAPIKey("fixture", snapshot)
	require.True(t, stored.Group.SyncIsolationEnabled)
	require.Equal(t, int64(3), stored.Group.SyncIsolationSourceGroupID)
	require.Equal(t, int64(3), stored.Group.PolicyGroupID())
	require.Equal(t, int64(4), (&Group{ID: 4}).PolicyGroupID())
}
