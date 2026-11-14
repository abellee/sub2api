package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageInputTokenEstimateSurvivesAsyncBoundary(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"gpt-6-sol","input":"hello world"}`))
	_, err := readLenientJSONRequestBodyWithPrealloc(req, nil)
	require.NoError(t, err)
	estimate := service.UsageInputTokenEstimateFromContext(req.Context())
	require.NotNil(t, estimate)
	require.Equal(t, 2, estimate.Tokens)
	task, _ := wrapUsageRecordTaskContext(req.Context(), func(ctx context.Context) {
		require.Equal(t, estimate, service.UsageInputTokenEstimateFromContext(ctx))
	})
	req = httptest.NewRequest("POST", "/next", nil)
	require.Nil(t, service.UsageInputTokenEstimateFromContext(req.Context()))
	task(context.Background())
}
