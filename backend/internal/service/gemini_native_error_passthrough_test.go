//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const geminiNativeBalanceMessage = "insufficient_balance: Insufficient account balance"
const geminiNativeBalanceBody = `{"error":{"code":402,"message":"insufficient_balance: Insufficient account balance","status":"RESOURCE_EXHAUSTED"}}`

func TestGeminiNativeErrorPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name            string
		noRule          bool
		keyword         string
		platform        string
		passthroughCode bool
		passthroughBody bool
		oauth           bool
		skipMonitoring  bool
		wantMatch       bool
	}{
		{name: "balance keyword rewrites response", wantMatch: true},
		{name: "preserve upstream status", passthroughCode: true, wantMatch: true},
		{name: "preserve upstream message", passthroughBody: true, wantMatch: true},
		{name: "no rule preserves original body", noRule: true},
		{name: "unmatched keyword preserves original body", keyword: "unrelated error"},
		{name: "other platform preserves original body", platform: PlatformOpenAI},
		{name: "oauth wrapper is unwrapped before matching", oauth: true, wantMatch: true},
		{name: "skip monitoring follows rule", skipMonitoring: true, wantMatch: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newGeminiNativeTestContext(t)
			if !tc.noRule {
				keyword := tc.keyword
				if keyword == "" {
					keyword = "account balance"
				}
				platform := tc.platform
				if platform == "" {
					platform = PlatformGemini
				}
				rule := newNonFailoverPassthroughRule(http.StatusPaymentRequired, keyword, http.StatusBadGateway, "当前服务拥挤，请休息一哈。")
				rule.ErrorCodes = nil
				rule.MatchMode = model.MatchModeAny
				rule.Platforms = []string{platform}
				rule.PassthroughCode = tc.passthroughCode
				rule.PassthroughBody = tc.passthroughBody
				rule.SkipMonitoring = tc.skipMonitoring
				ruleSvc := &ErrorPassthroughService{}
				ruleSvc.setLocalCache([]*model.ErrorPassthroughRule{rule})
				BindErrorPassthroughService(c, ruleSvc)
			}
			cfg := &config.Config{}
			cfg.Gateway.LogUpstreamErrorBody = true
			svc := &GeminiMessagesCompatService{cfg: cfg}
			body := geminiNativeBalanceBody
			if tc.oauth {
				body = `{"response":` + body + `}`
			}
			resp := &http.Response{StatusCode: http.StatusPaymentRequired, Header: http.Header{"Content-Type": {"application/json"}}}
			err := svc.writeGeminiNativeUpstreamError(c, geminiPoolModeAPIKeyAccount(), resp, []byte(body), "balance-request", tc.oauth)
			require.ErrorContains(t, err, geminiNativeBalanceMessage)
			require.True(t, IsResponseCommitted(c))
			require.Equal(t, http.StatusPaymentRequired, c.GetInt(OpsUpstreamStatusCodeKey))
			require.Equal(t, geminiNativeBalanceMessage, c.GetString(OpsUpstreamErrorMessageKey))
			require.JSONEq(t, geminiNativeBalanceBody, c.GetString(OpsUpstreamErrorDetailKey))
			eventsValue, exists := c.Get(OpsUpstreamErrorsKey)
			require.True(t, exists)
			events := eventsValue.([]*OpsUpstreamErrorEvent)
			require.Len(t, events, 1)
			require.Equal(t, http.StatusPaymentRequired, events[0].UpstreamStatusCode)
			require.Equal(t, geminiNativeBalanceMessage, events[0].Message)
			require.Equal(t, "balance-request", events[0].UpstreamRequestID)
			require.Equal(t, tc.skipMonitoring, c.GetBool(OpsSkipPassthroughKey))
			if !tc.wantMatch {
				require.Equal(t, http.StatusPaymentRequired, rec.Code)
				require.Equal(t, geminiNativeBalanceBody, rec.Body.String())
				return
			}
			wantCode := http.StatusBadGateway
			wantStatus := "INTERNAL"
			if tc.passthroughCode {
				wantCode = http.StatusPaymentRequired
				wantStatus = "UNKNOWN"
			}
			wantMessage := "当前服务拥挤，请休息一哈。"
			if tc.passthroughBody {
				wantMessage = geminiNativeBalanceMessage
			}
			require.Equal(t, wantCode, rec.Code)
			var payload struct {
				Error struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
					Status  string `json:"status"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
			require.Equal(t, wantCode, payload.Error.Code)
			require.Equal(t, wantMessage, payload.Error.Message)
			require.Equal(t, wantStatus, payload.Error.Status)
		})
	}
}

func TestGeminiForwardNative_BalanceErrorAppliesRule(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, pool := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			name := "ordinary"
			if pool {
				name = "pool"
			}
			if stream {
				name += "/stream"
			} else {
				name += "/nonstream"
			}
			t.Run(name, func(t *testing.T) {
				svc, _ := newGeminiSkippedWriteService(http.StatusPaymentRequired, geminiNativeBalanceBody)
				c, rec := newGeminiNativeTestContext(t)
				ruleSvc := &ErrorPassthroughService{}
				ruleSvc.setLocalCache([]*model.ErrorPassthroughRule{
					newNonFailoverPassthroughRule(http.StatusPaymentRequired, "account balance", http.StatusBadGateway, "自定义错误"),
				})
				BindErrorPassthroughService(c, ruleSvc)
				account := geminiPoolModeAPIKeyAccount()
				account.Credentials["pool_mode"] = pool
				action := "generateContent"
				if stream {
					action = "streamGenerateContent"
				}
				result, err := svc.ForwardNative(context.Background(), c, account, "gemini-2.5-flash", action, stream,
					[]byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
				require.Nil(t, result)
				require.ErrorContains(t, err, geminiNativeBalanceMessage)
				var failoverErr *UpstreamFailoverError
				require.False(t, errors.As(err, &failoverErr), "response rewrite must not start failover")
				require.Equal(t, http.StatusBadGateway, rec.Code)
				require.JSONEq(t, `{"error":{"code":502,"message":"自定义错误","status":"INTERNAL"}}`, rec.Body.String())
				require.True(t, IsResponseCommitted(c))
				require.Equal(t, http.StatusPaymentRequired, c.GetInt(OpsUpstreamStatusCodeKey))
				require.Equal(t, geminiNativeBalanceMessage, c.GetString(OpsUpstreamErrorMessageKey))
			})
		}
	}
}
