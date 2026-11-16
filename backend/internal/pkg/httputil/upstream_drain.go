package httputil

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

const DefaultUpstreamDrainTimeout = 180 * time.Second

const maxUpstreamStreamDrainDuration = 30 * time.Minute

var ErrUpstreamDrainTimeout = errors.New("upstream final usage drain timed out")

type upstreamDrainAttemptKey struct{}

type upstreamDrainAttempt struct {
	mu           sync.Mutex
	cancel       context.CancelCauseFunc
	timeout      time.Duration
	maxDuration  time.Duration
	timer        *time.Timer
	limit        *time.Timer
	startedAt    time.Time
	lastProgress time.Time
	stream       bool
	closed       bool
}

type upstreamDrainKey struct{}
type upstreamDrainPolicy struct {
	client  context.Context
	timeout time.Duration
}

// DetachUpstreamContext keeps billing reads alive after the client leaves. It
// records the original cancellation signal without starting a timer or changing
// existing construction-time release semantics. The transport owns the actual
// attempt lifetime and installs the bounded drain when sending the request.
func DetachUpstreamContext(ctx context.Context, timeout ...time.Duration) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	policy := upstreamDrainPolicy{client: ctx}
	if previous, ok := ctx.Value(upstreamDrainKey{}).(upstreamDrainPolicy); ok {
		policy = previous
	}
	if len(timeout) > 0 && timeout[0] > 0 {
		policy.timeout = timeout[0]
	}
	return context.WithValue(context.WithoutCancel(ctx), upstreamDrainKey{}, policy)
}

// NewUpstreamAttemptContext bounds reads after the client leaves. Buffered
// responses use a total budget; TrackUpstreamStreamDrain switches SSE responses
// to an idle budget with a separate total limit. release owns attempt cleanup.
func NewUpstreamAttemptContext(parent context.Context, defaultTimeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(parent)
	policy, ok := parent.Value(upstreamDrainKey{}).(upstreamDrainPolicy)
	if !ok || policy.client.Done() == nil {
		return ctx, func() { cancel(context.Canceled) }
	}
	timeout := policy.timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if timeout <= 0 {
		timeout = DefaultUpstreamDrainTimeout
	}
	attempt := &upstreamDrainAttempt{
		cancel: cancel, timeout: timeout,
		maxDuration: max(maxUpstreamStreamDrainDuration, timeout),
	}
	ctx = context.WithValue(ctx, upstreamDrainAttemptKey{}, attempt)
	startDrain := func() {
		attempt.mu.Lock()
		defer attempt.mu.Unlock()
		if !attempt.closed {
			attempt.startedAt = time.Now()
			attempt.lastProgress = attempt.startedAt
			attempt.timer = time.AfterFunc(timeout, attempt.expireIdle)
			if attempt.stream {
				attempt.limit = time.AfterFunc(attempt.maxDuration, attempt.expireTotal)
			}
		}
	}
	stop := context.AfterFunc(policy.client, startDrain)
	return ctx, func() {
		stop()
		attempt.mu.Lock()
		attempt.closed = true
		if attempt.timer != nil {
			attempt.timer.Stop()
		}
		if attempt.limit != nil {
			attempt.limit.Stop()
		}
		attempt.mu.Unlock()
		cancel(context.Canceled)
	}
}

func (a *upstreamDrainAttempt) expireIdle() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return
	}
	// A read can reset a timer whose callback is already waiting for this lock.
	if a.stream {
		if remaining := a.timeout - time.Since(a.lastProgress); remaining > 0 {
			a.timer.Reset(remaining)
			return
		}
	}
	a.cancel(ErrUpstreamDrainTimeout)
}

func (a *upstreamDrainAttempt) expireTotal() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.closed {
		a.cancel(ErrUpstreamDrainTimeout)
	}
}

// TrackUpstreamStreamDrain keeps active SSE output alive long enough to receive
// final usage. Even endless pings cannot extend the separate total drain limit.
func TrackUpstreamStreamDrain(ctx context.Context, body io.ReadCloser) io.ReadCloser {
	attempt, ok := ctx.Value(upstreamDrainAttemptKey{}).(*upstreamDrainAttempt)
	if !ok || body == nil {
		return body
	}
	attempt.mu.Lock()
	if !attempt.closed && !attempt.stream {
		attempt.stream = true
		if !attempt.startedAt.IsZero() {
			attempt.limit = time.AfterFunc(max(0, attempt.maxDuration-time.Since(attempt.startedAt)), attempt.expireTotal)
		}
	}
	attempt.mu.Unlock()
	return &upstreamStreamDrainBody{ReadCloser: body, ctx: ctx, attempt: attempt}
}

type upstreamStreamDrainBody struct {
	io.ReadCloser
	ctx     context.Context
	attempt *upstreamDrainAttempt
}

func (b *upstreamStreamDrainBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.attempt.mu.Lock()
		if !b.attempt.closed && b.attempt.timer != nil {
			b.attempt.lastProgress = time.Now()
			b.attempt.timer.Reset(b.attempt.timeout)
		}
		b.attempt.mu.Unlock()
	}
	if err != nil && errors.Is(context.Cause(b.ctx), ErrUpstreamDrainTimeout) {
		return n, fmt.Errorf("%w: %w", ErrUpstreamDrainTimeout, err)
	}
	return n, err
}
