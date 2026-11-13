package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiFailoverExhaustionAfterPartialOutput(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 2)
	svc := service.NewOpenAIGatewayService(failoverPriorityAccountRepo{count: 2}, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, newAccountErrorFailoverTestSettings(), nil)
	group := &service.Group{ID: 1, Platform: service.PlatformOpenAI, AccountErrorFailoverEnabled: true}
	ctx := svc.PrepareRequestAccountFailover(context.Background(), &group.ID, group, "", service.PlatformOpenAI)
	router := gin.New()
	router.Use(OpsErrorLoggerMiddleware(service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)))
	router.POST("/v1beta/models/gemini:streamGenerateContent", func(c *gin.Context) {
		fs := NewFailoverState(1, false)
		for id := int64(1); id <= 2; id++ {
			setOpsSelectedAccount(c, id, service.PlatformGemini)
			failure := &service.UpstreamFailoverError{StatusCode: 502, ResponseBody: []byte(`{"error":{"code":502,"message":"unavailable"}}`), SafeToFailoverAfterWrite: true}
			_, err := service.ForwardWithAccountRetries(c.Request.Context(), c, func() (*service.ForwardResult, error) {
				c.Header("Content-Type", "text/event-stream")
				if id == 1 {
					_, _ = c.Writer.WriteString("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"partial\"}]}}]}\n\n")
					c.Writer.Flush()
				}
				raw, _ := c.Get(service.OpsUpstreamErrorsKey)
				events, _ := raw.([]*service.OpsUpstreamErrorEvent)
				c.Set(service.OpsUpstreamErrorsKey, append(events, &service.OpsUpstreamErrorEvent{AtUnixMs: time.Now().UnixMilli(), AccountID: id, Kind: "stream_failed", UpstreamStatusCode: 502, Message: "unavailable"}))
				return &service.ForwardResult{Usage: service.ClaudeUsage{OutputTokens: 2}}, failure
			})
			require.Error(t, err)
			if fs.HandleFailoverError(c.Request.Context(), &mockTempUnscheduler{}, id, service.PlatformGemini, 0, failure) == FailoverExhausted {
				(&GatewayHandler{}).handleGeminiFailoverExhausted(c, failure)
				return
			}
		}
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini:streamGenerateContent", nil).WithContext(ctx))
	require.Equal(t, 200, rec.Code, "committed wire status cannot change")
	require.Equal(t, 1, strings.Count(rec.Body.String(), `"error":`))
	frames := strings.Split(strings.TrimSpace(rec.Body.String()), "\n\n")
	require.Len(t, frames, 2)
	require.True(t, strings.HasPrefix(strings.TrimSpace(frames[1]), "data: "))
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(frames[1]), "data: ")), &payload))
	require.Contains(t, payload, "error")
	require.EqualValues(t, 1, OpsErrorLogQueueLength())
	job := <-opsErrorLogQueue
	require.Equal(t, 502, job.entry.StatusCode)
	var events []*service.OpsUpstreamErrorEvent
	require.NoError(t, json.Unmarshal([]byte(*job.entry.UpstreamErrorsJSON), &events))
	outcome := events[len(events)-1].FailoverResult
	require.Equal(t, "failed", outcome.Result)
	require.EqualValues(t, 2, outcome.FinalAccountID)
	require.Len(t, outcome.Attempts, 2)
	for _, attempt := range outcome.Attempts {
		require.Equal(t, "failed", attempt.Result)
		require.Equal(t, 502, attempt.StatusCode)
	}
}

func TestOpsTerminalFailureCorrectsFinalSuccessfulAttempt(t *testing.T) {
	svc := service.NewOpenAIGatewayService(failoverPriorityAccountRepo{count: 2}, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, newAccountErrorFailoverTestSettings(), nil)
	group := &service.Group{ID: 1, Platform: service.PlatformOpenAI, AccountErrorFailoverEnabled: true}
	ctx := svc.PrepareRequestAccountFailover(context.Background(), &group.ID, group, "", service.PlatformOpenAI)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini:streamGenerateContent", nil).WithContext(ctx)
	setOpsSelectedAccount(c, 403, service.PlatformGemini)
	_, _ = service.ForwardWithAccountRetries(c.Request.Context(), c, func() (*service.ForwardResult, error) { return &service.ForwardResult{}, nil })
	c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{AccountID: 403, UpstreamStatusCode: 502, Message: "unavailable"}})
	entry := &service.OpsInsertErrorLogInput{StatusCode: 502, ErrorMessage: "unavailable"}
	applyOpsUpstreamFieldsFromContext(c, entry)
	outcome := entry.UpstreamErrors[0].FailoverResult
	require.Equal(t, "failed", outcome.Result)
	require.Len(t, outcome.Attempts, 1)
	require.Equal(t, "failed", outcome.Attempts[0].Result)
	require.Equal(t, 502, outcome.Attempts[0].StatusCode)
	require.Equal(t, "success", service.GetRequestAccountFailoverAttempts(c.Request.Context())[0].Result, "persisted correction must not mutate the shared trace")
}
