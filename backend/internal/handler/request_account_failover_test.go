package handler

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type accountErrorFailoverTestSettingsRepo struct{ service.SettingRepository }

func (accountErrorFailoverTestSettingsRepo) GetValue(_ context.Context, key string) (string, error) {
	if key == service.SettingKeyAccountErrorFailoverEnabled {
		return "true", nil
	}
	return "", service.ErrSettingNotFound
}

func (accountErrorFailoverTestSettingsRepo) GetMultiple(_ context.Context, _ []string) (map[string]string, error) {
	return map[string]string{service.SettingKeyAccountErrorFailoverEnabled: "true"}, nil
}

func newAccountErrorFailoverTestSettings() *service.SettingService {
	return service.NewSettingService(accountErrorFailoverTestSettingsRepo{}, &config.Config{})
}

// Use the production preparation path so single-account groups retain pool retries.
type failoverPriorityAccountRepo struct {
	service.AccountRepository
	count int
}

func (r failoverPriorityAccountRepo) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]service.Account, error) {
	accounts := make([]service.Account, r.count)
	for i := range accounts {
		accounts[i] = service.Account{ID: int64(i + 1), Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true}
	}
	return accounts, nil
}

func TestAccountFailoverPriorityAndExhaustion(t *testing.T) {
	for _, count := range []int{1, 3} {
		svc := service.NewOpenAIGatewayService(failoverPriorityAccountRepo{count: count}, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, newAccountErrorFailoverTestSettings(), nil)
		group := &service.Group{ID: 1, Platform: service.PlatformOpenAI, AccountErrorFailoverEnabled: true}
		ctx := svc.PrepareRequestAccountFailover(context.Background(), &group.ID, group, "", service.PlatformOpenAI)
		failure := &service.UpstreamFailoverError{StatusCode: 503, RetryableOnSameAccount: true, SameAccountRetryDeadline: time.Now().Add(time.Minute)}
		require.Equal(t, count == 1, sameAccountRetryAllowed(ctx, failure, 0, 3))
		if count == 1 {
			continue
		}
		fs := NewFailoverState(service.RequestAccountFailoverLimit(ctx, 10), false)
		unscheduler := &mockTempUnscheduler{}
		for id := int64(1); id <= int64(count); id++ {
			action := fs.HandleFailoverError(ctx, unscheduler, id, service.PlatformOpenAI, 3, failure)
			if id < int64(count) {
				require.Equal(t, FailoverContinue, action)
			} else {
				require.Equal(t, FailoverExhausted, action)
			}
			require.Contains(t, fs.FailedAccountIDs, id)
		}
		require.Empty(t, fs.SameAccountRetryCount)
		require.Equal(t, FailoverExhausted, fs.HandleSelectionExhausted(ctx))
		stop := &service.UpstreamFailoverError{StatusCode: 400, NextAccountAction: service.NextAccountStop, RetryableOnSameAccount: true}
		require.False(t, sameAccountRetryAllowed(ctx, stop, 0, 3))
		require.Equal(t, FailoverExhausted, NewFailoverState(10, false).HandleFailoverError(ctx, unscheduler, 1, service.PlatformOpenAI, 3, stop))
	}
}
