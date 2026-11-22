package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const failoverPreamble = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":9}}}\n\n"
const failoverText = "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"answer\"}}\n\n"
const failoverStop = "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
const failoverError = "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"Please retry later\"}}\n\n"

func TestAnthropicFailoverErrorsStayInternalUntilSuccess(t *testing.T) {
	for _, errorFrame := range []string{
		failoverError,
		"data: {\"error\":{\"type\":\"upstream_error\",\"message\":\"temporarily unavailable\"}}\n\n",
		"event: error\ndata: {\"type\":\"error\",\ndata: \"error\":{\"type\":\"upstream_error\",\"message\":\"retry later\"}}\n\n",
		strings.TrimRight(failoverError, "\n"),
	} {
		t.Run(errorFrame, func(t *testing.T) {
			ctx := withRequestAccountFailoverCount(context.Background(), 3)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			svc := &GatewayService{cfg: &config.Config{}, rateLimitService: &RateLimitService{}}
			for i, body := range []string{failoverPreamble + errorFrame, errorFrame, failoverPreamble + failoverText + strings.TrimRight(failoverStop, "\n")} {
				id := int64(i + 1)
				attemptCtx := context.WithValue(ctx, ctxkey.AccountID, id)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(attemptCtx)
				account := &Account{ID: id, Name: "account", Platform: PlatformAnthropic}
				resp := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
				result, err := ForwardWithAccountRetries(attemptCtx, c, func() (*ForwardResult, error) {
					stream, streamErr := svc.handleStreamingResponseAnthropicAPIKeyPassthrough(attemptCtx, resp, c, account, time.Now(), "claude")
					if streamErr != nil {
						return nil, streamErr
					}
					return &ForwardResult{Usage: *stream.usage, Stream: true}, nil
				})
				if i < 2 {
					var failure *UpstreamFailoverError
					require.ErrorAs(t, err, &failure)
					require.True(t, failure.ShouldRetryNextAccount())
					require.Empty(t, rec.Body.String(), "failed attempts must not reach the client")
				} else {
					require.NoError(t, err)
					require.Equal(t, 9, result.Usage.InputTokens)
				}
			}
			require.NotContains(t, rec.Body.String(), "error")
			require.Equal(t, 1, strings.Count(rec.Body.String(), "event: message_start"))
			require.Contains(t, rec.Body.String(), "answer")
			require.Contains(t, rec.Body.String(), "message_stop")
			attempts := GetRequestAccountFailoverAttempts(ctx)
			require.Len(t, attempts, 3)
			for i, attempt := range attempts {
				require.EqualValues(t, i+1, attempt.AccountID)
				require.GreaterOrEqual(t, attempt.FinishedAt, attempt.StartedAt)
			}
			require.Equal(t, "failed", attempts[0].Result)
			require.Equal(t, "success", attempts[2].Result)
			require.Equal(t, "success", RequestAccountFailoverResult(c))
		})
	}
}

func TestAnthropicFailoverStopsOnlyForInputErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		retry      bool
		output     bool
	}{
		{"ping then retry", "event: ping\ndata: {\"type\":\"ping\"}\n\n" + failoverError, true, false},
		{"input error", "data: {\"type\":\"error\",\"error\":{\"type\":\"invalid_request_error\",\"message\":\"input exceeds the context window\"}}\n\n", false, false},
		{"output then error", failoverPreamble + failoverText + failoverError, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := withRequestAccountFailoverCount(context.Background(), 3)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
			svc := &GatewayService{cfg: &config.Config{}, rateLimitService: &RateLimitService{}}
			_, err := svc.handleStreamingResponseAnthropicAPIKeyPassthrough(ctx,
				&http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(tc.body))},
				c, &Account{ID: 1, Platform: PlatformAnthropic}, time.Now(), "claude")
			var failure *UpstreamFailoverError
			require.ErrorAs(t, err, &failure)
			require.Equal(t, tc.retry, failure.ShouldRetryNextAccount())
			require.NotContains(t, rec.Body.String(), "error", "only the final handler may emit a terminal error")
			require.Equal(t, tc.output, strings.Contains(rec.Body.String(), "answer"))
		})
	}
}

func TestAccountFailoverTraceAndBillingCancellation(t *testing.T) {
	ctx := withRequestAccountFailoverCount(context.Background(), 3)
	ctx = context.WithValue(ctx, ctxkey.AccountID, int64(31))
	ctx, cancel := context.WithCancel(ctx)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
	_, err := ForwardWithAccountRetries(ctx, c, func() (*ForwardResult, error) {
		cancel()
		return nil, errors.New("connection closed")
	})
	require.ErrorIs(t, err, context.Canceled)
	attempts := GetRequestAccountFailoverAttempts(ctx)
	require.Len(t, attempts, 1)
	require.Equal(t, "canceled", attempts[0].Result)
	attempts[0].AccountID = 99
	require.EqualValues(t, 31, GetRequestAccountFailoverAttempts(ctx)[0].AccountID)

	billing := CopyAccountFailoverBillingContext(ctx, context.Background())
	billing = CopyAccountFailoverBillingContext(billing, context.Background())
	require.NoError(t, billing.Err())
	require.Empty(t, GetRequestAccountFailoverAttempts(billing), "async billing must not retain request traces")
	require.Equal(t, "client:request:account:31", accountAttemptUsageBillingRequestID(billing, "client:request", 31))
	require.Equal(t, "client:request:account:37", accountAttemptUsageBillingRequestID(billing, "client:request", 37))
}

func TestAnthropicFailoverPartialOutputThenSuccess(t *testing.T) {
	ctx := withRequestAccountFailoverCount(context.Background(), 3)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &GatewayService{cfg: &config.Config{}, rateLimitService: &RateLimitService{}}
	for i, body := range []string{failoverPreamble + failoverText + failoverError, failoverError, failoverPreamble + failoverText + failoverStop} {
		account := &Account{ID: int64(i + 1), Platform: PlatformAnthropic}
		attemptCtx := context.WithValue(ctx, ctxkey.AccountID, account.ID)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(attemptCtx)
		resp := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
		_, err := ForwardWithAccountRetries(attemptCtx, c, func() (*ForwardResult, error) {
			result, err := svc.handleStreamingResponseAnthropicAPIKeyPassthrough(attemptCtx, resp, c, account, time.Now(), "claude")
			if err != nil {
				return nil, err
			}
			return &ForwardResult{Usage: *result.usage, Stream: true}, nil
		})
		if i < 2 {
			var failure *UpstreamFailoverError
			require.ErrorAs(t, err, &failure)
			require.True(t, failure.ShouldRetryNextAccount())
			require.True(t, failure.SafeToFailoverAfterWrite)
		} else {
			require.NoError(t, err)
		}
	}
	require.Equal(t, "success", RequestAccountFailoverResult(c))
	require.Len(t, GetRequestAccountFailoverAttempts(ctx), 3)
	require.Equal(t, 2, strings.Count(rec.Body.String(), `"text":"answer"`))
	require.NotContains(t, rec.Body.String(), "event: error")
}
