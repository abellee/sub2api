package service

import (
	"encoding/json"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/tidwall/gjson"
)

// Reuse local token counters; transport metadata and encoded media are not text input.
func estimateCostGuardInputTokens(model string, body []byte) (int, error) {
	root := gjson.ParseBytes(body)
	if !root.IsObject() {
		return 0, fmt.Errorf("invalid cost precheck input")
	}
	if response := root.Get("response"); root.Get("type").String() == "response.create" && response.IsObject() {
		root = response
	}
	if root.Get("contents").Exists() {
		return estimateCostGuardGeminiInput(root), nil
	}
	var req openAIInputTokensCountRequest
	if root.Get("messages").Exists() {
		var converted *apicompat.ResponsesRequest
		var err error
		if costGuardAnthropicInput(root) {
			var messages apicompat.AnthropicRequest
			if err = json.Unmarshal([]byte(root.Raw), &messages); err != nil {
				return 0, err
			}
			converted, err = apicompat.AnthropicToResponses(&messages)
		} else {
			var chat apicompat.ChatCompletionsRequest
			if err = json.Unmarshal([]byte(root.Raw), &chat); err != nil {
				return 0, err
			}
			converted, err = apicompat.ChatCompletionsToResponses(&chat)
		}
		if err != nil {
			return 0, err
		}
		req = openAIInputTokensCountRequest{Instructions: converted.Instructions, Input: converted.Input, Tools: converted.Tools, ToolChoice: converted.ToolChoice}
	} else {
		if !root.Get("input").Exists() && !root.Get("instructions").Exists() {
			return 0, fmt.Errorf("unsupported cost precheck input")
		}
		if err := json.Unmarshal([]byte(root.Raw), &req); err != nil {
			return 0, err
		}
	}
	// The gateway's requested model also covers Gemini URLs and WS envelopes.
	req.Model = model
	return estimateOpenAIInputTokens(req)
}

func costGuardAnthropicInput(root gjson.Result) bool {
	if root.Get("system").Exists() || root.Get("tools.0.input_schema").Exists() {
		return true
	}
	anthropic := false
	root.Get("messages").ForEach(func(_, message gjson.Result) bool {
		message.Get("content").ForEach(func(_, part gjson.Result) bool {
			switch part.Get("type").String() {
			case "tool_use", "tool_result", "image", "document", "thinking", "redacted_thinking":
				anthropic = true
			}
			return !anthropic
		})
		return !anthropic
	})
	return anthropic
}

func estimateCostGuardGeminiInput(root gjson.Result) int {
	total := estimateGeminiCountTokens([]byte(root.Raw))
	// Native Gemini can also carry snake_case system instructions.
	root.Get("system_instruction.parts").ForEach(func(_, part gjson.Result) bool {
		total += estimateTokensForText(part.Get("text").String())
		return true
	})
	root.Get("tools").ForEach(func(_, tool gjson.Result) bool {
		for _, field := range []string{"functionDeclarations", "function_declarations"} {
			if declarations := tool.Get(field); declarations.Exists() {
				total += estimateTokensForText(declarations.Raw)
			}
		}
		return true
	})
	root.Get("contents").ForEach(func(_, content gjson.Result) bool {
		content.Get("parts").ForEach(func(_, part gjson.Result) bool {
			for _, field := range []string{"functionCall", "functionResponse", "function_call", "function_response"} {
				if value := part.Get(field); value.Exists() {
					total += estimateTokensForText(value.Raw)
				}
			}
			return true
		})
		return true
	})
	return total
}
