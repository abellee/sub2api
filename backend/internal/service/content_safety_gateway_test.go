//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func safetyGatewayContext(path string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	c.Set("api_key", &APIKey{ID: 22, UserID: 11})
	return c, w
}

func safetyUpstreamResponse() *http.Response {
	return &http.Response{StatusCode: 403, Header: http.Header{"X-Request-Id": []string{"upstream-safety"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"I'm sorry, I can't help with that request. Failed check: SAFETY_CHECK_TYPE"}}`))}
}

func TestContentSafetyGatewayGrokResponsesAndChatAreTerminal(t *testing.T) {
	for _, protocol := range []string{"responses", "chat/completions", "messages"} {
		t.Run(protocol, func(t *testing.T) {
			b := &safetyBillingFake{balance: 1}
			safety := newSafetyTestService(t, b)
			upstream := &httpUpstreamRecorder{resp: safetyUpstreamResponse()}
			svc := &OpenAIGatewayService{httpUpstream: upstream, contentSafetyBilling: safety}
			account := &Account{ID: 33, Platform: PlatformGrok, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "fake-token", "base_url": "https://api.x.ai"}}
			c, w := safetyGatewayContext("/v1/" + protocol)
			var err error
			switch protocol {
			case "responses":
				_, err = svc.forwardGrokResponses(context.Background(), c, account, []byte(`{"model":"grok-4.5","input":"hello"}`), "grok-4.5", false, time.Now())
			case "chat/completions":
				_, err = svc.ForwardAsChatCompletions(context.Background(), c, account, []byte(`{"model":"grok-4.5","messages":[{"role":"user","content":"hello"}]}`), "", "")
			case "messages":
				_, err = svc.ForwardAsAnthropic(context.Background(), c, account, []byte(`{"model":"grok-4.5","max_tokens":50,"messages":[{"role":"user","content":"hello"}]}`), "", "")
			}
			require.ErrorIs(t, err, errContentSafetyRejected)
			var failover *UpstreamFailoverError
			require.NotErrorAs(t, err, &failover)
			require.Equal(t, 403, w.Code)
			require.Len(t, upstream.requests, 1)
			require.Len(t, b.commands, 1)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
			require.Equal(t, safetyTestConfig().ClientMessage, payload["error"].(map[string]any)["message"])
			require.NotContains(t, w.Body.String(), "SAFETY_CHECK_TYPE")
			if protocol == "messages" {
				require.Equal(t, "error", payload["type"])
			}
		})
	}
}

func TestContentSafetyGatewayScopeAndNonGrok(t *testing.T) {
	b := &safetyBillingFake{balance: 1}
	safety := newSafetyTestService(t, b)
	svc := &OpenAIGatewayService{contentSafetyBilling: safety}
	c, _ := safetyGatewayContext("/v1/responses")
	body := []byte(`{"error":{"message":"Failed check: SAFETY_CHECK_TYPE"}}`)
	account := &Account{ID: 33, Platform: PlatformOpenAI}
	_, hit := svc.contentSafetyRejection(c, account, safetyUpstreamResponse(), body, "http")
	require.False(t, hit)
	account.Platform = PlatformGrok
	for i := 0; i < 3; i++ {
		_, hit = svc.contentSafetyRejection(c, account, safetyUpstreamResponse(), body, "ws:1")
		require.True(t, hit)
	}
	require.Len(t, b.commands, 1)
	_, hit = svc.contentSafetyRejection(c, account, safetyUpstreamResponse(), body, "ws:2")
	require.True(t, hit)
	require.Len(t, b.commands, 2)
}

func TestContentSafetyGatewayAccount65Forbidden(t *testing.T) {
	for _, tc := range []struct {
		name      string
		accountID int64
		status    int
		body      string
		disabled  bool
		want      bool
	}{
		{"short refusal", 65, 403, `{"error":{"message":"I'm sorry, I can't help with that request."}}`, false, true},
		{"plain forbidden", 65, 403, "Forbidden", false, true},
		{"empty body", 65, 403, "", false, true},
		{"other account", 66, 403, "Forbidden", false, false},
		{"original keyword rule", 66, 403, "Failed check: SAFETY_CHECK_TYPE", false, true},
		{"other status", 65, 400, "Forbidden", false, false},
		{"disabled", 65, 403, "Forbidden", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &safetyBillingFake{balance: 1}
			safety := newSafetyTestService(t, b)
			config := safetyTestConfig()
			config.Enabled = !tc.disabled
			require.NoError(t, safety.UpdateConfig(context.Background(), config))
			svc := &OpenAIGatewayService{contentSafetyBilling: safety}
			c, w := safetyGatewayContext("/v1/chat/completions")
			resp := &http.Response{StatusCode: tc.status, Header: http.Header{}}
			account := &Account{ID: tc.accountID, Platform: PlatformGrok}
			// Repeated handling of one request must create only one debit.
			for i := 0; i < 2; i++ {
				require.Equal(t, tc.want, svc.handleContentSafetyBilling(c, account, resp, []byte(tc.body)))
			}
			page, err := safety.Records(context.Background(), 1, 20)
			require.NoError(t, err)
			if !tc.want {
				require.Empty(t, b.commands)
				require.Empty(t, page.Items)
				require.Equal(t, float64(1), b.balance)
				return
			}
			require.Equal(t, 403, w.Code)
			require.Contains(t, w.Body.String(), config.ClientMessage)
			require.Len(t, b.commands, 1)
			require.Equal(t, 1-config.Amount, b.balance)
			require.Len(t, page.Items, 1)
			require.Equal(t, tc.accountID, page.Items[0].AccountID)
			require.Equal(t, "charged", page.Items[0].Status)
			if tc.accountID == 65 {
				require.Equal(t, "Account 65: upstream HTTP 403", page.Items[0].Notes)
			}
		})
	}
}

func TestContentSafetyGatewayMediaNativeErrorAndPendingDebit(t *testing.T) {
	b := &safetyBillingFake{balance: 1, fail: true}
	safety := newSafetyTestService(t, b)
	svc := &OpenAIGatewayService{contentSafetyBilling: safety}
	c, w := safetyGatewayContext("/v1/images/generations")
	resp := safetyUpstreamResponse()
	resp.Body = io.NopCloser(strings.NewReader(`{"code":"PermissionDenied","error":"I'm sorry, I can't help with that request. Failed check: SAFETY_CHECK_TYPE"}`))
	_, err := svc.handleGrokMediaErrorResponse(context.Background(), resp, c, &Account{ID: 33, Platform: PlatformGrok}, "xai-media", "grok-imagine")
	require.ErrorIs(t, err, errContentSafetyRejected)
	require.Equal(t, 403, w.Code)
	require.Contains(t, w.Body.String(), safetyTestConfig().ClientMessage)
	require.Equal(t, float64(1), b.balance)
	page, err := safety.Records(context.Background(), 1, 20)
	require.NoError(t, err)
	require.Equal(t, "pending", page.Items[0].Status)
	b.fail = false
	require.NoError(t, safety.Recover(context.Background()))
	require.Equal(t, 0.875, b.balance)
}
