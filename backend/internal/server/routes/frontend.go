package routes

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type frontendSettings interface {
	GetPublicTransitRuntime(context.Context) service.PublicTransitRuntime
}

// RegisterFrontendRoutes keeps the existing page gate and security middleware
// authoritative when an independent frontend serves HTML. No credentials or
// private settings are returned by this public, read-only check.
func RegisterFrontendRoutes(r *gin.Engine, settings frontendSettings) {
	r.GET("/internal/frontend/check", func(c *gin.Context) {
		path, err := url.ParseRequestURI(c.GetHeader("X-Frontend-Path"))
		if err != nil || !strings.HasPrefix(path.Path, "/") {
			c.Status(http.StatusForbidden)
			return
		}
		if strings.TrimRight(path.Path, "/") == service.PublicTransitPagePath &&
			(settings == nil || !settings.GetPublicTransitRuntime(c.Request.Context()).PageEnabled) {
			c.Status(http.StatusForbidden)
			return
		}
		c.Status(http.StatusNoContent)
	})
}
