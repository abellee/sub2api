//go:build unit

package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const geminiPartialForFailover = "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"partial\"}]}}],\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":2}}\n\n"
const geminiFailureForFailover = "data: {\"error\":{\"code\":502,\"message\":\"Upstream service temporarily unavailable\",\"status\":\"INTERNAL\"}}\n\n"

func TestGeminiNativeErrorEventAccountRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, failure string
		partial       bool
	}{
		{"before output", geminiFailureForFailover, false},
		{"after output", geminiPartialForFailover + geminiFailureForFailover, true},
		{"multiline error", "event: error\ndata: {\"error\":\ndata: {\"code\":502,\"message\":\"unavailable\",\"status\":\"INTERNAL\"}}\n\n", false},
		{"EOF error", strings.TrimRight(geminiFailureForFailover, "\n"), false},
		{"wrapped OAuth error", "data: {\"response\":{\"error\":{\"code\":503,\"message\":\"overloaded\",\"status\":\"UNAVAILABLE\"}}}\n\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := withRequestAccountFailoverCount(context.Background(), 3)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			for i, body := range []string{tc.failure, geminiFailureForFailover, geminiSignalTestStopSSE} {
				account := geminiSignalTestAccount()
				account.ID = int64(i + 1)
				attemptCtx := context.WithValue(ctx, ctxkey.AccountID, account.ID)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini:streamGenerateContent", nil).WithContext(attemptCtx)
				svc := newGeminiSignalService("text/event-stream", body)
				result, err := ForwardWithAccountRetries(attemptCtx, c, func() (*ForwardResult, error) {
					return svc.ForwardNative(attemptCtx, c, account, "gemini-3.7-flash", "streamGenerateContent", true, geminiSignalTestRequest())
				})
				if i < 2 {
					var failure *UpstreamFailoverError
					require.ErrorAs(t, err, &failure)
					require.True(t, failure.ShouldRetryNextAccount())
					require.True(t, failure.SafeToFailoverAfterWrite)
					require.Empty(t, GetOpsStreamErrors(c), "recovered errors must not mark final response failed")
					if i == 0 && tc.partial {
						require.Equal(t, 2, result.Usage.OutputTokens)
						require.Equal(t, 10, result.Usage.InputTokens)
					}
				} else {
					require.NoError(t, err)
					require.Equal(t, 4, result.Usage.OutputTokens)
				}
			}
			require.NotContains(t, rec.Body.String(), `"error"`)
			require.Contains(t, rec.Body.String(), `"finishReason":"STOP"`)
			attempts := GetRequestAccountFailoverAttempts(ctx)
			require.Len(t, attempts, 3)
			require.Equal(t, "failed", attempts[0].Result)
			require.GreaterOrEqual(t, attempts[0].StatusCode, 500)
			require.Equal(t, "failed", attempts[1].Result)
			require.Equal(t, "success", attempts[2].Result)
			require.Equal(t, "success", RequestAccountFailoverResult(c))
		})
	}
}

func TestGeminiNativeSignalOutcomeAndGates(t *testing.T) {
	for _, tc := range []struct {
		name, body    string
		count         int
		want          string
		retry, cancel bool
	}{
		{"switch disabled", geminiFailureForFailover, 0, "", false, false},
		{"single account", geminiFailureForFailover, 1, "", false, false},
		{"provider error", geminiPartialForFailover + geminiFailureForFailover, 2, "failed", true, false},
		{"invalid request", "data: {\"error\":{\"code\":400,\"message\":\"invalid parameter\",\"status\":\"INVALID_ARGUMENT\"}}\n\n", 2, "failed", false, false},
		{"content policy", geminiSignalTestProhibitedSSE, 2, "failed", false, false},
		{"normal", geminiSignalTestStopSSE, 2, "success", false, false},
		{"missing terminal", geminiPartialForFailover, 2, "failed", true, false},
		{"empty stream", "", 2, "failed", true, false},
		{"client canceled", geminiFailureForFailover, 2, "canceled", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := withRequestAccountFailoverCount(context.Background(), tc.count)
			ctx = context.WithValue(ctx, ctxkey.AccountID, int64(703))
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			c, rec := newGeminiNativeTestContext(t)
			c.Request = c.Request.WithContext(ctx)
			svc := newGeminiSignalService("text/event-stream", tc.body)
			_, err := ForwardWithAccountRetries(ctx, c, func() (*ForwardResult, error) {
				if tc.cancel {
					cancel()
				}
				return svc.ForwardNative(ctx, c, geminiSignalTestAccount(), "gemini-3.7-flash", "streamGenerateContent", true, geminiSignalTestRequest())
			})
			if tc.retry {
				var failure *UpstreamFailoverError
				require.ErrorAs(t, err, &failure)
				require.True(t, failure.ShouldRetryNextAccount())
				require.NotContains(t, rec.Body.String(), `"error"`)
			} else if tc.cancel {
				require.ErrorIs(t, err, context.Canceled)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.want, RequestAccountFailoverResult(c))
			if tc.count > 1 {
				attempts := GetRequestAccountFailoverAttempts(ctx)
				require.Len(t, attempts, 1)
				require.Equal(t, tc.want, attempts[0].Result)
				if tc.want == "failed" {
					require.GreaterOrEqual(t, attempts[0].StatusCode, 400)
				}
			}
		})
	}
}

func TestGeminiNativeJSONErrorRetriesWithoutWriting(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			ctx := withRequestAccountFailoverCount(context.Background(), 2)
			c, rec := newGeminiNativeTestContext(t)
			c.Request = c.Request.WithContext(ctx)
			svc := newGeminiSignalService("application/json", `{"error":{"code":503,"message":"temporarily unavailable","status":"UNAVAILABLE"},"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":2}}`)
			result, err := svc.ForwardNative(ctx, c, geminiSignalTestAccount(), "gemini-3.7-flash", "generateContent", stream, geminiSignalTestRequest())
			var failure *UpstreamFailoverError
			require.ErrorAs(t, err, &failure)
			require.True(t, failure.ShouldRetryNextAccount())
			require.Equal(t, 2, result.Usage.OutputTokens)
			require.Equal(t, 10, result.Usage.InputTokens)
			require.Empty(t, rec.Body.String())
		})
	}
}
