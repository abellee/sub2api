//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func publicErrorContext(t *testing.T, path string, withRule bool) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	if withRule {
		svc := &ErrorPassthroughService{}
		rule := newNonFailoverPassthroughRule(502, "private-provider", 429, "线路暂忙，请稍后重试")
		rule.MatchMode, rule.ErrorCodes = model.MatchModeAny, nil
		svc.setLocalCache([]*model.ErrorPassthroughRule{rule})
		BindErrorPassthroughService(c, svc)
	}
	return c, rec
}

func TestPublicResponsesMidstreamErrorRules(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		for _, rule := range []bool{false, true} {
			for _, bare := range []bool{false, true} {
				t.Run(fmt.Sprintf("passthrough=%v/rule=%v/bare=%v", passthrough, rule, bare), func(t *testing.T) {
					c, rec := publicErrorContext(t, "/v1/responses", rule)
					prefix := "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\",\"output_index\":0}\n\n"
					failed := `{"type":"response.failed","sequence_number":8,"response":{"id":"resp_test","created_at":1,"status":"failed","error":{"code":502,"message":"private-provider https://private.example/secret"},"debug":"PRIVATE_MARKER","usage":{"input_tokens":7,"output_tokens":2}}}`
					if bare {
						failed = `{"type":"error","sequence_number":8,"error":{"message":"private-provider https://private.example/secret","detail":"PRIVATE_MARKER"}}`
					}
					resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(&smallChunkReader{data: []byte(prefix + "data: " + failed + "\n\n"), max: 3})}
					svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
					account := &Account{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
					var err error
					if passthrough {
						_, err = svc.handleStreamingResponsePassthrough(context.Background(), resp, c, account, time.Now(), "model", "model")
					} else {
						_, err = svc.handleStreamingResponse(context.Background(), resp, c, account, time.Now(), "model", "model")
					}
					require.Error(t, err)
					require.Equal(t, 200, rec.Code)
					require.Contains(t, rec.Body.String(), "hello")
					want := UpstreamClientErrorMessage
					if rule {
						want = "线路暂忙，请稍后重试"
					}
					require.Contains(t, rec.Body.String(), want)
					require.NotContains(t, rec.Body.String(), "private-provider")
					require.NotContains(t, rec.Body.String(), "private.example")
					require.NotContains(t, rec.Body.String(), "PRIVATE_MARKER")
				})
			}
		}
	}
}

type smallChunkReader struct {
	data []byte
	max  int
}

func (r *smallChunkReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := min(len(p), r.max, len(r.data))
	copy(p, r.data[:n])
	r.data = r.data[n:]
	return n, nil
}

func TestPublicStreamNormalLargeFrameUnchanged(t *testing.T) {
	payload := []byte(`{"type":"response.output_text.delta","delta":"` + strings.Repeat("A", 2<<20) + `"}`)
	c, _ := publicErrorContext(t, "/v1/responses", true)
	require.Equal(t, payload, publicOpenAIStreamError(c, PlatformOpenAI, payload, "response.output_text.delta"))
}

func TestPublicStreamErrorResolver(t *testing.T) {
	c, _ := publicErrorContext(t, "/", true)
	status, kind, message := ResolveUpstreamClientError(c, PlatformOpenAI, 503, []byte(`{"error":{"message":"private-provider"}}`))
	require.Equal(t, 429, status)
	require.Equal(t, "upstream_error", kind)
	require.Equal(t, "线路暂忙，请稍后重试", message)
	payload := publicOpenAIStreamError(c, PlatformOpenAI, []byte(`{"type":"response.failed","response":{"id":"resp_2","created_at":17,"error":{"code":429,"message":"unmatched","private":"PRIVATE"}}}`), "response.failed")
	require.Equal(t, "429", gjson.GetBytes(payload, "response.error.code").String())
	require.Equal(t, "resp_2", gjson.GetBytes(payload, "response.id").String())
	require.Equal(t, int64(17), gjson.GetBytes(payload, "response.created_at").Int())
	require.Equal(t, UpstreamClientErrorMessage, gjson.GetBytes(payload, "response.error.message").String())
	require.NotContains(t, string(payload), "PRIVATE")
}

func TestPublicGeminiNativeMidstreamError(t *testing.T) {
	for _, rule := range []bool{false, true} {
		c, rec := publicErrorContext(t, "/v1beta/models/model:streamGenerateContent", rule)
		stream := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hello\"}]}}],\"usageMetadata\":{\"promptTokenCount\":7}}\n\n" +
			"data: {\"error\":\n" + "data: {\"code\":503,\"message\":\"private-provider\",\"detail\":\"PRIVATE\"}}\n\n"
		resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(&smallChunkReader{data: []byte(stream), max: 2})}
		svc := &GeminiMessagesCompatService{}
		result, err := svc.handleNativeStreamingResponse(c, resp, time.Now(), false, &Account{ID: 2, Platform: PlatformGemini}, "req")
		require.Error(t, err)
		require.Equal(t, 200, rec.Code)
		require.Contains(t, rec.Body.String(), "hello")
		want := UpstreamClientErrorMessage
		if rule {
			want = "线路暂忙，请稍后重试"
		}
		require.Contains(t, rec.Body.String(), want)
		require.NotContains(t, rec.Body.String(), "private-provider")
		require.NotContains(t, rec.Body.String(), "PRIVATE")
		require.Equal(t, 7, result.usage.InputTokens)
		require.NotEmpty(t, upstreamErrorEventsFromContext(t, c))
	}
}

func TestPublicRawChatMidstreamError(t *testing.T) {
	c, rec := publicErrorContext(t, "/v1/chat/completions", true)
	stream := "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"}}]}\n\n" +
		"data: {\"error\":{\"message\":\"private-provider\",\"debug\":\"PRIVATE\"}}\n\n"
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(stream))}
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
	_, err := svc.streamRawChatCompletions(c, resp, &Account{ID: 2, Platform: PlatformOpenAI}, "model", "model", "model", nil, nil, time.Now(), 0)
	require.Error(t, err)
	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), "hello")
	require.Contains(t, rec.Body.String(), "线路暂忙，请稍后重试")
	require.NotContains(t, rec.Body.String(), "PRIVATE")
	require.Contains(t, rec.Body.String(), "[DONE]")
}

func TestPublicRawChatNullErrorIsNormal(t *testing.T) {
	c, rec := publicErrorContext(t, "/v1/chat/completions", true)
	stream := "data: {\"error\":null,\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(stream))}
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
	_, err := svc.streamRawChatCompletions(c, resp, &Account{ID: 2, Platform: PlatformOpenAI}, "model", "model", "model", nil, nil, time.Now(), 0)
	require.NoError(t, err)
	require.Equal(t, stream, rec.Body.String())
}

func TestPublicAntigravityStreamErrors(t *testing.T) {
	for _, endpoint := range []string{"native", "messages", "chat", "responses"} {
		t.Run(endpoint, func(t *testing.T) {
			c, rec := publicErrorContext(t, "/v1/"+endpoint, true)
			stream := "data: {\"error\":{\"code\":503,\"message\":\"private-provider\",\"debug\":\"PRIVATE\"}}\n\n"
			resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(&smallChunkReader{data: []byte(stream), max: 3})}
			svc := &AntigravityGatewayService{settingService: NewSettingService(nil, &config.Config{})}
			var err error
			switch endpoint {
			case "native":
				_, err = svc.handleGeminiStreamingResponse(c, resp, time.Now())
			case "messages":
				_, err = svc.handleClaudeStreamingResponse(c, resp, time.Now(), "model")
			case "chat":
				_, err = svc.handleChatCompletionsStreamingFromAntigravity(c, resp, time.Now(), "model", true)
			case "responses":
				_, err = svc.handleResponsesStreamingFromAntigravity(c, resp, time.Now(), "model")
			}
			require.Error(t, err)
			require.Contains(t, rec.Body.String(), "线路暂忙，请稍后重试")
			require.NotContains(t, rec.Body.String(), "private-provider")
			require.NotContains(t, rec.Body.String(), "PRIVATE")
		})
	}
}
