package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"lotteryd/api/events"
	"time"
)

// Gateway snapshots contain identifiers and measurements only, never prompts or credentials.
func observeGameplayForward(c *gin.Context, account *Account, start time.Time, first *int, hasResult bool, err error) {
	defer func() { _ = recover() }()
	if !events.Enabled() || c == nil || account == nil {
		return
	}
	uid := int64(0)
	data := map[string]string{"account_id": events.S(account.ID), "platform": account.Platform, "model": c.GetString("ops_model"), "duration_ms": events.S(time.Since(start).Milliseconds())}
	if raw, ok := c.Get("api_key"); ok {
		if key, ok := raw.(*APIKey); ok {
			uid = key.UserID
			data["api_key_id"] = events.S(key.ID)
			if key.GroupID != nil {
				data["group_id"] = events.S(*key.GroupID)
			}
		}
	}
	status := "success"
	if err != nil || !hasResult {
		status = "failed"
	}
	data["status"] = status
	c.Set("gameplay_forward_failed", status != "success")
	requestID, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
	ref := requestID + ":" + events.S(c.GetInt("gameplay_attempt")) + ":" + events.S(account.ID)
	events.Publish("request.upstream_completed", uid, ref, data)
}

func observeGameplayForwardResult(c *gin.Context, account *Account, start time.Time, result *ForwardResult, err error) {
	defer func() { _ = recover() }()
	var first *int
	success := result != nil
	if result != nil {
		first = result.FirstTokenMs
		if result.ClientDisconnect {
			success = false
			if c != nil {
				c.Set("gameplay_client_disconnected", true)
			}
		}
	}
	observeGameplayForward(c, account, start, first, success, err)
}
func observeGameplayOpenAIForwardResult(c *gin.Context, account *Account, start time.Time, result *OpenAIForwardResult, err error) {
	defer func() { _ = recover() }()
	var first *int
	success := result != nil && result.SucceededForScheduling()
	if result != nil {
		first = result.FirstTokenMs
		if result.ClientDisconnect {
			success = false
			if c != nil {
				c.Set("gameplay_client_disconnected", true)
			}
		}
	}
	observeGameplayForward(c, account, start, first, success, err)
}

func (s *GatewayService) Forward(ctx context.Context, c *gin.Context, account *Account, parsed *ParsedRequest) (*ForwardResult, error) {
	start := time.Now()
	result, err := s.forwardWithGameplayObservation(ctx, c, account, parsed)
	observeGameplayForwardResult(c, account, start, result, err)
	return result, err
}
func (s *OpenAIGatewayService) Forward(ctx context.Context, c *gin.Context, account *Account, body []byte) (*OpenAIForwardResult, error) {
	start := time.Now()
	result, err := s.forwardWithGameplayObservation(ctx, c, account, body)
	observeGameplayOpenAIForwardResult(c, account, start, result, err)
	return result, err
}
func (s *GeminiMessagesCompatService) Forward(ctx context.Context, c *gin.Context, account *Account, body []byte) (*ForwardResult, error) {
	start := time.Now()
	result, err := s.forwardWithGameplayObservation(ctx, c, account, body)
	observeGameplayForwardResult(c, account, start, result, err)
	return result, err
}
func (s *GeminiMessagesCompatService) ForwardAsChatCompletions(ctx context.Context, c *gin.Context, account *Account, body []byte) (*ForwardResult, error) {
	start := time.Now()
	result, err := s.forwardChatWithGameplayObservation(ctx, c, account, body)
	observeGameplayForwardResult(c, account, start, result, err)
	return result, err
}
func (s *AntigravityGatewayService) Forward(ctx context.Context, c *gin.Context, account *Account, body []byte, isStickySession bool) (*ForwardResult, error) {
	start := time.Now()
	result, err := s.forwardWithGameplayObservation(ctx, c, account, body, isStickySession)
	observeGameplayForwardResult(c, account, start, result, err)
	return result, err
}
func (s *AntigravityGatewayService) ForwardGemini(ctx context.Context, c *gin.Context, account *Account, originalModel, action string, stream bool, body []byte, isStickySession bool, options ...ForwardGeminiOption) (*ForwardResult, error) {
	start := time.Now()
	result, err := s.forwardGeminiWithGameplayObservation(ctx, c, account, originalModel, action, stream, body, isStickySession, options...)
	observeGameplayForwardResult(c, account, start, result, err)
	return result, err
}
