package admin

import (
	"context"
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// BatchDuplicate creates numbered copies with automatic upstream billing probes enabled.
func (h *AccountHandler) BatchDuplicate(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	var req struct {
		Count int `json:"count" binding:"required,min=1,max=100"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "count must be an integer between 1 and 100")
		return
	}
	actorScope, operationKey := adminActorScope(c), c.GetHeader("Idempotency-Key")
	present := func(ctx context.Context, accounts []*service.Account) []any {
		result := make([]any, len(accounts))
		for i, account := range accounts {
			result[i] = h.buildAccountResponseWithRuntime(ctx, account)
		}
		return result
	}
	result, err := executeAdminIdempotent(c, "admin.accounts.batch_duplicate", struct {
		AccountID int64 `json:"account_id"`
		Count     int   `json:"count"`
	}{accountID, req.Count}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		accounts, err := h.adminService.BatchDuplicateAccounts(ctx, accountID, req.Count, actorScope, operationKey)
		if err != nil {
			return nil, err
		}
		return present(ctx, accounts), nil
	})
	if err != nil {
		reason := infraerrors.Reason(err)
		if reason == infraerrors.Reason(service.ErrIdempotencyInProgress) || reason == infraerrors.Reason(service.ErrIdempotencyStoreUnavail) {
			accounts, recoveryErr := h.adminService.RecoverBatchDuplicateAccounts(c.Request.Context(), accountID, req.Count, actorScope, operationKey)
			if recoveryErr == nil && len(accounts) == req.Count {
				c.Header("X-Idempotency-Recovered", "true")
				response.Success(c, present(c.Request.Context(), accounts))
				return
			}
		}
		response.ErrorFrom(c, err)
		return
	}
	if result.Replayed {
		c.Header("X-Idempotency-Replayed", "true")
	}
	response.Success(c, result.Data)
}
