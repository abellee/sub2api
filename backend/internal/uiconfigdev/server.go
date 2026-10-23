// Package uiconfigdev serves local UI configuration while Vite proxies the
// remaining API to a remote backend. Authentication is checked by that backend.
package uiconfigdev

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func NewHandler(store *service.GroupCategoryStore, authBackend string) http.Handler {
	client := &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	authenticate := func(admin bool) gin.HandlerFunc {
		return func(c *gin.Context) {
			authorization := c.GetHeader("Authorization")
			if !strings.HasPrefix(authorization, "Bearer ") || len(authorization) <= len("Bearer ") {
				response.Error(c, http.StatusUnauthorized, "Authentication required")
				c.Abort()
				return
			}
			req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet,
				strings.TrimRight(authBackend, "/")+"/api/v1/auth/me", nil)
			if err != nil {
				response.Error(c, http.StatusServiceUnavailable, "Authentication backend unavailable")
				c.Abort()
				return
			}
			req.Header.Set("Authorization", authorization)
			upstream, err := client.Do(req)
			if err != nil {
				response.Error(c, http.StatusServiceUnavailable, "Authentication backend unavailable")
				c.Abort()
				return
			}
			defer func() { _ = upstream.Body.Close() }()
			var result struct {
				Code int `json:"code"`
				Data struct {
					Role string `json:"role"`
				} `json:"data"`
			}
			if upstream.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(upstream.Body, 1<<20)).Decode(&result) != nil || result.Code != 0 || result.Data.Role == "" {
				status := http.StatusServiceUnavailable
				if upstream.StatusCode == http.StatusUnauthorized || upstream.StatusCode == http.StatusForbidden {
					status = upstream.StatusCode
				}
				response.Error(c, status, "Authentication verification failed")
				c.Abort()
				return
			}
			if admin && result.Data.Role != service.RoleAdmin {
				response.Error(c, http.StatusForbidden, "Administrator access required")
				c.Abort()
				return
			}
			c.Next()
		}
	}
	router := gin.New()
	router.Use(gin.Recovery())
	menu := handler.NewUserMenuHandler(store)
	router.GET("/api/v1/admin/user-menu", authenticate(true), menu.Get)
	router.PUT("/api/v1/admin/user-menu", authenticate(true), menu.Update)
	router.GET("/api/v1/user/menu", authenticate(false), menu.Get)
	return router
}
