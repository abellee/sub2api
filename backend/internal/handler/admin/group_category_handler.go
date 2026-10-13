package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// GroupCategoryHandler manages JSON-backed group categories.
type GroupCategoryHandler struct {
	store *service.GroupCategoryStore
}

func NewGroupCategoryHandler(store *service.GroupCategoryStore) *GroupCategoryHandler {
	if store == nil {
		store = service.DefaultGroupCategoryStore()
	}
	return &GroupCategoryHandler{store: store}
}

type createGroupCategoryRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Recommended bool   `json:"recommended"`
}

type updateGroupCategoryRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Recommended *bool   `json:"recommended"`
}

type assignGroupCategoryRequest struct {
	CategoryID *string `json:"category_id"`
}

type reorderGroupCategoriesRequest struct {
	IDs []string `json:"ids"`
}

// List GET /admin/group-categories
func (h *GroupCategoryHandler) List(c *gin.Context) {
	snap, err := h.store.List()
	if err != nil {
		response.InternalError(c, "Failed to load categories")
		return
	}
	response.Success(c, snap)
}

// Create POST /admin/group-categories
func (h *GroupCategoryHandler) Create(c *gin.Context) {
	var req createGroupCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	cat, err := h.store.Create(req.Name, req.Description, req.Recommended)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, cat)
}

// Update PUT /admin/group-categories/:id
func (h *GroupCategoryHandler) Update(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	var req updateGroupCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	name := ""
	if req.Name != nil {
		name = *req.Name
	}
	cat, err := h.store.Update(id, name, req.Description, req.Recommended)
	if err != nil {
		if err.Error() == "category not found" {
			response.NotFound(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, cat)
}

// Delete DELETE /admin/group-categories/:id
func (h *GroupCategoryHandler) Delete(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if err := h.store.Delete(id); err != nil {
		if err.Error() == "category not found" {
			response.NotFound(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// Assign PUT /admin/group-categories/assignments/:group_id
func (h *GroupCategoryHandler) Assign(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("group_id"), 10, 64)
	if err != nil || groupID <= 0 {
		response.BadRequest(c, "Invalid group id")
		return
	}
	var req assignGroupCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	categoryID := ""
	if req.CategoryID != nil {
		categoryID = *req.CategoryID
	}
	if err := h.store.Assign(groupID, categoryID); err != nil {
		if err.Error() == "category not found" {
			response.NotFound(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// Reorder PUT /admin/group-categories/sort-order
func (h *GroupCategoryHandler) Reorder(c *gin.Context) {
	var req reorderGroupCategoriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	if err := h.store.Reorder(req.IDs); err != nil {
		if err.Error() == "category not found" {
			response.NotFound(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"ok": true})
}
