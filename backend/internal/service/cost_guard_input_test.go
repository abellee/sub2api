package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCostGuardInputIgnoresTransportPayload(t *testing.T) {
	opaque := strings.Repeat("A", 6_300_000)
	for _, tc := range []struct {
		name, plain, padded string
	}{
		{
			"responses encrypted reasoning and metadata",
			`{"input":[{"role":"user","content":"hello"},{"type":"reasoning","id":"rs_1","encrypted_content":"short"}]}`,
			`{"input":[{"role":"user","content":"hello"},{"type":"reasoning","id":"rs_1","encrypted_content":"` + opaque + `"}],"metadata":{"trace":"` + opaque + `"}}`,
		},
		{
			"responses inline image",
			`{"input":[{"role":"user","content":[{"type":"input_text","text":"hello"},{"type":"input_image","image_url":"data:image/png;base64,short"}]}]}`,
			`{"input":[{"role":"user","content":[{"type":"input_text","text":"hello"},{"type":"input_image","image_url":"data:image/png;base64,` + opaque + `"}]}]}`,
		},
		{
			"chat inline image",
			`{"messages":[{"role":"user","content":[{"type":"text","text":"hello"},{"type":"image_url","image_url":{"url":"data:image/png;base64,short"}}]}]}`,
			`{"messages":[{"role":"user","content":[{"type":"text","text":"hello"},{"type":"image_url","image_url":{"url":"data:image/png;base64,` + opaque + `"}}]}]}`,
		},
		{
			"anthropic inline image",
			`{"messages":[{"role":"user","content":[{"type":"text","text":"hello"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"short"}}]}]}`,
			`{"messages":[{"role":"user","content":[{"type":"text","text":"hello"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"` + opaque + `"}}]}]}`,
		},
		{
			"gemini inline data and thought signature",
			`{"contents":[{"role":"user","parts":[{"text":"hello"},{"inlineData":{"mimeType":"image/png","data":"short"},"thoughtSignature":"short"}]}]}`,
			`{"contents":[{"role":"user","parts":[{"text":"hello"},{"inlineData":{"mimeType":"image/png","data":"` + opaque + `"},"thoughtSignature":"` + opaque + `"}]}]}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plain, err := estimateCostGuardInputTokens("gpt-6-astra", []byte(tc.plain))
			require.NoError(t, err)
			padded, err := estimateCostGuardInputTokens("gpt-6-astra", []byte(tc.padded))
			require.NoError(t, err)
			require.Positive(t, plain)
			require.Equal(t, plain, padded, "binary and transport fields must not inflate text input")
		})
	}
}

func TestCostGuardCountsDecodedInputAndConversation(t *testing.T) {
	for _, tc := range []struct{ name, plain, expanded string }{
		{"responses text", `{"input":"hello"}`, `{"input":"hello world hello world hello world"}`},
		{"responses instructions", `{"input":"hello"}`, `{"instructions":"Read the entire conversation carefully before answering.","input":"hello"}`},
		{"responses history", `{"input":[{"role":"user","content":"hello"}]}`, `{"input":[{"role":"user","content":"hello"},{"role":"assistant","content":"This is the previous assistant response."}]}`},
		{"tool arguments and results", `{"input":"hello"}`, `{"input":[{"type":"function_call","name":"lookup","arguments":"{\"query\":\"important search terms\"}"},{"type":"function_call_output","output":"These are the detailed results of the tool call."}]}`},
		{"tool definitions", `{"input":"hello"}`, `{"input":"hello","tools":[{"type":"function","name":"lookup","description":"Search the database for relevant results.","parameters":{"type":"object","properties":{"query":{"type":"string"}}}}]}`},
		{"chat history", `{"messages":[{"role":"user","content":"hello"}]}`, `{"messages":[{"role":"system","content":"You are a careful assistant."},{"role":"user","content":"hello"},{"role":"assistant","content":"This is the previous assistant response."}]}`},
		{"anthropic tools", `{"system":"hello","messages":[{"role":"user","content":"hello"}]}`, `{"system":"hello","messages":[{"role":"assistant","content":[{"type":"tool_use","id":"tool_1","name":"lookup","input":{"query":"important search terms"}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool_1","content":"These are the detailed results of the tool call."}]}]}`},
		{"gemini functions", `{"contents":[{"parts":[{"text":"hello"}]}]}`, `{"systemInstruction":{"parts":[{"text":"You are a careful assistant."}]},"contents":[{"parts":[{"text":"hello"},{"functionResponse":{"name":"lookup","response":{"result":"These are the detailed search results."}}}]}],"tools":[{"functionDeclarations":[{"name":"lookup","description":"Search for relevant results."}]}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plain, err := estimateCostGuardInputTokens("gpt-6-astra", []byte(tc.plain))
			require.NoError(t, err)
			expanded, err := estimateCostGuardInputTokens("gpt-6-astra", []byte(tc.expanded))
			require.NoError(t, err)
			require.Greater(t, expanded, plain)
		})
	}
	plain, err := estimateCostGuardInputTokens("gpt-6-astra", []byte(`{"input":"你好世界"}`))
	require.NoError(t, err)
	escaped, err := estimateCostGuardInputTokens("gpt-6-astra", []byte(`{"input":"\u4f60\u597d\u4e16\u754c"}`))
	require.NoError(t, err)
	require.Equal(t, plain, escaped)
	ws, err := estimateCostGuardInputTokens("gpt-6-astra", []byte(`{"type":"response.create","response":{"input":"你好世界"}}`))
	require.NoError(t, err)
	require.Equal(t, plain, ws)
}

func TestCostGuardLargeBodyDoesNotTriggerLongContextPrice(t *testing.T) {
	withCostGuard(t, true)
	model := "gpt-6-astra"
	billing := NewBillingService(&config.Config{}, &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		model: {
			InputCostPerToken: 10e-6, OutputCostPerToken: 50e-6, CacheReadInputTokenCost: 1e-6,
			MaxOutputTokens: 128000, LongContextInputTokenThreshold: 272000,
			LongContextInputCostMultiplier: 2, LongContextOutputCostMultiplier: 1.5,
		},
	}})
	d := inflightEstimateDeps{billing: billing, resolver: NewModelPricingResolver(nil, billing)}
	groupID := int64(2)
	key := &APIKey{ID: 1, User: &User{ID: 553}, GroupID: &groupID, Group: &Group{RateMultiplier: 0.039, LongContextPricingEnabled: true}}
	text := strings.Repeat("hello world ", 95000)
	plain, err := json.Marshal(map[string]any{"input": []any{map[string]any{"role": "user", "content": text}, map[string]any{"type": "reasoning", "encrypted_content": "short"}}})
	require.NoError(t, err)
	large := []byte(strings.Replace(string(plain), "short", strings.Repeat("A", 6_300_000), 1))
	tokens, err := estimateCostGuardInputTokens(model, large)
	require.NoError(t, err)
	require.Less(t, tokens, 272000)
	for _, body := range [][]byte{plain, large} {
		cost, priced := d.estimate(context.Background(), key, InflightEstimateRequest{Model: model, Body: body, BodyBytes: len(body)})
		require.True(t, priced)
		require.InDelta(t, (float64(tokens)*10e-6+128000*50e-6)*0.039, cost, 1e-12)
		require.False(t, costGuardRejects(1, cost), "one dollar must cover this welfare-group request")
	}
	// Real long text still uses the long-context rate.
	longBody, err := json.Marshal(map[string]any{"input": strings.Repeat("hello world ", 150000)})
	require.NoError(t, err)
	longTokens, err := estimateCostGuardInputTokens(model, longBody)
	require.NoError(t, err)
	require.Greater(t, longTokens, 272000)
	cost, priced := d.estimate(context.Background(), key, InflightEstimateRequest{Model: model, Body: longBody, BodyBytes: len(longBody)})
	require.True(t, priced)
	require.InDelta(t, (float64(longTokens)*20e-6+128000*75e-6)*0.039, cost, 1e-12)
}

func TestCostGuardInputFailureSkipsEstimateAndDisabledKeepsLegacy(t *testing.T) {
	settings := withCostGuard(t, true)
	d := inflightEstimateDeps{billing: costGuardGPT61Billing()}
	key := &APIKey{ID: 1, User: &User{ID: 1}, Group: &Group{RateMultiplier: 0.039}}
	for _, body := range []string{`not json`, `{"metadata":{"trace":"not input"}}`, `{"input":`} {
		req := InflightEstimateRequest{Model: "gpt-6.1-sol", Body: []byte(body), BodyBytes: 6_300_000, MaxTokens: 100}
		cost, priced := d.estimate(context.Background(), key, req)
		require.True(t, priced, "unknown input must not become a fail-closed pricing error")
		require.Zero(t, cost)
		settings.costGuardEnabled.Store(false)
		legacy, priced := d.estimate(context.Background(), key, req)
		require.True(t, priced)
		require.Positive(t, legacy, "disabled guard must not parse the body")
		settings.costGuardEnabled.Store(true)
	}
}
