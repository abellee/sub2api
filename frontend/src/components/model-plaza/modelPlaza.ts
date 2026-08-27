import Decimal from 'decimal.js'
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
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

export interface ModelPlazaEntry {
  id: string
  provider: string
  kind: ModelPlazaBillingKind
  input: string | null
  output: string | null
  cache: string | null
  tiers: ModelPlazaTierPrice[]
  groupId: number
  groupName: string
  rateMultiplier: number
}

export function modelPlazaProviderForGroup(
  group: Pick<ModelPlazaGroup, 'platform'>,
): string {
  return normalizeProvider(group.platform)
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
          tiers: mediaTiers(kind, model),
          groupId: group.id,
          groupName: group.name,
          rateMultiplier,
        }]
      }
      const official = model.official_pricing
      return [{
        id,
        provider: modelPlazaProviderForGroup(group),
        kind: 'token',
        input: perMillion(official?.input_price),
        output: perMillion(official?.output_price),
        cache: perMillion(official?.cache_read_price ?? official?.cache_write_price),
        tiers: [],
        groupId: group.id,
        groupName: group.name,
        rateMultiplier: group.user_rate_multiplier ?? group.rate_multiplier,
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
