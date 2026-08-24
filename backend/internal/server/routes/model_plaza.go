package routes

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterModelPlazaRoutes 注册模型广场路由。
//
// 挂 OptionalJWT：匿名可访问（开关与 require_auth 由 handler fail-closed 判定），
// 带 token 则识别用户以展示专属分组与个人倍率。
// BackendModeUserGuard 保证 backend 模式下广场不对非管理员开放（匿名无 role → 403）。
func RegisterModelPlazaRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	optionalJWT middleware.OptionalJWTAuthMiddleware,
	settingService *service.SettingService,
	panelRateLimiter *middleware.PanelRateLimiter,
) {
	plaza := v1.Group("/model-plaza")
	plaza.Use(panelRateLimiter.PublicIP())
	plaza.Use(gin.HandlerFunc(optionalJWT))
	plaza.Use(middleware.BackendModeUserGuard(settingService))
	{
		plaza.GET("", h.ModelPlaza.Get)
		plaza.Any("/downgrade-radar/:resource", proxyDowngradeRadarResource)
	}
}

// proxyDowngradeRadarResource exposes only the read-only codexradar datasets
// needed by the public radar page. codexradar does not send CORS headers, so
// keeping this proxy same-origin also makes the feature work in production.
func proxyDowngradeRadarResource(c *gin.Context) {
	resource := strings.TrimSpace(c.Param("resource"))
	allowed := map[string]bool{
		"radar-insights":                   true,
		"intelligence-efficiency-metrics":  true,
		"visual-spatial-reasoning":         true,
		"visual-spatial-reasoning-history": true,
		"model-ratings":                    true,
	}
	if !allowed[resource] {
		c.JSON(http.StatusNotFound, gin.H{"error": "radar resource not found"})
		return
	}

	target := "https://codexradar.com/api/" + resource
	if query := c.Request.URL.RawQuery; query != "" {
		target += "?" + query
	}
	method := c.Request.Method
	if method != http.MethodGet && method != http.MethodPost {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
		return
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), method, target, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("create radar request: %v", err)})
		return
	}
	req.Header.Set("Accept", "application/json")
	if contentType := c.GetHeader("Content-Type"); contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "codexradar is unavailable"})
		return
	}
	defer resp.Body.Close()

	c.Status(resp.StatusCode)
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		c.Header("Content-Type", contentType)
	}
	if cacheControl := resp.Header.Get("Cache-Control"); cacheControl != "" {
		c.Header("Cache-Control", cacheControl)
	}
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		return
	}
}
