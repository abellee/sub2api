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

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
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
	return &http.Response{StatusCode: 403, Header: http.Header{"X-Request-Id": []string{"upstream-safety"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"I'm sorry, I can't help with that request. Failed check: SAFETY_CHECK_TYPE_IMAGE"}}`))}
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
			require.NotContains(t, w.Body.String(), "SAFETY_CHECK_TYPE_IMAGE")
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
	body := []byte(`{"error":{"message":"Failed check: SAFETY_CHECK_TYPE_IMAGE"}}`)
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

func TestContentSafetyGatewayUsesSameRulesForEveryAccount(t *testing.T) {
	for _, tc := range []struct {
		name      string
		accountID int64
		status    int
		body      string
		disabled  bool
		want      bool
	}{
		{"short refusal", 65, 403, `{"error":{"message":"I'm sorry, I can't help with that request."}}`, false, true},
		{"plain forbidden", 65, 403, "Forbidden", false, false},
		{"empty body", 65, 403, "", false, false},
		{"other account", 66, 403, "Forbidden", false, false},
		{"original keyword rule", 66, 403, "Failed check: SAFETY_CHECK_TYPE_IMAGE", false, true},
		{"other status", 65, 400, "Forbidden", false, false},
		{"image moderation", 65, 400, "generated image rejected by content moderation", false, true},
		{"successful refusal", 65, 200, `{"choices":[{"message":{"content":"I cannot help with that"}}]}`, false, true},
		{"successful disabled", 65, 200, `{"output_text":"I cannot help with that"}`, true, false},
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
			require.NotContains(t, page.Items[0].Notes, "Account 65")
		})
	}
}

func TestContentSafetyGatewayMediaNativeErrorAndPendingDebit(t *testing.T) {
	b := &safetyBillingFake{balance: 1, fail: true}
	safety := newSafetyTestService(t, b)
	svc := &OpenAIGatewayService{contentSafetyBilling: safety}
	c, w := safetyGatewayContext("/v1/images/generations")
	resp := safetyUpstreamResponse()
	resp.Body = io.NopCloser(strings.NewReader(`{"code":"PermissionDenied","error":"I'm sorry, I can't help with that request. Failed check: SAFETY_CHECK_TYPE_IMAGE"}`))
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

func safetyRefusalSSE(rawChat bool) string {
	var parts []string
	for _, text := range []string{"I'm sorry, I ca", "n’t help wi", "th that request."} {
		var payload any = map[string]any{"type": "response.output_text.delta", "delta": text}
		if rawChat {
			payload = map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]string{"content": text}}}}
		}
		data, _ := json.Marshal(payload)
		parts = append(parts, "data: "+string(data)+"\n\n")
	}
	return strings.Join(parts, "")
}

func TestContentSafetyGatewaySuccessfulAndBadRequestAreTerminal(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
	}{
		{200, `{"id":"resp_safe","output":[{"type":"message","content":[{"type":"output_text","text":"I can't help with that request."}]}]}`},
		{400, `{"error":{"message":"Generated image rejected by content moderation"}}`},
	} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			billing := &safetyBillingFake{balance: 1}
			safety := newSafetyTestService(t, billing)
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: tc.status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(tc.body))}}
			svc := &OpenAIGatewayService{httpUpstream: upstream, contentSafetyBilling: safety}
			account := &Account{ID: 33, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "fake", "base_url": "https://api.x.ai"}}
			c, w := safetyGatewayContext("/v1/responses")
			result, err := svc.forwardGrokResponses(context.Background(), c, account, []byte(`{"model":"grok-4.5","input":"hello"}`), "grok-4.5", false, time.Now())
			require.ErrorIs(t, err, errContentSafetyRejected)
			require.Nil(t, result)
			require.Equal(t, 403, w.Code)
			require.Contains(t, w.Header().Get("Content-Type"), "application/json")
			require.Contains(t, w.Body.String(), safetyTestConfig().ClientMessage)
			require.Len(t, upstream.requests, 1)
			require.Len(t, billing.commands, 1)
			require.Equal(t, 1-safetyTestConfig().Amount, billing.balance)
		})
	}
}

func TestContentSafetyGatewayAllResponseReadersDetectSplitRefusals(t *testing.T) {
	for _, name := range []string{"responses", "responses passthrough", "chat", "messages", "chat buffered", "messages buffered", "raw chat", "raw chat json", "responses json", "passthrough json", "cc fallback stream", "cc fallback json"} {
		t.Run(name, func(t *testing.T) {
			billing := &safetyBillingFake{balance: 1}
			safety := newSafetyTestService(t, billing)
			svc := &OpenAIGatewayService{cfg: &config.Config{}, contentSafetyBilling: safety}
			account := &Account{ID: 33, Platform: PlatformGrok, Type: AccountTypeAPIKey}
			path := "/v1/responses"
			if strings.HasPrefix(name, "chat") || strings.HasPrefix(name, "raw chat") {
				path = "/v1/chat/completions"
			}
			if strings.HasPrefix(name, "messages") {
				path = "/v1/messages"
			}
			c, w := safetyGatewayContext(path)
			c.Set("api_key", &APIKey{ID: 22, UserID: 11, Group: &Group{RateMultiplier: 100}})
			c.Set("content_safety_account", account)
			c.Request = c.Request.WithContext(withRequestAccountFailoverCount(c.Request.Context(), 2))
			body := safetyRefusalSSE(strings.HasPrefix(name, "raw chat") || name == "cc fallback stream")
			contentType := "text/event-stream"
			if strings.HasSuffix(name, "json") {
				contentType = "application/json"
				body = `{"choices":[{"message":{"content":"I cannot help with that request."}}],"output_text":"I cannot help with that request."}`
			}
			resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(body))}
			defer resp.Body.Close()
			var err error
			switch name {
			case "responses":
				_, err = svc.handleStreamingResponse(c.Request.Context(), resp, c, account, time.Now(), "grok-4.5", "grok-4.5")
			case "responses passthrough":
				_, err = svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, account, time.Now(), "grok-4.5", "grok-4.5")
			case "chat":
				_, err = svc.handleChatStreamingResponse(resp, c, account, "grok-4.5", "grok-4.5", "grok-4.5", time.Now(), 0)
			case "messages":
				_, err = svc.handleAnthropicStreamingResponse(resp, c, account, "grok-4.5", "grok-4.5", "grok-4.5", time.Now())
			case "chat buffered":
				_, err = svc.handleChatBufferedStreamingResponse(resp, c, account, "grok-4.5", "grok-4.5", "grok-4.5", time.Now())
			case "messages buffered":
				_, err = svc.handleAnthropicBufferedStreamingResponse(resp, c, account, "grok-4.5", "grok-4.5", "grok-4.5", time.Now())
			case "raw chat":
				_, err = svc.streamRawChatCompletions(c, resp, account, "grok-4.5", "grok-4.5", "grok-4.5", nil, nil, time.Now(), 0)
			case "raw chat json":
				_, err = svc.bufferRawChatCompletions(c, resp, account, "grok-4.5", "grok-4.5", "grok-4.5", nil, nil, time.Now())
			case "responses json":
				_, err = svc.handleNonStreamingResponse(c.Request.Context(), resp, c, account, "grok-4.5", "grok-4.5")
			case "passthrough json":
				_, err = svc.handleNonStreamingResponsePassthrough(c.Request.Context(), resp, c, account, "grok-4.5", "grok-4.5")
			case "cc fallback stream":
				scan := svc.scanCCStream(c, resp, "test", "", time.Now(), func(_ *apicompat.ChatCompletionsChunk) {})
				err = scan.Err
			case "cc fallback json":
				_, _, err = svc.readCCUpstreamJSONResponse(c, resp, writeChatCompletionsError)
			}
			require.ErrorIs(t, err, errContentSafetyRejected)
			require.Nil(t, requestAccountStreamFailover(c, account, resp, err), "refusals must never become account retries")
			require.Contains(t, w.Body.String(), safetyTestConfig().ClientMessage)
			require.Contains(t, w.Body.String(), "invalid_request_error")
			require.Len(t, billing.commands, 1)
			require.Equal(t, 1-safetyTestConfig().Amount, billing.balance, "configured amount must not multiply by group rate")
			page, e := safety.Records(context.Background(), 1, 20)
			require.NoError(t, e)
			require.NotEmpty(t, page.Items[0].Notes)
		})
	}
}

func TestContentSafetyGatewayProbeDoesNotCombineAttempts(t *testing.T) {
	billing := &safetyBillingFake{balance: 1}
	svc := &OpenAIGatewayService{contentSafetyBilling: newSafetyTestService(t, billing)}
	c, _ := safetyGatewayContext("/v1/responses")
	account := &Account{ID: 33, Platform: PlatformGrok}
	first, second := &http.Response{StatusCode: 200}, &http.Response{StatusCode: 200}
	_, hit := svc.contentSafetyRejection(c, account, first, []byte(`{"type":"response.output_text.delta","delta":"I can't help with "}`), "http")
	require.False(t, hit)
	_, hit = svc.contentSafetyRejection(c, account, second, []byte(`{"type":"response.output_text.delta","delta":"that"}`), "http")
	require.False(t, hit)
	require.Empty(t, billing.commands)
}

func TestContentSafetyGatewayWebSocketSplitRefusal(t *testing.T) {
	billing := &safetyBillingFake{balance: 1}
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(safetyRefusalSSE(false)))}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream, contentSafetyBilling: newSafetyTestService(t, billing)}
	c, _ := safetyGatewayContext("/v1/responses")
	account := &Account{ID: 33, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "fake", "base_url": "https://api.x.ai"}}
	payload := []byte(`{"type":"response.create","model":"grok-4.5","input":"hello","stream":true}`)
	var packets [][]byte
	result, err := svc.proxyOpenAIWSHTTPBridgeTurn(context.Background(), c, account, "fake", payload, len(payload), "grok-4.5", "", "", "", "", 1, func(data []byte) error { packets = append(packets, append([]byte(nil), data...)); return nil })
	require.ErrorIs(t, err, errContentSafetyRejected)
	require.Nil(t, result)
	require.Len(t, billing.commands, 1)
	require.Contains(t, string(packets[len(packets)-1]), safetyTestConfig().ClientMessage)
}

func TestContentSafetyGatewayMediaSuccessRefusalPrecedesEmptyOutputFailover(t *testing.T) {
	for _, status := range []int{200, 400} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			billing := &safetyBillingFake{balance: 1}
			body := `{"message":"I cannot help with that request."}`
			if status == 400 {
				body = `{"error":{"message":"generated video rejected by content moderation"}}`
			}
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream, contentSafetyBilling: newSafetyTestService(t, billing)}
			c, w := safetyGatewayContext("/v1/images/generations")
			account := &Account{ID: 33, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "fake", "base_url": "https://api.x.ai"}}
			result, err := svc.ForwardGrokMedia(context.Background(), c, account, GrokMediaEndpointImagesGenerations, "", []byte(`{"model":"grok-imagine","prompt":"hello"}`), "application/json")
			require.ErrorIs(t, err, errContentSafetyRejected)
			require.Nil(t, result)
			require.Equal(t, 403, w.Code)
			require.Contains(t, w.Body.String(), safetyTestConfig().ClientMessage)
			require.Len(t, billing.commands, 1)
		})
	}
}
