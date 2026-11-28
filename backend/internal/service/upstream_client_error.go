package service

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

const UpstreamClientErrorMessage = "当前服务拥挤，请重试。"

// The edge strips this internal header after preserving a backend-selected error.
func MarkUpstreamClientErrorHandled(c *gin.Context) {
	if c != nil && c.Writer != nil && !c.Writer.Written() {
		c.Header("X-Sub2api-Error-Handled", "1")
	}
}

// ResolveUpstreamClientError runs only after an upstream error has been identified.
// Normal response chunks never enter the rule matcher.
func ResolveUpstreamClientError(c *gin.Context, platform string, status int, body []byte) (int, string, string) {
	MarkUpstreamClientErrorHandled(c)
	if status < 400 || status > 599 {
		status = http.StatusBadGateway
	}
	code, kind, message, rule, matched := applyErrorPassthroughRuleDetailed(
		c, platform, status, body, status, "upstream_error", UpstreamClientErrorMessage, ExtractUpstreamErrorMessage,
	)
	if !matched {
		return status, "upstream_error", UpstreamClientErrorMessage
	}
	if rule.PassthroughBody {
		message = sanitizeUpstreamErrorMessage(message)
	}
	if strings.TrimSpace(message) == "" {
		message = UpstreamClientErrorMessage
	}
	return code, kind, message
}

func publicOpenAIStreamError(c *gin.Context, platform string, payload []byte, eventType string) []byte {
	if eventType != "error" && eventType != "response.failed" {
		return payload
	}
	message := extractOpenAISSEErrorMessage(payload)
	status := openAIStreamFailedEventSemanticStatus(payload, message)
	_, kind, message := ResolveUpstreamClientError(c, platform, status, openAIStreamFailedEventPassthroughBody(payload, message))
	code := publicStreamErrorCode(payload)
	if code == "context_length_exceeded" || code == "invalid_encrypted_content" {
		kind = "invalid_request_error"
	}
	errorBody := gin.H{"type": kind, "code": code, "message": message}
	var out gin.H
	if eventType == "response.failed" {
		errorBody["code"] = fmt.Sprint(errorBody["code"])
		response := gin.H{"id": "resp_" + strings.ReplaceAll(uuid.NewString(), "-", ""), "created_at": time.Now().Unix(), "object": "response", "status": "failed", "output": []any{}, "error": errorBody}
		for _, key := range []string{"id", "created_at", "model"} {
			if value := gjson.GetBytes(payload, "response."+key); value.Exists() {
				response[key] = value.Value()
			}
		}
		out = gin.H{"type": eventType, "response": response, "sequence_number": openAIResponsesFailedSequenceNumber(payload)}
	} else {
		out = gin.H{"type": "error", "code": errorBody["code"], "message": message, "param": nil}
		if sequence := gjson.GetBytes(payload, "sequence_number"); sequence.Exists() {
			out["sequence_number"] = sequence.Int()
		}
	}
	encoded, err := marshalOpenAIUpstreamJSON(out)
	if err != nil {
		return []byte(`{"type":"error","code":"upstream_error","message":"当前服务拥挤，请重试。"}`)
	}
	return encoded
}

func publicGeminiStreamError(c *gin.Context, platform string, body []byte) ([]byte, int, string, bool) {
	sig, ok := detectGeminiResponseSignal(body)
	if !ok || sig.Kind != geminiSignalError {
		return body, 0, "", false
	}
	status, _, message := ResolveUpstreamClientError(c, platform, sig.Status, body)
	encoded, _ := marshalOpenAIUpstreamJSON(gin.H{"error": gin.H{"code": status, "message": message, "status": "UNKNOWN"}})
	return encoded, status, message, true
}

func publicStreamErrorCode(payload []byte) any {
	for _, path := range []string{"response.error.code", "error.code", "code"} {
		value := gjson.GetBytes(payload, path)
		if value.Type == gjson.Number {
			return value.Int()
		}
		switch value.String() {
		case "context_length_exceeded", "invalid_encrypted_content", "rate_limit_exceeded", "server_error", "upstream_error", "invalid_request_error", "cyber_policy":
			return value.String()
		}
	}
	return "upstream_error"
}

func publicOpenAIResponseFailedSSE(c *gin.Context, platform, responseID, model string, createdAt int64, source []byte, message string) string {
	frame := buildOpenAIResponseFailedSSE(responseID, model, createdAt, source, message)
	_, data, _ := strings.Cut(frame, "data: ")
	payload := publicOpenAIStreamError(c, platform, []byte(strings.TrimSpace(data)), "response.failed")
	return "event: response.failed\ndata: " + string(payload) + "\n\n"
}
