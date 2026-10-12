package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterIQDetectPublicRoutes 注册智商检测公开接口，供 widget.llmfree.work
// 上的独立结果页（无登录态）读取。只读 + 公网 IP 限流 + CORS。
func RegisterIQDetectPublicRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	panelRateLimiter *middleware.PanelRateLimiter,
) {
	pub := v1.Group("/public/iq-detection")
	pub.Use(panelRateLimiter.PublicIP())
	pub.Use(gin.HandlerFunc(h.IQDetectPublic.CORS()))
	{
		// 基础信息：检测设置 + 分组 + 每组最近 48 条结果元信息（不含内容）
		pub.GET("/overview", h.IQDetectPublic.ListOverview)
		// 按 id 获取该时间点的完整检测结果（糖果答案 + 鹈鹕 HTML + 耗时）
		pub.GET("/results/:id", h.IQDetectPublic.GetResultDetail)
	}
}
