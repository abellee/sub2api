package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type accountRetryStreamHandler func(*http.Response, *gin.Context) (bool, error)

func accountRetryStreamHandlers(interval ...int) map[string]accountRetryStreamHandler {
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
	if len(interval) > 0 {
		cfg.Gateway.StreamDataIntervalTimeout = interval[0]
	}
	gateway := &GatewayService{cfg: cfg, rateLimitService: &RateLimitService{}}
	openai := &OpenAIGatewayService{cfg: cfg}
	account := &Account{ID: 42, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
	return map[string]accountRetryStreamHandler{
		"anthropic messages": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := gateway.handleStreamingResponse(c.Request.Context(), r, c, account, time.Now(), "model", "model", false)
			return v != nil, err
		},
		"anthropic passthrough": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := gateway.handleStreamingResponseAnthropicAPIKeyPassthrough(c.Request.Context(), r, c, account, time.Now(), "model")
			return v != nil, err
		},
		"anthropic chat streaming": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := gateway.handleCCStreamingFromAnthropic(r, c, "model", "model", nil, time.Now())
			return v != nil, err
		},
		"anthropic responses streaming": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := gateway.handleResponsesStreamingResponse(r, c, "model", "model", nil, time.Now(), apicompat.ResponsesClientToolMapping{})
			return v != nil, err
		},
		"anthropic chat buffered": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := gateway.handleCCBufferedFromAnthropic(r, c, "model", "model", nil, time.Now())
			return v != nil, err
		},
		"anthropic responses buffered": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := gateway.handleResponsesBufferedStreamingResponse(r, c, "model", "model", nil, time.Now(), apicompat.ResponsesClientToolMapping{})
			return v != nil, err
		},
		"native messages": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := openai.handleNativeAnthropicStreamingResponse(c.Request.Context(), r, c, account, "model", "model", "model", nil, time.Now())
			return v != nil, err
		},
		"native chat streaming": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := openai.handleCCStreamingFromNativeAnthropic(r, c, "model", "model", "model", nil, time.Now())
			return v != nil, err
		},
		"native responses streaming": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := openai.handleResponsesStreamingFromNativeAnthropic(r, c, "model", "model", "model", nil, time.Now(), apicompat.ResponsesClientToolMapping{})
			return v != nil, err
		},
		"native chat buffered": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := openai.handleCCBufferedFromNativeAnthropic(r, c, "model", "model", "model", nil, time.Now())
			return v != nil, err
		},
		"native responses buffered": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := openai.handleResponsesBufferedFromNativeAnthropic(r, c, "model", "model", "model", nil, time.Now(), apicompat.ResponsesClientToolMapping{})
			return v != nil, err
		},
		"openai chat": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := openai.handleChatStreamingResponse(r, c, account, "model", "model", "model", time.Now(), 0)
			return v != nil, err
		},
		"openai messages": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := openai.handleAnthropicStreamingResponse(r, c, account, "model", "model", "model", time.Now())
			return v != nil, err
		},
		"openai responses": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := openai.handleStreamingResponse(c.Request.Context(), r, c, account, time.Now(), "model", "model")
			return v != nil, err
		},
		"openai passthrough": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := openai.handleStreamingResponsePassthrough(c.Request.Context(), r, c, account, time.Now(), "model", "model")
			return v != nil, err
		},
		"openai chat buffered": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := openai.handleChatBufferedStreamingResponse(r, c, account, "model", "model", "model", time.Now())
			return v != nil, err
		},
		"openai messages buffered": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := openai.handleAnthropicBufferedStreamingResponse(r, c, account, "model", "model", "model", time.Now())
			return v != nil, err
		},
		"openai raw chat": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := openai.streamRawChatCompletions(c, r, account, "model", "model", "model", nil, nil, time.Now(), 0)
			return v != nil, err
		},
		"openai raw chat json": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := openai.bufferRawChatCompletions(c, r, account, "model", "model", "model", nil, nil, time.Now())
			return v != nil, err
		},
		"openai json": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := openai.handleNonStreamingResponse(c.Request.Context(), r, c, account, "model", "model")
			return v != nil, err
		},
		"openai passthrough json": func(r *http.Response, c *gin.Context) (bool, error) {
			v, err := openai.handleNonStreamingResponsePassthrough(c.Request.Context(), r, c, account, "model", "model")
			return v != nil, err
		},
	}
}

func TestAccountStreamOutputNeverReplayed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, handle := range accountRetryStreamHandlers() {
		if strings.Contains(name, "buffered") || strings.HasPrefix(name, "openai") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := withRequestAccountFailoverCount(context.Background(), 2)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
			stream := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_partial\",\"role\":\"assistant\",\"content\":[],\"usage\":{\"input_tokens\":23}}}\n\n" +
				"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
				"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n"
			calls := 0
			_, err := ForwardWithAccountRetries(ctx, c, func() (*ForwardResult, error) {
				calls++
				resp := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(stream))}
				defer func() { _ = resp.Body.Close() }()
				_, err := handle(resp, c)
				return nil, err
			})
			require.Error(t, err)
			require.Equal(t, 1, calls)
			require.Equal(t, 1, strings.Count(rec.Body.String(), "hello"))
		})
	}
}

func TestAccountStreamFailuresRetryBeforeOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, handle := range accountRetryStreamHandlers() {
		for _, failure := range []struct {
			name string
			err  error
		}{
			{"empty EOF", io.EOF},
			{"unexpected EOF", io.ErrUnexpectedEOF},
			{"reset", errors.New("connection reset by peer")},
			{"upstream timeout", context.DeadlineExceeded},
			{"upstream canceled", context.Canceled},
		} {
			t.Run(name+"/"+failure.name, func(t *testing.T) {
				t.Parallel()
				ctx := withRequestAccountFailoverCount(context.Background(), 2)
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
				calls := 0
				_, err := ForwardWithAccountRetries(ctx, c, func() (*ForwardResult, error) {
					calls++
					resp := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(iotest.ErrReader(failure.err))}
					defer func() { _ = resp.Body.Close() }()
					partial, err := handle(resp, c)
					if partial {
						return &ForwardResult{}, err
					}
					return nil, err
				})
				require.Equal(t, 3, calls)
				var retry *UpstreamFailoverError
				require.ErrorAs(t, err, &retry)
				require.True(t, retry.SkipAccountCooldown)
				require.Empty(t, rec.Body.String(), "failed attempts must not leak error frames")
				require.False(t, IsResponseCommitted(c))
			})
		}
	}
}

func TestAccountStreamBufferedPartialUsageCanRecover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, handle := range accountRetryStreamHandlers() {
		if !strings.Contains(name, "buffered") || strings.HasPrefix(name, "openai") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := withRequestAccountFailoverCount(context.Background(), 2)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
			start := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_retry\",\"role\":\"assistant\",\"content\":[],\"usage\":{\"input_tokens\":23}}}\n\n"
			calls := 0
			_, err := ForwardWithAccountRetries(ctx, c, func() (*ForwardResult, error) {
				calls++
				body := start
				if calls == 3 {
					body += "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":1}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
				}
				resp := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
				defer func() { _ = resp.Body.Close() }()
				partial, err := handle(resp, c)
				if partial {
					return &ForwardResult{}, err
				}
				return nil, err
			})
			require.NoError(t, err)
			require.Equal(t, 3, calls)
			require.NotContains(t, rec.Body.String(), `"error"`)
		})
	}
}

func TestAccountStreamFailureRespectsClientAndOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name     string
		accounts int
		cancel   bool
		output   bool
	}{
		{"client canceled", 2, true, false},
		{"partial response", 2, false, true},
		{"single account", 1, false, false},
		{"disabled", 0, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(withRequestAccountFailoverCount(context.Background(), tt.accounts))
			defer cancel()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
			if tt.cancel {
				cancel()
			}
			if tt.output {
				_, _ = c.Writer.WriteString("data: partial answer\n\n")
			}
			require.Nil(t, requestAccountStreamFailover(c, nil, nil, io.ErrUnexpectedEOF))
		})
	}
}

func TestMissingTerminalEventCanFailoverAfterOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(
		withRequestAccountFailoverCount(context.Background(), 2),
	)
	_, _ = c.Writer.WriteString("event: content_block_delta\\ndata: partial\\n\\n")

	failure := requestAccountStreamFailover(c, nil, nil, errors.New("upstream stream ended without a terminal event"))
	require.NotNil(t, failure)
	require.True(t, failure.SafeToFailoverAfterWrite)

	otherFailure := requestAccountStreamFailover(c, nil, nil, errors.New("stream data interval timeout"))
	require.Nil(t, otherFailure)
}

func TestAccountStreamIdleRetriesBeforeOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, handle := range accountRetryStreamHandlers(1) {
		if strings.Contains(name, "json") || strings.Contains(name, "raw") || name == "openai passthrough" || name == "anthropic chat buffered" || name == "anthropic responses buffered" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(withRequestAccountFailoverCount(context.Background(), 2), 15*time.Second)
			defer cancel()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
			calls := 0
			_, err := ForwardWithAccountRetries(ctx, c, func() (*ForwardResult, error) {
				calls++
				reader, writer := io.Pipe()
				defer func() { _ = reader.Close(); _ = writer.Close() }()
				partial, err := handle(&http.Response{StatusCode: 200, Header: http.Header{}, Body: reader}, c)
				if partial {
					return &ForwardResult{}, err
				}
				return nil, err
			})
			var retry *UpstreamFailoverError
			require.ErrorAs(t, err, &retry)
			require.Equal(t, http.StatusGatewayTimeout, retry.StatusCode)
			require.Equal(t, 3, calls)
			require.False(t, c.Writer.Written())
		})
	}
}

func TestAccountStreamExplicitStopPreserved(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(withRequestAccountFailoverCount(context.Background(), 2))
	stop := &UpstreamFailoverError{StatusCode: 400, NextAccountAction: NextAccountStop}
	result := &ForwardResult{}
	err := error(stop)
	finishRequestAccountStream(c, nil, nil, &result, &err)
	require.Same(t, stop, err)
	require.NotNil(t, result)
	require.Nil(t, requestAccountStreamFailover(c, nil, nil, &sseStreamErrorEventError{RawData: `{"type":"error","error":{"type":"invalid_request_error","message":"invalid request"}}`}))
}

func TestAccountStreamRawUpstreamTimeoutAfterOutputIsFailure(t *testing.T) {
	ctx := withRequestAccountFailoverCount(context.Background(), 2)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)
	stream := "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"}}]}\n\n"
	upstream := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(io.MultiReader(strings.NewReader(stream), iotest.ErrReader(context.DeadlineExceeded)))}
	defer func() { _ = upstream.Body.Close() }()
	calls := 0
	_, err := ForwardWithAccountRetries(ctx, c, func() (*ForwardResult, error) {
		calls++
		_, err := accountRetryStreamHandlers()["openai raw chat"](upstream, c)
		return nil, err
	})
	require.Error(t, err)
	require.Equal(t, 1, calls)
	require.True(t, c.Writer.Written())
}
