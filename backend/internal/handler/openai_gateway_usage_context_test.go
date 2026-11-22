package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSubmitUsageRecordTaskCopiesRequestContext(t *testing.T) {
	parent := context.WithValue(context.Background(), ctxkey.ClientRequestID, "client-request-123")
	parent = context.WithValue(parent, ctxkey.RequestID, "request-456")

	var gotClientRequestID string
	var gotRequestID string
	h := &GatewayHandler{}
	h.submitUsageRecordTask(parent, func(ctx context.Context) {
		gotClientRequestID, _ = ctx.Value(ctxkey.ClientRequestID).(string)
		gotRequestID, _ = ctx.Value(ctxkey.RequestID).(string)
	})

	require.Equal(t, "client-request-123", gotClientRequestID)
	require.Equal(t, "request-456", gotRequestID)
}

func TestUsageRecordTaskPreservesAccountBillingDiscriminator(t *testing.T) {
	svc := service.NewOpenAIGatewayService(failoverPriorityAccountRepo{count: 3}, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, newAccountErrorFailoverTestSettings(), nil)
	group := &service.Group{ID: 1, Platform: service.PlatformOpenAI, AccountErrorFailoverEnabled: true}
	ctx := svc.PrepareRequestAccountFailover(context.Background(), &group.ID, group, "", service.PlatformOpenAI)
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	called := false
	(&GatewayHandler{}).submitUsageRecordTask(ctx, func(worker context.Context) {
		called = true
		require.NoError(t, worker.Err())
		require.True(t, service.RequestAccountFailoverEnabled(worker))
	})
	require.True(t, called)
}

func TestOpenAISubmitUsageRecordTaskCopiesRequestContext(t *testing.T) {
	parent := context.WithValue(context.Background(), ctxkey.ClientRequestID, "openai-client-request-123")
	parent = context.WithValue(parent, ctxkey.RequestID, "openai-request-456")

	var gotClientRequestID string
	var gotRequestID string
	h := &OpenAIGatewayHandler{}
	h.submitUsageRecordTask(parent, func(ctx context.Context) {
		gotClientRequestID, _ = ctx.Value(ctxkey.ClientRequestID).(string)
		gotRequestID, _ = ctx.Value(ctxkey.RequestID).(string)
	})

	require.Equal(t, "openai-client-request-123", gotClientRequestID)
	require.Equal(t, "openai-request-456", gotRequestID)
}
