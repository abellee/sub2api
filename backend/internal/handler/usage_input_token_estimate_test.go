package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRequestBodyReadDoesNotEstimateInputTokens(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"gpt-6-sol","input":"hello world"}`))
	body, err := readLenientJSONRequestBodyWithPrealloc(req, nil)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-6-sol","input":"hello world"}`, string(body))
	require.Nil(t, service.UsageInputTokenEstimateFromContext(req.Context()))

	task, _ := wrapUsageRecordTaskContext(req.Context(), func(ctx context.Context) {
		require.Nil(t, service.UsageInputTokenEstimateFromContext(ctx))
	})
	task(context.Background())
}
