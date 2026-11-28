package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"lotteryd/api/events"
)

// Called at existing semantic-output observations; never reads or changes stream bytes.
func observeGameplayFirstOutput(c *gin.Context) {
	if writer := conversationWriter(c); writer != nil {
		writer.markOutput()
	}
	defer func() { _ = recover() }()
	if !events.Enabled() || c == nil || c.Request == nil || c.GetBool("gameplay_first_output") {
		return
	}
	if c.Request.Context().Err() != nil {
		return
	}
	raw, ok := c.Get("api_key")
	if !ok {
		return
	}
	key, ok := raw.(*APIKey)
	if !ok || key == nil {
		return
	}
	c.Set("gameplay_first_output", true)
	requestID, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
	data := map[string]string{"api_key_id": events.S(key.ID), "model": c.GetString("ops_model"), "account_id": events.S(c.GetInt64("ops_account_id")), "status": "output"}
	if key.GroupID != nil {
		data["group_id"] = events.S(*key.GroupID)
	}
	events.Publish("request.first_output", key.UserID, requestID, data)
}

func observeGameplayAnthropicOutput(c *gin.Context, payload string) {
	if c == nil || (!events.Enabled() && conversationWriter(c) == nil) || c.GetBool("gameplay_first_output") {
		return
	}
	for _, field := range []string{"delta.text", "delta.thinking", "delta.partial_json", "content_block.text", "content_block.thinking", "content_block.source"} {
		if v := gjson.Get(payload, field); v.Exists() && v.String() != "" {
			observeGameplayFirstOutput(c)
			return
		}
	}
	if gjson.Get(payload, "content_block.type").String() == "tool_use" {
		observeGameplayFirstOutput(c)
	}
}

func observeGameplayChatOutput(c *gin.Context, payload string) {
	if c == nil || (!events.Enabled() && conversationWriter(c) == nil) || c.GetBool("gameplay_first_output") {
		return
	}
	for _, field := range []string{"choices.0.delta.content", "choices.0.delta.reasoning_content", "choices.0.delta.tool_calls.0", "choices.0.delta.function_call"} {
		if v := gjson.Get(payload, field); v.Exists() && v.String() != "" {
			observeGameplayFirstOutput(c)
			return
		}
	}
}

func observeGameplayGeminiOutput(c *gin.Context, payload []byte) {
	if c == nil || (!events.Enabled() && conversationWriter(c) == nil) || c.GetBool("gameplay_first_output") {
		return
	}
	for _, field := range []string{"candidates.0.content.parts", "response.candidates.0.content.parts"} {
		for _, part := range gjson.GetBytes(payload, field).Array() {
			if part.Get("text").String() != "" || part.Get("functionCall").Exists() || part.Get("inlineData").Exists() {
				observeGameplayFirstOutput(c)
				return
			}
		}
	}
}

// Keep protocol-specific return values and errors intact; observation happens after forwarding.
func (s *GeminiMessagesCompatService) ForwardNative(ctx context.Context, c *gin.Context, account *Account, model, action string, stream bool, body []byte) (*ForwardResult, error) {
	start := time.Now()
	result, err := s.forwardNativeWithGameplayObservation(ctx, c, account, model, action, stream, body)
	observeGameplayForwardResult(c, account, start, result, err)
	return result, err
}
func (s *GatewayService) ForwardAsChatCompletions(ctx context.Context, c *gin.Context, account *Account, body []byte, parsed *ParsedRequest) (*ForwardResult, error) {
	start := time.Now()
	result, err := s.forwardChatCompletionsWithGameplayObservation(ctx, c, account, body, parsed)
	observeGameplayForwardResult(c, account, start, result, err)
	return result, err
}
func (s *GatewayService) ForwardAsResponses(ctx context.Context, c *gin.Context, account *Account, body []byte, parsed *ParsedRequest) (*ForwardResult, error) {
	start := time.Now()
	result, err := s.forwardResponsesWithGameplayObservation(ctx, c, account, body, parsed)
	observeGameplayForwardResult(c, account, start, result, err)
	return result, err
}
func (s *AntigravityGatewayService) ForwardAsChatCompletions(ctx context.Context, c *gin.Context, account *Account, body []byte, parsed *ParsedRequest) (*ForwardResult, error) {
	start := time.Now()
	result, err := s.forwardChatCompletionsWithGameplayObservation(ctx, c, account, body, parsed)
	observeGameplayForwardResult(c, account, start, result, err)
	return result, err
}
func (s *AntigravityGatewayService) ForwardAsResponses(ctx context.Context, c *gin.Context, account *Account, body []byte, parsed *ParsedRequest) (*ForwardResult, error) {
	start := time.Now()
	result, err := s.forwardResponsesWithGameplayObservation(ctx, c, account, body, parsed)
	observeGameplayForwardResult(c, account, start, result, err)
	return result, err
}
func (s *OpenAIGatewayService) ForwardAsChatCompletions(ctx context.Context, c *gin.Context, account *Account, body []byte, promptCacheKey, defaultMappedModel string) (*OpenAIForwardResult, error) {
	start := time.Now()
	result, err := s.forwardChatCompletionsWithGameplayObservation(ctx, c, account, body, promptCacheKey, defaultMappedModel)
	observeGameplayOpenAIForwardResult(c, account, start, result, err)
	return result, err
}
func (s *OpenAIGatewayService) ForwardAsAnthropic(ctx context.Context, c *gin.Context, account *Account, body []byte, promptCacheKey, defaultMappedModel string) (*OpenAIForwardResult, error) {
	start := time.Now()
	result, err := s.forwardAnthropicWithGameplayObservation(ctx, c, account, body, promptCacheKey, defaultMappedModel)
	observeGameplayOpenAIForwardResult(c, account, start, result, err)
	return result, err
}
func (s *OpenAIGatewayService) ForwardEmbeddings(ctx context.Context, c *gin.Context, account *Account, body []byte, defaultMappedModel string) (*OpenAIForwardResult, error) {
	start := time.Now()
	result, err := s.forwardEmbeddingsWithGameplayObservation(ctx, c, account, body, defaultMappedModel)
	observeGameplayOpenAIForwardResult(c, account, start, result, err)
	return result, err
}
func (s *OpenAIGatewayService) ForwardImages(ctx context.Context, c *gin.Context, account *Account, body []byte, parsed *OpenAIImagesRequest, channelMappedModel string) (*OpenAIForwardResult, error) {
	start := time.Now()
	result, err := s.forwardImagesWithGameplayObservation(ctx, c, account, body, parsed, channelMappedModel)
	observeGameplayOpenAIForwardResult(c, account, start, result, err)
	return result, err
}
func (s *OpenAIGatewayService) ForwardGrokMedia(ctx context.Context, c *gin.Context, account *Account, endpoint GrokMediaEndpoint, requestID string, body []byte, contentType string) (*OpenAIForwardResult, error) {
	start := time.Now()
	result, err := s.forwardGrokMediaWithGameplayObservation(ctx, c, account, endpoint, requestID, body, contentType)
	observeGameplayOpenAIForwardResult(c, account, start, result, err)
	return result, err
}
func (s *OpenAIGatewayService) ForwardSeedance(ctx context.Context, c *gin.Context, account *Account, endpoint GrokMediaEndpoint, taskID string, body []byte) (*OpenAIForwardResult, error) {
	start := time.Now()
	result, err := s.forwardSeedanceWithGameplayObservation(ctx, c, account, endpoint, taskID, body)
	observeGameplayOpenAIForwardResult(c, account, start, result, err)
	return result, err
}
func (s *OpenAIGatewayService) ForwardAlphaSearch(ctx context.Context, c *gin.Context, account *Account, body []byte) (*OpenAIForwardResult, error) {
	start := time.Now()
	result, err := s.forwardAlphaSearchWithGameplayObservation(ctx, c, account, body)
	observeGameplayOpenAIForwardResult(c, account, start, result, err)
	return result, err
}
func (s *OpenAIGatewayService) ForwardGrokVoice(ctx context.Context, c *gin.Context, account *Account, endpoint string, body []byte, contentType string) (*OpenAIForwardResult, error) {
	start := time.Now()
	result, err := s.forwardGrokVoiceWithGameplayObservation(ctx, c, account, endpoint, body, contentType)
	observeGameplayOpenAIForwardResult(c, account, start, result, err)
	return result, err
}
func (s *GatewayService) ForwardSystemOne(ctx context.Context, c *gin.Context, account *Account, body []byte) (*SystemOneForwardResult, error) {
	start := time.Now()
	result, err := s.forwardSystemOneWithGameplayObservation(ctx, c, account, body)
	var forward *ForwardResult
	if result != nil {
		forward = &result.ForwardResult
	}
	observeGameplayForwardResult(c, account, start, forward, err)
	return result, err
}
