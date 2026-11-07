package routes

// mediahub 路由注册:sub2api 对接 mediad(组装中转媒体服务)的入口。
// 本文件是媒体定制在路由层的唯一落点;mediad 未配置时静默跳过,
// 保持零配置部署与官方版本的合并面最小(仅需 router.go 一行调用)。

import (
	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
)

// RegisterMediaHubRoutes 注册:
//   - POST/GET /media/*path    客户端媒体请求(apiKey 鉴权 → 冻结 → 签名转发 mediad)
//   - POST /internal/media/settle  mediad 终态回调(HMAC 验签 → 结算)
func RegisterMediaHubRoutes(
	r *gin.Engine,
	h *handler.Handlers,
	apiKeyAuth middleware.APIKeyAuthMiddleware,
) {
	if h == nil || h.MediaHub == nil {
		return
	}

	media := r.Group("/media")
	media.Use(gin.HandlerFunc(apiKeyAuth))
	media.Any("/*path", h.MediaHub.Relay)

	// 回调端点不做会话鉴权:安全性由 HMAC 签名保证(见 mediahub.SettleServer)。
	r.POST("/internal/media/settle", h.MediaHub.Callback)
	r.POST("/internal/media/result", h.MediaHub.Callback)
}
