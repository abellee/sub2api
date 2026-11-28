package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func conversationFailoverTestContext(t *testing.T) (context.Context, *gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	ctx := withRequestAccountFailoverCount(t.Context(), 3)
	ctx = context.WithValue(ctx, ctxkey.AccountID, int64(1))
	trace, ok := ctx.Value(requestAccountFailoverTraceKey{}).(*RequestAccountFailoverTrace)
	require.True(t, ok)
	trace.timeout = 20 * time.Millisecond
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
	return ctx, c, recorder
}

func awaitConversationSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal("conversation worker did not finish")
	}
}

func TestConversationFailoverBillingKeepsWorkerAndAccountSlot(t *testing.T) {
	ctx, c, recorder := conversationFailoverTestContext(t)
	MarkRequestAccountFailoverAction(ctx, "switch_account", 3)
	allowLate := make(chan struct{})
	defer func() {
		select {
		case <-allowLate:
		default:
			close(allowLate)
		}
	}()
	billed := make(chan *ForwardResult, 1)
	released := make(chan struct{})
	result, err := ForwardConversationWithAccountRetries(ctx, c, []byte(`{"model":"claude"}`), "claude", func(attemptCtx context.Context, attemptC *gin.Context) (*ForwardResult, error) {
		<-allowLate
		if attemptCtx.Err() != nil {
			return nil, attemptCtx.Err()
		}
		_, _ = attemptC.Writer.WriteString("late answer")
		return &ForwardResult{Usage: ClaudeUsage{InputTokens: 25}}, nil
	}, func(billingCtx context.Context, result *ForwardResult) {
		if billingCtx.Err() == nil {
			billed <- result
		}
	})
	require.Nil(t, result)
	require.Error(t, err)
	ReleaseAfterConversationAttempt(ctx, func() { close(released) })
	select {
	case <-released:
		t.Fatal("released account slot while old upstream was still running")
	default:
	}
	nextCtx := context.WithValue(ctx, ctxkey.AccountID, int64(2))
	c.Request = c.Request.WithContext(nextCtx)
	next, nextErr := ForwardConversationWithAccountRetries(nextCtx, c, nil, "claude", func(_ context.Context, attemptC *gin.Context) (*ForwardResult, error) {
		_, _ = attemptC.Writer.WriteString("new answer")
		return &ForwardResult{}, nil
	}, nil)
	require.NoError(t, nextErr)
	require.NotNil(t, next)
	close(allowLate)
	awaitConversationSignal(t, released)
	select {
	case late := <-billed:
		require.Equal(t, 25, late.Usage.InputTokens)
	case <-time.After(2 * time.Second):
		t.Fatal("late usage was not billed")
	}
	require.Equal(t, "new answer", recorder.Body.String())
	select {
	case <-billed:
		t.Fatal("late attempt billed twice")
	default:
	}
}

func TestConversationFailoverBillingKeepsReportedUsageAfterError(t *testing.T) {
	ctx, c, _ := conversationFailoverTestContext(t)
	MarkRequestAccountFailoverAction(ctx, "switch_account", 3)
	allowLate := make(chan struct{})
	billed := make(chan struct{}, 1)
	_, _ = ForwardConversationWithAccountRetries(ctx, c, nil, "gpt-5", func(context.Context, *gin.Context) (*OpenAIForwardResult, error) {
		<-allowLate
		return &OpenAIForwardResult{Usage: OpenAIUsage{InputTokens: 17}}, errors.New("upstream stream ended")
	}, func(context.Context, *OpenAIForwardResult) { billed <- struct{}{} })
	close(allowLate)
	select {
	case <-billed:
	case <-time.After(2 * time.Second):
		t.Fatal("reported usage was lost")
	}
}

func TestConversationFailoverCompletesNormallyAndCopiesMetadata(t *testing.T) {
	ctx, c, recorder := conversationFailoverTestContext(t)
	result, err := ForwardConversationWithAccountRetries(ctx, c, nil, "gpt-5", func(_ context.Context, attemptC *gin.Context) (*ForwardResult, error) {
		attemptC.Header("X-Upstream", "selected")
		attemptC.Set("upstream_meta", "value")
		attemptC.Status(http.StatusCreated)
		_, _ = attemptC.Writer.WriteString("answer")
		return &ForwardResult{}, nil
	}, func(context.Context, *ForwardResult) { t.Error("foreground success must not be billed as late") })
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "answer", recorder.Body.String())
	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Equal(t, "selected", recorder.Header().Get("X-Upstream"))
	require.Equal(t, "value", c.GetString("upstream_meta"))
}

func TestConversationFailoverExcludesMediaAndBackgroundTasks(t *testing.T) {
	for _, body := range []string{
		`{"model":"gpt-image-1"}`, `{"model":"sora-2"}`, `{"model":"gpt-audio"}`,
		`{"model":"gemini","generationConfig":{"responseModalities":["TEXT","IMAGE"]}}`,
		`{"model":"gpt-5","tools":[{"type":"image_generation"}]}`,
		`{"model":"gpt-5","modalities":["text","audio"]}`, `{"model":"gpt-5","background":true}`,
	} {
		ctx, c, _ := conversationFailoverTestContext(t)
		_, err := ForwardConversationWithAccountRetries(ctx, c, []byte(body), "", func(attemptCtx context.Context, attemptC *gin.Context) (*ForwardResult, error) {
			require.Same(t, c, attemptC, "media must keep the original forwarding path")
			require.Equal(t, ctx, attemptCtx)
			return &ForwardResult{}, nil
		}, nil)
		require.NoError(t, err)
	}
	require.True(t, conversationFailoverTimeoutAllowed(nil, []byte(`{"model":"gpt-5","input":[{"type":"input_image"}]}`), "gpt-5"))
	require.False(t, conversationFailoverTimeoutAllowed(nil, []byte(`{"model":"alias"}`), "gemini-image"))
}

func TestConversationFailoverCancellationRemainsCancellation(t *testing.T) {
	parent, c, _ := conversationFailoverTestContext(t)
	ctx, cancel := context.WithCancel(parent)
	c.Request = c.Request.WithContext(ctx)
	_, err := ForwardConversationWithAccountRetries(ctx, c, nil, "gpt-5", func(attemptCtx context.Context, _ *gin.Context) (*ForwardResult, error) {
		cancel()
		return nil, context.Canceled
	}, nil)
	require.ErrorIs(t, err, context.Canceled)
}

func TestConversationFailoverFirstAttemptHasNoResponseDeadline(t *testing.T) {
	ctx, c, recorder := conversationFailoverTestContext(t)
	result, err := ForwardConversationWithAccountRetries(ctx, c, nil, "gpt-5", func(_ context.Context, attemptC *gin.Context) (*ForwardResult, error) {
		<-time.After(80 * time.Millisecond)
		_, _ = attemptC.Writer.WriteString("first answer")
		return &ForwardResult{}, nil
	}, func(context.Context, *ForwardResult) { t.Error("initial attempt must finish in the foreground") })
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "first answer", recorder.Body.String())
}

func TestConversationFailoverDeadlineStartsAfterErrorAndSwitch(t *testing.T) {
	ctx, c, _ := conversationFailoverTestContext(t)
	_, err := ForwardConversationWithAccountRetries(ctx, c, nil, "gpt-5", func(context.Context, *gin.Context) (*ForwardResult, error) {
		return nil, &UpstreamFailoverError{StatusCode: http.StatusServiceUnavailable}
	}, nil)
	require.Error(t, err)
	MarkRequestAccountFailoverAction(ctx, "switch_account", 1)
	nextCtx := context.WithValue(ctx, ctxkey.AccountID, int64(2))
	c.Request = c.Request.WithContext(nextCtx)
	finish := make(chan struct{})
	defer close(finish)
	_, err = ForwardConversationWithAccountRetries(nextCtx, c, nil, "gpt-5", func(context.Context, *gin.Context) (*ForwardResult, error) {
		<-finish
		return nil, errors.New("late error")
	}, nil)
	var failure *UpstreamFailoverError
	require.ErrorAs(t, err, &failure)
	require.Equal(t, http.StatusGatewayTimeout, failure.StatusCode)
	require.True(t, failure.ShouldRetryNextAccount())
}

func TestConversationFailoverOutputStopsDeadline(t *testing.T) {
	for _, protocol := range []string{"responses", "chat", "anthropic", "gemini", "json"} {
		t.Run(protocol, func(t *testing.T) {
			ctx, c, recorder := conversationFailoverTestContext(t)
			MarkRequestAccountFailoverAction(ctx, "switch_account", 3)
			result, err := ForwardConversationWithAccountRetries(ctx, c, nil, "gpt-5", func(_ context.Context, attemptC *gin.Context) (*ForwardResult, error) {
				attemptC.Header("Content-Type", "text/event-stream")
				switch protocol {
				case "responses":
					markLatencyStreamEvent(attemptC, true)
				case "chat":
					observeGameplayChatOutput(attemptC, `{"choices":[{"delta":{"content":"answer"}}]}`)
				case "anthropic":
					observeGameplayAnthropicOutput(attemptC, `{"delta":{"thinking":"thinking"}}`)
				case "gemini":
					observeGameplayGeminiOutput(attemptC, []byte(`{"candidates":[{"content":{"parts":[{"functionCall":{"name":"tool"}}]}}]}`))
				case "json":
					attemptC.Header("Content-Type", "application/json")
				}
				_, _ = attemptC.Writer.WriteString("answer")
				attemptC.Writer.Flush()
				<-time.After(80 * time.Millisecond)
				_, _ = attemptC.Writer.WriteString(" finished")
				return &ForwardResult{}, nil
			}, func(context.Context, *ForwardResult) {
				t.Error("responding attempt must not become a background attempt")
			})
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, "answer finished", recorder.Body.String())
		})
	}
}

func TestConversationFailoverHeartbeatsDoNotStopDeadline(t *testing.T) {
	ctx, c, recorder := conversationFailoverTestContext(t)
	MarkRequestAccountFailoverAction(ctx, "switch_account", 3)
	finish := make(chan struct{})
	finished := make(chan struct{})
	_, err := ForwardConversationWithAccountRetries(ctx, c, nil, "gpt-5", func(_ context.Context, attemptC *gin.Context) (*ForwardResult, error) {
		defer close(finished)
		attemptC.Header("Content-Type", "text/event-stream")
		attemptC.Writer.WriteHeaderNow()
		_, _ = attemptC.Writer.WriteString(": ping\n\n")
		observeGameplayAnthropicOutput(attemptC, `{"type":"ping"}`)
		observeGameplayChatOutput(attemptC, `{"choices":[{"delta":{"role":"assistant"}}]}`)
		observeGameplayGeminiOutput(attemptC, []byte(`{"usageMetadata":{"promptTokenCount":1}}`))
		markLatencyStreamEvent(attemptC, false)
		<-finish
		observeGameplayFirstOutput(attemptC)
		_, _ = attemptC.Writer.WriteString("late answer")
		return &ForwardResult{}, nil
	}, nil)
	close(finish)
	awaitConversationSignal(t, finished)
	var failure *UpstreamFailoverError
	require.ErrorAs(t, err, &failure)
	require.Equal(t, http.StatusGatewayTimeout, failure.StatusCode)
	require.Equal(t, ": ping\n\n", recorder.Body.String())
	require.False(t, c.GetBool(conversationOutputStartedKey))
}

func TestConversationFailoverDoesNotSwitchAfterOutputError(t *testing.T) {
	for _, retrying := range []bool{false, true} {
		t.Run(map[bool]string{false: "initial", true: "retry"}[retrying], func(t *testing.T) {
			ctx, c, recorder := conversationFailoverTestContext(t)
			if retrying {
				MarkRequestAccountFailoverAction(ctx, "switch_account", 3)
			}
			_, err := ForwardConversationWithAccountRetries(ctx, c, nil, "gpt-5", func(_ context.Context, attemptC *gin.Context) (*ForwardResult, error) {
				_, _ = attemptC.Writer.WriteString("partial answer")
				return &ForwardResult{Usage: ClaudeUsage{OutputTokens: 2}}, upstreamStreamReadError(errors.New("connection reset"))
			}, nil)
			var failure *UpstreamFailoverError
			require.ErrorAs(t, err, &failure)
			require.False(t, failure.ShouldRetryNextAccount())
			require.False(t, failure.SafeToFailoverAfterWrite)
			require.Equal(t, "failed", RequestAccountFailoverResult(c))
			require.Equal(t, "partial answer", recorder.Body.String())
		})
	}
}

func TestConversationFailoverOutputWinsAtDeadline(t *testing.T) {
	_, c, _ := conversationFailoverTestContext(t)
	w := newConversationAttemptWriter(c.Writer)
	w.markOutput()
	require.False(t, w.detachBeforeOutput())
	require.False(t, w.detached)
	require.True(t, w.outputStarted())
}
