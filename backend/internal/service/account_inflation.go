package service

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

const accountInflationKey = "account_inflation"

type accountInflationProbe struct {
	prompt            string
	encoding          string
	text              strings.Builder
	reasoning         strings.Builder
	reasoningRedacted bool
	outputTokens      *int
	outputField       string
	reasoningTokens   *int
	tooLarge          bool
}

type accountInflationResult struct {
	TokenCount          int    `json:"token_count"`
	OutputTokens        *int   `json:"output_tokens"`
	OutputField         string `json:"output_field"`
	ReasoningTokens     *int   `json:"reasoning_tokens"`
	ReasoningTextTokens *int   `json:"reasoning_text_tokens,omitempty"`
	ReasoningRedacted   bool   `json:"reasoning_redacted,omitempty"`
	Encoding            string `json:"encoding"`
	Error               string `json:"error,omitempty"`
}

// Only expose reasoning the upstream actually returned; never signatures or encrypted blocks.
func (s *AccountTestService) captureInflationReasoning(c *gin.Context, data map[string]any) {
	if inflationProbe(c) == nil {
		return
	}
	emit := func(value any) {
		if text, ok := value.(string); ok && text != "" {
			s.sendEvent(c, TestEvent{Type: "reasoning", Text: text})
		}
	}
	block := func(value any) {
		if b, ok := value.(map[string]any); ok {
			if b["type"] == "redacted_thinking" {
				s.sendEvent(c, TestEvent{Type: "reasoning_redacted"})
			}
			if b["type"] == "thinking" {
				emit(b["thinking"])
			}
		}
	}
	switch data["type"] {
	case "content_block_start":
		block(data["content_block"])
	case "content_block_delta":
		if delta, ok := data["delta"].(map[string]any); ok && delta["type"] == "thinking_delta" {
			emit(delta["thinking"])
		}
	case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
		emit(data["delta"])
	}
	if blocks, ok := data["content"].([]any); ok {
		for _, b := range blocks {
			block(b)
		}
	}
	if choices, ok := data["choices"].([]any); ok {
		for _, value := range choices {
			if choice, ok := value.(map[string]any); ok {
				delta, ok := choice["delta"].(map[string]any)
				if !ok {
					delta, _ = choice["message"].(map[string]any)
				}
				if text, ok := delta["reasoning_content"].(string); ok {
					emit(text)
				} else {
					emit(delta["reasoning"])
				}
			}
		}
	}
	if response, ok := data["response"].(map[string]any); ok {
		s.captureInflationReasoning(c, response)
	}
	if candidates, ok := data["candidates"].([]any); ok && len(candidates) > 0 {
		candidate, _ := candidates[0].(map[string]any)
		content, _ := candidate["content"].(map[string]any)
		parts, _ := content["parts"].([]any)
		for _, value := range parts {
			if part, ok := value.(map[string]any); ok && part["thought"] == true {
				emit(part["text"])
			}
		}
	}
}

func inflationProbe(c *gin.Context) *accountInflationProbe {
	v, _ := c.Get(accountInflationKey)
	p, _ := v.(*accountInflationProbe)
	return p
}

// Keep the provider's required client fields, replacing only the user prompt.
func applyInflationPrompt(c *gin.Context, payload map[string]any) {
	p := inflationProbe(c)
	if p == nil {
		return
	}
	if _, ok := payload["messages"]; ok {
		payload["messages"] = []map[string]any{{"role": "user", "content": p.prompt}}
	}
	if _, ok := payload["input"]; ok {
		payload["input"] = []map[string]any{{"role": "user", "content": []map[string]any{{"type": "input_text", "text": p.prompt}}}}
	}
	if stream, _ := payload["stream"].(bool); stream {
		if _, ok := payload["messages"]; ok {
			if _, anthropic := payload["max_tokens"]; !anthropic {
				payload["stream_options"] = map[string]any{"include_usage": true}
			}
		}
	}
}

// Usage values are cumulative snapshots, never a sum of streamed chunks.
func captureInflationUsage(c *gin.Context, data map[string]any) {
	p := inflationProbe(c)
	if p == nil {
		return
	}
	for _, key := range []string{"response", "message"} {
		if nested, ok := data[key].(map[string]any); ok {
			captureInflationUsage(c, nested)
		}
	}
	if usage, ok := data["usage"].(map[string]any); ok {
		for _, key := range []string{"output_tokens", "completion_tokens"} {
			if n, ok := usage[key].(float64); ok && n >= 0 && n == float64(int(n)) {
				value := int(n)
				p.outputTokens = &value
				p.outputField = key
				break
			}
		}
		for _, key := range []string{"output_tokens_details", "completion_tokens_details"} {
			if details, ok := usage[key].(map[string]any); ok {
				if n, ok := details["reasoning_tokens"].(float64); ok && n >= 0 {
					value := int(n)
					p.reasoningTokens = &value
				}
			}
		}
	}
	if usage, ok := data["usageMetadata"].(map[string]any); ok {
		if n, ok := usage["thoughtsTokenCount"].(float64); ok && n >= 0 {
			value := int(n)
			p.reasoningTokens = &value
		}
		if n, ok := usage["candidatesTokenCount"].(float64); ok && n >= 0 {
			value := int(n)
			p.outputField = "candidatesTokenCount"
			if p.reasoningTokens != nil {
				value += *p.reasoningTokens
				p.outputField += "+thoughtsTokenCount"
			}
			p.outputTokens = &value
		}
	}
}

func (p *accountInflationProbe) result() accountInflationResult {
	r := accountInflationResult{OutputTokens: p.outputTokens, ReasoningTokens: p.reasoningTokens, Encoding: p.encoding, OutputField: p.outputField, ReasoningRedacted: p.reasoningRedacted}
	if p.tooLarge {
		r.Error = "Output exceeded the 1 MiB measurement limit"
		return r
	}
	codec, err := openAIO200kCodec()
	if p.encoding == "cl100k_base" {
		codec, err = openAICl100kCodec()
	}
	if err != nil {
		r.Error = fmt.Sprintf("Tokenizer unavailable: %v", err)
		return r
	}
	r.TokenCount, err = codec.Count(p.text.String())
	if err != nil {
		r.Error = fmt.Sprintf("Tokenizer failed: %v", err)
	}
	if p.reasoning.Len() > 0 {
		count, countErr := codec.Count(p.reasoning.String())
		if countErr != nil {
			r.Error = fmt.Sprintf("Reasoning tokenizer failed: %v", countErr)
		} else {
			r.ReasoningTextTokens = &count
		}
	}
	return r
}
