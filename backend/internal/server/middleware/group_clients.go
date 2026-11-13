package middleware

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// RejectGroupClients runs before protocol conversion or account selection.
func RejectGroupClients() gin.HandlerFunc {
	return func(c *gin.Context) {
		key, ok := GetAPIKeyFromContext(c)
		if !ok || key == nil || key.Group == nil || !groupBlocksUserAgent(key.Group.BlockedUserAgents, c.Request.Header.Values("User-Agent")) {
			c.Next()
			return
		}
		const message = "该分组不支持当前客户端。使用当前客户端可能会导致缓存丢失，请更换其它客户端。"
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalPolicyDenied)
		MarkIngressRejected(c, IngressRejectClientBlocked)
		path := strings.TrimPrefix(c.FullPath(), "/antigravity")
		switch {
		case strings.HasPrefix(path, "/v1beta/"):
			GoogleErrorWriter(c, http.StatusForbidden, message)
		case groupRequestProtocol(c) == service.GroupProtocolAnthropic:
			AnthropicErrorWriter(c, http.StatusForbidden, message)
		default:
			c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"type": "permission_error", "code": "client_blocked", "message": message}})
		}
		c.Abort()
	}
}

func groupBlocksUserAgent(blocked string, userAgents []string) bool {
	if blocked == "" || len(userAgents) == 0 {
		return false
	}
	for _, ua := range userAgents {
		ua = strings.ToLower(ua)
		for _, keyword := range strings.Split(blocked, ",") {
			keyword = strings.ToLower(strings.TrimSpace(keyword))
			if keyword != "" && strings.HasPrefix(ua, keyword) {
				return true
			}
		}
	}
	return false
}
