package service

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountFailoverStreamErrorPolicy(t *testing.T) {
	for _, count := range []int{0, 1, 3} {
		ctx := withRequestAccountFailoverCount(context.Background(), count)
		for _, tc := range []struct {
			name  string
			err   error
			retry bool
		}{
			{"missing terminal", errUpstreamStreamMissingTerminal, true},
			{"connection interrupted", upstreamStreamReadError(io.ErrUnexpectedEOF), true},
			{"openai read error", newOpenAIUpstreamStreamReadError(io.ErrUnexpectedEOF), true},
			{"balance", errors.New("insufficient balance"), false},
			{"parameters", errors.New("invalid parameter"), false},
			{"untyped text is not a classifier", errors.New("stream usage incomplete: missing terminal event"), false},
			{"canceled", upstreamStreamReadError(context.Canceled), false},
			{"deadline", upstreamStreamReadError(context.DeadlineExceeded), false},
			{"size limit", upstreamStreamReadError(bufio.ErrTooLong), false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				got := requestAccountFailoverError(ctx, tc.err)
				var failure *UpstreamFailoverError
				require.Equal(t, count > 1 && tc.retry, errors.As(got, &failure))
				require.ErrorIs(t, got, tc.err)
				if failure != nil {
					require.True(t, failure.ShouldRetryNextAccount())
					require.True(t, failure.SafeToFailoverAfterWrite)
				}
			})
		}
	}
	ctx := withRequestAccountFailoverCount(context.Background(), 3)
	stop := &UpstreamFailoverError{Cause: errUpstreamStreamMissingTerminal, NextAccountAction: NextAccountStop}
	require.Same(t, stop, requestAccountFailoverError(ctx, stop))
	require.False(t, stop.ShouldRetryNextAccount())
}

func TestAccountFailoverMissingTerminalAcrossStreamReaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
	openai := &OpenAIGatewayService{cfg: cfg}
	anthropic := &GatewayService{cfg: cfg}
	gemini := &GeminiMessagesCompatService{}
	ag := &AntigravityGatewayService{settingService: NewSettingService(nil, cfg)}
	account := &Account{ID: 1, Type: AccountTypeAPIKey}
	for _, name := range []string{"anthropic", "anthropic passthrough", "openai responses", "openai passthrough", "gemini messages", "gemini native", "antigravity messages"} {
		t.Run(name, func(t *testing.T) {
			ctx := withRequestAccountFailoverCount(context.Background(), 2)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
			payload := "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":7}}}\n\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"partial\"}}\n\n"
			if strings.HasPrefix(name, "openai") {
				payload = "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\",\"output_index\":0}\n\n"
			}
			if strings.HasPrefix(name, "gemini") || strings.HasPrefix(name, "antigravity") {
				payload = "data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"partial\"}]}}],\"usageMetadata\":{\"promptTokenCount\":7}}}\n\n"
			}
			resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(payload))}
			_, err := ForwardWithAccountRetries(ctx, c, func() (*ForwardResult, error) {
				var err error
				switch name {
				case "anthropic":
					_, err = anthropic.handleStreamingResponse(ctx, resp, c, account, time.Now(), "model", "model", false)
				case "anthropic passthrough":
					_, err = anthropic.handleStreamingResponseAnthropicAPIKeyPassthrough(ctx, resp, c, account, time.Now(), "model")
				case "openai responses":
					_, err = openai.handleStreamingResponse(ctx, resp, c, account, time.Now(), "model", "model")
				case "openai passthrough":
					_, err = openai.handleStreamingResponsePassthrough(ctx, resp, c, account, time.Now(), "model", "model")
				case "gemini messages":
					_, err = gemini.handleStreamingResponse(c, resp, time.Now(), "model")
				case "gemini native":
					_, err = gemini.handleNativeStreamingResponse(c, resp, time.Now(), true, account, "")
				case "antigravity messages":
					_, err = ag.handleClaudeStreamingResponse(c, resp, time.Now(), "model")
				}
				return nil, err
			})
			var failure *UpstreamFailoverError
			require.ErrorAs(t, err, &failure)
			require.True(t, failure.SafeToFailoverAfterWrite)
			require.Contains(t, recorder.Body.String(), "partial")
			require.NotContains(t, recorder.Body.String(), "[DONE]")
		})
	}
}

func TestAccountAttemptUsageDedup(t *testing.T) {
	ctx := withRequestAccountFailoverCount(context.Background(), 3)
	first := accountAttemptUsageBillingRequestID(ctx, "local:request", 11)
	require.Equal(t, first, accountAttemptUsageBillingRequestID(ctx, "local:request", 11))
	require.NotEqual(t, first, accountAttemptUsageBillingRequestID(ctx, "local:request", 12))
	require.Equal(t, "web_search:task", accountAttemptUsageBillingRequestID(ctx, "web_search:task", 11))
	require.Equal(t, "local:request", accountAttemptUsageBillingRequestID(withRequestAccountFailoverCount(ctx, 1), "local:request", 11))
	require.False(t, (*ForwardResult)(nil).HasObservedUsage())
	require.False(t, (&OpenAIForwardResult{}).HasObservedUsage())
	require.True(t, (&ForwardResult{Usage: ClaudeUsage{InputTokens: 7}}).HasObservedUsage())
	require.True(t, (&OpenAIForwardResult{Usage: OpenAIUsage{InputTokens: 7}}).HasObservedUsage())
}
