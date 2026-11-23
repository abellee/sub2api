package handler

import (
	"database/sql"
	"errors"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type InflationMonitorHandler struct {
	svc *service.InflationMonitorService
}

func NewInflationMonitorHandler(svc *service.InflationMonitorService) *InflationMonitorHandler {
	svc.Start()
	return &InflationMonitorHandler{svc: svc}
}
func (h *InflationMonitorHandler) Plans(c *gin.Context) {
	plans, err := h.svc.Plans(c.Request.Context())
	if err != nil {
		response.InternalError(c, "加载膨胀检测配置失败")
		return
	}
	response.Success(c, plans)
}
func (h *InflationMonitorHandler) SavePlans(c *gin.Context) {
	var plans []service.InflationMonitorPlan
	if err := c.ShouldBindJSON(&plans); err != nil {
		response.BadRequest(c, "配置格式无效")
		return
	}
	if err := h.svc.SavePlans(c.Request.Context(), plans); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"saved": true})
}
func (h *InflationMonitorHandler) Summaries(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	items, err := h.svc.Summaries(c.Request.Context(), subject.UserID)
	if err != nil {
		response.InternalError(c, "加载膨胀检测失败")
		return
	}
	response.Success(c, items)
}
func (h *InflationMonitorHandler) History(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid key ID")
		return
	}
	item, err := h.svc.History(c.Request.Context(), subject.UserID, id)
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(c, "无此密钥或未配置检测")
		return
	}
	if err != nil {
		response.InternalError(c, "加载膨胀检测历史失败")
		return
	}
	response.Success(c, item)
}
