package handler

import (
	"context"
	"encoding/json"
	"fmt"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type costGuardSettingRepo struct{ service.SettingRepository }

type costGuardLogOnlySettingRepo struct{ costGuardSettingRepo }

func (costGuardLogOnlySettingRepo) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	values, err := (costGuardSettingRepo{}).GetMultiple(ctx, keys)
	values[service.SettingKeyCostGuardEnabled] = "false"
	return values, err
}

type costGuardLogOnlyCache struct{ *handlerInflightCache }

func (*costGuardLogOnlyCache) ReserveInflightBalance(context.Context, int64, string, float64, float64, time.Duration) (bool, float64, error) {
	panic("logging-only requests must never reserve a balance")
}

func TestCostGuardHandlerLogOnlyNeverReservesOrRejects(t *testing.T) {
	settings := service.NewSettingService(costGuardLogOnlySettingRepo{}, nil)
	guard := service.NewCostGuardService(settings)
	t.Cleanup(guard.Stop)
	require.NoError(t, guard.Start(context.Background()))
	key := &service.APIKey{ID: 739, User: &service.User{ID: 553}}
	for _, legacyEnabled := range []bool{false, true} {
		for _, tc := range []struct {
			name      string
			cost      float64
			priced    bool
			estimated string
		}{
			{"enough", 0.1, true, "allow"},
			{"over balance", 5, true, "reject"},
			{"unknown price", 0, false, "skip"},
		} {
			t.Run(fmt.Sprintf("legacy=%t/%s", legacyEnabled, tc.name), func(t *testing.T) {
				core, logs := observer.New(zap.InfoLevel)
				ctx := logger.IntoContext(context.Background(), zap.New(core))
				cache := &costGuardLogOnlyCache{newHandlerInflightCache(1)}
				cfg := &config.Config{}
				cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: legacyEnabled, FailClosedOnUnpriced: true}
				billing := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
				t.Cleanup(billing.Stop)
				estimator := &countingEstimator{cost: tc.cost, priced: tc.priced}
				resultCtx, done, err := reserveInflightBalanceCtx(ctx, billing, estimator, key, nil, tokenInflightEstimate("model", nil))
				require.NoError(t, err)
				done()
				require.Equal(t, 1, estimator.calls)
				require.Nil(t, service.InflightReservationFromContext(resultCtx))
				require.Zero(t, cache.count())
				entries := logs.FilterMessage("cost_guard.precheck").All()
				require.Len(t, entries, 1)
				fields := entries[0].ContextMap()
				require.Equal(t, "log_only", fields["mode"])
				require.Equal(t, "allow", fields["decision"])
				require.Equal(t, "log_only", fields["reason"])
				require.Equal(t, tc.estimated, fields["estimated_decision"])
				require.Equal(t, tc.cost, fields["estimated_cost_usd"])
				require.Zero(t, fields["reservation_usd"])
			})
		}
	}
}

func TestCostGuardHandlerLogOnlySkipsSimpleAndSubscription(t *testing.T) {
	guard := service.NewCostGuardService(service.NewSettingService(costGuardLogOnlySettingRepo{}, nil))
	t.Cleanup(guard.Stop)
	require.NoError(t, guard.Start(context.Background()))
	for _, simple := range []bool{false, true} {
		cfg := &config.Config{}
		key := &service.APIKey{User: &service.User{ID: 553}, Group: &service.Group{SubscriptionType: service.SubscriptionTypeSubscription}}
		if simple {
			cfg.RunMode = config.RunModeSimple
			key.Group = nil
		}
		billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
		t.Cleanup(billing.Stop)
		estimator := &countingEstimator{cost: 5, priced: true}
		core, logs := observer.New(zap.InfoLevel)
		ctx := logger.IntoContext(context.Background(), zap.New(core))
		_, done, err := reserveInflightBalanceCtx(ctx, billing, estimator, key, &service.UserSubscription{}, tokenInflightEstimate("model", nil))
		require.NoError(t, err)
		done()
		require.Zero(t, estimator.calls)
		require.Zero(t, logs.Len())
	}
}

func TestCostGuardHandlerLogsEachFinalDecision(t *testing.T) {
	settings := service.NewSettingService(costGuardSettingRepo{}, nil)
	guard := service.NewCostGuardService(settings)
	t.Cleanup(guard.Stop)
	require.NoError(t, guard.Start(context.Background()))
	core, logs := observer.New(zap.InfoLevel)
	ctx := logger.IntoContext(context.Background(), zap.New(core).With(zap.String("request_id", "handler-check")))
	cache := newHandlerInflightCache(1)
	billing := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(billing.Stop)
	key := &service.APIKey{ID: 739, User: &service.User{ID: 553}}
	for _, tc := range []struct {
		cost     float64
		decision string
	}{{0.1, "allow"}, {5, "reject"}} {
		_, done, err := reserveInflightBalanceCtx(ctx, billing, &countingEstimator{cost: tc.cost, priced: true}, key, nil, tokenInflightEstimate("model", nil))
		if tc.decision == "reject" {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		done()
		entries := logs.FilterMessage("cost_guard.precheck").All()
		require.Len(t, entries, 1)
		require.Equal(t, tc.decision, entries[0].ContextMap()["decision"])
		require.Equal(t, tc.cost, entries[0].ContextMap()["estimated_cost_usd"])
		logs.TakeAll()
	}
}

func (costGuardSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{
		service.SettingKeyCostGuardEnabled:         "true",
		service.SettingKeyCostGuardErrorStatusCode: "429",
		service.SettingKeyCostGuardErrorMessage:    "余额不足，请减少本次用量",
	}, nil
}

func TestCostGuardHandlerRejectsLargeEstimateAndPreservesFailOpen(t *testing.T) {
	settings := service.NewSettingService(costGuardSettingRepo{}, nil)
	guard := service.NewCostGuardService(settings)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(guard.Stop)
	require.NoError(t, guard.Start(ctx))
	cache := newHandlerInflightCache(1)
	cfg := &config.Config{}
	cfg.Billing.InflightReservation.Enabled = true
	billing := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	key := &service.APIKey{ID: 1, User: &service.User{ID: 1}}
	estimator := &countingEstimator{cost: 5, priced: true}
	_, done, err := reserveInflightBalanceCtx(ctx, billing, estimator, key, nil, tokenInflightEstimate("model", nil))
	require.ErrorIs(t, err, service.ErrInsufficientBalance)
	assertCostGuardHTTPResponse(t, err)
	done()
	estimator.cost = 0.6
	_, firstDone, err := reserveInflightBalanceCtx(ctx, billing, estimator, key, nil, tokenInflightEstimate("model", nil))
	require.NoError(t, err)
	_, _, err = reserveInflightBalanceCtx(ctx, billing, estimator, key, nil, tokenInflightEstimate("model", nil))
	require.ErrorIs(t, err, service.ErrInsufficientBalance)
	assertCostGuardHTTPResponse(t, err)
	firstDone()
	estimator.cost, estimator.priced = 0, false
	_, done, err = reserveInflightBalanceCtx(ctx, billing, estimator, key, nil, tokenInflightEstimate("unknown", nil))
	require.NoError(t, err, "missing prices must not block requests")
	done()
	guard.Stop()
	estimator.cost, estimator.priced = 5, true
	_, done, err = reserveInflightBalanceCtx(ctx, billing, estimator, key, nil, tokenInflightEstimate("model", nil))
	require.NoError(t, err, "disabled fork must preserve the host's existing behavior")
	done()
}

func assertCostGuardHTTPResponse(t *testing.T, rejection error) {
	t.Helper()
	status, code, message, retry := billingErrorDetails(fmt.Errorf("wrapped: %w", rejection))
	require.Equal(t, 429, status)
	require.Equal(t, "余额不足，请减少本次用量", message)
	require.Zero(t, retry)
	for _, protocol := range []string{"openai", "anthropic", "gemini"} {
		t.Run(protocol, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			switch protocol {
			case "openai":
				(&OpenAIGatewayHandler{}).handleStreamingAwareError(c, status, code, message, false)
			case "anthropic":
				(&OpenAIGatewayHandler{}).anthropicErrorResponse(c, status, code, message)
			case "gemini":
				googleErrorWithType(c, status, code, "", message)
			}
			require.Equal(t, 429, rec.Code)
			var body struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, message, body.Error.Message)
		})
	}
}

func TestCostGuardWebSocketSendsConfiguredRejection(t *testing.T) {
	message := "余额不足，请减少本次用量"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		writeCostGuardWSError(r.Context(), conn, &service.CostGuardRejectedError{StatusCode: 429, Message: message})
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()
	_, data, err := conn.Read(ctx)
	require.NoError(t, err)
	var event struct {
		Type   string `json:"type"`
		Status int    `json:"status_code"`
		Error  struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(data, &event))
	require.Equal(t, "error", event.Type)
	require.Equal(t, 429, event.Status)
	require.Equal(t, 429, event.Error.Code)
	require.Equal(t, message, event.Error.Message)
}

func TestCostGuardStreamingErrorIncludesConfiguredCode(t *testing.T) {
	for _, protocol := range []string{"openai", "anthropic", "gateway", "responses"} {
		t.Run(protocol, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			path := "/v1/chat/completions"
			if protocol == "responses" {
				path = "/v1/responses"
			}
			c.Request = httptest.NewRequest("POST", path, nil)
			// A waiting heartbeat has already committed the HTTP 200 response.
			rec.WriteHeader(http.StatusOK)
			c.Writer.WriteHeaderNow()
			switch protocol {
			case "gateway":
				(&GatewayHandler{}).handleStreamingAwareError(c, 429, costGuardErrorType, "余额不足", true)
			case "anthropic":
				(&OpenAIGatewayHandler{}).anthropicStreamingAwareError(c, 429, costGuardErrorType, "余额不足", true)
			default:
				(&OpenAIGatewayHandler{}).handleStreamingAwareError(c, 429, costGuardErrorType, "余额不足", true)
			}
			require.Equal(t, 200, rec.Code)
			require.Contains(t, rec.Body.String(), `"code":"429"`)
			require.Contains(t, rec.Body.String(), `"message":"余额不足"`)
			if protocol == "responses" {
				require.Contains(t, rec.Body.String(), "response.failed")
			}
		})
	}
}
