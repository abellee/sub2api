package admin

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GET /api/v1/admin/accounts/api-summary
func (h *AccountHandler) GetAPIAccountSummary(c *gin.Context) {
	if adminaccess.FromContext(c.Request.Context()) != nil {
		response.Forbidden(c, "此统计仅对完整管理员开放")
		return
	}
	if h.accountUsageService == nil {
		response.Error(c, 503, "统计汇总暂不可用")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	result, err := h.accountUsageService.GetAPIAccountSummary(ctx, service.APIAccountSummaryFilter{
		BaseURL: c.Query("base_url"), StartDate: c.Query("start_date"), EndDate: c.Query("end_date"),
		Timezone: c.Query("timezone"), Granularity: c.Query("granularity"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
