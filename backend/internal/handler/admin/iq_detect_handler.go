package admin

import (
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
