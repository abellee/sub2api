package service

import (
	"context"
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

func TestAccountErrorClassificationAcrossGateways(t *testing.T) {
	ctx := withRequestAccountFailoverCount(context.Background(), 15)
	for _, tc := range []struct {
		name   string
		status int
		body   string
		retry  bool
	}{
		{"endpoint missing", 404, `404 page not found`, true},
		{"provider payment", 402, `payment required`, true},
		{"provider Chinese credit", 403, `{"error":{"message":"用户额度不足, 剩余额度: -0.01"}}`, true},
		{"provider credit wrapped as input", 400, `{"error":{"type":"invalid_request_error","message":"insufficient balance"}}`, true},
		{"quoted model", 400, `{"error":{"type":"invalid_request_error","message":"Model \"gpt-6\" is not supported by this provider"}}`, true},
		{"upstream timeout", 408, ``, true},
		{"unexpected upstream status", 409, `upstream busy`, true},
		{"provider key", 401, `invalid api key`, true},
		{"server failure", 502, `Bad Gateway`, true},
		{"input validation", 400, `{"error":{"type":"invalid_request_error","message":"invalid input"}}`, false},
		{"input wrapped as server error", 502, `{"error":{"message":"invalid parameter: size"}}`, false},
		{"Google input", 400, `{"error":{"status":"INVALID_ARGUMENT","message":"bad input"}}`, false},
		{"Google account configuration", 400, `{"error":{"status":"INVALID_ARGUMENT","message":"invalid project resource name"}}`, true},
		{"context length", 400, `{"error":{"code":"context_length_exceeded","message":"maximum context length exceeded"}}`, false},
		{"content policy", 403, `{"error":{"code":"content_policy_violation","message":"content policy violation"}}`, false},
		{"payload too large", 413, `request too large`, false},
		{"bad media type", 415, ``, false},
		{"echoed prompt must not affect decision", 404, `{"error":{"message":"not found"},"request":{"prompt":"invalid parameter"}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(tc.body)
			account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
			svc := &OpenAIGatewayService{}
			require.Equal(t, tc.retry, svc.shouldFailoverOpenAIUpstreamResponse(account, tc.status, "", body, ctx))
			require.Equal(t, tc.retry, shouldFailoverOpenAIPassthroughResponse(account, tc.status, body, ctx))
			require.Equal(t, tc.retry, svc.shouldFailoverGrokUpstreamError(tc.status, body, ctx))
			resp := &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}
			require.Equal(t, tc.retry, shouldFailoverAccountResponse(ctx, resp, false))
			restored, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, tc.body, string(restored))
		})
	}
}

func TestAccountErrorPolicyKeepsDisabledAndSingleAccountBehavior(t *testing.T) {
	for _, count := range []int{0, 1} {
		ctx := withRequestAccountFailoverCount(context.Background(), count)
		require.False(t, shouldFailoverAccountError(ctx, 404, nil, false))
		require.True(t, shouldFailoverAccountError(ctx, 500, nil, true))
	}
}

func TestAccountStreamErrorUsesSameClassification(t *testing.T) {
	ctx := withRequestAccountFailoverCount(context.Background(), 3)
	for _, tc := range []struct {
		body  string
		retry bool
	}{
		{`{"type":"error","error":{"type":"invalid_request_error","message":"insufficient balance"}}`, true},
		{`{"type":"response.failed","response":{"error":{"type":"invalid_request_error","message":"invalid parameter: size"}}}`, false},
		{`{"type":"error","error":{"message":"unknown upstream failure"}}`, true},
	} {
		payload := []byte(tc.body)
		require.Equal(t, tc.retry, openAIStreamFailedEventShouldFailover(payload, "", ctx))
		require.Equal(t, tc.retry, openAIStreamErrorEventShouldFailover(payload, "", ctx))
	}
	payload := `{"type":"error","error":{"type":"invalid_request_error","message":"insufficient balance"}}`
	require.False(t, openAIStreamDataStartsRequestOutput(ctx, payload, "error"))
}

func TestAccountStreamFailureAfterPreviousAttemptOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := withRequestAccountFailoverCount(context.Background(), 3)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
	// An earlier retryable stream attempt committed its headers/preamble.
	_, _ = c.Writer.WriteString("event: message_start\n\n")
	MarkResponseCommitted(c)
	_, err := ForwardWithAccountRetries(ctx, c, func() (*ForwardResult, error) {
		require.False(t, IsResponseCommitted(c))
		return nil, requestAccountStreamFailover(c, &Account{ID: 2}, nil, io.ErrUnexpectedEOF)
	})
	var failure *UpstreamFailoverError
	require.ErrorAs(t, err, &failure, "previous account output must not stop the next account's pre-output retry")
	require.Equal(t, "failed", RequestAccountFailoverResult(c))
	_, err = ForwardWithAccountRetries(ctx, c, func() (*ForwardResult, error) { return &ForwardResult{}, nil })
	require.NoError(t, err)
	require.Equal(t, "success", RequestAccountFailoverResult(c))
}

func TestAccountErrorPeekIsBoundedAndPreservesClose(t *testing.T) {
	ctx := withRequestAccountFailoverCount(context.Background(), 2)
	body := &accountErrorTestBody{Reader: strings.NewReader(strings.Repeat("x", 2<<20))}
	resp := &http.Response{StatusCode: 404, Body: body}
	require.True(t, shouldFailoverAccountResponse(ctx, resp, false))
	require.Equal(t, 1<<20, body.Len())
	restored, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Len(t, restored, 2<<20)
	require.NoError(t, resp.Body.Close())
	require.True(t, body.closed)
}

type accountErrorTestBody struct {
	*strings.Reader
	closed bool
}

func (b *accountErrorTestBody) Close() error { b.closed = true; return nil }

func TestAccountFailoverForwardsPastSixFailures(t *testing.T) {
	ctx := withRequestAccountFailoverCount(context.Background(), 8)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	body := []byte(`{"model":"gpt-5.4","stream":false,"instructions":"test","input":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(string(body))).WithContext(ctx)
	upstream := &httpUpstreamRecorder{}
	for _, status := range []int{404, 402, 403, 408, 409, 429, 502} {
		upstream.responses = append(upstream.responses, &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"provider unavailable"}}`))})
	}
	upstream.responses = append(upstream.responses, &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"resp_ok","status":"completed","model":"gpt-5.4","output":[],"usage":{"input_tokens":1,"output_tokens":1}}`))})
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	for id := int64(1); id <= 8; id++ {
		account := openAIFailoverCachedBodyTestAccount(id, "test", nil)
		result, err := ForwardWithAccountRetries(ctx, c, func() (*OpenAIForwardResult, error) { return svc.Forward(ctx, c, account, body) })
		if id < 8 {
			var failover *UpstreamFailoverError
			require.ErrorAs(t, err, &failover)
			require.False(t, c.Writer.Written())
			require.False(t, failover.RetryableOnSameAccount)
		} else {
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, "success", RequestAccountFailoverResult(c))
		}
	}
	require.Len(t, upstream.bodies, 8)
}

func TestAccountFailoverAnthropicHTTP404BeforeResponseWrite(t *testing.T) {
	ctx := withRequestAccountFailoverCount(context.Background(), 2)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
	upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{StatusCode: 404, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"endpoint not found"}}`))}}
	svc := &GatewayService{cfg: &config.Config{}, httpUpstream: upstream, rateLimitService: &RateLimitService{}}
	_, err := svc.forwardAnthropicAPIKeyPassthrough(ctx, c, newAnthropicAPIKeyAccountForTest(), []byte(`{"model":"claude-test","messages":[]}`), "claude-test", "claude-test", false, time.Now())
	var failure *UpstreamFailoverError
	require.ErrorAs(t, err, &failure)
	require.Equal(t, 404, failure.StatusCode)
	require.False(t, c.Writer.Written())
}
