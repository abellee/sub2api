//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConversationFailoverLateSuccessBillsOriginalAccount(t *testing.T) {
	ctx, c, _ := conversationFailoverTestContext(t)
	usageRepo := &openAIRecordUsageLogRepoStub{}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newGatewayRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{})
	allowLate := make(chan struct{})
	billed := make(chan error, 1)
	apiKey := &APIKey{ID: 1}
	user := &User{ID: 2}
	originalAccount := &Account{ID: 1}
	_, err := ForwardConversationWithAccountRetries(ctx, c, nil, "claude-sonnet-4", func(context.Context, *gin.Context) (*ForwardResult, error) {
		<-allowLate
		return &ForwardResult{Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 1000, OutputTokens: 200}}, nil
	}, func(billingCtx context.Context, result *ForwardResult) {
		billed <- svc.RecordUsage(billingCtx, &RecordUsageInput{Result: result, APIKey: apiKey, User: user, Account: originalAccount})
	})
	require.Error(t, err)
	close(allowLate)
	select {
	case err := <-billed:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("late billing did not finish")
	}
	require.Equal(t, 1, userRepo.deductCalls)
	require.Equal(t, int64(1), usageRepo.lastLog.AccountID)
	require.Equal(t, int64(2), usageRepo.lastLog.UserID)
	require.Greater(t, usageRepo.lastLog.ActualCost, float64(0))
	err = svc.RecordUsage(ctx, &RecordUsageInput{
		Result: &ForwardResult{Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 1000, OutputTokens: 200}},
		APIKey: apiKey, User: user, Account: &Account{ID: 2},
	})
	require.NoError(t, err)
	require.Equal(t, 2, userRepo.deductCalls, "both the late original and the next successful attempt are billed")
	require.Equal(t, int64(2), usageRepo.lastLog.AccountID)
}
