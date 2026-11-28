package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

type conversationAttemptKey struct{}

func ConversationBackgroundBillingEnabled(ctx context.Context) bool {
	trace, _ := ctx.Value(requestAccountFailoverTraceKey{}).(*RequestAccountFailoverTrace)
	return trace != nil && trace.timeout > 0
}

// Media requests keep their existing lifetime, including media tools invoked
// through conversation endpoints. Input images alone are still conversations.
func conversationFailoverTimeoutAllowed(c *gin.Context, body []byte, upstreamModel string) bool {
	model := strings.ToLower(upstreamModel + " " + gjson.GetBytes(body, "model").String())
	if c != nil && c.Request != nil {
		model += " " + strings.ToLower(c.Request.URL.Path)
		if strings.HasSuffix(c.Request.URL.Path, ":countTokens") {
			return false
		}
	}
	for _, marker := range []string{"image", "imagen", "nano-banana", "video", "veo", "sora", "seedance", "dall-e", "flux", "audio", "tts", "whisper"} {
		if strings.Contains(model, marker) {
			return false
		}
	}
	if gjson.GetBytes(body, "background").Bool() {
		return false
	}
	for _, path := range []string{"modalities", "generationConfig.responseModalities", "generation_config.response_modalities"} {
		for _, modality := range gjson.GetBytes(body, path).Array() {
			if !strings.EqualFold(modality.String(), "text") {
				return false
			}
		}
	}
	for _, tool := range gjson.GetBytes(body, "tools").Array() {
		for _, media := range []string{"image", "video", "audio"} {
			if strings.Contains(strings.ToLower(tool.Get("type").String()), media) {
				return false
			}
		}
	}
	return true
}

// Each attempt owns its context, Gin keys and response writer. A timed-out
// worker can finish accounting without touching the next attempt's response.
func ForwardConversationWithAccountRetries[T any](ctx context.Context, c *gin.Context, body []byte, upstreamModel string, forward func(context.Context, *gin.Context) (*T, error), billLate func(context.Context, *T)) (*T, error) {
	trace, _ := ctx.Value(requestAccountFailoverTraceKey{}).(*RequestAccountFailoverTrace)
	if trace == nil || trace.timeout <= 0 || !RequestAccountFailoverEnabled(ctx) || !conversationFailoverTimeoutAllowed(c, body, upstreamModel) || c == nil || c.Request == nil {
		return ForwardWithAccountRetries(ctx, c, func() (*T, error) { return forward(ctx, c) })
	}
	return ForwardWithAccountRetries(ctx, c, func() (*T, error) {
		base := context.WithoutCancel(ctx)
		attemptCtx, cancel := context.WithCancel(context.WithValue(base, conversationAttemptKey{}, true))
		attemptC := c.Copy()
		attemptC.Request = c.Request.Clone(attemptCtx)
		if events, ok := c.Get(OpsUpstreamErrorsKey); ok {
			if events, ok := events.([]*OpsUpstreamErrorEvent); ok {
				attemptC.Set(OpsUpstreamErrorsKey, append([]*OpsUpstreamErrorEvent(nil), events...))
			}
		}
		writer := newConversationAttemptWriter(c.Writer)
		attemptC.Writer = writer
		finished := make(chan struct{})
		accountID, _ := c.Request.Context().Value(ctxkey.AccountID).(int64)
		trace.mu.Lock()
		if trace.pending == nil {
			trace.pending = make(map[int64]<-chan struct{})
		}
		trace.pending[accountID] = finished
		trace.mu.Unlock()
		type outcome struct {
			result *T
			err    error
		}
		done := make(chan outcome, 1)
		finishInBackground := func() {
			go func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						logger.FromContext(ctx).Error("gateway.late_conversation_billing_panicked", zap.Any("panic", recovered))
					}
				}()
				completed := <-done
				billable := completed.err == nil
				switch result := any(completed.result).(type) {
				case *ForwardResult:
					billable = billable || result.HasObservedUsage()
				case *OpenAIForwardResult:
					billable = billable || result.HasObservedUsage()
				}
				if completed.result != nil && billable && billLate != nil {
					billLate(context.WithoutCancel(ctx), completed.result)
				}
			}()
		}
		go func() {
			defer close(finished)
			defer cancel()
			// Handler recovery cannot catch a panic in this worker.
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.FromContext(ctx).Error("gateway.conversation_attempt_panicked", zap.Any("panic", recovered))
					done <- outcome{err: errors.New("upstream conversation attempt panicked")}
				}
			}()
			result, err := forward(attemptCtx, attemptC)
			done <- outcome{result, err}
		}()
		timer := time.NewTimer(trace.timeout)
		defer timer.Stop()
		complete := func(completed outcome) (*T, error) {
			for key, value := range attemptC.Keys {
				c.Set(key, value)
			}
			return completed.result, completed.err
		}
		select {
		case completed := <-done:
			writer.detach()
			return complete(completed)
		case <-ctx.Done():
			writer.detach()
			cancel()
			// Preserve completed upstream events/usage on disconnect, while the
			// same attempt deadline still bounds how long the handler waits.
			select {
			case completed := <-done:
				return complete(completed)
			case <-timer.C:
				finishInBackground()
				return nil, ctx.Err()
			}
		case <-timer.C:
			writer.detach()
			if ctx.Err() != nil {
				finishInBackground()
				return nil, ctx.Err()
			}
			finishInBackground()
			message := "Upstream conversation attempt timed out"
			setOpsUpstreamError(c, http.StatusGatewayTimeout, message, "")
			accountID, _ := ctx.Value(ctxkey.AccountID).(int64)
			if selected, ok := c.Request.Context().Value(ctxkey.AccountID).(int64); ok {
				accountID = selected
			}
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{AccountID: accountID, Kind: "failover", UpstreamStatusCode: http.StatusGatewayTimeout, Message: message})
			return nil, &UpstreamFailoverError{
				Cause: context.DeadlineExceeded, StatusCode: http.StatusGatewayTimeout,
				ClientStatusCode: http.StatusGatewayTimeout, ClientMessage: message,
				NextAccountAction: NextAccountRetry, SkipAccountCooldown: true, SafeToFailoverAfterWrite: true,
			}
		}
	})
}

// Keep a background attempt's account slot until its upstream actually exits.
func ReleaseAfterConversationAttempt(ctx context.Context, release func()) {
	trace, _ := ctx.Value(requestAccountFailoverTraceKey{}).(*RequestAccountFailoverTrace)
	accountID, _ := ctx.Value(ctxkey.AccountID).(int64)
	if trace != nil && accountID > 0 {
		trace.mu.Lock()
		finished := trace.pending[accountID]
		trace.mu.Unlock()
		if finished != nil {
			select {
			case <-finished:
				release()
			default:
				go func() { <-finished; release() }()
			}
			return
		}
	}
	release()
}

type conversationAttemptWriter struct {
	gin.ResponseWriter
	mu       sync.Mutex
	header   http.Header
	detached bool
	status   int
	size     int
}

func newConversationAttemptWriter(target gin.ResponseWriter) *conversationAttemptWriter {
	return &conversationAttemptWriter{ResponseWriter: target, header: target.Header().Clone(), status: target.Status(), size: target.Size()}
}

func (w *conversationAttemptWriter) detach()             { w.mu.Lock(); w.detached = true; w.mu.Unlock() }
func (w *conversationAttemptWriter) Header() http.Header { return w.header }
func (w *conversationAttemptWriter) Status() int         { w.mu.Lock(); defer w.mu.Unlock(); return w.status }
func (w *conversationAttemptWriter) Size() int           { w.mu.Lock(); defer w.mu.Unlock(); return w.size }
func (w *conversationAttemptWriter) Written() bool       { return w.Size() >= 0 }
func (w *conversationAttemptWriter) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.size < 0 {
		w.status = status
	}
}
func (w *conversationAttemptWriter) commit() {
	if w.size >= 0 {
		return
	}
	w.size = 0
	if !w.detached {
		for key, values := range w.header {
			w.ResponseWriter.Header()[key] = append([]string(nil), values...)
		}
		w.ResponseWriter.WriteHeader(w.status)
		w.ResponseWriter.WriteHeaderNow()
	}
}
func (w *conversationAttemptWriter) WriteHeaderNow() { w.mu.Lock(); defer w.mu.Unlock(); w.commit() }
func (w *conversationAttemptWriter) Write(body []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.commit()
	n, err := len(body), error(nil)
	if !w.detached {
		n, err = w.ResponseWriter.Write(body)
	}
	w.size += n
	return n, err
}
func (w *conversationAttemptWriter) WriteString(body string) (int, error) {
	return w.Write([]byte(body))
}
func (w *conversationAttemptWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.commit()
	if !w.detached {
		w.ResponseWriter.Flush()
	}
}
func (w *conversationAttemptWriter) CloseNotify() <-chan bool { return make(chan bool) }
