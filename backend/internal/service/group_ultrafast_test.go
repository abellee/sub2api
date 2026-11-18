//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGroupUltrafastHTTPAndWSPolicy(t *testing.T) {
	for _, action := range []string{BetaPolicyActionPass, BetaPolicyActionFilter, BetaPolicyActionBlock, OpenAIFastPolicyActionForcePriority} {
		t.Run(action, func(t *testing.T) {
			svc := newOpenAIGatewayServiceWithSettings(t, &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{ServiceTier: OpenAIFastTierUltrafast, Action: action, Scope: BetaPolicyScopeAll}}})
			group := &Group{ID: 7, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, ForceOpenAIUltrafast: true}
			ctx := context.WithValue(context.Background(), ctxkey.Group, group)
			account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
			for _, body := range []string{`{"model":"gpt-6-astra"}`, `{"model":"gpt-6-astra","service_tier":"priority"}`, `{"model":"gpt-6-astra","service_tier":"flex"}`} {
				updated, err := svc.applyOpenAIFastPolicyToBody(ctx, account, "gpt-6-astra", []byte(body))
				if action == BetaPolicyActionBlock {
					require.ErrorAs(t, err, new(*OpenAIFastBlockedError))
				} else {
					require.NoError(t, err)
					want := OpenAIFastTierUltrafast
					if action == BetaPolicyActionFilter {
						want = ""
					}
					if action == OpenAIFastPolicyActionForcePriority {
						want = OpenAIFastTierPriority
					}
					require.Equal(t, want, gjson.GetBytes(updated, "service_tier").String())
				}
			}
			frame := []byte(`{"type":"response.create","model":"gpt-6-astra"}`)
			updated, blocked, err := svc.applyOpenAIFastPolicyToWSResponseCreate(ctx, account, "gpt-6-astra", frame)
			require.NoError(t, err)
			if action == BetaPolicyActionBlock {
				require.NotNil(t, blocked)
			} else {
				require.Nil(t, blocked)
				want := OpenAIFastTierUltrafast
				if action == BetaPolicyActionFilter {
					want = ""
				}
				if action == OpenAIFastPolicyActionForcePriority {
					want = OpenAIFastTierPriority
				}
				require.Equal(t, want, gjson.GetBytes(updated, "service_tier").String())
			}
			cancel := []byte(`{"type":"response.cancel"}`)
			updated, blocked, err = svc.applyOpenAIFastPolicyToWSResponseCreate(ctx, account, "gpt-6-astra", cancel)
			require.NoError(t, err)
			require.Nil(t, blocked)
			require.Equal(t, cancel, updated)
		})
	}
}

func TestGroupUltrafastRequiresTrustedOpenAIContext(t *testing.T) {
	group := &Group{ID: 7, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, ForceOpenAIUltrafast: true}
	ctx := context.WithValue(context.Background(), ctxkey.Group, group)
	require.Empty(t, openAIGroupForcedServiceTier(ctx, &Account{Platform: PlatformGrok}, "gpt-6-astra"))
	require.Empty(t, openAIGroupForcedServiceTier(context.Background(), &Account{Platform: PlatformOpenAI}, "gpt-6-astra"))
	group.Hydrated = false
	require.Empty(t, openAIGroupForcedServiceTier(ctx, &Account{Platform: PlatformOpenAI}, "gpt-6-astra"))
}

func TestGroupUltrafastTokenBillingSixTimesStandard(t *testing.T) {
	for _, model := range []string{"gpt-6-astra", "gpt-6.1-sol"} {
		for _, customPricing := range []bool{false, true} {
			svc := &OpenAIGatewayService{billingService: newTestBillingService()}
			key := &APIKey{Group: &Group{ID: 7, Platform: PlatformOpenAI, ForceOpenAIUltrafast: true, FreeOpenAIFast: true}}
			if customPricing {
				inputPrice, outputPrice, cachePrice, fastMultiplier := 0.001, 0.002, 0.0001, 0.0
				key.Group.ModelPricing = []ChannelModelPricing{{Models: []string{model}, BillingMode: BillingModeToken, InputPrice: &inputPrice, OutputPrice: &outputPrice, CacheWritePrice: &cachePrice, CacheReadPrice: &cachePrice, FastMultiplier: &fastMultiplier}}
				svc.resolver = NewModelPricingResolver(nil, svc.billingService)
			}
			tokens := UsageTokens{InputTokens: 300000, OutputTokens: 500, CacheReadTokens: 2000, CacheCreationTokens: 1000, CacheCreation12hTokens: 100}
			standard, err := svc.calculateOpenAIRecordUsageTokenCost(context.Background(), key, model, 0.5, time.Now(), tokens, "", "", nil, false)
			require.NoError(t, err)
			ultrafast, err := svc.calculateOpenAIRecordUsageTokenCost(context.Background(), key, model, 0.5, time.Now(), tokens, OpenAIFastTierUltrafast, "", nil, false)
			require.NoError(t, err)
			require.InDelta(t, standard.TotalCost*6, ultrafast.TotalCost, 1e-10)
			require.InDelta(t, ultrafast.TotalCost*0.5, ultrafast.ActualCost, 1e-10)
			require.InDelta(t, standard.InputCost*6, ultrafast.InputCost, 1e-10)
			require.InDelta(t, standard.OutputCost*6, ultrafast.OutputCost, 1e-10)
			require.InDelta(t, standard.CacheReadCost*6, ultrafast.CacheReadCost, 1e-10)
			require.InDelta(t, standard.CacheCreationCost*6, ultrafast.CacheCreationCost, 1e-10)
			require.False(t, groupBillsOpenAIFastAtStandard(key, &Account{Platform: PlatformOpenAI}, OpenAIFastTierUltrafast))
		}
	}
}

func TestGroupUltrafastRecordUsageIgnoresFreeFast(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.resolver = NewModelPricingResolver(nil, svc.billingService)
	groupID := int64(77)
	inputPrice, outputPrice := 0.001, 0.002
	key := &APIKey{ID: 7, GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, RateMultiplier: 0.5, ForceOpenAIUltrafast: true, FreeOpenAIFast: true,
		ModelPricing: []ChannelModelPricing{{Models: []string{"gpt-6.1-sol"}, BillingMode: BillingModeToken, InputPrice: &inputPrice, OutputPrice: &outputPrice}}}}
	tier := OpenAIFastTierUltrafast
	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{RequestID: "ultrafast-test", Model: "gpt-6.1-sol", ServiceTier: &tier, Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 50}},
		APIKey: key, User: &User{ID: 2}, Account: &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
	})
	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	require.InDelta(t, (100*inputPrice+50*outputPrice)*6, usageRepo.lastLog.TotalCost, 1e-10)
	require.InDelta(t, usageRepo.lastLog.TotalCost*0.5, usageRepo.lastLog.ActualCost, 1e-10)
}

func TestGroupUltrafastHonorsChannelMultiplier(t *testing.T) {
	const groupID int64 = 133
	const rate = 0.13
	price, longPrice, flat := 8e-6, 16e-6, 1.0
	cache := newEmptyChannelCache()
	cache.channelByGroupID[groupID] = &Channel{ID: 32, Status: StatusActive}
	cache.groupPlatform[groupID] = PlatformOpenAI
	cache.loadedAt = time.Now()
	channelService := &ChannelService{}
	channelService.cache.Store(cache)
	svc := &OpenAIGatewayService{billingService: newTestBillingService()}
	svc.resolver = NewModelPricingResolver(channelService, svc.billingService)
	key := &APIKey{Group: &Group{ID: groupID, Platform: PlatformOpenAI, ForceOpenAIUltrafast: true, LongContextPricingEnabled: true, RateMultiplier: rate}}
	at := time.Date(2027, 3, 1, 8, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		model      string
		multiplier float64
		interval   PricingInterval
	}{
		{"gpt-6.1-sol", 6, PricingInterval{InputMultiplier: &flat}},
		{"gpt-6-astra", 6, PricingInterval{InputMultiplier: &flat}},
		{"gpt-6.2-sol", 3, PricingInterval{MinTokens: 272000, InputPrice: &longPrice}},
		{"gpt-6.2-astra", 3, PricingInterval{MinTokens: 272000, InputPrice: &longPrice}},
	} {
		cache.pricingByGroupModel[channelModelKey{groupID: groupID, platform: PlatformOpenAI, model: tc.model}] = &ChannelModelPricing{
			BillingMode: BillingModeToken, InputPrice: &price, UltrafastMultiplier: &tc.multiplier, Intervals: []PricingInterval{tc.interval},
		}
		t.Run(tc.model, func(t *testing.T) {
			for _, inputTokens := range []int{272000, 272001} {
				tokens := UsageTokens{InputTokens: inputTokens}
				cost, err := svc.calculateOpenAIRecordUsageTokenCost(context.Background(), key, tc.model, rate, at, tokens, OpenAIFastTierUltrafast, "", nil, true)
				require.NoError(t, err)
				wantPrice := price
				if inputTokens > 272000 && tc.multiplier == 3 {
					wantPrice = longPrice
				}
				want := float64(inputTokens) * wantPrice * tc.multiplier
				require.InDelta(t, want, cost.TotalCost, 1e-12)
				require.InDelta(t, want*rate, cost.ActualCost, 1e-12)
			}
		})
	}
}

func TestGroupUltrafastLeavesNonTokenFeesUnchanged(t *testing.T) {
	svc := &OpenAIGatewayService{billingService: newTestBillingService()}
	svc.resolver = NewModelPricingResolver(nil, svc.billingService)
	price, searchPrice := 0.25, 10.0
	key := &APIKey{Group: &Group{ID: 7, Platform: PlatformOpenAI, ForceOpenAIUltrafast: true,
		SearchPricePer1k: &searchPrice,
		ModelPricing:     []ChannelModelPricing{{Models: []string{"custom"}, BillingMode: BillingModePerRequest, PerRequestPrice: &price}}}}
	cost, err := svc.calculateOpenAIRecordUsageTokenCost(context.Background(), key, "custom", 0.5, time.Now(), UsageTokens{}, OpenAIFastTierUltrafast, "", nil, false)
	require.NoError(t, err)
	require.InDelta(t, price, cost.TotalCost, 1e-12)
	require.InDelta(t, price*0.5, cost.ActualCost, 1e-12)

	key.Group.ModelPricing = nil
	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 500}
	standard, err := svc.calculateOpenAIRecordUsageTokenCost(context.Background(), key, "gpt-6.1-sol", 0.5, time.Now(), tokens, "", "", nil, false)
	require.NoError(t, err)
	cost, err = svc.calculateOpenAIRecordUsageCost(context.Background(), &OpenAIForwardResult{SearchCount: 2}, key, []string{"gpt-6.1-sol"}, 0.5, 0.5, 0.5, 0.5, tokens, OpenAIFastTierUltrafast, nil, false, time.Now())
	require.NoError(t, err)
	wantTotal := standard.TotalCost*6 + 2*searchPrice/1000
	require.InDelta(t, wantTotal, cost.TotalCost, 1e-12)
	require.InDelta(t, wantTotal*0.5, cost.ActualCost, 1e-12)
}
