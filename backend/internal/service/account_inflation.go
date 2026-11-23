package service

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

const accountInflationKey = "account_inflation"

type accountInflationProbe struct {
	prompt          string
	encoding        string
	text            strings.Builder
	outputTokens    *int
	outputField     string
	reasoningTokens *int
	tooLarge        bool
}

type accountInflationResult struct {
	TokenCount      int    `json:"token_count"`
	OutputTokens    *int   `json:"output_tokens"`
	OutputField     string `json:"output_field"`
	ReasoningTokens *int   `json:"reasoning_tokens"`
	Encoding        string `json:"encoding"`
	Error           string `json:"error,omitempty"`
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
	r := accountInflationResult{OutputTokens: p.outputTokens, ReasoningTokens: p.reasoningTokens, Encoding: p.encoding, OutputField: p.outputField}
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
	return r
}
