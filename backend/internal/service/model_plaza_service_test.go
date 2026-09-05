//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

// newPlazaService 构造 ListGroups 测试用的 ModelPlazaService（不接计费服务：展示定价原样透传）。
func newPlazaService(channels []Channel, groups []Group, pricing *PricingService) *ModelPlazaService {
	repo := &mockChannelRepository{
		listAllFn: func(ctx context.Context) ([]Channel, error) { return channels, nil },
	}
	return NewModelPlazaService(repo, &stubGroupRepoForAvailable{activeGroups: groups}, pricing, nil, nil)
}

func plazaPricedChannel(id int64, name string, groupIDs []int64, platform string, models ...string) Channel {
	return Channel{
		ID:       id,
		Name:     name,
		Status:   StatusActive,
		GroupIDs: groupIDs,
		ModelPricing: []ChannelModelPricing{{
			Platform:    platform,
			Models:      models,
			BillingMode: BillingModeToken,
			InputPrice:  testPtrFloat64(3e-6),
			OutputPrice: testPtrFloat64(1.5e-5),
		}},
	}
}

func TestListPlazaGroups_GroupCentricAggregation(t *testing.T) {
	// 两个渠道挂同一分组:模型并入同一 PlazaGroup;无模型的分组不返回。
	channels := []Channel{
		plazaPricedChannel(1, "chA", []int64{10}, "anthropic", "claude-sonnet"),
		plazaPricedChannel(2, "chB", []int64{10}, "anthropic", "claude-opus"),
	}
	groups := []Group{
		{ID: 10, Name: "g-main", Description: "desc", Platform: "anthropic", RateMultiplier: 1},
		{ID: 20, Name: "g-empty", Platform: "anthropic", RateMultiplier: 0.5},
	}
	svc := newPlazaService(channels, groups, nil)
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1, "无模型的分组不应返回")
	require.Equal(t, int64(10), out[0].ID)
	require.Equal(t, "desc", out[0].Description)
	require.Len(t, out[0].Models, 2)
	// 组内模型按名称排序
	require.Equal(t, "claude-opus", out[0].Models[0].Name)
	require.Equal(t, "claude-sonnet", out[0].Models[1].Name)
}

func TestListConfiguredPlazaGroups_UsesGroupModelListWithoutChannelIntersection(t *testing.T) {
	pricingSvc := newStubPricingServiceFromMap(map[string]*LiteLLMModelPricing{
		"gemini-2.5-flash": {
			Mode:               "chat",
			InputCostPerToken:  3e-7,
			OutputCostPerToken: 2.5e-6,
		},
		"gemini-3.5-flash": {
			Mode:               "chat",
			InputCostPerToken:  1.5e-6,
			OutputCostPerToken: 9e-6,
		},
		"gemini-3.7-flash": {
			Mode:               "chat",
			InputCostPerToken:  0.75e-6,
			OutputCostPerToken: 3.75e-6,
		},
	})
	groups := []Group{
		{
			ID: 12, Name: "Gemini", Platform: PlatformGemini, RateMultiplier: 0.15,
			ModelsListConfig: GroupModelsListConfig{
				Enabled: false,
				Models: []string{
					"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.5-flash",
					"gemini-3.5-flash-extra-low", "gemini-3.5-flash-low", "gemini-3.7-flash-high",
					"unknown-gemini",
				},
			},
		},
		{ID: 13, Name: "empty", Platform: PlatformOpenAI, RateMultiplier: 1},
	}

	svc := newPlazaService(nil, groups, pricingSvc)
	svc.billingService = NewBillingService(&config.Config{}, pricingSvc)
	svc.resolver = NewModelPricingResolver(nil, svc.billingService)
	out, err := svc.ListConfiguredGroups(context.Background())

	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "Gemini", out[0].Name)
	require.Equal(t, []string{
		"gemini-2.5-flash", "gemini-2.5-pro", "gemini-3.5-flash-extra-low",
		"gemini-3.5-flash-low", "gemini-3.7-flash-high", "unknown-gemini",
	}, []string{
		out[0].Models[0].Name, out[0].Models[1].Name, out[0].Models[2].Name,
		out[0].Models[3].Name, out[0].Models[4].Name, out[0].Models[5].Name,
	})
	require.NotNil(t, out[0].Models[0].OfficialPricing)
	require.NotNil(t, out[0].Models[2].OfficialPricing)
	require.NotNil(t, out[0].Models[3].OfficialPricing)
	require.NotNil(t, out[0].Models[4].OfficialPricing)
	require.InDelta(t, 1.5e-6, *out[0].Models[2].OfficialPricing.InputPrice, 1e-12)
	require.InDelta(t, 1.5e-6, *out[0].Models[3].OfficialPricing.InputPrice, 1e-12)
	require.InDelta(t, 0.75e-6, *out[0].Models[4].OfficialPricing.InputPrice, 1e-12)
	require.Nil(t, out[0].Models[5].OfficialPricing, "missing price must not remove a configured model")
}

func TestListPlazaGroups_DedupFirstWinsWithPricingUpgrade(t *testing.T) {
	// 同名模型:先见者胜;仅当已存条目无定价而新条目有定价时升级替换。
	unpriced := Channel{
		ID: 1, Name: "alpha", Status: StatusActive, GroupIDs: []int64{10},
		// mapping-only → SupportedModels 产出无定价条目
		ModelMapping: map[string]map[string]string{
			"anthropic": {"claude-sonnet": "claude-sonnet"},
		},
	}
	priced := plazaPricedChannel(2, "beta", []int64{10}, "anthropic", "claude-sonnet")
	groups := []Group{{ID: 10, Name: "g", Platform: "anthropic", RateMultiplier: 1}}

	// alpha(无价)按名称序先于 beta(有价):先见者无价,应被有价条目升级。
	svc := newPlazaService([]Channel{priced, unpriced}, groups, nil)
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Len(t, out[0].Models, 1)
	require.NotNil(t, out[0].Models[0].Pricing, "无价条目应被有价条目升级")
	require.NotNil(t, out[0].Models[0].Pricing.InputPrice)
}

func TestListPlazaGroups_PlatformIsolation(t *testing.T) {
	// 渠道同时有 anthropic/openai 定价,anthropic 分组只应看到 anthropic 模型。
	ch := Channel{
		ID: 1, Name: "multi", Status: StatusActive, GroupIDs: []int64{10, 20},
		ModelPricing: []ChannelModelPricing{
			{Platform: "anthropic", Models: []string{"claude-sonnet"}, InputPrice: testPtrFloat64(3e-6)},
			{Platform: "openai", Models: []string{"gpt-5"}, InputPrice: testPtrFloat64(2e-6)},
		},
	}
	groups := []Group{
		{ID: 10, Name: "g-claude", Platform: "anthropic", RateMultiplier: 1},
		{ID: 20, Name: "g-gpt", Platform: "openai", RateMultiplier: 1},
	}
	svc := newPlazaService([]Channel{ch}, groups, nil)
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 2)
	byName := map[string][]PlazaModel{}
	for _, g := range out {
		byName[g.Name] = g.Models
	}
	require.Len(t, byName["g-claude"], 1)
	require.Equal(t, "claude-sonnet", byName["g-claude"][0].Name)
	require.Len(t, byName["g-gpt"], 1)
	require.Equal(t, "gpt-5", byName["g-gpt"][0].Name)
}

func TestListPlazaGroups_CompositeIncludesConfiguredConcretePlatforms(t *testing.T) {
	anthropicPrice := 3e-6
	openAIPrice := 2e-6
	ch := Channel{
		ID: 1, Name: "multi", Status: StatusActive, GroupIDs: []int64{10},
		ModelPricing: []ChannelModelPricing{
			{Platform: PlatformAnthropic, Models: []string{"shared-model"}, InputPrice: &anthropicPrice},
			{Platform: PlatformOpenAI, Models: []string{"shared-model"}, InputPrice: &openAIPrice},
			{Platform: "", Models: []string{"empty-platform"}},
			{Platform: PlatformComposite, Models: []string{"nested-composite"}},
			{Platform: "unknown-platform", Models: []string{"unknown-platform"}},
		},
	}
	groups := []Group{{ID: 10, Name: "composite", Platform: PlatformComposite, RateMultiplier: 1}}

	out, err := newPlazaService([]Channel{ch}, groups, nil).ListGroups(context.Background())

	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Len(t, out[0].Models, 2, "only concrete platforms are included and same-named models remain distinct")
	require.Equal(t, PlatformAnthropic, out[0].Models[0].Platform)
	require.Equal(t, PlatformOpenAI, out[0].Models[1].Platform)
	require.InDelta(t, anthropicPrice, *out[0].Models[0].Pricing.InputPrice, 1e-12)
	require.InDelta(t, openAIPrice, *out[0].Models[1].Pricing.InputPrice, 1e-12)
}

func TestListPlazaGroups_CompositeAndOrdinaryGroupsDoNotLeakPlatforms(t *testing.T) {
	ch := Channel{
		ID: 1, Name: "multi", Status: StatusActive, GroupIDs: []int64{10, 20},
		ModelPricing: []ChannelModelPricing{
			{Platform: PlatformAnthropic, Models: []string{"claude-sonnet"}, InputPrice: testPtrFloat64(3e-6)},
			{Platform: PlatformOpenAI, Models: []string{"gpt-5"}, InputPrice: testPtrFloat64(2e-6)},
		},
	}
	groups := []Group{
		{ID: 10, Name: "anthropic-only", Platform: PlatformAnthropic, RateMultiplier: 1},
		{ID: 20, Name: "composite", Platform: PlatformComposite, RateMultiplier: 1},
	}

	out, err := newPlazaService([]Channel{ch}, groups, nil).ListGroups(context.Background())

	require.NoError(t, err)
	require.Len(t, out, 2)
	byName := map[string]PlazaGroup{}
	for _, group := range out {
		byName[group.Name] = group
	}
	require.Len(t, byName["anthropic-only"].Models, 1)
	require.Equal(t, []PlazaModel{{
		Name: "claude-sonnet", Platform: PlatformAnthropic, Pricing: byName["anthropic-only"].Models[0].Pricing,
	}}, byName["anthropic-only"].Models)
	require.Len(t, byName["composite"].Models, 2)
	require.Equal(t, []string{"claude-sonnet", "gpt-5"}, []string{
		byName["composite"].Models[0].Name,
		byName["composite"].Models[1].Name,
	})
	require.Equal(t, []string{PlatformAnthropic, PlatformOpenAI}, []string{
		byName["composite"].Models[0].Platform,
		byName["composite"].Models[1].Platform,
	})
}

func TestListPlazaGroups_InactiveChannelSkipped(t *testing.T) {
	inactive := plazaPricedChannel(1, "off", []int64{10}, "anthropic", "claude-sonnet")
	inactive.Status = "inactive"
	groups := []Group{{ID: 10, Name: "g", Platform: "anthropic", RateMultiplier: 1}}
	svc := newPlazaService([]Channel{inactive}, groups, nil)
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestListPlazaGroups_SortedByRateMultiplierAsc(t *testing.T) {
	channels := []Channel{
		plazaPricedChannel(1, "ch", []int64{10, 20, 30}, "anthropic", "claude-sonnet"),
	}
	groups := []Group{
		{ID: 10, Name: "b-standard", Platform: "anthropic", RateMultiplier: 1},
		{ID: 20, Name: "a-standard", Platform: "anthropic", RateMultiplier: 1},
		{ID: 30, Name: "cheap", Platform: "anthropic", RateMultiplier: 0.5},
	}
	svc := newPlazaService(channels, groups, nil)
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 3)
	require.Equal(t, "cheap", out[0].Name, "倍率低者在前")
	require.Equal(t, "a-standard", out[1].Name, "同倍率按名称")
	require.Equal(t, "b-standard", out[2].Name)
}

func TestListPlazaGroups_OfficialPricingFill(t *testing.T) {
	pricingSvc := newStubPricingServiceFromMap(map[string]*LiteLLMModelPricing{
		"claude-sonnet": {
			Mode:                                "chat",
			InputCostPerToken:                   3e-6,
			OutputCostPerToken:                  1.5e-5,
			CacheCreationInputTokenCost:         3.75e-6,
			CacheCreationInputTokenCostAbove1hr: 6e-6,
			CacheReadInputTokenCost:             3e-7,
		},
		"token-absent": {Mode: "image_generation", TokenPricingAbsent: true, OutputCostPerImage: 0.04},
	})
	channels := []Channel{
		plazaPricedChannel(1, "ch", []int64{10}, "anthropic", "claude-sonnet", "unknown-model", "token-absent"),
	}
	groups := []Group{{ID: 10, Name: "g", Platform: "anthropic", RateMultiplier: 1}}
	svc := newPlazaService(channels, groups, pricingSvc)
	// 官方价与计费同源：需要计费服务与解析器（官方参考不查渠道，解析器无需渠道服务）。
	svc.billingService = NewBillingService(&config.Config{}, pricingSvc)
	svc.resolver = NewModelPricingResolver(nil, svc.billingService)
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Len(t, out[0].Models, 3)

	byName := map[string]PlazaModel{}
	for _, m := range out[0].Models {
		byName[m.Name] = m
	}
	// 命中:填充完整官方价(含 1h 缓存写入)
	official := byName["claude-sonnet"].OfficialPricing
	require.NotNil(t, official)
	require.InDelta(t, 3e-6, *official.InputPrice, 1e-12)
	require.InDelta(t, 6e-6, *official.CacheWrite1hPrice, 1e-12)
	require.InDelta(t, 3e-7, *official.CacheReadPrice, 1e-12)
	// 未命中:nil(GetModelPricing 的 claude 系列模糊匹配对非 claude 名不生效)
	require.Nil(t, byName["unknown-model"].OfficialPricing)
	// TokenPricingAbsent 条目不作为官方 token 价展示
	require.Nil(t, byName["token-absent"].OfficialPricing)
}

func TestListPlazaGroups_GroupImagePriceOverridesChannelPricing(t *testing.T) {
	// 图片计费模型:档位价按实收口径合成(分组图片价 > 渠道档位价 > 渠道默认按次价),
	// 分组独立倍率字段透传;未配图片价的分组保持渠道定价原样。
	perReq := 0.2
	tier4K := 0.3
	imgPrice := 0.02
	channels := []Channel{{
		ID: 1, Name: "img-ch", Status: StatusActive, GroupIDs: []int64{10, 20},
		ModelPricing: []ChannelModelPricing{{
			Platform:        "openai",
			Models:          []string{"gpt-image-2"},
			BillingMode:     BillingModeImage,
			PerRequestPrice: &perReq,
			Intervals:       []PricingInterval{{TierLabel: "4K", PerRequestPrice: &tier4K}},
		}},
	}}
	groups := []Group{
		{ID: 10, Name: "g-media", Platform: "openai", RateMultiplier: 1,
			ImagePrice1K: &imgPrice, ImageRateIndependent: true, ImageRateMultiplier: 1},
		{ID: 20, Name: "g-plain", Platform: "openai", RateMultiplier: 0.1},
	}
	svc := newPlazaService(channels, groups, nil)
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 2)
	byName := map[string]PlazaGroup{}
	for _, g := range out {
		byName[g.Name] = g
	}

	media := byName["g-media"]
	require.True(t, media.ImageRateIndependent)
	require.InDelta(t, 1.0, media.ImageRateMultiplier, 1e-9)
	require.Len(t, media.Models, 1)
	p := media.Models[0].Pricing
	require.NotNil(t, p)
	require.Len(t, p.Intervals, 3)
	tierPrices := map[string]float64{}
	for _, iv := range p.Intervals {
		require.NotNil(t, iv.PerRequestPrice)
		tierPrices[iv.TierLabel] = *iv.PerRequestPrice
	}
	require.InDelta(t, 0.02, tierPrices["1K"], 1e-9, "1K 用分组图片价")
	require.InDelta(t, 0.2, tierPrices["2K"], 1e-9, "2K 分组未配,回落渠道默认按次价")
	require.InDelta(t, 0.3, tierPrices["4K"], 1e-9, "4K 分组未配,回落渠道档位价")

	plain := byName["g-plain"]
	require.False(t, plain.ImageRateIndependent)
	require.Len(t, plain.Models, 1)
	pp := plain.Models[0].Pricing
	require.NotNil(t, pp)
	require.Len(t, pp.Intervals, 1, "未配分组图片价:渠道定价原样")
	require.InDelta(t, 0.2, *pp.PerRequestPrice, 1e-9)

	// 合成为克隆,渠道原始定价不被修改
	require.Len(t, channels[0].ModelPricing[0].Intervals, 1)
}

func TestListPlazaGroups_GroupImagePriceIgnoredForNonImageModes(t *testing.T) {
	// token 模式定价不受分组图片价影响。
	imgPrice := 0.02
	channels := []Channel{plazaPricedChannel(1, "ch", []int64{10}, "openai", "gpt-5")}
	groups := []Group{{ID: 10, Name: "g", Platform: "openai", RateMultiplier: 1, ImagePrice1K: &imgPrice}}
	svc := newPlazaService(channels, groups, nil)
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	p := out[0].Models[0].Pricing
	require.NotNil(t, p)
	require.Empty(t, p.Intervals)
	require.NotNil(t, p.InputPrice)
	require.Nil(t, p.PerRequestPrice)
}

func TestListPlazaGroups_RepoErrorsPropagate(t *testing.T) {
	sentinel := errors.New("boom")
	repo := &mockChannelRepository{
		listAllFn: func(ctx context.Context) ([]Channel, error) { return nil, sentinel },
	}
	svc := NewModelPlazaService(repo, &stubGroupRepoForAvailable{}, nil, nil, nil)
	out, err := svc.ListGroups(context.Background())
	require.Nil(t, out)
	require.ErrorIs(t, err, sentinel)

	svc2 := NewModelPlazaService(
		&mockChannelRepository{listAllFn: func(ctx context.Context) ([]Channel, error) { return nil, nil }},
		&stubGroupRepoForAvailable{listActiveErr: sentinel},
		nil, nil, nil,
	)
	out2, err2 := svc2.ListGroups(context.Background())
	require.Nil(t, out2)
	require.ErrorIs(t, err2, sentinel)
}

// newPlazaServiceWithBilling 构造接入计费服务与解析器的广场服务：解析器的渠道服务与广场共用同一份渠道数据。
func newPlazaServiceWithBilling(channels []Channel, groups []Group, groupPlatforms map[int64]string, catalog *PricingService) *ModelPlazaService {
	repo := &mockChannelRepository{
		listAllFn: func(ctx context.Context) ([]Channel, error) { return channels, nil },
		getGroupPlatformsFn: func(ctx context.Context, _ []int64) (map[int64]string, error) {
			return groupPlatforms, nil
		},
	}
	cs := NewChannelService(repo, nil, nil, nil)
	bs := NewBillingService(&config.Config{}, catalog)
	return NewModelPlazaService(repo, &stubGroupRepoForAvailable{activeGroups: groups}, catalog, bs, NewModelPricingResolver(cs, bs))
}

func plazaModelsByName(models []PlazaModel) map[string]PlazaModel {
	out := make(map[string]PlazaModel, len(models))
	for _, m := range models {
		out[m.Name] = m
	}
	return out
}

func TestListGroups_TokenLadderFollowsGroupToggle(t *testing.T) {
	// 同一渠道挂开启/关闭阶梯的两个分组：实付档位随分组开关，官方阶梯不受影响。
	channels := []Channel{{
		ID: 1, Name: "ch", Status: StatusActive, GroupIDs: []int64{10, 20},
		ModelPricing: []ChannelModelPricing{{
			Platform: PlatformOpenAI, Models: []string{"gpt-5.4"}, BillingMode: BillingModeToken,
			Intervals: []PricingInterval{
				{MinTokens: 0, MaxTokens: testPtrInt(272000), InputPrice: testPtrFloat64(2.5e-6), OutputPrice: testPtrFloat64(10e-6), CacheWritePrice: testPtrFloat64(2.5e-6), CacheReadPrice: testPtrFloat64(0.3e-6)},
				{MinTokens: 272000, MaxTokens: nil, InputPrice: testPtrFloat64(5e-6), OutputPrice: testPtrFloat64(22.5e-6), CacheWritePrice: testPtrFloat64(5e-6), CacheReadPrice: testPtrFloat64(0.5e-6)},
			},
		}},
	}}
	groups := []Group{
		{ID: 10, Name: "on", Platform: PlatformOpenAI, RateMultiplier: 1, LongContextPricingEnabled: true},
		{ID: 20, Name: "off", Platform: PlatformOpenAI, RateMultiplier: 2, LongContextPricingEnabled: false},
	}
	svc := newPlazaServiceWithBilling(channels, groups, map[int64]string{10: PlatformOpenAI, 20: PlatformOpenAI},
		newStubPricingServiceFromJSON(t, openAILadderCatalogJSON))
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 2)

	on, off := out[0], out[1]
	require.True(t, on.LongContextPricingEnabled)
	require.False(t, off.LongContextPricingEnabled)

	onModel := on.Models[0]
	require.Equal(t, ContextPricingBasisWholeRequest, onModel.LongContextBasis)
	require.True(t, onModel.HasChannelContextPricing)
	// The base tier is represented by Pricing's scalar fields; only additional
	// channel-defined context tiers are emitted in Intervals.
	require.Len(t, onModel.Pricing.Intervals, 1)
	require.Equal(t, ">272K", onModel.Pricing.Intervals[0].TierLabel)
	require.InDelta(t, 2.5e-6, *onModel.Pricing.InputPrice, 1e-15)
	require.InDelta(t, 5e-6, *onModel.Pricing.Intervals[0].InputPrice, 1e-15)
	require.InDelta(t, 22.5e-6, *onModel.Pricing.Intervals[0].OutputPrice, 1e-15)
	require.InDelta(t, 5e-6, *onModel.Pricing.Intervals[0].CacheWritePrice, 1e-15)
	require.InDelta(t, 0.5e-6, *onModel.Pricing.Intervals[0].CacheReadPrice, 1e-15)

	offModel := off.Models[0]
	require.False(t, offModel.HasChannelContextPricing)
	require.Empty(t, offModel.LongContextBasis)
	require.Empty(t, offModel.Pricing.Intervals)
	require.InDelta(t, 2.5e-6, *offModel.Pricing.InputPrice, 1e-15)

	for _, m := range []PlazaModel{onModel, offModel} {
		require.NotNil(t, m.OfficialPricing)
		require.Len(t, m.OfficialPricing.Intervals, 2, "官方阶梯不受分组开关影响")
		require.InDelta(t, 5e-6, *m.OfficialPricing.Intervals[1].InputPrice, 1e-15)
		require.InDelta(t, 2.5e-6, *m.OfficialPricing.InputPrice, 1e-15)
	}
}

func TestListGroups_CompositeUsesModelPlatformForChannelContextPricing(t *testing.T) {
	// Composite groups can contain the same model name on multiple concrete
	// platforms. The channel ownership lookup must use the model's platform,
	// otherwise the composite platform iteration may select a flat price from
	// another platform while the schedule probe correctly uses Grok intervals.
	channels := []Channel{{
		ID: 1, Name: "multi-platform", Status: StatusActive, GroupIDs: []int64{10},
		ModelPricing: []ChannelModelPricing{
			{
				Platform: PlatformOpenAI, Models: []string{"shared-model"}, BillingMode: BillingModeToken,
				InputPrice: testPtrFloat64(1e-6), OutputPrice: testPtrFloat64(4e-6),
			},
			{
				Platform: PlatformGrok, Models: []string{"shared-model"}, BillingMode: BillingModeToken,
				InputPrice: testPtrFloat64(2e-6), OutputPrice: testPtrFloat64(6e-6),
				Intervals: []PricingInterval{{
					MinTokens: 200000, InputPrice: testPtrFloat64(4e-6),
					OutputPrice: testPtrFloat64(12e-6), CacheReadPrice: testPtrFloat64(1e-6),
				}},
			},
		},
	}}
	groups := []Group{{
		ID: 10, Name: "composite", Platform: PlatformComposite, RateMultiplier: 1,
		LongContextPricingEnabled: true,
	}}
	svc := newPlazaServiceWithBilling(channels, groups, map[int64]string{10: PlatformComposite}, nil)
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)

	var grokModel *PlazaModel
	for i := range out[0].Models {
		if out[0].Models[i].Platform == PlatformGrok && out[0].Models[i].Name == "shared-model" {
			grokModel = &out[0].Models[i]
			break
		}
	}
	require.NotNil(t, grokModel)
	require.True(t, grokModel.HasChannelContextPricing)
	require.Len(t, grokModel.Pricing.Intervals, 1)
	require.InDelta(t, 1e-6, *grokModel.Pricing.Intervals[0].CacheReadPrice, 1e-15)
}

func TestListGroups_GeminiLegacyRuleShownAsMarginal(t *testing.T) {
	channels := []Channel{{
		ID: 1, Name: "ch", Status: StatusActive, GroupIDs: []int64{10},
		ModelMapping: map[string]map[string]string{PlatformGemini: {"gemini-2.5-pro": "gemini-2.5-pro"}},
	}}
	groups := []Group{{ID: 10, Name: "g", Platform: PlatformGemini, RateMultiplier: 1}}
	svc := newPlazaServiceWithBilling(channels, groups, map[int64]string{10: PlatformGemini}, geminiCatalogStub())
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	m := out[0].Models[0]
	require.Empty(t, m.Pricing.Intervals)
	// 官方参考不套用站内旧规则
	require.NotNil(t, m.OfficialPricing)
}

func TestListGroups_GroupTokenCardOverridesChannelPricing(t *testing.T) {
	channels := []Channel{plazaPricedChannel(1, "ch", []int64{10}, PlatformAnthropic, "claude-sonnet-4")}
	groups := []Group{{
		ID: 10, Name: "g", Platform: PlatformAnthropic, RateMultiplier: 1,
		ModelPricing: []ChannelModelPricing{{Models: []string{"claude-sonnet-*"}, BillingMode: BillingModeToken, InputPrice: testPtrFloat64(1e-6)}},
	}}
	svc := newPlazaServiceWithBilling(channels, groups, map[int64]string{10: PlatformAnthropic}, nil)
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	m := out[0].Models[0]
	require.InDelta(t, 1e-6, *m.Pricing.InputPrice, 1e-15, "分组价卡优先于渠道平价")
	require.InDelta(t, 15e-6, *m.Pricing.OutputPrice, 1e-15, "卡未配置的项回落目录价")
	require.Empty(t, m.Pricing.Intervals)
}

func TestListGroups_ImageModelKeepsTierSynthesisWithBilling(t *testing.T) {
	channels := []Channel{{
		ID: 1, Name: "ch", Status: StatusActive, GroupIDs: []int64{10},
		ModelPricing: []ChannelModelPricing{{
			Platform: PlatformOpenAI, Models: []string{"gpt-image-2"}, BillingMode: BillingModeImage,
			PerRequestPrice: testPtrFloat64(0.04),
		}},
	}}
	groups := []Group{{
		ID: 10, Name: "g", Platform: PlatformOpenAI, RateMultiplier: 1,
		ImagePrice1K: testPtrFloat64(0.02),
	}}
	svc := newPlazaServiceWithBilling(channels, groups, map[int64]string{10: PlatformOpenAI}, nil)
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	m := out[0].Models[0]
	require.Equal(t, BillingModeImage, m.Pricing.BillingMode)
	require.Len(t, m.Pricing.Intervals, 3)
	require.InDelta(t, 0.02, *m.Pricing.Intervals[0].PerRequestPrice, 1e-12)
	require.InDelta(t, 0.04, *m.Pricing.Intervals[1].PerRequestPrice, 1e-12)
}

func TestListGroups_CatalogMissingStillShowsChannelFlatPricing(t *testing.T) {
	// 目录查不到的模型：计费按渠道平价（未配置项 $0），广场单档展示渠道平价，官方价为空。
	channels := []Channel{plazaPricedChannel(1, "ch", []int64{10}, PlatformAnthropic, "unknown-model-xyz")}
	groups := []Group{{ID: 10, Name: "g", Platform: PlatformAnthropic, RateMultiplier: 1}}
	svc := newPlazaServiceWithBilling(channels, groups, map[int64]string{10: PlatformAnthropic}, nil)
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	m := out[0].Models[0]
	require.NotNil(t, m.Pricing)
	require.InDelta(t, 3e-6, *m.Pricing.InputPrice, 1e-15)
	require.Empty(t, m.Pricing.Intervals)
	require.Nil(t, m.Pricing.CacheWritePrice, "目录无价且渠道未配置 → 无价")
	require.Nil(t, m.OfficialPricing)
}

func TestListGroups_TimePricingPassthrough(t *testing.T) {
	channels := []Channel{{
		ID: 1, Name: "ch", Status: StatusActive, GroupIDs: []int64{10},
		ModelPricing: []ChannelModelPricing{{
			Platform: PlatformDeepseek, Models: []string{"deepseek-chat"}, BillingMode: BillingModeToken,
			InputPrice: testPtrFloat64(0.28e-6), OutputPrice: testPtrFloat64(0.42e-6),
			TimePricing: &ChannelTimePricing{Timezone: "Asia/Shanghai", Periods: []ChannelTimePricingPeriod{
				{StartTime: "00:30", EndTime: "08:30", Multiplier: 0.5},
			}},
		}},
	}}
	groups := []Group{{ID: 10, Name: "cn", Platform: PlatformDeepseek, RateMultiplier: 1}}
	svc := newPlazaServiceWithBilling(channels, groups, map[int64]string{10: PlatformDeepseek}, nil)
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	m := out[0].Models[0]
	require.NotNil(t, m.TimePricing)
	require.Equal(t, "Asia/Shanghai", m.TimePricing.Timezone)
	require.Len(t, m.TimePricing.Periods, 1)
	require.InDelta(t, 0.5, m.TimePricing.Periods[0].Multiplier, 1e-12)
	// 展示单价为标准时段价
	require.InDelta(t, 0.28e-6, *m.Pricing.InputPrice, 1e-15)
}

func grokHeavyPlazaGroup() Group {
	return Group{
		ID: 10, Name: "Grok Heavy", Platform: PlatformGrok, RateMultiplier: 0.2,
		LongContextPricingEnabled: true,
		VideoRateIndependent:      true,
		VideoRateMultiplier:       0.5,
		ModelsListConfig: GroupModelsListConfig{
			Models: []string{
				"grok-imagine-image",
				"grok-imagine-video",
				"grok-imagine-video-1.5",
				"grok-4",
			},
		},
	}
}

func grokHeavyChannel() Channel {
	return Channel{
		ID: 1, Name: "grok-ch", Status: StatusActive, GroupIDs: []int64{10},
		ModelPricing: []ChannelModelPricing{
			{
				Platform:    PlatformGrok,
				Models:      []string{"grok-imagine-image"},
				BillingMode: BillingModeImage,
				Intervals: []PricingInterval{
					{TierLabel: "1K", PerRequestPrice: testPtrFloat64(0.03)},
					{TierLabel: "2K", PerRequestPrice: testPtrFloat64(0.05)},
				},
			},
			{
				Platform:    PlatformGrok,
				Models:      []string{"grok-imagine-video"},
				BillingMode: BillingModeVideo,
				Intervals: []PricingInterval{
					{TierLabel: "480p", PerRequestPrice: testPtrFloat64(0.09)},
					{TierLabel: "720p", PerRequestPrice: testPtrFloat64(0.12)},
				},
			},
			{
				Platform:    PlatformGrok,
				Models:      []string{"grok-imagine-video-1.5"},
				BillingMode: BillingModeVideo,
				Intervals: []PricingInterval{
					{TierLabel: "480p", PerRequestPrice: testPtrFloat64(0.09)},
					{TierLabel: "720p", PerRequestPrice: testPtrFloat64(0.14)},
					{TierLabel: "1080p", PerRequestPrice: testPtrFloat64(0.25)},
				},
			},
			{
				Platform:    PlatformGrok,
				Models:      []string{"grok-4"},
				BillingMode: BillingModeToken,
				InputPrice:  testPtrFloat64(3e-6),
				OutputPrice: testPtrFloat64(1.5e-5),
				Intervals: []PricingInterval{{
					MinTokens:   128000,
					InputPrice:  testPtrFloat64(6e-6),
					OutputPrice: testPtrFloat64(3e-5),
				}},
			},
		},
	}
}

func plazaIntervalPrices(p *ChannelModelPricing) map[string]float64 {
	out := map[string]float64{}
	if p == nil {
		return out
	}
	for _, iv := range p.Intervals {
		if iv.PerRequestPrice != nil {
			out[iv.TierLabel] = *iv.PerRequestPrice
		}
	}
	return out
}

func TestListPlazaGroups_GrokHeavyChannelMediaPricing(t *testing.T) {
	// 混合品牌分组不在分组里定价：生图/生视频价格来自渠道价卡，只展示已配置档位。
	channels := []Channel{grokHeavyChannel()}
	groups := []Group{grokHeavyPlazaGroup()}
	svc := newPlazaService(channels, groups, nil)
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.True(t, out[0].VideoRateIndependent)
	require.InDelta(t, 0.5, out[0].VideoRateMultiplier, 1e-9)

	byName := plazaModelsByName(out[0].Models)
	image := byName["grok-imagine-image"]
	require.Equal(t, BillingModeImage, image.Pricing.BillingMode)
	require.Equal(t, map[string]float64{"1K": 0.03, "2K": 0.05}, plazaIntervalPrices(image.Pricing))
	_, has4K := plazaIntervalPrices(image.Pricing)["4K"]
	require.False(t, has4K, "未配置的 4K 档不应展示")

	video := byName["grok-imagine-video"]
	require.Equal(t, BillingModeVideo, video.Pricing.BillingMode)
	require.Equal(t, map[string]float64{"480p": 0.09, "720p": 0.12}, plazaIntervalPrices(video.Pricing))
	_, has1080 := plazaIntervalPrices(video.Pricing)["1080p"]
	require.False(t, has1080, "未配置的 1080p 档不应展示")

	video15 := byName["grok-imagine-video-1.5"]
	require.Equal(t, BillingModeVideo, video15.Pricing.BillingMode)
	require.Equal(t, map[string]float64{"480p": 0.09, "720p": 0.14, "1080p": 0.25}, plazaIntervalPrices(video15.Pricing))

	text := byName["grok-4"]
	require.Equal(t, BillingModeToken, text.Pricing.BillingMode)
	require.Len(t, text.Pricing.Intervals, 1, "未接计费服务时保留渠道长上下文档")
	require.Equal(t, 128000, text.Pricing.Intervals[0].MinTokens)
}

func TestListConfiguredPlazaGroups_GrokHeavyChannelMediaPricing(t *testing.T) {
	// 公共广场模型列表来自分组配置，价格仍应解析到关联渠道的 image/video 价卡。
	channels := []Channel{grokHeavyChannel()}
	groups := []Group{grokHeavyPlazaGroup()}
	svc := newPlazaServiceWithBilling(channels, groups, map[int64]string{10: PlatformGrok}, nil)
	out, err := svc.ListConfiguredGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	byName := plazaModelsByName(out[0].Models)

	image := byName["grok-imagine-image"]
	require.Equal(t, BillingModeImage, image.Pricing.BillingMode)
	require.Equal(t, map[string]float64{"1K": 0.03, "2K": 0.05}, plazaIntervalPrices(image.Pricing))

	video := byName["grok-imagine-video"]
	require.Equal(t, BillingModeVideo, video.Pricing.BillingMode)
	require.Equal(t, map[string]float64{"480p": 0.09, "720p": 0.12}, plazaIntervalPrices(video.Pricing))

	video15 := byName["grok-imagine-video-1.5"]
	require.Equal(t, map[string]float64{"480p": 0.09, "720p": 0.14, "1080p": 0.25}, plazaIntervalPrices(video15.Pricing))

	text := byName["grok-4"]
	require.Equal(t, BillingModeToken, text.Pricing.BillingMode)
	require.InDelta(t, 3e-6, *text.Pricing.InputPrice, 1e-15)
	require.Len(t, text.Pricing.Intervals, 1)
	require.Equal(t, ">128K", text.Pricing.Intervals[0].TierLabel)
	require.InDelta(t, 6e-6, *text.Pricing.Intervals[0].InputPrice, 1e-15)
	require.InDelta(t, 3e-5, *text.Pricing.Intervals[0].OutputPrice, 1e-15)
}

func TestListConfiguredPlazaGroups_GrokMediaEnabledWithoutPricesHidesDefaults(t *testing.T) {
	// 仅开启生图/生视频、渠道和分组都未配置档位价时，不展示计费默认价。
	groups := []Group{{
		ID: 10, Name: "Grok Heavy", Platform: PlatformGrok, RateMultiplier: 1,
		AllowImageGeneration: true,
		ModelsListConfig: GroupModelsListConfig{
			Models: []string{"grok-imagine-image", "grok-imagine-video"},
		},
	}}
	svc := newPlazaService(nil, groups, nil)
	out, err := svc.ListConfiguredGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	byName := plazaModelsByName(out[0].Models)
	require.Nil(t, byName["grok-imagine-image"].Pricing)
	require.Nil(t, byName["grok-imagine-video"].Pricing)
}

func TestListGroups_GrokHeavyChannelMediaPricingWithBilling(t *testing.T) {
	// 生产路径会先解析 token 阶梯；渠道 image/video 价卡必须覆盖 token 展示。
	channels := []Channel{grokHeavyChannel()}
	groups := []Group{grokHeavyPlazaGroup()}
	svc := newPlazaServiceWithBilling(channels, groups, map[int64]string{10: PlatformGrok}, nil)
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	byName := plazaModelsByName(out[0].Models)

	image := byName["grok-imagine-image"]
	require.Equal(t, BillingModeImage, image.Pricing.BillingMode)
	require.Equal(t, map[string]float64{"1K": 0.03, "2K": 0.05}, plazaIntervalPrices(image.Pricing))
	require.Nil(t, image.TimePricing)

	video := byName["grok-imagine-video"]
	require.Equal(t, BillingModeVideo, video.Pricing.BillingMode)
	require.Equal(t, map[string]float64{"480p": 0.09, "720p": 0.12}, plazaIntervalPrices(video.Pricing))

	video15 := byName["grok-imagine-video-1.5"]
	require.Equal(t, map[string]float64{"480p": 0.09, "720p": 0.14, "1080p": 0.25}, plazaIntervalPrices(video15.Pricing))

	text := byName["grok-4"]
	require.Equal(t, BillingModeToken, text.Pricing.BillingMode)
	require.InDelta(t, 3e-6, *text.Pricing.InputPrice, 1e-15)
	require.Len(t, text.Pricing.Intervals, 1)
	require.Equal(t, ">128K", text.Pricing.Intervals[0].TierLabel)
	require.InDelta(t, 6e-6, *text.Pricing.Intervals[0].InputPrice, 1e-15)
	require.InDelta(t, 3e-5, *text.Pricing.Intervals[0].OutputPrice, 1e-15)
}

func TestListConfiguredPlazaGroups_HidesCatalogLongContextWithoutChannelIntervals(t *testing.T) {
	// A catalog long-context ladder must not leak into the paid column when the
	// selected channel only defines a flat price. This applies to Claude and all
	// other token models, not only to Grok.
	channels := []Channel{plazaPricedChannel(1, "anthropic", []int64{10}, PlatformAnthropic, "claude-flat")}
	groups := []Group{{
		ID: 10, Name: "g", Platform: PlatformAnthropic, RateMultiplier: 1,
		LongContextPricingEnabled: true,
		ModelsListConfig:          GroupModelsListConfig{Models: []string{"claude-flat"}},
	}}
	catalog := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"claude-flat": {
			InputCostPerToken:               3e-6,
			OutputCostPerToken:              15e-6,
			LongContextInputTokenThreshold:  128000,
			LongContextInputCostMultiplier:  2,
			LongContextOutputCostMultiplier: 2,
		},
	}}
	svc := newPlazaServiceWithBilling(channels, groups, map[int64]string{10: PlatformAnthropic}, catalog)
	out, err := svc.ListConfiguredGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Empty(t, out[0].Models[0].Pricing.Intervals)
	require.False(t, out[0].Models[0].HasChannelContextPricing)
}

func TestListConfiguredPlazaGroups_GroupVideoPriceOverridesChannelTiers(t *testing.T) {
	// 分组如果额外配了视频档位，只覆盖已配置项，其余回落渠道价卡。
	channels := []Channel{grokHeavyChannel()}
	group := grokHeavyPlazaGroup()
	group.VideoPrice720P = testPtrFloat64(0.2)
	group.VideoModelPrices = map[string]map[string]float64{
		VideoPriceFamilyGrokImagineVideo15: {VideoBillingResolution1080P: 0.4},
	}
	svc := newPlazaServiceWithBilling(channels, []Group{group}, map[int64]string{10: PlatformGrok}, nil)
	out, err := svc.ListConfiguredGroups(context.Background())
	require.NoError(t, err)
	byName := plazaModelsByName(out[0].Models)

	video := byName["grok-imagine-video"]
	require.Equal(t, map[string]float64{"480p": 0.09, "720p": 0.2}, plazaIntervalPrices(video.Pricing))
	video15 := byName["grok-imagine-video-1.5"]
	require.Equal(t, map[string]float64{"480p": 0.09, "720p": 0.2, "1080p": 0.4}, plazaIntervalPrices(video15.Pricing))
}

func TestListConfiguredPlazaGroups_GroupMediaPricesDoNotExpandConfiguredModels(t *testing.T) {
	// 已配置的媒体价格只负责定价，不能让未在分组模型列表中的模型出现在广场。
	groups := []Group{{
		ID: 18, Name: "Grok Heavy", Platform: PlatformGrok, RateMultiplier: 0.11,
		ImagePrice1K:         testPtrFloat64(0.03),
		ImagePrice2K:         testPtrFloat64(0.05),
		VideoPrice480P:       testPtrFloat64(0.09),
		VideoPrice720P:       testPtrFloat64(0.12),
		VideoRateIndependent: true,
		VideoRateMultiplier:  0.5,
		VideoModelPrices: map[string]map[string]float64{
			VideoPriceFamilyGrokImagineVideo15: {VideoBillingResolution1080P: 0.25},
		},
		ModelsListConfig: GroupModelsListConfig{Models: []string{"grok-4.5", "grok-4.6"}},
	}}
	svc := newPlazaService(nil, groups, nil)
	out, err := svc.ListConfiguredGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.True(t, out[0].VideoRateIndependent)
	require.InDelta(t, 0.5, out[0].VideoRateMultiplier, 1e-9)

	byName := plazaModelsByName(out[0].Models)
	require.Len(t, byName, 2)
	require.Contains(t, byName, "grok-4.5")
	require.Contains(t, byName, "grok-4.6")
	require.NotContains(t, byName, "grok-imagine-image")
	require.NotContains(t, byName, "grok-imagine-video")
	require.NotContains(t, byName, "grok-imagine-video-1.5")
}

func TestListConfiguredPlazaGroups_NoMediaPricesDoesNotInjectGrokModels(t *testing.T) {
	groups := []Group{{
		ID: 18, Name: "Grok Heavy", Platform: PlatformGrok, RateMultiplier: 0.11,
		AllowImageGeneration: true,
		ModelsListConfig:     GroupModelsListConfig{Models: []string{"grok-4.5"}},
	}}
	svc := newPlazaService(nil, groups, nil)
	out, err := svc.ListConfiguredGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	byName := plazaModelsByName(out[0].Models)
	require.Len(t, byName, 1)
	require.Contains(t, byName, "grok-4.5")
	require.NotContains(t, byName, "grok-imagine-image")
	require.NotContains(t, byName, "grok-imagine-video")
}

func TestListConfiguredPlazaGroups_KeepsConfiguredGrokMediaOnce(t *testing.T) {
	groups := []Group{{
		ID:           10,
		Name:         "Grok Heavy",
		Platform:     PlatformGrok,
		ImagePrice1K: testPtrFloat64(0.03),
		ModelsListConfig: GroupModelsListConfig{
			Models: []string{"grok-imagine-image", "grok-4"},
		},
	}}
	svc := newPlazaService(nil, groups, nil)
	out, err := svc.ListConfiguredGroups(context.Background())
	require.NoError(t, err)
	count := 0
	for _, m := range out[0].Models {
		if m.Name == "grok-imagine-image" {
			count++
		}
	}
	require.Equal(t, 1, count)
	image := plazaModelsByName(out[0].Models)["grok-imagine-image"]
	require.Equal(t, map[string]float64{"1K": 0.03}, plazaIntervalPrices(image.Pricing))
}

func TestListConfiguredPlazaGroups_PreservesConfiguredGrokImageVariants(t *testing.T) {
	groups := []Group{{
		ID:           29,
		Name:         "生图/视频",
		Platform:     PlatformGrok,
		ImagePrice1K: testPtrFloat64(0.03),
		ModelsListConfig: GroupModelsListConfig{
			Models: []string{"grok-imagine", "grok-imagine-image-quality"},
		},
	}}
	svc := newPlazaService(nil, groups, nil)
	out, err := svc.ListConfiguredGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	byName := plazaModelsByName(out[0].Models)
	require.Contains(t, byName, "grok-imagine")
	require.Contains(t, byName, "grok-imagine-image-quality")
	require.NotContains(t, byName, "grok-imagine-image")
	for _, name := range []string{"grok-imagine", "grok-imagine-image-quality"} {
		require.Equal(t, BillingModeImage, byName[name].Pricing.BillingMode)
		require.Equal(t, map[string]float64{"1K": 0.03}, plazaIntervalPrices(byName[name].Pricing))
	}
}

func TestListConfiguredPlazaGroups_GrokImagineAliasUsesQualityChannelPricing(t *testing.T) {
	channels := []Channel{{
		ID: 1, Name: "grok-media", Status: StatusActive, GroupIDs: []int64{29},
		ModelPricing: []ChannelModelPricing{{
			Platform: PlatformGrok, Models: []string{xai.DefaultImagineImageQualityModel},
			BillingMode: BillingModeImage, PerRequestPrice: testPtrFloat64(0.08),
		}},
	}}
	groups := []Group{{
		ID: 29, Name: "生图/视频", Platform: PlatformGrok, RateMultiplier: 1,
		ModelsListConfig: GroupModelsListConfig{Models: []string{"grok-imagine", xai.DefaultImagineImageQualityModel}},
	}}
	svc := newPlazaServiceWithBilling(channels, groups, map[int64]string{29: PlatformGrok}, nil)
	out, err := svc.ListConfiguredGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	byName := plazaModelsByName(out[0].Models)
	for _, name := range []string{"grok-imagine", xai.DefaultImagineImageQualityModel} {
		require.Equal(t, BillingModeImage, byName[name].Pricing.BillingMode)
		require.InDelta(t, 0.08, *byName[name].Pricing.PerRequestPrice, 1e-12)
	}
}

func TestListGroups_GroupMediaPricesDoNotInjectGrokModels(t *testing.T) {
	channels := []Channel{plazaPricedChannel(1, "ch", []int64{10}, PlatformGrok, "grok-4.5")}
	groups := []Group{{
		ID:             10,
		Name:           "Grok Heavy",
		Platform:       PlatformGrok,
		RateMultiplier: 0.11,
		ImagePrice1K:   testPtrFloat64(0.03),
		VideoPrice720P: testPtrFloat64(0.12),
	}}
	svc := newPlazaService(channels, groups, nil)
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	byName := plazaModelsByName(out[0].Models)
	require.Len(t, byName, 1)
	require.Contains(t, byName, "grok-4.5")
	require.NotContains(t, byName, "grok-imagine-image")
	require.NotContains(t, byName, "grok-imagine-video")
	require.NotContains(t, byName, "grok-imagine-video-1.5")
}
