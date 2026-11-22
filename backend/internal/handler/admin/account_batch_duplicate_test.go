//go:build unit

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type batchDuplicateAdminStub struct {
	service.AdminService
	calls, count int
}

func (s *batchDuplicateAdminStub) BatchDuplicateAccounts(_ context.Context, id int64, count int, actorScope, operationKey string) ([]*service.Account, error) {
	s.calls++
	s.count = count
	return []*service.Account{{ID: 43, Name: "GPT-UltraFast1-1", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "top-secret-key"}, Extra: map[string]any{service.UpstreamBillingProbeEnabledExtraKey: true}}}, nil
}

func TestBatchDuplicateAccountsHandlerValidatesAndRedacts(t *testing.T) {
	for _, body := range []string{`{"count":10}`, `{}`, `{"count":0}`, `{"count":101}`, `{"count":1.5}`, `{"count":"10"}`} {
		t.Run(body, func(t *testing.T) {
			svc := &batchDuplicateAdminStub{}
			router := setupDuplicateAccountRouter(t, svc)
			h := &AccountHandler{adminService: svc}
			router.POST("/api/v1/admin/accounts/:id/batch-duplicate", h.BatchDuplicate)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/42/batch-duplicate", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if body == `{"count":10}` {
				require.Equal(t, http.StatusOK, recorder.Code)
				require.Equal(t, 10, svc.count)
				require.Contains(t, recorder.Body.String(), "GPT-UltraFast1-1")
				require.Contains(t, recorder.Body.String(), `"upstream_billing_probe_enabled":true`)
				require.NotContains(t, recorder.Body.String(), "top-secret-key")
			} else {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Zero(t, svc.calls)
			}
		})
	}
}
