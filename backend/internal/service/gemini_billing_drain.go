package service

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
)

// A dispatched Gemini stream can incur upstream charges before usageMetadata
// arrives. Keep that request alive after the client leaves, but bound the time
// spent collecting its final usage. Never use this context to start a retry.
type geminiBillingDrain struct {
	client     context.Context
	cancel     context.CancelFunc
	stopWatch  func() bool
	mu         sync.Mutex
	timer      *time.Timer
	closed     bool
	gone       atomic.Bool
	timedOut   atomic.Bool
	dispatched bool
	writer     gin.ResponseWriter
	limit      time.Duration
}

func (s *GeminiMessagesCompatService) beginBillingDrain(ctx context.Context, c *gin.Context, stream bool) (context.Context, *geminiBillingDrain) {
	var cfg *config.Config
	if s != nil {
		cfg = s.cfg
	}
	return beginGeminiBillingDrain(ctx, c, stream, cfg)
}

func beginGeminiBillingDrain(ctx context.Context, c *gin.Context, stream bool, cfg *config.Config) (context.Context, *geminiBillingDrain) {
	d := &geminiBillingDrain{client: ctx}
	if !stream {
		return ctx, d
	}
	d.limit = 180 * time.Second
	if cfg != nil && cfg.Gateway.StreamDataIntervalTimeout > 0 {
		d.limit = time.Duration(cfg.Gateway.StreamDataIntervalTimeout) * time.Second
	}
	upstream, cancel := context.WithCancel(context.WithoutCancel(ctx))
	d.cancel = cancel
	// Production ctx is the inbound request context. Also watch c.Request when
	// a caller supplies a context carrying additional forwarding values.
	client := ctx
	if c != nil && c.Request != nil {
		client = c.Request.Context()
	}
	d.client = client
	d.stopWatch = context.AfterFunc(client, d.disconnect)
	if c != nil {
		d.writer = c.Writer
		c.Writer = &geminiDrainWriter{ResponseWriter: c.Writer, drain: d}
	}
	return upstream, d
}

func (d *geminiBillingDrain) disconnect() {
	d.gone.Store(true)
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.closed && d.timer == nil && d.cancel != nil {
		d.timer = time.AfterFunc(d.limit, func() {
			d.timedOut.Store(true)
			d.cancel()
		})
	}
}

func (d *geminiBillingDrain) disconnected() bool {
	if d.cancel == nil {
		return false
	}
	if d.client.Err() != nil {
		d.disconnect()
	}
	return d.gone.Load()
}

// finish runs synchronously while the gin context is still owned by the
// handler. The watcher only touches atomic/mutex-protected lifecycle state.
func (d *geminiBillingDrain) finish(c *gin.Context, account *Account, result *ForwardResult) {
	if d.cancel == nil {
		return
	}
	gone := d.disconnected()
	d.stopWatch()
	d.mu.Lock()
	d.closed = true
	if d.timer != nil {
		d.timer.Stop()
	}
	d.mu.Unlock()
	d.cancel()
	c.Writer = d.writer
	if !gone || !d.dispatched {
		return
	}
	if result != nil {
		result.ClientDisconnect = true
	}
	if d.timedOut.Load() || result == nil || (!result.Usage.hasObservedTokens() && result.ImageCount == 0 && result.SearchCount == 0) {
		// HTTP 200 may already be committed. Mark a request-scoped exception so
		// the billing gap remains visible without blaming provider availability.
		MarkOpsStreamErrorValue(c, OpsStreamError{
			ErrType:        "billing_usage_unavailable",
			Code:           "gemini_usage_unavailable_after_disconnect",
			Message:        "Gemini client disconnected; upstream billing usage could not be collected",
			IntendedStatus: 499,
			RequestScoped:  true,
		})
		logger.LegacyPrintf("service.gemini_billing", "Gemini account %d: client disconnected, upstream usage unavailable; billing reconciliation required", account.ID)
		return
	}
	logger.LegacyPrintf("service.gemini_billing", "Gemini account %d: client disconnected, collected upstream usage for billing (input=%d output=%d cache_read=%d)", account.ID, result.Usage.InputTokens, result.Usage.OutputTokens, result.Usage.CacheReadInputTokens)
}

// Swallow downstream write failures only after marking the client gone. This
// lets existing Gemini converters continue parsing usage instead of returning
// on the first failed write. No response bytes are retained while draining.
type geminiDrainWriter struct {
	gin.ResponseWriter
	drain *geminiBillingDrain
}

func (w *geminiDrainWriter) Write(p []byte) (int, error) {
	if !w.drain.disconnected() {
		if n, err := w.ResponseWriter.Write(p); err == nil && n == len(p) {
			return n, nil
		}
		w.drain.disconnect()
	}
	return len(p), nil
}

func (w *geminiDrainWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *geminiDrainWriter) Flush() {
	if !w.drain.disconnected() {
		w.ResponseWriter.Flush()
	}
}

func (w *geminiDrainWriter) WriteHeader(code int) {
	if !w.drain.disconnected() {
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *geminiDrainWriter) WriteHeaderNow() {
	if !w.drain.disconnected() {
		w.ResponseWriter.WriteHeaderNow()
	}
}
