package admin

import (
	"strconv"

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

func (h *IQDetectHandler) GetDetectionModels(c *gin.Context) {
	if !h.requireEnabled(c) {
		return
	}
	settings, err := h.svc.GetDetectionModels(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *IQDetectHandler) ReplaceDetectionModels(c *gin.Context) {
	if !h.requireEnabled(c) {
		return
	}
	var req iqdetectd.PutDetectionModelsInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "invalid json")
		return
	}
	settings, err := h.svc.ReplaceDetectionModels(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *IQDetectHandler) TriggerTestRun(c *gin.Context) {
	if !h.requireEnabled(c) {
		return
	}
	var req struct {
		GroupID int64 `json:"group_id"`
	}
	_ = c.ShouldBindJSON(&req) // 空 body = 触发全部分组
	plan, err := h.svc.TriggerTestRun(c.Request.Context(), req.GroupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plan)
}

// StopTestRun cancels all in-flight detection groups on the sidecar.
func (h *IQDetectHandler) StopTestRun(c *gin.Context) {
	if !h.requireEnabled(c) {
		return
	}
	stopped, err := h.svc.StopTestRun(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"stopped": stopped})
}

func (h *IQDetectHandler) ListTestResults(c *gin.Context) {
	if !h.requireEnabled(c) {
		return
	}
	limit, offset := 20, 0
	if v, err := strconv.Atoi(c.Query("limit")); err == nil {
		limit = v
	}
	if v, err := strconv.Atoi(c.Query("offset")); err == nil {
		offset = v
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	// 分组 tab 过滤：0/缺省 = 全部
	var groupID int64
	if v, err := strconv.ParseInt(c.Query("group_id"), 10, 64); err == nil && v > 0 {
		groupID = v
	}
	items, total, err := h.svc.ListTestResults(c.Request.Context(), int64(limit), int64(offset), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": total})
}

func (h *IQDetectHandler) GetTestResultPelican(c *gin.Context) {
	if !h.requireEnabled(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "invalid id")
		return
	}
	html, err := h.svc.GetTestResultPelican(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"html": html})
}

func (h *IQDetectHandler) UpdateTestResultReview(c *gin.Context) {
	if !h.requireEnabled(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "invalid id")
		return
	}
	var req iqdetectd.TestResultReviewInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "invalid json")
		return
	}
	if err := h.svc.UpdateTestResultReview(c.Request.Context(), id, req.Probe, req.Verdict); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *IQDetectHandler) requireEnabled(c *gin.Context) bool {
	if err := h.svc.EnsureEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return false
	}
	return true
}
