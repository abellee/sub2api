package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpsCostGuardRejectionsRemainVisibleWithoutAffectingSLA(t *testing.T) {
	for _, status := range []int{403, 429, 499, 503} {
		for _, message := range []string{"余额不足，请充值", "Insufficient balance for this request.", "自定义提示"} {
			for _, protocol := range []string{"openai", "anthropic", "gemini", "openai-sse", "anthropic-sse", "gateway-sse", "responses-sse", "websocket"} {
				t.Run(fmt.Sprintf("%s/%d/%s", protocol, status, message), func(t *testing.T) {
					setupOpsErrorLogTestQueue(t, 2)
					gin.SetMode(gin.TestMode)
					settings := &opsAdvancedSettingsRepoStub{advanced: `{"ignore_insufficient_balance_errors":true}`}
					ops := service.NewOpsService(nil, settings, nil, nil, nil, nil, nil, nil, nil, nil, nil)
					require.True(t, ops.OpsAdvancedSettingsSnapshot().IgnoreInsufficientBalanceErrors)
					router := gin.New()
					router.Use(OpsErrorLoggerMiddleware(ops))
					path := "/v1/messages"
					if protocol == "responses-sse" {
						path = "/v1/responses"
					}
					router.POST(path, func(c *gin.Context) {
						c.Set(opsUserBalanceErrorKey, true)
						switch protocol {
						case "openai":
							(&OpenAIGatewayHandler{}).handleStreamingAwareError(c, status, costGuardErrorType, message, false)
						case "anthropic":
							(&OpenAIGatewayHandler{}).anthropicErrorResponse(c, status, costGuardErrorType, message)
						case "gemini":
							googleErrorWithType(c, status, costGuardErrorType, "", message)
						case "websocket":
							c.Status(http.StatusSwitchingProtocols)
							service.MarkOpsStreamErrorValue(c, service.OpsStreamError{
								ErrType: costGuardErrorType, Code: fmt.Sprint(status), Message: message,
								IntendedStatus: status, RequestScoped: true,
							})
						default:
							c.Writer.WriteHeader(http.StatusOK)
							c.Writer.WriteHeaderNow()
							switch protocol {
							case "anthropic-sse":
								(&OpenAIGatewayHandler{}).anthropicStreamingAwareError(c, status, costGuardErrorType, message, true)
							case "gateway-sse":
								(&GatewayHandler{}).handleStreamingAwareError(c, status, costGuardErrorType, message, true)
							default:
								(&OpenAIGatewayHandler{}).handleStreamingAwareError(c, status, costGuardErrorType, message, true)
							}
						}
					})
					router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, path, nil))
					require.Equal(t, int64(1), OpsErrorLogQueueLength(), "the rejection must remain visible exactly once")
					entry := (<-opsErrorLogQueue).entry
					require.Equal(t, service.OpsUserBalanceErrorType, entry.ErrorType)
					require.Equal(t, status, entry.StatusCode)
					require.Equal(t, message, entry.ErrorMessage)
					require.Equal(t, "request", entry.ErrorPhase)
					require.Equal(t, "client", entry.ErrorOwner)
					require.Equal(t, "client_request", entry.ErrorSource)
					require.Equal(t, "P3", entry.Severity)
					require.True(t, entry.IsBusinessLimited, "SLA excludes business-limited errors")
					require.Nil(t, entry.UpstreamStatusCode, "local precheck rejection is not an upstream failure")
				})
			}
		}
	}
}

func TestOpsCostGuardClassificationKeepsRealUpstreamFailuresInSLA(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	service.SetOpsUpstreamError(c, http.StatusServiceUnavailable, "real upstream failure", "")
	phase, excluded, owner, _ := classifyOpsErrorLog(c, "upstream_error", "upstream request failed", "", http.StatusServiceUnavailable)
	require.Equal(t, "upstream", phase)
	require.Equal(t, "provider", owner)
	require.False(t, excluded)
}

func TestOpsUnpricedRejectionIsNotUserBalance(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 2)
	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.Use(OpsErrorLoggerMiddleware(ops))
	router.POST("/v1/messages", func(c *gin.Context) {
		c.Set(opsUserBalanceErrorKey, false)
		(&GatewayHandler{}).handleStreamingAwareError(c, 403, costGuardErrorType, "Insufficient balance for the estimated request cost.", false)
	})
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/messages", nil))
	require.Equal(t, int64(1), OpsErrorLogQueueLength())
	entry := (<-opsErrorLogQueue).entry
	require.Equal(t, costGuardErrorType, entry.ErrorType)
}
