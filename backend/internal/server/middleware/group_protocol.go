package middleware

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// RequireGroupProtocol checks the client-facing route before any conversion or
// account selection. Headers and the selected upstream do not identify the client protocol.
func RequireGroupProtocol() gin.HandlerFunc {
	return func(c *gin.Context) {
		key, ok := GetAPIKeyFromContext(c)
		if !ok || key == nil || key.Group == nil || key.Group.RequiredProtocol == "" {
			c.Next()
			return
		}
		protocol := groupRequestProtocol(c)
		if protocol == "" || protocol == key.Group.RequiredProtocol {
			c.Next()
			return
		}
		message := requiredProtocolMessage(key.Group.RequiredProtocol)
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalPolicyDenied)
		MarkIngressRejected(c, IngressRejectProtocolMismatch)
		switch protocol {
		case service.GroupProtocolGoogle:
			GoogleErrorWriter(c, http.StatusBadRequest, message)
		case service.GroupProtocolAnthropic:
			c.JSON(http.StatusBadRequest, gin.H{"type": "error", "error": gin.H{"type": "invalid_request_error", "message": message}})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "code": "protocol_mismatch", "message": message}})
		}
		c.Abort()
	}
}

func groupRequestProtocol(c *gin.Context) string {
	// Use the registered route so model names and wildcard suffixes cannot spoof a protocol.
	path := strings.TrimPrefix(c.FullPath(), "/antigravity")
	path = strings.TrimPrefix(path, "/backend-api/codex")
	// Model discovery, usage and billing are shared helper endpoints, not generation protocols.
	if c.Request.Method == http.MethodGet {
		switch path {
		case "/models", "/models/:model", "/v1/models", "/v1/models/:model",
			"/v1beta/models", "/v1beta/models/:model", "/v1/usage", "/v1/sub2api/billing":
			return ""
		}
	}
	if strings.HasPrefix(path, "/v1beta/") {
		return service.GroupProtocolGoogle
	}
	path = strings.TrimPrefix(path, "/v1/")
	path = strings.TrimPrefix(path, "/")
	switch path {
	case "messages", "messages/count_tokens":
		return service.GroupProtocolAnthropic
	case "systemone":
		return "systemone"
	default:
		return service.GroupProtocolOpenAI
	}
}

func requiredProtocolMessage(protocol string) string {
	switch protocol {
	case service.GroupProtocolAnthropic:
		return "该分组仅允许 Anthropic 协议。请在客户端选择 Anthropic 协议，使用 /v1/messages 接口，避免协议转换导致缓存丢失。"
	case service.GroupProtocolOpenAI:
		return "该分组仅允许 OpenAI 协议。请在客户端选择 OpenAI 协议，使用 /v1/chat/completions 或 /v1/responses 接口，避免协议转换导致缓存丢失。"
	case service.GroupProtocolGoogle:
		return "该分组仅允许 Google 协议。请在客户端选择 Google（Gemini）原生协议，使用 /v1beta/models/{model}:generateContent 或 :streamGenerateContent 接口，避免协议转换导致缓存丢失。"
	default:
		return "该分组的强制协议配置无效，请联系管理员。"
	}
}
