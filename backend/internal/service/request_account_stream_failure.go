package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

// Only successful-upstream response readers call this helper. Local request
// validation and explicit upstream stop decisions must not become retries.
func requestAccountStreamFailover(c *gin.Context, account *Account, resp *http.Response, err error) *UpstreamFailoverError {
	if err == nil || c == nil || c.Request == nil || !RequestAccountFailoverEnabled(c.Request.Context()) ||
		c.Request.Context().Err() != nil || errors.Is(err, ErrInsufficientBalance) {
		return nil
	}
	var failure *UpstreamFailoverError
	var eventErr *sseStreamErrorEventError
	if errors.As(err, &failure) {
		return nil // Keep existing HTTP error classification and stop rules.
	}
	if errors.As(err, &eventErr) {
		if !openAIStreamFailedEventShouldFailover([]byte(eventErr.RawData), extractUpstreamErrorMessage([]byte(eventErr.RawData)), accountFailoverContext(c)) {
			return nil
		}
	}
	status := http.StatusBadGateway
	if errors.Is(err, errAnthropicNativeStreamIdle) || errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "stream data interval timeout") {
		status = http.StatusGatewayTimeout
	}
	message := "Upstream response stream ended before completion"
	body, _ := json.Marshal(gin.H{"error": gin.H{"type": "upstream_error", "message": message}})
	if eventErr != nil {
		body = []byte(eventErr.RawData)
		if upstreamMessage := extractUpstreamErrorMessage(body); upstreamMessage != "" {
			message = upstreamMessage
		}
		status = openAIStreamFailedEventSemanticStatus(body, message)
	}
	setOpsUpstreamError(c, status, message, "")
	event := OpsUpstreamErrorEvent{Kind: "failover", UpstreamStatusCode: status, Message: message + ": " + sanitizeStreamError(err)}
	if account != nil {
		event.AccountID, event.AccountName, event.Platform = account.ID, account.Name, account.Platform
		event.ProxyID, event.ProxyName = opsUpstreamProxyAttribution(account)
	} else {
		event.AccountID, _ = c.Request.Context().Value(ctxkey.AccountID).(int64)
		event.Platform, _ = c.Request.Context().Value(ctxkey.Platform).(string)
	}
	if resp != nil {
		event.UpstreamRequestID = resp.Header.Get("x-request-id")
	}
	appendOpsUpstreamError(c, event)
	return &UpstreamFailoverError{Cause: err, StatusCode: status, ResponseBody: body, SkipAccountCooldown: true, SafeToFailoverAfterWrite: true, NextAccountAction: NextAccountRetry}
}

// Keep reported usage for per-account billing, including failed attempts.
func finishRequestAccountStream[T any](c *gin.Context, account *Account, resp *http.Response, result **T, err *error) {
	if failure := requestAccountStreamFailover(c, account, resp, *err); failure != nil {
		*err = failure
	}
}
