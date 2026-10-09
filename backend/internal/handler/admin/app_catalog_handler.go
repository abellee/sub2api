package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/appcatalog"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type AppCatalogHandler struct {
	svc *service.AppCatalogService
}

func NewAppCatalogHandler(svc *service.AppCatalogService) *AppCatalogHandler {
	return &AppCatalogHandler{svc: svc}
}

func (h *AppCatalogHandler) GetAgentHealth(c *gin.Context) {
	health := h.svc.GetAgentHealth(c.Request.Context())
	response.Success(c, health)
}

func (h *AppCatalogHandler) List(c *gin.Context) {
	if !h.requireEnabled(c) {
		return
	}
	items, err := h.svc.ListApps(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

func (h *AppCatalogHandler) ListPublic(c *gin.Context) {
	items, err := h.svc.ListApps(c.Request.Context())
	if err != nil {
		response.Success(c, gin.H{"items": []appcatalog.App{}, "total": 0})
		return
	}
	if items == nil {
		items = []appcatalog.App{}
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

func (h *AppCatalogHandler) Create(c *gin.Context) {
	if !h.requireEnabled(c) {
		return
	}
	req, ok := h.bindAppInput(c)
	if !ok {
		return
	}
	app, err := h.svc.CreateApp(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, app)
}

func (h *AppCatalogHandler) Update(c *gin.Context) {
	if !h.requireEnabled(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "invalid id")
		return
	}
	req, ok := h.bindAppInput(c)
	if !ok {
		return
	}
	app, err := h.svc.UpdateApp(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, app)
}

func (h *AppCatalogHandler) Fetch(c *gin.Context) {
	if !h.requireEnabled(c) {
		return
	}
	req, ok := h.bindAppInput(c)
	if !ok {
		return
	}
	result, err := h.svc.FetchPreview(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AppCatalogHandler) bindAppInput(c *gin.Context) (appcatalog.CreateAppInput, bool) {
	var req appcatalog.CreateAppInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "invalid json")
		return req, false
	}
	return req, true
}

func (h *AppCatalogHandler) Delete(c *gin.Context) {
	if !h.requireEnabled(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "invalid id")
		return
	}
	if err := h.svc.DeleteApp(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *AppCatalogHandler) requireEnabled(c *gin.Context) bool {
	if err := h.svc.EnsureEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return false
	}
	return true
}
