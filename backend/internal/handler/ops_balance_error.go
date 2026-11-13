package handler

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const opsUserBalanceErrorKey = "ops_local_user_balance_error"

// Only locally rejected requests belong to the user's balance history. An
// upstream account can return the same words/code and must still fail over.
func isOpsUserBalanceError(c *gin.Context, errType, code, message string) bool {
	if value, ok := c.Get(opsUserBalanceErrorKey); ok {
		marked, _ := value.(bool)
		return marked
	}
	if hasOpsUpstreamErrorContext(c) {
		return false
	}
	if errType == service.OpsUserBalanceErrorType || strings.EqualFold(code, opsCodeInsufficientBalance) {
		return true
	}
	msg := strings.ToLower(message)
	return strings.Contains(msg, opsErrInsufficientBalance) ||
		strings.Contains(msg, opsErrInsufficientAccountBalance) ||
		strings.Contains(msg, "用户余额不足")
}

// WebSocket checks occur after upgrade, so a close frame alone cannot record
// the first local rejection. Mark it explicitly without changing the wire reply.
func markOpsUserBalanceError(c *gin.Context, err error) {
	balance := service.IsUserBalanceError(err)
	c.Set(opsUserBalanceErrorKey, balance)
	if balance && c.Writer.Status() == http.StatusSwitchingProtocols {
		status, code, message, _ := billingErrorDetails(err)
		service.MarkOpsStreamErrorValue(c, service.OpsStreamError{
			ErrType: code, Message: message, IntendedStatus: status, RequestScoped: true,
		})
	}
}
