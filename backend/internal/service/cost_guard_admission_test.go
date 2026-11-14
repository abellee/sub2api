//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCostGuardCheckPreservesAdmissionAcrossSettingChanges(t *testing.T) {
	settings := withCostGuard(t, false)
	key := &APIKey{User: &User{ID: 553}}
	check := BeginCostGuardCheck(key)
	require.NotNil(t, check)
	require.True(t, check.EnforcementDisabled())
	settings.costGuardEnabled.Store(true)
	require.True(t, check.EnforcementDisabled(), "enabling precheck cannot reject an already admitted request")
	enabled := BeginCostGuardCheck(key)
	require.False(t, enabled.EnforcementDisabled())
	settings.costGuardEnabled.Store(false)
	require.True(t, enabled.EnforcementDisabled())
	require.Nil(t, BeginCostGuardCheck(nil))
	require.Nil(t, BeginCostGuardCheck(&APIKey{}))
	costGuardDefault.Load().Stop()
	absent := BeginCostGuardCheck(key)
	require.Nil(t, absent)
	require.False(t, absent.EnforcementDisabled(), "uninitialized precheck must preserve legacy admission")
}

func TestCostGuardDisabledKeepsEstimateAndHistory(t *testing.T) {
	settings := withCostGuard(t, false)
	key := &APIKey{ID: 739, User: &User{ID: 553}, Group: &Group{LongContextPricingEnabled: true}}
	d := inflightEstimateDeps{billing: costGuardGPT61Billing()}
	costGuardObserveUsage(key, &UsageLog{Model: "gpt-6.1-sol", InputTokens: 20, CacheReadTokens: 80})
	req := InflightEstimateRequest{Model: "gpt-6.1-sol", Body: []byte(`{"input":"hello world"}`), BodyBytes: 6_300_000}
	disabled, priced := d.estimate(context.Background(), key, req)
	require.True(t, priced)
	require.Positive(t, disabled)
	settings.costGuardEnabled.Store(true)
	enabled, priced := d.estimate(context.Background(), key, req)
	require.True(t, priced)
	require.Equal(t, disabled, enabled)
	req.Body = nil
	req.BodyBytes = 400000 * 4
	settings.costGuardEnabled.Store(false)
	disabled, priced = d.estimate(context.Background(), key, req)
	require.True(t, priced)
	settings.costGuardEnabled.Store(true)
	enabled, priced = d.estimate(context.Background(), key, req)
	require.True(t, priced)
	require.Equal(t, disabled, enabled, "long-context and cache pricing must not change between modes")
}

func TestCostGuardKeepsHighestCandidate(t *testing.T) {
	withCostGuard(t, true)
	groupID := int64(2)
	key := &APIKey{ID: 739, User: &User{ID: 553}, GroupID: &groupID, Group: &Group{RateMultiplier: 0.039}}
	billing := costGuardGPT61Billing()
	billing.pricingService.pricingData["higher"] = &LiteLLMModelPricing{InputCostPerToken: 10e-6, OutputCostPerToken: 50e-6, MaxOutputTokens: 1000}
	d := inflightEstimateDeps{billing: billing, resolveMapping: func(context.Context, int64, string) ChannelMappingResult {
		return ChannelMappingResult{MappedModel: "higher"}
	}}
	req := InflightEstimateRequest{Model: "gpt-6.1-sol", Body: []byte(`{"input":"hello"}`), MaxTokens: 100}
	cost, priced := d.estimate(context.Background(), key, req)
	require.True(t, priced)
	input, err := estimateCostGuardInputTokens(req.Model, req.Body)
	require.NoError(t, err)
	require.InDelta(t, (float64(input)*10e-6+100*50e-6)*0.039, cost, 1e-12)
}
