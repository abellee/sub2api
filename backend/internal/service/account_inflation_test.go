//go:build unit

package service

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountInflationStreams(t *testing.T) {
	svc := &AccountTestService{}
	for _, tc := range []struct {
		name   string
		stream string
		parse  func(*gin.Context, *strings.Reader) error
		output int
		field  string
	}{
		{"responses", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello world\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"output_tokens\":4,\"output_tokens_details\":{\"reasoning_tokens\":2}}}}\n\n", func(c *gin.Context, r *strings.Reader) error { return svc.processOpenAIStream(c, r) }, 4, "output_tokens"},
		{"chat", "data: {\"choices\":[{\"delta\":{\"content\":\"Hello world\"},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"completion_tokens\":4}}\n\ndata: [DONE]\n\n", func(c *gin.Context, r *strings.Reader) error { return svc.processOpenAIChatCompletionsStream(c, r) }, 4, "completion_tokens"},
		{"claude", "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"output_tokens\":1}}}\n\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"Hello world\"}}\n\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":4}}\n\ndata: {\"type\":\"message_stop\"}\n\n", func(c *gin.Context, r *strings.Reader) error { return svc.processClaudeStream(c, r) }, 4, "output_tokens"},
		{"gemini", "data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hidden\",\"thought\":true},{\"text\":\"Hello world\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"candidatesTokenCount\":2,\"thoughtsTokenCount\":2}}}\n\n", func(c *gin.Context, r *strings.Reader) error { return svc.processGeminiStream(c, r) }, 4, "candidatesTokenCount+thoughtsTokenCount"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newTestContext()
			p := &accountInflationProbe{encoding: "o200k_base"}
			c.Set(accountInflationKey, p)
			require.NoError(t, tc.parse(c, strings.NewReader(tc.stream)))
			require.Equal(t, "Hello world", p.text.String())
			var result accountInflationResult
			count := 0
			for _, line := range strings.Split(rec.Body.String(), "\n") {
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				var event struct {
					Type string                 `json:"type"`
					Data accountInflationResult `json:"data"`
				}
				require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event))
				if event.Type == "inflation_result" {
					result = event.Data
					count++
				}
			}
			require.Equal(t, 1, count)
			require.Empty(t, result.Error)
			require.Equal(t, 2, result.TokenCount)
			require.NotNil(t, result.OutputTokens)
			require.Equal(t, tc.output, *result.OutputTokens)
			require.Equal(t, tc.field, result.OutputField)
		})
	}
}

func TestAccountInflationMissingUsageAndLimits(t *testing.T) {
	p := &accountInflationProbe{encoding: "cl100k_base"}
	p.text.WriteString("Hello world")
	r := p.result()
	require.Equal(t, 2, r.TokenCount)
	require.Nil(t, r.OutputTokens)
	p.tooLarge = true
	require.NotEmpty(t, p.result().Error)
}

func TestAccountInflationSharedPrompt(t *testing.T) {
	c, _ := newTestContext()
	c.Set(accountInflationKey, &accountInflationProbe{prompt: "Same prompt 中文"})
	for _, payload := range []map[string]any{
		{"messages": []any{}, "stream": true},
		{"messages": []any{}, "stream": true, "max_tokens": 1024, "system": "required client identity"},
		{"input": []any{}, "stream": true, "instructions": "required client identity"},
	} {
		applyInflationPrompt(c, payload)
		b, err := json.Marshal(payload)
		require.NoError(t, err)
		require.Contains(t, string(b), "Same prompt 中文")
		if _, hasInput := payload["input"]; !hasInput {
			_, anthropic := payload["max_tokens"]
			_, hasUsage := payload["stream_options"]
			require.Equal(t, !anthropic, hasUsage)
		}
	}
}

func TestAccountInflationConnection(t *testing.T) {
	account := &Account{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "test-token"}}
	repo := &openAIAccountTestRepo{mockAccountRepoForGemini: mockAccountRepoForGemini{accountsByID: map[int64]*Account{2: account}}}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(http.StatusOK,
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello world\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"output_tokens\":4}}}\n\n")}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}
	c, rec := newTestContext()
	require.NoError(t, svc.TestAccountConnection(c, 2, "gpt-5.6-sol", "Shared prompt 中文", "text", AccountTestOptions{MeasureTokens: true, TokenizerEncoding: "o200k_base"}))
	require.Len(t, upstream.requests, 1)
	body, err := io.ReadAll(upstream.requests[0].Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Shared prompt 中文")
	require.Contains(t, rec.Body.String(), `"token_count":2,"output_tokens":4`)

	c, rec = newTestContext()
	require.Error(t, svc.TestAccountConnection(c, 2, "gpt-5.6-sol", "", "text", AccountTestOptions{MeasureTokens: true, TokenizerEncoding: "o200k_base"}))
	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Body.String(), "Prompt must contain")
	require.Len(t, upstream.requests, 1)
}

func TestAccountInflationReasoningStreams(t *testing.T) {
	svc := &AccountTestService{}
	for _, tc := range []struct {
		name   string
		stream string
		parse  func(*gin.Context, io.Reader) error
	}{
		{"anthropic", `data: {"type":"content_block_start","content_block":{"type":"thinking","thinking":"Hello "}}

data: {"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"world"}}

data: {"type":"content_block_delta","delta":{"type":"signature_delta","signature":"secret-signature"}}

data: {"type":"content_block_start","content_block":{"type":"redacted_thinking","data":"secret-encrypted-data"}}

data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Answer"}}

data: {"type":"message_stop"}

`, svc.processClaudeStream},
		{"responses-summary", `data: {"type":"response.reasoning_summary_text.delta","delta":"Hello world"}

data: {"type":"response.reasoning_summary_text.done","text":"Hello world"}

data: {"type":"response.output_text.delta","delta":"Answer"}

data: {"type":"response.completed"}

`, svc.processOpenAIStream},
		{"chat", `data: {"choices":[{"delta":{"reasoning_content":"Hello world"}}]}

data: {"choices":[{"delta":{"content":"Answer"},"finish_reason":"stop"}]}

data: [DONE]

`, svc.processOpenAIChatCompletionsStream},
		{"gemini", `data: {"response":{"candidates":[{"content":{"parts":[{"thought":true,"text":"Hello world","thoughtSignature":"secret-signature"},{"text":"Answer"}]},"finishReason":"STOP"}]}}

`, svc.processGeminiStream},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newTestContext()
			p := &accountInflationProbe{encoding: "o200k_base"}
			c.Set(accountInflationKey, p)
			require.NoError(t, tc.parse(c, strings.NewReader(tc.stream)))
			require.Equal(t, "Answer", p.text.String())
			require.Equal(t, "Hello world", p.reasoning.String())
			r := p.result()
			require.Equal(t, 1, r.TokenCount)
			require.NotNil(t, r.ReasoningTextTokens)
			require.Equal(t, 2, *r.ReasoningTextTokens)
			require.Equal(t, tc.name == "anthropic", r.ReasoningRedacted)
			require.Contains(t, rec.Body.String(), `"type":"reasoning"`)
			require.NotContains(t, rec.Body.String(), "secret-")
		})
	}
}

func TestAccountInflationReasoningNonStreamingAndLimit(t *testing.T) {
	svc := &AccountTestService{}
	data := map[string]any{"content": []any{map[string]any{"type": "thinking", "thinking": "Hello world", "signature": "secret"}}}
	c, rec := newTestContext()
	svc.captureInflationReasoning(c, data)
	require.Empty(t, rec.Body.String())
	p := &accountInflationProbe{encoding: "o200k_base"}
	c.Set(accountInflationKey, p)
	svc.captureInflationReasoning(c, data)
	require.Equal(t, "Hello world", p.reasoning.String())
	svc.sendEvent(c, TestEvent{Type: "reasoning", Text: strings.Repeat("x", 1<<20)})
	require.True(t, p.tooLarge)
	require.NotEmpty(t, p.result().Error)
}

func TestAccountInflationAntigravityReasoning(t *testing.T) {
	sse := []byte(`data: {"response":{"candidates":[{"content":{"parts":[{"thought":true,"text":"Hello world"},{"text":"Answer"}]}}]}}`)
	text, thinking, images := extractContentWithReasoningFromSSEResponse(sse)
	require.Equal(t, "Answer", text)
	require.Equal(t, "Hello world", thinking)
	require.Empty(t, images)
	text, _, images = extractContentWithReasoningFromSSEResponse(sse)
	require.Equal(t, "Answer", text)
	require.Empty(t, images)
}
