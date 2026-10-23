package service

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var errContentSafetyRejected = errors.New("upstream content safety rejection")

func (s *OpenAIGatewayService) SetContentSafetyBilling(safety *ContentSafetyBillingService) {
	s.contentSafetyBilling = safety
}

// This runs before account failover/cooldown. A matched request is terminal, so
// another upstream account cannot turn a single safety refusal into many debits.
func (s *OpenAIGatewayService) contentSafetyRejection(c *gin.Context, account *Account, resp *http.Response, body []byte, scope string) (string, bool) {
	if s.contentSafetyBilling == nil || c == nil || account == nil || !account.IsGrok() || resp.StatusCode != http.StatusForbidden {
		return "", false
	}
	key := "content_safety_billing:" + scope
	if value, exists := c.Get(key); exists {
		if message, ok := value.(string); ok {
			return message, true
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	config, err := s.contentSafetyBilling.Match(ctx, resp.StatusCode, body)
	if err != nil {
		slog.Error("content safety configuration unavailable", "error", err)
		return "", false
	}
	if config == nil {
		return "", false
	}
	c.Set(key, config.ClientMessage)
	setOpsUpstreamError(c, http.StatusForbidden, config.MatchDescription, "")
	v, _ := c.Get("api_key")
	apiKey, _ := v.(*APIKey)
	if apiKey == nil || apiKey.UserID <= 0 {
		slog.Error("content safety debit missing authenticated user", "account_id", account.ID)
	} else {
		d := ContentSafetyDebit{ID: newContentSafetyDebitID(), UserID: apiKey.UserID, APIKeyID: apiKey.ID, AccountID: account.ID,
			UpstreamRequestID: firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("xai-request-id"))}
		if err := s.contentSafetyBilling.Debit(ctx, d, *config); err != nil {
			slog.Error("content safety debit pending", "debit_id", d.ID, "user_id", d.UserID, "error", err)
		}
	}
	return config.ClientMessage, true
}

func (s *OpenAIGatewayService) handleContentSafetyBilling(c *gin.Context, account *Account, resp *http.Response, body []byte) bool {
	message, matched := s.contentSafetyRejection(c, account, resp, body, "http")
	if !matched {
		return false
	}
	MarkResponseCommitted(c)
	if c.Request != nil && strings.HasSuffix(c.Request.URL.Path, "/messages") {
		writeAnthropicError(c, http.StatusForbidden, "invalid_request_error", message)
	} else {
		writeChatCompletionsError(c, http.StatusForbidden, "invalid_request_error", message)
	}
	return true
}
