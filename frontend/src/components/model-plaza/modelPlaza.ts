import Decimal from 'decimal.js'
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import type { UserPricingInterval } from '@/api/channels'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_TOKEN,
  BILLING_MODE_VIDEO,
  type BillingMode,
} from '@/constants/channel'
import { formatDecimal } from '@/utils/pricing'

export const MODEL_PLAZA_IMAGE_TIERS = ['1K', '2K', '4K'] as const
export const MODEL_PLAZA_VIDEO_TIERS = ['480p', '720p', '1080p'] as const

export type ModelPlazaBillingKind = 'token' | 'image' | 'video'

export interface ModelPlazaTierPrice {
  label: string
  original: string | null
}

export interface ModelPlazaTokenTierPrice {
  label: string
  input: string | null
  output: string | null
  cache: string | null
  cacheWrite: string | null
  cacheWrite1h: string | null
  cacheRead: string | null
}

export interface ModelPlazaEntry {
  id: string
  provider: string
  kind: ModelPlazaBillingKind
  input: string | null
  output: string | null
  cache: string | null
  cacheWrite: string | null
  cacheWrite1h: string | null
  cacheRead: string | null
  tiers: ModelPlazaTierPrice[]
  tokenTiers: ModelPlazaTokenTierPrice[]
  /** Sub2API official context tiers shown on demand when the channel has none. */
  officialTokenTiers: ModelPlazaTokenTierPrice[]
  groupId: number
  groupName: string
  rateMultiplier: number
  currency: 'USD' | 'CNY'
  timePricing: PlazaModel['time_pricing'] | null
}

export function modelPlazaProviderForGroup(
  group: Pick<ModelPlazaGroup, 'platform'>,
): string {
  return normalizeProvider(group.platform)
}

/** Whether token intervals are owned by the selected channel. */
export function hasModelPlazaChannelContextPricing(model: PlazaModel): boolean {
  if (model.has_channel_context_pricing === true) return true
  if (model.has_channel_context_pricing != null) return false
  // Compatibility for the live API response emitted before the ownership
  // flag was added. Other providers remain conservative to avoid exposing
  // catalog-only ladders as paid channel pricing.
  return normalizeProvider(model.platform) === 'grok'
    && (model.pricing?.intervals?.length ?? 0) > 0
}

export function buildModelPlazaEntries(
  groups: ModelPlazaGroup[],
): ModelPlazaEntry[] {
  return groups.flatMap<ModelPlazaEntry>((group) => {
    const seen = new Set<string>()
    return group.models.flatMap<ModelPlazaEntry>((model): ModelPlazaEntry[] => {
      const id = model.name.trim()
      if (seen.has(id)) return []
      seen.add(id)
      const kind = modelPlazaEntryKind(group, model)
      const rateMultiplier = mediaRateMultiplier(group, kind)
      if (kind === 'image' || kind === 'video') {
        return [{
          id,
          provider: modelPlazaProviderForGroup(group),
          kind,
          input: null,
          output: null,
          cache: null,
          cacheWrite: null,
          cacheWrite1h: null,
          cacheRead: null,
          tiers: mediaTiers(kind, model),
          tokenTiers: [],
          officialTokenTiers: [],
          groupId: group.id,
          groupName: group.name,
          rateMultiplier,
          currency: currencyForModel(modelPlazaProviderForGroup(group), model),
          timePricing: model.time_pricing ?? null,
        }]
      }
      const channel = hasMeaningfulPricing(model.pricing) ? model.pricing : null
      const official = model.official_pricing
      const price = (field: 'input_price' | 'output_price' | 'cache_read_price' | 'cache_write_price' | 'cache_write_1h_price') => channel?.[field] ?? official?.[field] ?? null
      const cacheWrite = perMillion(price('cache_write_price'))
      const cacheWrite1h = perMillion(price('cache_write_1h_price'))
      const cacheRead = perMillion(price('cache_read_price'))
      // Older production responses (before the ownership flag was added)
      // already contained Grok channel intervals in pricing. Keep those
      // payloads readable while leaving omitted flags on other providers
      // conservative, since their intervals may be catalog-only.
      const channelIntervals = hasModelPlazaChannelContextPricing(model)
        ? channel?.intervals ?? []
        : []
      const channelTiers = tokenTiers(channelIntervals, official?.intervals ?? [])
      const officialTiers = channelTiers.length > 0 ? [] : tokenTiers(official?.intervals ?? [], official?.intervals ?? [])
      return [{
        id,
        provider: modelPlazaProviderForGroup(group),
        kind: 'token',
        input: perMillion(price('input_price')),
        output: perMillion(price('output_price')),
        cache: cacheRead ?? cacheWrite,
        cacheWrite,
        cacheWrite1h,
        cacheRead,
        tiers: [],
        tokenTiers: channelTiers,
        officialTokenTiers: officialTiers,
        groupId: group.id,
        groupName: group.name,
        rateMultiplier: group.user_rate_multiplier ?? group.rate_multiplier,
        currency: currencyForModel(modelPlazaProviderForGroup(group), model),
        timePricing: model.time_pricing ?? null,
      }]
    })
  })
}

export function modelPlazaEntryKind(
  group: ModelPlazaGroup,
  model: PlazaModel,
): ModelPlazaBillingKind {
  const mode = billingModeOf(model)
  if (mode === BILLING_MODE_IMAGE) return 'image'
  if (mode === BILLING_MODE_VIDEO) return 'video'
  if (mode === BILLING_MODE_TOKEN) return 'token'
  if (isGrokImagineVideo(model.name)) return 'video'
  if (isGrokImagineImage(model.name)) return 'image'
  if (isGptImageModel(model.name) && !hasTokenPaidPrice(model) && isMediaRateGroup(group)) {
    return 'image'
  }
  return 'token'
}

export function formatActualModelPrice(value: string | number | null, rateMultiplier: string | number): string {
  if (value == null) return '-'
  try {
    return formatDecimal(new Decimal(value).mul(rateMultiplier), 2, true)
  } catch {
    return '-'
  }
}

function mediaRateMultiplier(group: ModelPlazaGroup, kind: ModelPlazaBillingKind): number {
  const groupRate = group.user_rate_multiplier ?? group.rate_multiplier
  if (kind === 'image' && group.image_rate_independent) {
    return group.image_rate_multiplier ?? 1
  }
  if (kind === 'video' && group.video_rate_independent) {
    return group.video_rate_multiplier ?? 1
  }
  return groupRate
}

function mediaTiers(kind: 'image' | 'video', model: PlazaModel): ModelPlazaTierPrice[] {
  const labels = kind === 'image' ? MODEL_PLAZA_IMAGE_TIERS : MODEL_PLAZA_VIDEO_TIERS
  const prices = requestTierMap(model)
  const tiers = labels
    .filter((label) => prices.has(label))
    .map((label) => ({ label, original: prices.get(label) ?? null }))
  if (tiers.length > 0) return tiers
  const flatPrice = formatRequestPrice(model.pricing?.per_request_price)
  return flatPrice == null ? [] : [{ label: '', original: flatPrice }]
}

function requestTierMap(model: PlazaModel): Map<string, string> {
  const map = new Map<string, string>()
  for (const interval of model.pricing?.intervals ?? []) {
    if (!interval.tier_label || interval.per_request_price == null) continue
    const formatted = formatRequestPrice(interval.per_request_price)
    if (formatted != null) map.set(interval.tier_label, formatted)
  }
  return map
}

function tokenTiers(channelIntervals: UserPricingInterval[], officialIntervals: UserPricingInterval[]): ModelPlazaTokenTierPrice[] {
	// Long-context prices are only advertised when the selected channel has
	// explicitly configured token intervals. Official intervals remain a
	// per-field fallback for those same channel-defined tiers.
	const intervals = channelIntervals
  return intervals
    .filter((interval) => interval.min_tokens > 0 && (
      interval.input_price != null
      || interval.output_price != null
      || interval.cache_read_price != null
      || interval.cache_write_price != null
      || interval.cache_write_1h_price != null
    ))
    .map((interval) => {
      const official = officialIntervals.find((candidate) => (
        candidate.tier_label === interval.tier_label
        || (candidate.min_tokens === interval.min_tokens && candidate.max_tokens === interval.max_tokens)
      ))
      const price = (field: 'input_price' | 'output_price' | 'cache_read_price' | 'cache_write_price' | 'cache_write_1h_price') => interval[field] ?? official?.[field] ?? null
      const cacheWrite = perMillion(price('cache_write_price'))
      const cacheWrite1h = perMillion(price('cache_write_1h_price'))
      const cacheRead = perMillion(price('cache_read_price'))
      return {
        label: interval.tier_label || tokenRangeLabel(interval.min_tokens, interval.max_tokens),
        input: perMillion(price('input_price')),
        output: perMillion(price('output_price')),
        cache: cacheRead ?? cacheWrite,
        cacheWrite,
        cacheWrite1h,
        cacheRead,
      }
    })
}

function hasMeaningfulPricing(pricing: PlazaModel['pricing']): pricing is NonNullable<PlazaModel['pricing']> {
  if (!pricing) return false
  return pricing.input_price != null
    || pricing.output_price != null
    || pricing.cache_read_price != null
    || pricing.cache_write_price != null
    || pricing.cache_write_1h_price != null
    || pricing.per_request_price != null
    || pricing.intervals.length > 0
}

function currencyForProvider(provider: string): 'USD' | 'CNY' {
  return ['deepseek', 'kimi', 'zhipu', 'glm', 'moonshot', 'minimax', 'doubao'].includes(provider.toLowerCase()) ? 'CNY' : 'USD'
}

function currencyForModel(provider: string, model: PlazaModel): 'USD' | 'CNY' {
  const identity = `${provider} ${model.platform} ${model.name}`.toLowerCase()
  return /deepseek|(?:^|\s)glm[-\s]|kimi|moonshot|minimax|doubao|zhipu/.test(identity) ? 'CNY' : currencyForProvider(provider)
}

function tokenRangeLabel(min: number, max: number | null): string {
  if (max == null) return `>${formatTokenCount(min)}`
  if (min <= 0) return `<=${formatTokenCount(max)}`
  return `${formatTokenCount(min)}-${formatTokenCount(max)}`
}

function formatTokenCount(value: number): string {
  if (value >= 1_000_000) return `${formatTokenScale(value, 1_000_000)}M`
  if (value >= 1_000) return `${formatTokenScale(value, 1_000)}K`
  return String(value)
}

function formatTokenScale(value: number, scale: number): string {
  return String(Math.round((value / scale) * 100) / 100)
}

function formatRequestPrice(value: number | null | undefined): string | null {
  if (value == null || !Number.isFinite(value)) return null
  try {
    return new Decimal(value).toFixed()
  } catch {
    return null
  }
}

function billingModeOf(model: PlazaModel): BillingMode | null {
  return (model.pricing?.billing_mode as BillingMode | undefined) ?? null
}

function hasTokenPaidPrice(model: PlazaModel): boolean {
  const pricing = model.pricing
  if (!pricing || pricing.billing_mode === BILLING_MODE_IMAGE || pricing.billing_mode === BILLING_MODE_VIDEO) {
    return false
  }
  return pricing.input_price != null
    || pricing.output_price != null
    || pricing.cache_read_price != null
    || pricing.cache_write_price != null
    || pricing.cache_write_1h_price != null
}

function isMediaRateGroup(group: ModelPlazaGroup): boolean {
  return Boolean(group.image_rate_independent || group.video_rate_independent)
}

function isGrokImagineImage(name: string): boolean {
  const model = normalizeName(name)
  return model === 'grok-imagine' || model === 'grok-imagine-edit' || model.startsWith('grok-imagine-image')
}

function isGrokImagineVideo(name: string): boolean {
  const model = normalizeName(name)
  return model.includes('grok-imagine-video') || /(?:^|\/)grok-video(?:-|$)/.test(model)
}

function isGptImageModel(name: string): boolean {
  return normalizeName(name).startsWith('gpt-image')
}

function normalizeName(name: string): string {
  return name.trim().toLowerCase()
}

function perMillion(value: number | null | undefined): string | null {
  if (value == null || !Number.isFinite(value)) return null
  try {
    return new Decimal(value).mul(1_000_000).toFixed()
  } catch {
    return null
  }
}

function normalizeProvider(provider: string) {
  const value = provider.toLowerCase()
  return value === 'google' ? 'gemini' : value
}
