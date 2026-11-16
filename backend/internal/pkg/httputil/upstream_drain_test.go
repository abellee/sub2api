package httputil

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUpstreamDrainStartsOnlyAfterClientCancellation(t *testing.T) {
	client, cancelClient := context.WithCancel(context.Background())
	defer cancelClient()
	upstream, release := NewUpstreamAttemptContext(DetachUpstreamContext(client), 30*time.Millisecond)
	defer release()
	time.Sleep(60 * time.Millisecond)
	require.NoError(t, upstream.Err())
	cancelClient()
	select {
	case <-upstream.Done():
		t.Fatal("cancelled before late usage drain window")
	case <-time.After(10 * time.Millisecond):
	}
	select {
	case <-upstream.Done():
		require.ErrorIs(t, upstream.Err(), context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("drain exceeded total budget")
	}
}

func TestUpstreamDrainOverrideAndNestedDetachKeepOriginalSignal(t *testing.T) {
	client, cancelClient := context.WithCancel(context.Background())
	parent := DetachUpstreamContext(DetachUpstreamContext(client, 20*time.Millisecond))
	upstream, release := NewUpstreamAttemptContext(parent, time.Second)
	defer release()
	cancelClient()
	select {
	case <-upstream.Done():
	case <-time.After(200 * time.Millisecond):
		t.Fatal("nested detach lost original cancellation or override")
	}
}

func TestUpstreamDrainReleaseDoesNotCancelCaller(t *testing.T) {
	client, cancelClient := context.WithCancel(context.Background())
	defer cancelClient()
	upstream, release := NewUpstreamAttemptContext(DetachUpstreamContext(client), time.Second)
	release()
	release()
	require.ErrorIs(t, upstream.Err(), context.Canceled)
	require.NoError(t, client.Err())
	cancelClient()
}

func TestUpstreamStreamDrainContinuousDataStillHasTotalLimit(t *testing.T) {
	client, cancelClient := context.WithCancel(t.Context())
	defer cancelClient()
	upstream, release := NewUpstreamAttemptContext(DetachUpstreamContext(client), 80*time.Millisecond)
	defer release()
	attempt, ok := upstream.Value(upstreamDrainAttemptKey{}).(*upstreamDrainAttempt)
	require.True(t, ok)
	attempt.maxDuration = 250 * time.Millisecond
	reader, writer := io.Pipe()
	defer func() { _ = reader.Close() }()
	body := TrackUpstreamStreamDrain(upstream, reader)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = writer.Close() }()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-upstream.Done():
				_ = writer.CloseWithError(upstream.Err())
				return
			case <-ticker.C:
				if _, err := io.WriteString(writer, ": ping\n\n"); err != nil {
					return
				}
			}
		}
	}()
	cancelClient()
	started := time.Now()
	data, err := io.ReadAll(body)
	require.ErrorIs(t, err, ErrUpstreamDrainTimeout)
	require.ErrorIs(t, context.Cause(upstream), ErrUpstreamDrainTimeout)
	require.NotEmpty(t, data)
	require.GreaterOrEqual(t, time.Since(started), 200*time.Millisecond)
	require.Less(t, time.Since(started), 2*time.Second)
	<-done
}
