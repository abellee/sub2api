package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/tidwall/gjson"
)

// InputTokenEstimate is a temporary observation, never a billing input.
type InputTokenEstimate struct {
	Tokens   int    `json:"tokens"`
	Encoding string `json:"encoding"`
	Partial  bool   `json:"partial,omitempty"`
}

type usageInputTokenEstimateKey struct{}

func WithUsageInputTokenEstimate(ctx context.Context, estimate *InputTokenEstimate) context.Context {
	return context.WithValue(ctx, usageInputTokenEstimateKey{}, estimate)
}

func UsageInputTokenEstimateFromContext(ctx context.Context) *InputTokenEstimate {
	if ctx == nil {
		return nil
	}
	estimate, _ := ctx.Value(usageInputTokenEstimateKey{}).(*InputTokenEstimate)
	return estimate
}

// EstimateUsageInputTokens counts submitted text, history and tools locally.
// shortcut: non-OpenAI models use a reference vocabulary until provider tokenizers are available.
func EstimateUsageInputTokens(body []byte, fallbackModel, requestPath string) *InputTokenEstimate {
	if !gjson.ValidBytes(body) {
		return nil
	}
	root := gjson.ParseBytes(body)
	model := strings.TrimSpace(root.Get("model").String())
	if model == "" {
		model = fallbackModel
	}
	var req openAIInputTokensCountRequest
	var converted *apicompat.ResponsesRequest
	var err error
	switch {
	case root.Get("contents").Exists():
		return estimateGeminiUsageInputTokens(root, model)
	case root.Get("messages").Exists() && (strings.Contains(requestPath, "/messages") || root.Get("system").Exists()):
		var anthropic apicompat.AnthropicRequest
		if err = json.Unmarshal(body, &anthropic); err == nil {
			converted, err = apicompat.AnthropicToResponsesWithOptions(&anthropic, apicompat.AnthropicToResponsesOptions{DeferReasoningEffortValidation: true})
		}
	case root.Get("messages").Exists():
		var chat apicompat.ChatCompletionsRequest
		if err = json.Unmarshal(body, &chat); err == nil {
			chat.ReasoningEffort = "" // Output settings must not prevent an input-only observation.
			converted, err = apicompat.ChatCompletionsToResponses(&chat)
		}
	case root.Get("input").Exists() || root.Get("instructions").Exists():
		err = json.Unmarshal(body, &req)
	case root.Get("prompt").Type == gjson.String:
		req.Input = json.RawMessage(root.Get("prompt").Raw)
	default:
		return nil
	}
	if err != nil {
		return nil
	}
	if converted != nil {
		req.Instructions, req.Input = converted.Instructions, converted.Input
		req.Tools, req.ToolChoice = converted.Tools, converted.ToolChoice
	}
	req.Model = model
	tokens, err := estimateOpenAIInputTokens(req)
	if err != nil {
		return nil
	}
	codec, err := openAIInputTokensCodecForModel(model)
	if err != nil {
		return nil
	}
	for _, path := range []string{"response_format", "text.format"} {
		if value := root.Get(path); value.Exists() {
			text, err := compactOpenAIInputTokensJSON(json.RawMessage(value.Raw))
			if err != nil {
				return nil
			}
			n, err := codec.Count(text)
			if err != nil {
				return nil
			}
			tokens += n
		}
	}
	return &InputTokenEstimate{Tokens: tokens, Encoding: string(openAIInputTokensEncodingForModel(model)), Partial: usageInputHasOpaqueContent(root)}
}

func estimateGeminiUsageInputTokens(root gjson.Result, model string) *InputTokenEstimate {
	codec, err := openAIInputTokensCodecForModel(model)
	if err != nil {
		return nil
	}
	total := 0
	var countErr error
	for _, field := range []string{"systemInstruction", "system_instruction", "contents"} {
		value := root.Get(field)
		if !value.Exists() {
			continue
		}
		items := value.Array()
		if value.IsObject() {
			items = []gjson.Result{value}
		}
		for _, item := range items {
			total += openAIResponsesInputItemTokenOverhead
			for _, part := range item.Get("parts").Array() {
				text := part.Get("text")
				if text.Exists() {
					n, err := codec.Count(text.String())
					if err != nil {
						countErr = err
					}
					total += n
				}
				for _, key := range []string{"functionCall", "functionResponse"} {
					if raw := part.Get(key); raw.Exists() {
						n, err := codec.Count(raw.Raw)
						if err != nil {
							countErr = err
						}
						total += n
					}
				}
			}
		}
	}
	for _, field := range []string{"tools", "toolConfig", "generationConfig.responseSchema", "generationConfig.responseJsonSchema"} {
		if raw := root.Get(field); raw.Exists() {
			text, err := compactOpenAIInputTokensJSON(json.RawMessage(raw.Raw))
			if err != nil {
				return nil
			}
			n, err := codec.Count(text)
			if err != nil {
				return nil
			}
			total += n
		}
	}
	if countErr != nil {
		return nil
	}
	return &InputTokenEstimate{Tokens: total, Encoding: codec.GetName(), Partial: usageInputHasOpaqueContent(root)}
}

func usageInputHasOpaqueContent(value gjson.Result) bool {
	partial := false
	if value.IsObject() || value.IsArray() {
		value.ForEach(func(key, child gjson.Result) bool {
			switch key.String() {
			case "previous_response_id", "conversation", "cachedContent", "cached_content", "image_url", "image_file", "inlineData", "inline_data", "fileData", "file_data", "encrypted_content":
				partial = child.Exists() && child.String() != ""
			case "type":
				switch child.String() {
				case "image", "input_image", "input_audio", "audio", "input_file", "document", "video", "compaction":
					partial = true
				}
			}
			partial = partial || usageInputHasOpaqueContent(child)
			return !partial
		})
	}
	return partial
}
