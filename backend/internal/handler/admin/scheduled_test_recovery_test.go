package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type scheduledRecoveryPlanRepo struct {
	service.ScheduledTestPlanRepository
	plan *service.ScheduledTestPlan
}

func (r *scheduledRecoveryPlanRepo) Create(ctx context.Context, plan *service.ScheduledTestPlan) (*service.ScheduledTestPlan, error) {
	plan.ID = 7
	r.plan = plan
	return plan, nil
}

func (r *scheduledRecoveryPlanRepo) GetByID(context.Context, int64) (*service.ScheduledTestPlan, error) {
	return r.plan, nil
}

func (r *scheduledRecoveryPlanRepo) Update(ctx context.Context, plan *service.ScheduledTestPlan) (*service.ScheduledTestPlan, error) {
	r.plan = plan
	return plan, nil
}

func TestScheduledRecoveryAPIRequiresExplicitOptInAndAllowsDisabling(t *testing.T) {
	repo := &scheduledRecoveryPlanRepo{}
	h := NewScheduledTestHandler(service.NewScheduledTestService(repo, nil))
	router := gin.New()
	router.POST("/plans", h.Create)
	router.PUT("/plans/:id", h.Update)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, r)
		return w
	}
	w := request(http.MethodPost, "/plans", `{"account_id":42,"model_id":"probe-model","cron_expression":"*/5 * * * *","auto_recover":true}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.False(t, repo.plan.RecoverScheduling, "existing auto-recover plans must not enable scheduling")
	require.Equal(t, 3, repo.plan.RecoveryProbeIntervalSeconds)
	w = request(http.MethodPut, "/plans/7", `{"recover_scheduling":true,"recovery_probe_interval_seconds":17}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	var plan service.ScheduledTestPlan
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &plan))
	require.True(t, plan.RecoverScheduling)
	require.Equal(t, 17, plan.RecoveryProbeIntervalSeconds)
	require.EqualValues(t, 42, plan.AccountID)
	w = request(http.MethodPut, "/plans/7", `{"recover_scheduling":false}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.False(t, repo.plan.RecoverScheduling)
	require.Equal(t, 17, repo.plan.RecoveryProbeIntervalSeconds, "an omitted interval keeps its saved value")
	for _, body := range []string{`{"recovery_probe_interval_seconds":0}`, `{"recovery_probe_interval_seconds":-1}`, `{"recovery_probe_interval_seconds":3601}`, `{"recovery_probe_interval_seconds":1.5}`} {
		w = request(http.MethodPut, "/plans/7", body)
		require.Equal(t, 400, w.Code, body)
		require.Equal(t, 17, repo.plan.RecoveryProbeIntervalSeconds)
	}
	w = request(http.MethodPost, "/plans", `{"account_id":42,"cron_expression":"*/5 * * * *","recover_scheduling":true}`)
	require.Equal(t, 400, w.Code, "recovery probes need a chosen model")
}
