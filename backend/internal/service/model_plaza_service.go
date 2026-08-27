package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

// PlazaOfficialPricing 模型广场展示用的官方参考价（USD per token），与计费同源：
// LiteLLM → 内置兜底价卡 → 模型策略。字段为 nil 表示该项缺失（0 视为未配置）。
type PlazaOfficialPricing struct {
	InputPrice        *float64
	OutputPrice       *float64
	CacheWritePrice   *float64 // 5m 缓存写入（= LiteLLM cache_creation）
	CacheWrite1hPrice *float64 // 1h 缓存写入，仅计费会区分 5m/1h 时给出
	CacheReadPrice    *float64
}

// PlazaModel 模型广场中单个模型条目：按实收口径合成的展示定价 + 官方参考价。
type PlazaModel struct {
	Name            string
	Platform        string
	Pricing         *ChannelModelPricing
	OfficialPricing *PlazaOfficialPricing
	// TimePricing 计费会生效的分时倍率时段；无分时为 nil。
	TimePricing *TimePricingSchedule
}

// PlazaGroup 模型广场中以分组为顶层的条目。
//
// 与 AvailableGroupRef 相比多了 Description 与 Models；Models 来自该分组关联渠道的
// 支持模型（普通分组按分组平台隔离，Composite 分组展开关联渠道已配置的
// 具体平台），与「可用渠道」页口径一致。
type PlazaGroup struct {
	ID                 int64
	Name               string
	Description        string
	Platform           string
	SubscriptionType   string
	RateMultiplier     float64
	PeakRateEnabled    bool
	PeakStart          string
	PeakEnd            string
	PeakRateMultiplier float64
	IsExclusive        bool
	// 图片按次实付倍率：ImageRateIndependent 为 true 时，图片计费模型的实付
	// = 档位价 × ImageRateMultiplier，不乘分组/用户专属倍率（与计费口径一致）。
	ImageRateIndependent bool
	ImageRateMultiplier  float64
	// 视频按秒实付倍率：VideoRateIndependent 为 true 时，视频计费模型的实付
	// = 档位价 × VideoRateMultiplier，不乘分组/用户专属倍率（与计费口径一致）。
	VideoRateIndependent bool
	VideoRateMultiplier  float64
	Models               []PlazaModel
}

// ModelPlazaService 聚合模型广场数据。
//
// 模型枚举来自渠道配置；token 模型的展示单价与阶梯由 BillingService 的阶梯表
// 查询给出（与扣费走同一条解析链与计费函数），图片/按次模型沿用渠道/分组档位价。
type ModelPlazaService struct {
	channelRepo    ChannelRepository
	groupRepo      GroupRepository
	pricingService *PricingService
	billingService *BillingService
	resolver       *ModelPricingResolver
}

// NewModelPlazaService 创建模型广场服务。
func NewModelPlazaService(
	channelRepo ChannelRepository,
	groupRepo GroupRepository,
	pricingService *PricingService,
	billingService *BillingService,
	resolver *ModelPricingResolver,
) *ModelPlazaService {
	return &ModelPlazaService{
		channelRepo:    channelRepo,
		groupRepo:      groupRepo,
		pricingService: pricingService,
		billingService: billingService,
		resolver:       resolver,
	}
}

// ListGroups 返回模型广场数据：每个活跃分组附带其可用模型与定价。
//
// 模型枚举口径与 ListAvailable 一致（Active 渠道、SupportedModels ∪ 全局定价回落、
// 平台隔离），仅把顶层从渠道换成分组：
//   - 渠道按 lower(name) 排序后遍历，保证同名模型去重结果确定；
//   - 同分组同名模型「先见者胜」，仅当已存条目无定价而新条目有定价时升级替换；
//   - token 模型的单价与阶梯按实收口径合成（见 ResolveContextPricingSchedule）；
//     图片/视频计费模型优先使用渠道价卡（见 plazaResolvedRequestPricing），
//     分组档位价仅覆盖已配置项（见 plazaImageDisplayPricing / plazaVideoDisplayPricing）；
//   - 每个模型附带官方参考价（查不到为 nil）；
//   - 只返回 Models 非空的分组；分组按 RateMultiplier 升序（同倍率按名称），
//     组内模型按名称排序。
//
// 可见性过滤（专属分组）不在此层做，由 handler 按登录态裁剪。
func (s *ModelPlazaService) ListGroups(ctx context.Context) ([]PlazaGroup, error) {
	channels, err := s.channelRepo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active groups: %w", err)
	}

	sort.SliceStable(channels, func(i, j int) bool {
		return strings.ToLower(channels[i].Name) < strings.ToLower(channels[j].Name)
	})

	byGroup := make(map[int64]*PlazaGroup, len(groups))
	groupEnt := make(map[int64]*Group, len(groups))
	order := make([]int64, 0, len(groups))
	for i := range groups {
		g := &groups[i]
		byGroup[g.ID] = &PlazaGroup{
			ID:                   g.ID,
			Name:                 g.Name,
			Description:          g.Description,
			Platform:             g.Platform,
			SubscriptionType:     g.SubscriptionType,
			RateMultiplier:       g.RateMultiplier,
			PeakRateEnabled:      g.PeakRateEnabled,
			PeakStart:            g.PeakStart,
			PeakEnd:              g.PeakEnd,
			PeakRateMultiplier:   g.PeakRateMultiplier,
			IsExclusive:          g.IsExclusive,
			ImageRateIndependent: g.ImageRateIndependent,
			ImageRateMultiplier:  g.ImageRateMultiplier,
			VideoRateIndependent: g.VideoRateIndependent,
			VideoRateMultiplier:  g.VideoRateMultiplier,
		}
		groupEnt[g.ID] = g
		order = append(order, g.ID)
	}

	type modelKey struct {
		platform string
		name     string
	}
	// modelIdx[groupID][platform+modelName] = index into byGroup[groupID].Models
	modelIdx := make(map[int64]map[modelKey]int, len(groups))
	for i := range channels {
		ch := &channels[i]
		if ch.Status != StatusActive {
			continue
		}
		ch.normalizeBillingModelSource()
		supported := ch.SupportedModels()
		fillGlobalPricingFallback(s.pricingService, supported)

		for _, gid := range ch.GroupIDs {
			pg, ok := byGroup[gid]
			if !ok {
				continue
			}
			idx := modelIdx[gid]
			if idx == nil {
				idx = make(map[modelKey]int, len(supported))
				modelIdx[gid] = idx
			}
			for j := range supported {
				m := supported[j]
				if pg.Platform == PlatformComposite {
					if !isConcreteRequestPlatform(m.Platform) {
						continue
					}
				} else if m.Platform != pg.Platform {
					continue
				}
				key := modelKey{platform: m.Platform, name: m.Name}
				if at, seen := idx[key]; seen {
					// 先见者胜；仅当已存条目无定价而新条目有定价时升级。
					if pg.Models[at].Pricing == nil && m.Pricing != nil {
						pg.Models[at].Pricing = m.Pricing
					}
					continue
				}
				idx[key] = len(pg.Models)
				pg.Models = append(pg.Models, PlazaModel{
					Name:     m.Name,
					Platform: m.Platform,
					Pricing:  m.Pricing,
				})
			}
		}
	}

	officialMemo := make(map[string]*PlazaOfficialPricing)
	out := make([]PlazaGroup, 0, len(order))
	for _, gid := range order {
		pg := byGroup[gid]
		g := groupEnt[gid]
		if len(pg.Models) == 0 {
			continue
		}
		sort.SliceStable(pg.Models, func(i, j int) bool {
			if pg.Models[i].Name != pg.Models[j].Name {
				return pg.Models[i].Name < pg.Models[j].Name
			}
			return pg.Models[i].Platform < pg.Models[j].Platform
		})
		for j := range pg.Models {
			s.fillDisplayPricing(ctx, &pg.Models[j], g)
			pg.Models[j].OfficialPricing = s.lookupOfficialPricing(ctx, pg.Models[j].Name, officialMemo)
		}
		out = append(out, *pg)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].RateMultiplier != out[j].RateMultiplier {
			return out[i].RateMultiplier < out[j].RateMultiplier
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// ListConfiguredGroups returns the catalog exactly as configured in each
// active group's models_list_config. Channel support and pricing only enrich
// those configured entries and never determine which models are visible.
func (s *ModelPlazaService) ListConfiguredGroups(ctx context.Context) ([]PlazaGroup, error) {
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active groups: %w", err)
	}

	officialMemo := make(map[string]*PlazaOfficialPricing)
	out := make([]PlazaGroup, 0, len(groups))
	for i := range groups {
		g := &groups[i]
		if len(g.ModelsListConfig.Models) == 0 {
			continue
		}
		pg := PlazaGroup{
			ID:                   g.ID,
			Name:                 g.Name,
			Description:          g.Description,
			Platform:             g.Platform,
			SubscriptionType:     g.SubscriptionType,
			RateMultiplier:       g.RateMultiplier,
			PeakRateEnabled:      g.PeakRateEnabled,
			PeakStart:            g.PeakStart,
			PeakEnd:              g.PeakEnd,
			PeakRateMultiplier:   g.PeakRateMultiplier,
			IsExclusive:          g.IsExclusive,
			ImageRateIndependent: g.ImageRateIndependent,
			ImageRateMultiplier:  g.ImageRateMultiplier,
			VideoRateIndependent: g.VideoRateIndependent,
			VideoRateMultiplier:  g.VideoRateMultiplier,
		}
		seen := make(map[string]struct{}, len(g.ModelsListConfig.Models))
		for _, configuredName := range g.ModelsListConfig.Models {
			name := strings.TrimSpace(configuredName)
			if name == "" {
				continue
			}
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			pg.Models = append(pg.Models, PlazaModel{
				Name:     name,
				Platform: g.Platform,
			})
		}
		if len(pg.Models) == 0 {
			continue
		}
		for j := range pg.Models {
			// Configured models are intentionally kept even without an active
			// channel, but when a channel exists use the same billing resolver as
			// ListGroups so channel interval pricing is visible on the public page.
			s.fillDisplayPricing(ctx, &pg.Models[j], g)
			pg.Models[j].OfficialPricing = s.lookupOfficialPricing(ctx, pg.Models[j].Name, officialMemo)
		}
		sort.SliceStable(pg.Models, func(i, j int) bool {
			return pg.Models[i].Name < pg.Models[j].Name
		})
		out = append(out, pg)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].RateMultiplier != out[j].RateMultiplier {
			return out[i].RateMultiplier < out[j].RateMultiplier
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// fillDisplayPricing 把模型的展示定价换成实收口径：
// token 模型取计费阶梯表（单价与档位均由真实计费函数得出）；
// 图片/视频/按次模型优先用渠道价卡，分组档位价只覆盖已配置项。
// 公开页模型最初可能没有渠道定价指针，因此这里会再走一遍 Resolver。
func (s *ModelPlazaService) fillDisplayPricing(ctx context.Context, m *PlazaModel, g *Group) {
	if s.resolver != nil {
		if requestPricing := plazaResolvedRequestPricing(ctx, s.resolver, m, g); requestPricing != nil {
			m.Pricing = plazaImageDisplayPricing(requestPricing, g)
			m.Pricing = plazaVideoDisplayPricing(m.Name, m.Pricing, g)
			m.TimePricing = nil
			m.Pricing = plazaApplyGroupMediaDisplayPricing(m.Name, m.Pricing, g)
			return
		}
	}
	if !plazaIsGrokMediaModel(m.Name) && s.billingService != nil && s.resolver != nil {
		sched, err := s.billingService.ResolveContextPricingSchedule(ctx, s.resolver, ContextPricingScheduleInput{
			Model:    m.Name,
			Group:    g,
			Platform: m.Platform,
		})
		if err == nil && sched != nil && len(sched.Tiers) > 0 {
			m.Pricing = plazaPricingFromSchedule(m.Pricing, sched)
			m.TimePricing = sched.TimePricing
			return
		}
	}
	m.Pricing = plazaImageDisplayPricing(m.Pricing, g)
	m.Pricing = plazaVideoDisplayPricing(m.Name, m.Pricing, g)
	m.Pricing = plazaApplyGroupMediaDisplayPricing(m.Name, m.Pricing, g)
}

func plazaIsGrokImagineImage(model string) bool {
	m := strings.ToLower(xai.StripGrokProviderPrefix(model))
	return m == "grok-imagine" || m == "grok-imagine-1" || m == "grok-imagine-edit" || strings.HasPrefix(m, "grok-imagine-image")
}

func plazaGrokImagePricingModel(model string) string {
	m := strings.ToLower(xai.StripGrokProviderPrefix(model))
	switch m {
	case "grok-imagine", "grok-imagine-1", "grok-imagine-edit":
		return xai.DefaultImagineImageQualityModel
	default:
		return strings.TrimSpace(model)
	}
}

func plazaIsGrokMediaModel(model string) bool {
	return plazaIsGrokImagineImage(model) || CanonicalGrokImagineVideoPriceFamily(model) != ""
}

func plazaHasRequestTiers(p *ChannelModelPricing) bool {
	if p == nil {
		return false
	}
	if p.PerRequestPrice != nil {
		return true
	}
	for _, iv := range p.Intervals {
		if iv.PerRequestPrice != nil {
			return true
		}
	}
	return false
}

func plazaApplyGroupMediaDisplayPricing(model string, p *ChannelModelPricing, g *Group) *ChannelModelPricing {
	if plazaIsGrokImagineImage(model) {
		base := p
		if base == nil || base.BillingMode != BillingModeImage {
			base = &ChannelModelPricing{BillingMode: BillingModeImage}
		}
		out := plazaImageDisplayPricing(base, g)
		if plazaHasRequestTiers(out) {
			return out
		}
		if plazaHasRequestTiers(p) {
			return p
		}
		return nil
	}
	if CanonicalGrokImagineVideoPriceFamily(model) != "" {
		base := p
		if base == nil || base.BillingMode != BillingModeVideo {
			base = &ChannelModelPricing{BillingMode: BillingModeVideo}
		}
		out := plazaVideoDisplayPricing(model, base, g)
		if plazaHasRequestTiers(out) {
			return out
		}
		if plazaHasRequestTiers(p) {
			return p
		}
		return nil
	}
	return p
}

// plazaResolvedRequestPricing 从计费解析器取出渠道/分组配置的按次、图片、视频价卡。
// token 模式返回 nil，让调用方继续走阶梯表。
func plazaResolvedRequestPricing(ctx context.Context, resolver *ModelPricingResolver, m *PlazaModel, g *Group) *ChannelModelPricing {
	if resolver == nil || m == nil {
		return nil
	}
	input := PricingInput{Group: g}
	if g != nil {
		gid := g.ID
		input.GroupID = &gid
	}
	if m.Platform != "" {
		ctx = WithResolvedTargetPlatform(ctx, m.Platform)
	}
	resolve := func(model string) *ResolvedPricing {
		input.Model = model
		return resolver.Resolve(ctx, input)
	}
	resolved := resolve(m.Name)
	if canonical := plazaGrokImagePricingModel(m.Name); canonical != strings.TrimSpace(m.Name) &&
		(resolved == nil || (resolved.Mode != BillingModeImage && resolved.Mode != BillingModePerRequest)) {
		resolved = resolve(canonical)
	}
	if resolved == nil {
		return nil
	}
	switch resolved.Mode {
	case BillingModeImage, BillingModeVideo, BillingModePerRequest:
	default:
		return nil
	}
	out := ChannelModelPricing{BillingMode: resolved.Mode}
	if resolved.channelPricing != nil {
		out.ImageInputPrice = resolved.channelPricing.ImageInputPrice
		out.ImageOutputPrice = resolved.channelPricing.ImageOutputPrice
		out.PerRequestPrice = resolved.channelPricing.PerRequestPrice
	}
	if resolved.DefaultPerRequestPrice > 0 && out.PerRequestPrice == nil {
		v := resolved.DefaultPerRequestPrice
		out.PerRequestPrice = &v
	}
	if len(resolved.RequestTiers) > 0 {
		out.Intervals = append([]PricingInterval(nil), resolved.RequestTiers...)
	}
	if len(out.Intervals) == 0 && out.PerRequestPrice == nil && out.ImageInputPrice == nil && out.ImageOutputPrice == nil {
		return nil
	}
	return &out
}

// plazaPricingFromSchedule 把计费阶梯表转换为模型广场展示定价。
// 基础字段始终取第一档；整单计价且存在多档时同时公开全部上下文档位。
// 边际计价不能表示成普通整单档位，因此仍只展示基础字段。
func plazaPricingFromSchedule(raw *ChannelModelPricing, sched *ContextPricingSchedule) *ChannelModelPricing {
	out := &ChannelModelPricing{BillingMode: BillingModeToken}
	if raw != nil {
		out.ImageInputPrice = raw.ImageInputPrice
		out.ImageOutputPrice = raw.ImageOutputPrice
		out.PerRequestPrice = raw.PerRequestPrice
	}
	first := sched.Tiers[0]
	out.InputPrice = first.Input
	out.OutputPrice = first.Output
	out.CacheWritePrice = first.CacheWrite
	out.CacheReadPrice = first.CacheRead
	if sched.Basis == ContextPricingBasisWholeRequest && len(sched.Tiers) > 1 {
		out.Intervals = make([]PricingInterval, 0, len(sched.Tiers))
		for i := range sched.Tiers {
			tier := &sched.Tiers[i]
			out.Intervals = append(out.Intervals, PricingInterval{
				MinTokens:       tier.MinTokens,
				MaxTokens:       tier.MaxTokens,
				TierLabel:       tier.Label,
				InputPrice:      tier.Input,
				OutputPrice:     tier.Output,
				CacheWritePrice: tier.CacheWrite,
				CacheReadPrice:  tier.CacheRead,
				SortOrder:       i,
			})
		}
	}
	return out
}

// plazaImageDisplayPricing 为图片计费模型合成展示定价，使档位价与实收口径一致：
// 每档（1K/2K/4K）单价 = 分组图片价 > 渠道同档位价 > 渠道默认按次价，无价的档不展示。
// 分组未配任何图片价、或定价非图片模式时原样返回。返回克隆，不修改入参
// （渠道定价指针指向缓存共享数据）。
func plazaImageDisplayPricing(p *ChannelModelPricing, g *Group) *ChannelModelPricing {
	if p == nil || g == nil || p.BillingMode != BillingModeImage {
		return p
	}
	if g.ImagePrice1K == nil && g.ImagePrice2K == nil && g.ImagePrice4K == nil {
		return p
	}
	channelTierPrice := func(label string) *float64 {
		for i := range p.Intervals {
			if p.Intervals[i].TierLabel == label && p.Intervals[i].PerRequestPrice != nil {
				return p.Intervals[i].PerRequestPrice
			}
		}
		return p.PerRequestPrice
	}
	tiers := []struct {
		label      string
		groupPrice *float64
	}{
		{"1K", g.ImagePrice1K},
		{"2K", g.ImagePrice2K},
		{"4K", g.ImagePrice4K},
	}
	clone := *p
	clone.Intervals = make([]PricingInterval, 0, len(tiers))
	for i, t := range tiers {
		price := t.groupPrice
		if price == nil {
			price = channelTierPrice(t.label)
		}
		if price == nil {
			continue
		}
		v := *price
		clone.Intervals = append(clone.Intervals, PricingInterval{
			TierLabel:       t.label,
			PerRequestPrice: &v,
			SortOrder:       i,
		})
	}
	return &clone
}

// plazaVideoDisplayPricing 为视频计费模型合成展示定价：
// 每档（480p/720p/1080p）单价 = 分组模型族价 > 分组平面价 > 渠道同档位价 > 渠道默认按次价。
// 只展示有价格的档位；分组未配视频价时保持渠道定价原样。
func plazaVideoDisplayPricing(model string, p *ChannelModelPricing, g *Group) *ChannelModelPricing {
	if p == nil || g == nil || p.BillingMode != BillingModeVideo {
		return p
	}
	hasGroupVideoPrice := g.VideoPrice480P != nil || g.VideoPrice720P != nil || g.VideoPrice1080P != nil ||
		LookupVideoModelPrice(g.VideoModelPrices, model, VideoBillingResolution480P) != nil ||
		LookupVideoModelPrice(g.VideoModelPrices, model, VideoBillingResolution720P) != nil ||
		LookupVideoModelPrice(g.VideoModelPrices, model, VideoBillingResolution1080P) != nil
	if !hasGroupVideoPrice {
		return p
	}
	channelTierPrice := func(label string) *float64 {
		for i := range p.Intervals {
			if p.Intervals[i].TierLabel == label && p.Intervals[i].PerRequestPrice != nil {
				return p.Intervals[i].PerRequestPrice
			}
		}
		return p.PerRequestPrice
	}
	tiers := []string{
		VideoBillingResolution480P,
		VideoBillingResolution720P,
		VideoBillingResolution1080P,
	}
	clone := *p
	clone.Intervals = make([]PricingInterval, 0, len(tiers))
	for i, label := range tiers {
		price := LookupVideoModelPrice(g.VideoModelPrices, model, label)
		if price == nil {
			price = g.GetVideoPrice(label)
		}
		if price == nil {
			price = channelTierPrice(label)
		}
		if price == nil {
			continue
		}
		v := *price
		clone.Intervals = append(clone.Intervals, PricingInterval{
			TierLabel:       label,
			PerRequestPrice: &v,
			SortOrder:       i,
		})
	}
	return &clone
}

// lookupOfficialPricing 查询模型的官方参考价（与计费同源：LiteLLM → 内置兜底 → 模型策略），
// 带 memo 避免同名模型重复解析。官方阶梯按无分组、无渠道的口径查阶梯表。
// billingService 为 nil（测试场景）或查不到时返回 nil。
func (s *ModelPlazaService) lookupOfficialPricing(ctx context.Context, modelName string, memo map[string]*PlazaOfficialPricing) *PlazaOfficialPricing {
	if s.billingService == nil {
		return nil
	}
	if cached, ok := memo[modelName]; ok {
		return cached
	}
	var result *PlazaOfficialPricing
	if mp, err := s.billingService.GetModelPricing(modelName); err == nil && mp != nil {
		result = &PlazaOfficialPricing{
			InputPrice:      nonZeroPtr(mp.InputPricePerToken),
			OutputPrice:     nonZeroPtr(mp.OutputPricePerToken),
			CacheWritePrice: nonZeroPtr(mp.CacheCreationPricePerToken),
			CacheReadPrice:  nonZeroPtr(mp.CacheReadPricePerToken),
		}
		// 计费只在支持 5m/1h 分档时使用 1h 价，其余情况 1h 价对用户无意义。
		if mp.SupportsCacheBreakdown {
			result.CacheWrite1hPrice = nonZeroPtr(mp.CacheCreation1hPrice)
		}
		if result.InputPrice == nil && result.OutputPrice == nil && result.CacheWritePrice == nil &&
			result.CacheWrite1hPrice == nil && result.CacheReadPrice == nil {
			result = nil
		}
	}
	memo[modelName] = result
	return result
}
