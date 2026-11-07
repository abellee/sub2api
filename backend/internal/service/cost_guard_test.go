package service

import (
	"context"
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func withCostGuard(t *testing.T, enabled bool) *SettingService {
	t.Helper()
	settings := NewSettingService(nil, nil)
	settings.costGuardEnabled.Store(enabled)
	s := NewCostGuardService(settings)
	t.Cleanup(s.Stop)
	return settings
}

func TestCostGuardUsesCurrentRequestAndPreviousUsage(t *testing.T) {
	settings := withCostGuard(t, true)
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{DefaultMaxTokens: 200, MaxInputTokens: 1, MaxOutputTokens: 1}
	billing := NewBillingService(cfg, nil)
	groupID := int64(7)
	key := &APIKey{ID: 3, User: &User{ID: 1}, GroupID: &groupID, Group: &Group{RateMultiplier: 2}}
	model := "claude-sonnet-4-5"
	p, err := billing.GetModelPricing(model)
	require.NoError(t, err)
	d := inflightEstimateDeps{cfg: cfg, billing: billing}
	req := InflightEstimateRequest{Model: model, BodyBytes: 4000, MaxTokens: 100}
	full := d.estimateOne(context.Background(), key, model, req, 2, 2)
	require.InDelta(t, (1000*p.InputPricePerToken+100*p.OutputPricePerToken)*2, full, 1e-12)
	costGuardObserveUsage(key, &UsageLog{RequestedModel: model, Model: "upstream-alias", InputTokens: 20, CacheReadTokens: 80})
	discounted := d.estimateOne(context.Background(), key, model, req, 2, 2)
	require.InDelta(t, (1000*(p.InputPricePerToken*0.2+p.CacheReadPricePerToken*0.8)+100*p.OutputPricePerToken)*2, discounted, 1e-12)
	other := *key
	other.ID++
	require.Equal(t, full, d.estimateOne(context.Background(), &other, model, req, 2, 2))
	costGuardObserveUsage(key, &UsageLog{RequestedModel: model, InputTokens: 100})
	require.Equal(t, full, d.estimateOne(context.Background(), key, model, req, 2, 2))
	settings.costGuardEnabled.Store(false)
	input, output := costGuardTokenCounts(billing, cfg.Billing.InflightReservation, model, 4000, 100)
	require.Equal(t, 1000, input, "logging-only estimates use the same token rules")
	require.Equal(t, 100, output)
	require.False(t, costGuardRejects(0.01, 5))
}

func TestCostGuardOutputLimitAndFailOpen(t *testing.T) {
	withCostGuard(t, true)
	ps := &PricingService{pricingData: map[string]*LiteLLMModelPricing{"known": {MaxOutputTokens: 10000, InputCostPerToken: 1, OutputCostPerToken: 1}}}
	billing := &BillingService{pricingService: ps}
	cfg := config.InflightReservationConfig{DefaultMaxTokens: 8000}
	for _, tc := range []struct {
		model           string
		requested, want int
	}{{"known", 0, 10000}, {"known", 500, 500}, {"known", 20000, 10000}, {"unknown", 0, 8000}} {
		_, got := costGuardTokenCounts(billing, cfg, tc.model, 100, tc.requested)
		require.Equal(t, tc.want, got)
	}
	require.True(t, costGuardRejects(0.01, 5))
	require.False(t, costGuardRejects(5, 5))
	require.False(t, costGuardRejects(5, math.NaN()))
	require.False(t, costGuardRejects(math.NaN(), 5))
	require.False(t, costGuardRejects(5, math.Inf(1)))
	require.NotPanics(t, func() { costGuardObserveUsage(nil, nil); costGuardInputPrice(nil, nil, "") })
}

func TestCostGuardPricingOutputMetadata(t *testing.T) {
	ps := &PricingService{}
	data, err := ps.parsePricingData([]byte(`{"a":{"input_cost_per_token":1,"max_output_tokens":10000},"b":{"input_cost_per_token":1,"max_tokens":20000}}`))
	require.NoError(t, err)
	require.Equal(t, 10000, data["a"].MaxOutputTokens)
	require.Equal(t, 20000, data["b"].MaxOutputTokens)
}

func costGuardGPT61Billing() *BillingService {
	return NewBillingService(&config.Config{}, &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"gpt-6.1-sol": {
			InputCostPerToken: 2e-6, OutputCostPerToken: 10e-6,
			CacheReadInputTokenCost: 0.1e-6, CacheCreationInputTokenCost: 2.5e-6,
			MaxOutputTokens: 128000, LongContextInputTokenThreshold: 272000,
			LongContextInputCostMultiplier: 2, LongContextOutputCostMultiplier: 1.5,
		},
	}})
}

func TestCostGuardLongContextMatchesBilling(t *testing.T) {
	withCostGuard(t, true)
	billing := costGuardGPT61Billing()
	resolver := NewModelPricingResolver(nil, billing)
	key := &APIKey{ID: 3, Group: &Group{LongContextPricingEnabled: true}}
	d := inflightEstimateDeps{billing: billing, resolver: resolver}
	for _, tc := range []struct {
		name    string
		input   int
		enabled bool
		want    float64
	}{
		{"100k", 100000, true, 0.05772},
		{"at threshold", 272000, true, (272000*2e-6 + 128000*10e-6) * 0.039},
		{"above threshold", 272001, true, (272001*4e-6 + 128000*15e-6) * 0.039},
		{"400k", 400000, true, 0.13728},
		{"group disabled", 400000, false, 0.08112},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key.Group.LongContextPricingEnabled = tc.enabled
			req := InflightEstimateRequest{Model: "gpt-6.1-sol", BodyBytes: tc.input * 4}
			got := d.estimateOne(context.Background(), key, req.Model, req, 0.039, 0.039)
			require.InDelta(t, tc.want, got, 1e-12)
			actual, err := billing.CalculateCostUnified(CostInput{
				Model: req.Model, Group: key.Group, Resolver: resolver, RateMultiplier: 0.039,
				Tokens: UsageTokens{InputTokens: tc.input, OutputTokens: 128000},
			})
			require.NoError(t, err)
			require.InDelta(t, actual.ActualCost, got, 1e-12)
		})
	}
	key.Group.LongContextPricingEnabled = true
	costGuardObserveUsage(key, &UsageLog{RequestedModel: "gpt-6.1-sol", InputTokens: 20, CacheReadTokens: 70, CacheCreationTokens: 10})
	req := InflightEstimateRequest{Model: "gpt-6.1-sol", BodyBytes: 400000 * 4}
	got := d.estimateOne(context.Background(), key, req.Model, req, 0.039, 0.039)
	want := (400000*(0.2*4e-6+0.7*0.2e-6+0.1*5e-6) + 128000*15e-6) * 0.039
	require.InDelta(t, want, got, 1e-12, "all input cache prices use the long-context multiplier")
	p, err := billing.GetModelPricing(req.Model)
	require.NoError(t, err)
	require.Equal(t, 2e-6, p.InputPricePerToken, "shared prices must remain unchanged")
	require.Equal(t, 10e-6, p.OutputPricePerToken)
}

func TestCostGuardCustomIntervalsAreNotMultipliedTwice(t *testing.T) {
	withCostGuard(t, true)
	billing := costGuardGPT61Billing()
	p, err := billing.GetModelPricing("gpt-6.1-sol")
	require.NoError(t, err)
	inputPrice, outputPrice := 6e-6, 20e-6
	resolved := &ResolvedPricing{BasePricing: p, longContextPricingEnabled: true, Intervals: []PricingInterval{
		{MinTokens: 272000, InputPrice: &inputPrice, OutputPrice: &outputPrice},
	}}
	resolver := NewModelPricingResolver(nil, billing)
	interval := resolver.GetIntervalPricing(resolved, 400000)
	got := costGuardLongContextPricing(billing, resolved, interval, nil, 400000)
	require.Equal(t, 6e-6, got.InputPricePerToken)
	require.Equal(t, 20e-6, got.OutputPricePerToken)
}
