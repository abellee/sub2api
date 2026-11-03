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
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type costGuardSettingRepo struct{ service.SettingRepository }

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
		defer conn.CloseNow()
		writeCostGuardWSError(r.Context(), conn, &service.CostGuardRejectedError{StatusCode: 429, Message: message})
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	require.NoError(t, err)
	defer conn.CloseNow()
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
