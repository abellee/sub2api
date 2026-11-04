//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestCostGuardLogExplainsEstimateAndDecision(t *testing.T) {
	withCostGuard(t, true)
	groupID := int64(2)
	key := &APIKey{ID: 739, User: &User{ID: 553}, GroupID: &groupID, Group: &Group{RateMultiplier: 0.039}}
	billing := costGuardGPT61Billing()
	d := inflightEstimateDeps{billing: billing, resolver: NewModelPricingResolver(nil, billing)}
	body := []byte(`{"input":"hello world","max_output_tokens":100,"metadata":{"private":"must not be logged"}}`)
	requestTime := time.Now().Add(-time.Second)
	for _, tc := range []struct {
		name, decision, reason string
		balance, pending       float64
	}{
		{"allow", "allow", "sufficient_balance", 1, 0},
		{"single cost", "reject", "single_request_cost_exceeds_balance", 1e-9, 0},
		{"inflight", "reject", "inflight_balance_insufficient", 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			core, logs := observer.New(zap.InfoLevel)
			ctx := logger.IntoContext(context.Background(), zap.New(core).With(zap.String("request_id", "server-req"), zap.String("client_request_id", "client-req")))
			ctx = context.WithValue(ctx, ctxkey.RequestStartedAt, requestTime)
			ctx, check := BeginCostGuardCheck(ctx, key, "gpt-6.1-sol")
			costGuardObserveUsage(key, &UsageLog{Model: "gpt-6.1-sol", InputTokens: 20, CacheReadTokens: 70, CacheCreationTokens: 10})
			cost, priced := d.estimate(ctx, key, InflightEstimateRequest{Model: "gpt-6.1-sol", Body: body, BodyBytes: len(body), MaxTokens: 100, CostGuardCheck: check})
			// A later usage update must not change the cache estimate logged for this request.
			costGuardObserveUsage(key, &UsageLog{Model: "gpt-6.1-sol", InputTokens: 100})
			cache := newMemInflightCache(tc.balance)
			if tc.pending > 0 {
				cache.res["other"] = tc.pending
				cache.exp["other"] = time.Now().Add(time.Minute)
			}
			cfg := &config.Config{}
			cfg.Billing.InflightReservation.MaxReservationUSD = cost / 2
			svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(svc.Stop)
			reservation, err := svc.ReserveInflight(ctx, key.User, key.Group, nil, cost)
			check.Finish(ctx, cost, priced, err)
			reservation.HandlerDone()
			entries := logs.FilterMessage("cost_guard.precheck").All()
			require.Len(t, entries, 1)
			fields := entries[0].ContextMap()
			require.Equal(t, "server-req", fields["request_id"])
			require.Equal(t, "client-req", fields["client_request_id"])
			require.EqualValues(t, 553, fields["user_id"])
			require.EqualValues(t, 739, fields["api_key_id"])
			require.EqualValues(t, 2, fields["group_id"])
			require.Equal(t, requestTime.Format(time.RFC3339Nano), fields["request_time"])
			require.Equal(t, tc.decision, fields["decision"])
			require.Equal(t, tc.reason, fields["reason"])
			require.Equal(t, "request_content", fields["input_source"])
			require.Equal(t, "request_limit", fields["output_source"])
			require.Equal(t, "previous_usage_ratio", fields["cache_source"])
			require.EqualValues(t, 100, fields["estimated_output_tokens"])
			input := float64(fields["estimated_input_tokens"].(int64))
			require.InDelta(t, input*0.7, fields["estimated_cache_read_tokens"], 1e-12)
			require.InDelta(t, input*0.1, fields["estimated_cache_creation_tokens"], 1e-12)
			require.InDelta(t, input*0.2, fields["estimated_uncached_input_tokens"], 1e-12)
			require.Equal(t, 0.039, fields["rate_multiplier"])
			require.Equal(t, cost, fields["estimated_cost_usd"])
			require.InDelta(t, fields["estimated_input_cost_usd"].(float64)+fields["estimated_output_cost_usd"].(float64), cost, 1e-12)
			require.Equal(t, cost/2, fields["reservation_usd"])
			require.Equal(t, tc.balance, fields["balance_usd"])
			if tc.decision == "reject" && tc.pending == 0 {
				require.Nil(t, fields["inflight_usd"], "no Redis call was made for a single-request rejection")
			} else {
				require.Equal(t, tc.pending, fields["inflight_usd"])
				require.Equal(t, tc.balance-tc.pending, fields["available_balance_usd"])
			}
			require.True(t, fields[logger.OpsSystemLogSkipField].(bool))
			require.NotContains(t, fields, "body")
			require.NotContains(t, fields, "input")
		})
	}
}

func TestCostGuardLogSkipsAndDisabled(t *testing.T) {
	settings := withCostGuard(t, true)
	key := &APIKey{ID: 739, User: &User{ID: 553}}
	d := inflightEstimateDeps{billing: costGuardGPT61Billing()}
	core, logs := observer.New(zap.InfoLevel)
	ctx := logger.IntoContext(context.Background(), zap.New(core))
	for _, tc := range []struct{ body, decision, reason string }{
		{`{"metadata":{"private":"not input"}}`, "skip", "input_estimate_unavailable"},
		{`{"input":"hello"}`, "allow", ""},
	} {
		checkCtx, check := BeginCostGuardCheck(ctx, key, "gpt-6.1-sol")
		cost, priced := d.estimate(checkCtx, key, InflightEstimateRequest{Model: "gpt-6.1-sol", Body: []byte(tc.body), CostGuardCheck: check})
		check.Finish(checkCtx, cost, priced, nil)
		fields := logs.FilterMessage("cost_guard.precheck").All()[logs.FilterMessage("cost_guard.precheck").Len()-1].ContextMap()
		require.Equal(t, tc.decision, fields["decision"])
		require.Equal(t, tc.reason, fields["reason"])
		require.Equal(t, "no_history", fields["cache_source"])
		require.Nil(t, fields["balance_usd"])
	}
	settings.costGuardEnabled.Store(false)
	_, disabled := BeginCostGuardCheck(ctx, key, "gpt-6.1-sol")
	require.Nil(t, disabled)
	count := logs.Len()
	disabled.Finish(ctx, 1, true, nil)
	require.Equal(t, count, logs.Len())
}

func TestCostGuardLogKeepsHighestCandidate(t *testing.T) {
	withCostGuard(t, true)
	groupID := int64(2)
	key := &APIKey{ID: 739, User: &User{ID: 553}, GroupID: &groupID, Group: &Group{RateMultiplier: 0.039}}
	billing := costGuardGPT61Billing()
	billing.pricingService.pricingData["higher"] = &LiteLLMModelPricing{InputCostPerToken: 10e-6, OutputCostPerToken: 50e-6, MaxOutputTokens: 1000}
	d := inflightEstimateDeps{billing: billing, resolveMapping: func(context.Context, int64, string) ChannelMappingResult {
		return ChannelMappingResult{MappedModel: "higher"}
	}}
	ctx, check := BeginCostGuardCheck(context.Background(), key, "gpt-6.1-sol")
	cost, priced := d.estimate(ctx, key, InflightEstimateRequest{Model: "gpt-6.1-sol", Body: []byte(`{"input":"hello"}`), MaxTokens: 100, CostGuardCheck: check})
	require.True(t, priced)
	require.Equal(t, "higher", check.estimate.model)
	require.Equal(t, 100, check.estimate.output)
	require.Equal(t, cost, check.estimate.cost)
}
