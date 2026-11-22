//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiBillingReconciliation_HTTP200AndCanceledClientStillRecordsException(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 4)
	gin.SetMode(gin.TestMode)
	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	router := gin.New()
	router.Use(OpsErrorLoggerMiddleware(ops))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		setOpsRequestContext(c, "gemini-3-flash-preview", true)
		c.Status(http.StatusOK)
		c.Writer.Flush()
		cancel()
		service.MarkOpsStreamErrorValue(c, service.OpsStreamError{
			ErrType:        "billing_usage_unavailable",
			Code:           "gemini_usage_unavailable_after_disconnect",
			Message:        "Gemini client disconnected; upstream billing usage could not be collected",
			IntendedStatus: 499,
			RequestScoped:  true,
		})
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int64(1), OpsErrorLogQueueLength())
	job := <-opsErrorLogQueue
	require.Equal(t, 499, job.entry.StatusCode)
	require.True(t, job.entry.Stream)
	require.True(t, job.entry.IsBusinessLimited, "reconciliation must not be counted as provider failure")
	require.Contains(t, job.entry.ErrorBody, "gemini_usage_unavailable_after_disconnect")
}
