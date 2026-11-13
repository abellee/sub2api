package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

// Only provider responses reach this classifier. Local balance/auth/validation
// failures are rejected before forwarding and must never enter account failover.
func accountErrorShouldFailover(status int, body []byte) bool {
	if status < 400 {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	if message == "" && !gjson.ValidBytes(body) {
		message = strings.ToLower(strings.TrimSpace(string(body)))
	}
	code := strings.ToLower(extractUpstreamErrorCode(body))
	if message == "" {
		message = strings.ToLower(gjson.GetBytes(body, "response.error.message").String())
	}
	if code == "" {
		code = strings.ToLower(gjson.GetBytes(body, "response.error.code").String())
	}
	if hit, _, _ := detectOpenAICyberPolicy(body); hit {
		return false
	}
	if isOpenAIContextWindowError(message, body) || isGrokContentPolicyRejection(status, body) {
		return false
	}
	combined := code + " " + message
	if containsAccountErrorMarker(combined, []string{
		"content_policy", "content_filter", "safety_violation", "safety policy", "safety filter",
		"invalid_parameter", "missing_required_parameter", "context_length_exceeded",
		"invalid_image", "invalid_audio", "invalid_json", "invalid_prompt",
		"parameter is invalid", "invalid parameter", "missing required", "unsupported parameter",
		"unrecognized request argument", "must be a multiple of", "maximum context length",
		"参数错误", "参数无效", "缺少必填", "内容违规", "内容安全",
	}) {
		return false
	}
	// A provider may describe its own account's credit/auth/model failure as
	// invalid_request_error or HTTP 400. Those still need another account.
	if containsAccountErrorMarker(combined, []string{
		"insufficient_quota", "insufficient_balance", "insufficient_funds", "insufficient credit", "insufficient balance", "insufficient account balance",
		"credit balance", "credit exhausted", "quota exceeded", "quota exhausted", "quota_exceeded", "resource_exhausted", "billing",
		"payment required", "余额不足", "额度不足", "欠费", "余额已用完", "额度已用完",
		"invalid_api_key", "invalid api key", "incorrect api key", "authentication", "unauthorized",
		"api key expired", "token expired", "account suspended", "account disabled", "permission_denied",
		"model_not_found", "unknown model", "unknown provider", "model not found",
		"model is not supported", "is not supported by", "does not exist or you do not have access", "no available channel", "无可用渠道",
		"rate_limit", "rate limit", "too many requests", "overloaded", "capacity", "temporarily unavailable",
	}) || isGoogleProjectConfigError(message) || isGrokDecoderCompatibilityError(status, body) ||
		isOpenAIUpstreamAccessStateError(message, body) {
		return true
	}
	errType := strings.ToLower(gjson.GetBytes(body, "error.type").String())
	if errType == "" {
		errType = strings.ToLower(gjson.GetBytes(body, "response.error.type").String())
	}
	if code == "invalid_argument" || strings.EqualFold(gjson.GetBytes(body, "error.status").String(), "INVALID_ARGUMENT") ||
		gjson.GetBytes(body, "error.param").String() != "" ||
		((status == 400 || status == 422) && (errType == "invalid_request_error" || errType == "validation_error")) {
		return false
	}
	// Payload/media-type validation cannot be repaired by changing credentials.
	return status != http.StatusRequestEntityTooLarge && status != http.StatusUnsupportedMediaType
}

func containsAccountErrorMarker(value string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func shouldFailoverAccountError(ctx context.Context, status int, body []byte, legacy bool) bool {
	if !RequestAccountFailoverEnabled(ctx) {
		return legacy
	}
	return accountErrorShouldFailover(status, body)
}

// Status-only legacy paths need a bounded peek; restore all consumed bytes and
// retain Close so later response mapping and connection cleanup remain intact.
func shouldFailoverAccountResponse(ctx context.Context, resp *http.Response, legacy bool) bool {
	if !RequestAccountFailoverEnabled(ctx) || resp == nil {
		return legacy
	}
	if resp.Body == nil {
		return accountErrorShouldFailover(resp.StatusCode, nil)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body = &accountErrorPeekBody{Reader: io.MultiReader(bytes.NewReader(body), resp.Body), Closer: resp.Body}
	return accountErrorShouldFailover(resp.StatusCode, body)
}

type accountErrorPeekBody struct {
	io.Reader
	io.Closer
}
