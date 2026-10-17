package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type UserMenuHandler struct {
	store *service.GroupCategoryStore
}

func NewUserMenuHandler(store *service.GroupCategoryStore) *UserMenuHandler {
	return &UserMenuHandler{store: store}
}

func (h *UserMenuHandler) Get(c *gin.Context) {
	config, err := h.store.GetUserMenu(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

// Update is registered only under the authenticated admin route group.
func (h *UserMenuHandler) Update(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var input service.UserMenuConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid user menu configuration")
		return
	}
	config, err := h.store.SaveUserMenu(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}
