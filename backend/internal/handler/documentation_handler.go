package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type DocumentationHandler struct{ store *service.GroupCategoryStore }

func NewDocumentationHandler(store *service.GroupCategoryStore) *DocumentationHandler {
	return &DocumentationHandler{store: store}
}

func (h *DocumentationHandler) Get(c *gin.Context) {
	snap, err := h.store.Documentation(c.Request.Context())
	if err != nil {
		response.InternalError(c, "文档加载失败")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, snap)
}

func (h *DocumentationHandler) ListPublished(c *gin.Context) {
	snap, err := h.store.Documentation(c.Request.Context())
	if err != nil {
		response.InternalError(c, "文档加载失败")
		return
	}
	category := strings.TrimSpace(c.Query("category"))
	articles := make([]service.DocumentationArticle, 0)
	for _, article := range snap.Articles {
		if article.Published && (category == "" || article.CategoryID == category) {
			articles = append(articles, article)
		}
	}
	snap.Articles = articles
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, snap)
}

func (h *DocumentationHandler) Categories(c *gin.Context) {
	snap, err := h.store.Documentation(c.Request.Context())
	if err != nil {
		response.InternalError(c, "文档分类加载失败")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, snap.Categories)
}

func (h *DocumentationHandler) DownloadSkill(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	snap, err := h.store.Documentation(c.Request.Context())
	if err != nil {
		response.InternalError(c, "文档加载失败")
		return
	}
	for _, article := range snap.Articles {
		if article.ID != c.Param("id") || !article.Published || !article.Agent.Enabled {
			continue
		}
		data, err := service.BuildDocumentationSkill(article)
		if err != nil {
			response.InternalError(c, "Skill 生成失败")
			return
		}
		c.Header("Content-Disposition", `attachment; filename="tokenfree-creative.zip"`)
		c.Data(http.StatusOK, "application/zip", data)
		return
	}
	response.NotFound(c, "文档不存在或未启用 Skill")
}

func (h *DocumentationHandler) Update(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8*1024*1024)
	var snap service.DocumentationSnapshot
	if err := c.ShouldBindJSON(&snap); err != nil {
		response.BadRequest(c, "文档格式无效或内容超过 8 MB")
		return
	}
	if err := service.ValidateDocumentation(&snap); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.store.SaveDocumentation(c.Request.Context(), &snap); err != nil {
		if errors.Is(err, service.ErrDocumentationConflict) {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
			return
		}
		response.InternalError(c, "文档保存失败")
		return
	}
	response.Success(c, &snap)
}
