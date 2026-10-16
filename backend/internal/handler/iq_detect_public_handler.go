package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// IQDetectPublicHandler serves the public IQ detection widget hosted on
// widget.llmfree.work. It is unauthenticated, GET-only and returns sanitized
// results of groups with scheduled detection enabled (no account identifiers).
type IQDetectPublicHandler struct {
	svc *service.IQDetectService
}

func NewIQDetectPublicHandler(svc *service.IQDetectService) *IQDetectPublicHandler {
	return &IQDetectPublicHandler{svc: svc}
}

// CORS allows the standalone widget page to call this endpoint cross-origin.
func (h *IQDetectPublicHandler) CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "*")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// ListOverview returns the detection settings, the published groups and each
// group's last 48 result metadata rows (no probe output content).
func (h *IQDetectPublicHandler) ListOverview(c *gin.Context) {
	overview, err := h.svc.GetPublicOverview(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, overview)
}

// GetResultDetail returns the full sanitized content of one result pair
// (candy final answer + pelican HTML + latencies).
func (h *IQDetectPublicHandler) GetResultDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "invalid id")
		return
	}
	detail, err := h.svc.GetPublicResultDetail(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, detail)
}
