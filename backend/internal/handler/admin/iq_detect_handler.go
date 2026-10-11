package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/iqdetectd"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// IQDetectHandler proxies the iqdetectd sidecar for the admin console. The
// IQ detection page only needs the sidecar health for now; more proxied
// endpoints will be added with the page itself.
type IQDetectHandler struct {
	svc *service.IQDetectService
}

func NewIQDetectHandler(svc *service.IQDetectService) *IQDetectHandler {
	return &IQDetectHandler{svc: svc}
}

func (h *IQDetectHandler) GetAgentHealth(c *gin.Context) {
	health := h.svc.GetAgentHealth(c.Request.Context())
	response.Success(c, health)
}

func (h *IQDetectHandler) ListAccountCredentials(c *gin.Context) {
	if !h.requireEnabled(c) {
		return
	}
	items, err := h.svc.ListAccountCredentials(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

func (h *IQDetectHandler) UpsertAccountCredential(c *gin.Context) {
	if !h.requireEnabled(c) {
		return
	}
	var req iqdetectd.UpsertAccountCredentialInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "invalid json")
		return
	}
	credential, err := h.svc.UpsertAccountCredential(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, credential)
}

func (h *IQDetectHandler) GetSelection(c *gin.Context) {
	if !h.requireEnabled(c) {
		return
	}
	selection, err := h.svc.GetDetectionSelection(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, selection)
}

func (h *IQDetectHandler) SaveSelection(c *gin.Context) {
	if !h.requireEnabled(c) {
		return
	}
	var req iqdetectd.DetectionSelection
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "invalid json")
		return
	}
	selection, err := h.svc.SaveDetectionSelection(c.Request.Context(), req.Groups)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, selection)
}

func (h *IQDetectHandler) requireEnabled(c *gin.Context) bool {
	if err := h.svc.EnsureEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return false
	}
	return true
}
