package handler

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLocalBalanceClassification(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.True(t, isOpsUserBalanceError(c, "billing_error", "INSUFFICIENT_BALANCE", ""))
	service.SetOpsUpstreamError(c, 403, "insufficient balance", "")
	require.False(t, isOpsUserBalanceError(c, "insufficient_balance", "INSUFFICIENT_BALANCE", "insufficient balance"))
	c.Set(opsUserBalanceErrorKey, true)
	require.True(t, isOpsUserBalanceError(c, costGuardErrorType, "503", "custom message"))
	c.Set(opsUserBalanceErrorKey, false)
	require.False(t, isOpsUserBalanceError(c, costGuardErrorType, "403", "insufficient balance"))
	require.True(t, service.IsUserBalanceError(service.ErrInsufficientBalance))
	require.True(t, service.IsUserBalanceError(&service.CostGuardRejectedError{Message: "custom"}))
	require.False(t, service.IsUserBalanceError(&service.CostGuardRejectedError{Message: "custom", Unpriced: true}))
	require.False(t, service.IsUserBalanceError(errors.New("insufficient balance")))
}

func TestWebSocketBalanceRejectionHasExplicitLocalRecord(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Writer.WriteHeader(101)
	markOpsUserBalanceError(c, service.ErrInsufficientBalance)
	stream, ok := service.GetOpsStreamError(c)
	require.True(t, ok)
	require.True(t, stream.RequestScoped)
	require.Equal(t, 403, stream.IntendedStatus)
	require.True(t, isOpsUserBalanceError(c, stream.ErrType, stream.Code, stream.Message))
}
