package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Only the final attempt selects the message rule; rules never change its HTTP status.
func failoverClientResponse(c *gin.Context, rules *service.ErrorPassthroughService, failure *service.UpstreamFailoverError, platform string) (int, string, string) {
	status := http.StatusBadGateway
	message := "Upstream request failed"
	errType := "upstream_error"
	if failure == nil {
		return status, errType, message
	}
	if failure.StatusCode >= 400 && failure.StatusCode <= 599 {
		status = failure.StatusCode
	} else if failure.IsCredentialFailure() {
		status, _ = credentialFailoverClientResponse(failure)
	}
	copyFailoverRetryAfter(c, failure.ResponseHeaders)
	service.SetOpsUpstreamError(c, failure.StatusCode, service.SanitizeUpstreamErrorMessage(service.ExtractUpstreamErrorMessage(failure.ResponseBody)), "")
	if c != nil && c.Request != nil {
		platform = resolveOpsPlatform(c.Request.Context(), getOpsAPIKey(c), platform)
	}
	if rules != nil {
		if rule := rules.MatchRule(platform, status, failure.ResponseBody); rule != nil {
			// Never fall back to upstream text, even on an old body-passthrough rule.
			if rule.CustomMessage != nil {
				message = *rule.CustomMessage
			}
			if rule.SkipMonitoring && c != nil {
				c.Set(service.OpsSkipPassthroughKey, true)
			}
			return status, errType, message
		}
	}
	switch status {
	case http.StatusUnauthorized:
		message = "Upstream authentication failed, please contact administrator"
	case http.StatusForbidden:
		message = "Upstream access forbidden, please contact administrator"
	case http.StatusTooManyRequests:
		errType, message = "rate_limit_error", "Upstream rate limit exceeded, please retry later"
	case 529:
		message = "Upstream service overloaded, please retry later"
	case 500, 502, 503, 504:
		message = "Upstream service temporarily unavailable"
	}
	if failure.IsCredentialFailure() {
		_, message = credentialFailoverClientResponse(failure)
	} else if failure.IsOpenAIRequestBodyTooLarge() {
		errType, message = "invalid_request_error", service.OpenAIRequestBodyTooLargeClientMessage
	} else if failure.Reason == service.OpenAIImagesInsufficientBalanceReason {
		message = service.OpenAIImagesInsufficientBalanceMessage
	} else if service.IsOpenAISilentRefusalErrorBody(failure.ResponseBody) {
		message = service.OpenAISilentRefusalClientMessage()
	}
	return status, errType, message
}
