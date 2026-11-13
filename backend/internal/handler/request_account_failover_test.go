package handler

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type accountErrorFailoverTestSettingsRepo struct{ service.SettingRepository }

func (accountErrorFailoverTestSettingsRepo) GetValue(_ context.Context, key string) (string, error) {
	if key == service.SettingKeyAccountErrorFailoverEnabled {
		return "true", nil
	}
	return "", service.ErrSettingNotFound
}

func TestOpsFailoverOutcomeDoesNotUseHTTP200AsSuccess(t *testing.T) {
	for _, outcome := range []string{"success", "failed", "canceled"} {
		t.Run(outcome, func(t *testing.T) {
			svc := service.NewOpenAIGatewayService(failoverPriorityAccountRepo{count: 8}, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, newAccountErrorFailoverTestSettings(), nil)
			group := &service.Group{ID: 1, Platform: service.PlatformOpenAI, AccountErrorFailoverEnabled: true}
			ctx := svc.PrepareRequestAccountFailover(context.Background(), &group.ID, group, "", service.PlatformOpenAI)
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
			c.Set(opsAccountIDKey, int64(12))
			event := &service.OpsUpstreamErrorEvent{AccountID: 333, UpstreamStatusCode: 502, Message: "upstream failed"}
			c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{event})
			_, _ = service.ForwardWithAccountRetries(ctx, c, func() (*service.ForwardResult, error) {
				if outcome == "failed" {
					return nil, errors.New("upstream failed")
				}
				return &service.ForwardResult{}, nil
			})
			if outcome == "canceled" {
				cancel()
			}
			entry := &service.OpsInsertErrorLogInput{StatusCode: http.StatusOK}
			applyOpsUpstreamFieldsFromContext(c, entry)
			require.Len(t, entry.UpstreamErrors, 1)
			result := entry.UpstreamErrors[0].FailoverResult
			require.NotNil(t, result)
			require.Equal(t, outcome, result.Result)
			if outcome == "success" {
				require.EqualValues(t, 12, result.FinalAccountID)
			} else {
				require.Zero(t, result.FinalAccountID)
			}
			require.EqualValues(t, 333, entry.UpstreamErrors[0].AccountID, "preserve attribution of failed upstream")
			require.Nil(t, event.FailoverResult, "never mutate shared attempt data")
		})
	}
}

func TestOpsFailoverMiddlewareRecordsActualOutcome(t *testing.T) {
	for _, tc := range []struct {
		name, frame string
		success     bool
		status      int
	}{
		{"failed after headers", ": keepalive\n\n", false, 502},
		{"input error preserves status", "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"invalid_request_error\",\"message\":\"invalid parameter\"}}\n\n", false, 400},
		{"success", "data: {\"type\":\"message_stop\"}\n\n", true, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupOpsErrorLogTestQueue(t, 2)
			svc := service.NewOpenAIGatewayService(failoverPriorityAccountRepo{count: 8}, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, newAccountErrorFailoverTestSettings(), nil)
			group := &service.Group{ID: 1, Platform: service.PlatformOpenAI, AccountErrorFailoverEnabled: true}
			ctx := svc.PrepareRequestAccountFailover(context.Background(), &group.ID, group, "", service.PlatformOpenAI)
			router := gin.New()
			router.Use(OpsErrorLoggerMiddleware(service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)))
			router.POST("/v1/messages", func(c *gin.Context) {
				c.Set(opsAccountIDKey, int64(12))
				c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{AccountID: 333, UpstreamStatusCode: 502, Message: "provider failure"}})
				_, _ = service.ForwardWithAccountRetries(ctx, c, func() (*service.ForwardResult, error) {
					c.Header("Content-Type", "text/event-stream")
					_, _ = c.Writer.WriteString(tc.frame)
					if !tc.success {
						return nil, errors.New("upstream failure")
					}
					return &service.ForwardResult{}, nil
				})
			})
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx))
			require.Equal(t, 200, rec.Code)
			require.EqualValues(t, 1, OpsErrorLogQueueLength())
			job := <-opsErrorLogQueue
			require.Equal(t, tc.status, job.entry.StatusCode)
			require.NotNil(t, job.entry.UpstreamErrorsJSON)
			var persisted []*service.OpsUpstreamErrorEvent
			require.NoError(t, json.Unmarshal([]byte(*job.entry.UpstreamErrorsJSON), &persisted))
			require.NotEmpty(t, persisted)
			outcome := persisted[len(persisted)-1].FailoverResult
			require.NotNil(t, outcome)
			if tc.success {
				require.Equal(t, "success", outcome.Result)
				require.EqualValues(t, 12, outcome.FinalAccountID)
			} else {
				require.Equal(t, "failed", outcome.Result)
				require.NotContains(t, job.entry.ErrorMessage, "Recovered")
			}
		})
	}
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
	for _, count := range []int{1, 3, 8, 15, 20} {
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
