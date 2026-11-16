package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestFinalFailoverResponseUsesOnlyRuleMessageAndFinalStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	paths := []struct {
		name, path, platform string
		write                func(*gin.Context, *service.ErrorPassthroughService, *service.UpstreamFailoverError)
	}{
		{"openai", "/v1/responses", service.PlatformOpenAI, func(c *gin.Context, rules *service.ErrorPassthroughService, err *service.UpstreamFailoverError) {
			(&OpenAIGatewayHandler{errorPassthroughService: rules}).handleFailoverExhausted(c, err, false)
		}},
		{"openai_messages", "/v1/messages", service.PlatformOpenAI, func(c *gin.Context, rules *service.ErrorPassthroughService, err *service.UpstreamFailoverError) {
			(&OpenAIGatewayHandler{errorPassthroughService: rules}).handleAnthropicFailoverExhausted(c, err, false)
		}},
		{"messages", "/v1/messages", service.PlatformAnthropic, func(c *gin.Context, rules *service.ErrorPassthroughService, err *service.UpstreamFailoverError) {
			(&GatewayHandler{errorPassthroughService: rules}).handleFailoverExhausted(c, err, service.PlatformAnthropic, false)
		}},
		{"chat", "/v1/chat/completions", service.PlatformAnthropic, func(c *gin.Context, rules *service.ErrorPassthroughService, err *service.UpstreamFailoverError) {
			(&GatewayHandler{errorPassthroughService: rules}).handleCCFailoverExhausted(c, err, false)
		}},
		{"responses", "/v1/responses", service.PlatformAnthropic, func(c *gin.Context, rules *service.ErrorPassthroughService, err *service.UpstreamFailoverError) {
			(&GatewayHandler{errorPassthroughService: rules}).handleResponsesFailoverExhausted(c, err, false)
		}},
		{"gemini", "/v1beta/models/gemini:generateContent", service.PlatformGemini, func(c *gin.Context, rules *service.ErrorPassthroughService, err *service.UpstreamFailoverError) {
			(&GatewayHandler{errorPassthroughService: rules}).handleGeminiFailoverExhausted(c, err)
		}},
		{"bound_cache", "/v1beta/models/gemini:generateContent", service.PlatformGemini, func(c *gin.Context, rules *service.ErrorPassthroughService, err *service.UpstreamFailoverError) {
			err.BoundUpstreamPassthrough = true
			(&GatewayHandler{errorPassthroughService: rules}).handleGeminiCachedContentFailoverExhausted(c, err)
		}},
	}
	for _, path := range paths {
		for _, status := range []int{400, 401, 402, 403, 404, 413, 429, 500, 502, 503, 504, 524, 529} {
			t.Run(fmt.Sprintf("%s/%d", path.name, status), func(t *testing.T) {
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, path.path, nil)
				message, earlier, overriddenStatus := "请稍后重试", "must-not-use-earlier-rule", 418
				rules := service.NewErrorPassthroughService(&errorPassthroughRepoStub{rules: []*model.ErrorPassthroughRule{
					{Enabled: true, MatchMode: model.MatchModeAny, Keywords: []string{"earlier-failure"}, CustomMessage: &earlier},
					{Enabled: true, MatchMode: model.MatchModeAny, ErrorCodes: []int{status}, Platforms: []string{path.platform}, ResponseCode: &overriddenStatus, PassthroughBody: true, CustomMessage: &message, SkipMonitoring: true},
				}}, nil)
				path.write(c, rules, &service.UpstreamFailoverError{
					StatusCode: status, ResponseBody: []byte(`{"error":{"message":"must-not-leak","code":"private_code","param":"private_param"},"debug":"secret"}`),
					ClientStatusCode: 503, ClientMessage: "must-not-leak-client-message", ResponseHeaders: http.Header{"Retry-After": []string{"20"}},
				})
				require.Equal(t, status, rec.Code)
				require.Equal(t, message, gjson.GetBytes(rec.Body.Bytes(), "error.message").String())
				require.NotContains(t, rec.Body.String(), "must-not-")
				require.NotContains(t, rec.Body.String(), "private_")
				require.Equal(t, "20", rec.Header().Get("Retry-After"))
				require.Equal(t, true, c.GetBool(service.OpsSkipPassthroughKey))
			})
		}
	}
}

func TestFinalFailoverResponseDoesNotExposeUnconfiguredUpstreamText(t *testing.T) {
	for _, rule := range []*model.ErrorPassthroughRule{
		nil,
		{Enabled: true, MatchMode: model.MatchModeAny, ErrorCodes: []int{503}, PassthroughBody: true},
		{Enabled: true, MatchMode: model.MatchModeAll, ErrorCodes: []int{503}, Keywords: []string{"different"}},
	} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		h := newBodyLimitPassthroughHandler(t, rule)
		h.handleFailoverExhausted(c, &service.UpstreamFailoverError{StatusCode: 503, ResponseBody: []byte(`{"error":{"message":"secret"}}`)}, false)
		require.Equal(t, 503, rec.Code)
		require.NotContains(t, rec.Body.String(), "secret")
	}
}

func TestFinalFailoverResponseMatchesEmptyBodyAndSpecialErrors(t *testing.T) {
	message, overriddenStatus := "规则中的提示", 418
	rules := service.NewErrorPassthroughService(&errorPassthroughRepoStub{rules: []*model.ErrorPassthroughRule{
		{Enabled: true, MatchMode: model.MatchModeAny, ErrorCodes: []int{401, 403, 413, 503}, CustomMessage: &message, ResponseCode: &overriddenStatus},
	}}, nil)
	for _, failure := range []*service.UpstreamFailoverError{
		{StatusCode: 503},
		bodyLimitFailoverTestError(),
		{StatusCode: 401, Stage: service.GatewayFailureStageAccountAuth, Reason: service.AntigravityCredentialRejectedReason},
		{StatusCode: 403, Reason: service.OpenAIImagesInsufficientBalanceReason},
	} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		(&OpenAIGatewayHandler{errorPassthroughService: rules}).handleFailoverExhausted(c, failure, false)
		require.Equal(t, failure.StatusCode, rec.Code)
		require.Equal(t, message, gjson.GetBytes(rec.Body.Bytes(), "error.message").String())
	}
}

func TestFinalFailoverResponseRespectsResolvedPlatform(t *testing.T) {
	message := "Grok rule"
	rules := service.NewErrorPassthroughService(&errorPassthroughRepoStub{rules: []*model.ErrorPassthroughRule{
		{Enabled: true, MatchMode: model.MatchModeAny, ErrorCodes: []int{503}, Platforms: []string{service.PlatformGrok}, CustomMessage: &message},
	}}, nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	ctx := service.WithResolvedTargetPlatform(context.Background(), service.PlatformGrok)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
	(&OpenAIGatewayHandler{errorPassthroughService: rules}).handleFailoverExhausted(c, &service.UpstreamFailoverError{StatusCode: 503}, false)
	require.Equal(t, message, gjson.GetBytes(rec.Body.Bytes(), "error.message").String())
}

func TestFinalFailoverResponseCommittedStreamKeepsStatusAndRuleText(t *testing.T) {
	message := "规则中的提示"
	rules := service.NewErrorPassthroughService(&errorPassthroughRepoStub{rules: []*model.ErrorPassthroughRule{
		{Enabled: true, MatchMode: model.MatchModeAny, ErrorCodes: []int{503}, CustomMessage: &message},
	}}, nil)
	for _, path := range []string{"/v1/responses", "/v1/messages", "/v1/chat/completions", "/v1beta/models/gemini:streamGenerateContent"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, path, nil)
			c.Header("Content-Type", "text/event-stream")
			_, err := c.Writer.WriteString(": keepalive\n\n")
			require.NoError(t, err)
			c.Writer.Flush()
			failure := &service.UpstreamFailoverError{StatusCode: 503, ResponseBody: []byte(`{"error":{"message":"secret"}}`)}
			switch path {
			case "/v1/chat/completions":
				(&GatewayHandler{errorPassthroughService: rules}).handleCCFailoverExhausted(c, failure, true)
			case "/v1beta/models/gemini:streamGenerateContent":
				(&GatewayHandler{errorPassthroughService: rules}).handleGeminiFailoverExhausted(c, failure)
			default:
				(&OpenAIGatewayHandler{errorPassthroughService: rules}).handleFailoverExhausted(c, failure, true)
			}
			require.Equal(t, 200, rec.Code)
			require.Contains(t, rec.Body.String(), message)
			require.NotContains(t, rec.Body.String(), "secret")
			streamErr, ok := service.GetOpsStreamError(c)
			require.True(t, ok)
			require.Equal(t, 503, streamErr.IntendedStatus)
		})
	}
}

func TestFinalFailoverResponseWebSocketUsesRule(t *testing.T) {
	message, ignoredStatus := "规则中的长连接提示", 418
	rules := service.NewErrorPassthroughService(&errorPassthroughRepoStub{rules: []*model.ErrorPassthroughRule{
		{Enabled: true, MatchMode: model.MatchModeAny, ErrorCodes: []int{524}, CustomMessage: &message, ResponseCode: &ignoredStatus},
	}}, nil)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	(&OpenAIGatewayHandler{errorPassthroughService: rules}).closeOpenAIWSFailoverExhausted(c, nil, &service.UpstreamFailoverError{StatusCode: 524, ResponseBody: []byte(`{"error":{"message":"secret"}}`)})
	streamErr, ok := service.GetOpsStreamError(c)
	require.True(t, ok)
	require.Equal(t, 524, streamErr.IntendedStatus)
	require.Equal(t, message, streamErr.Message)
}
