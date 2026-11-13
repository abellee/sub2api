package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type AccountMatchingHandler struct {
	repo service.AccountMatchingRepository
}

func NewAccountMatchingHandler(repo service.AccountMatchingRepository) *AccountMatchingHandler {
	return &AccountMatchingHandler{repo: repo}
}

func (h *AccountMatchingHandler) List(c *gin.Context) {
	rules, err := h.repo.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rules)
}

func (h *AccountMatchingHandler) SearchUsers(c *gin.Context) {
	email := c.Query("email")
	if len(email) > 320 {
		response.BadRequest(c, "邮箱过长")
		return
	}
	users, err := h.repo.SearchUsers(c.Request.Context(), email)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, users)
}

func (h *AccountMatchingHandler) Save(c *gin.Context) {
	var rule service.AccountMatchingRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		response.BadRequest(c, "请填写用户、分组和账号")
		return
	}
	rule.ID = 0
	if c.Param("id") != "" {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "无效规则编号")
			return
		}
		rule.ID = id
	}
	if err := h.repo.Save(c.Request.Context(), &rule); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rule)
}

func (h *AccountMatchingHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "无效规则编号")
		return
	}
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "已删除"})
}
