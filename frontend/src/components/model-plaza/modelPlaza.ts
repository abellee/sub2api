import Decimal from 'decimal.js'
import type { ModelPlazaGroup } from '@/api/modelPlaza'
import { formatDecimal } from '@/utils/pricing'

export interface ModelPlazaEntry {
  id: string
  provider: string
  input: string | null
  output: string | null
  cache: string | null
  groupId: number
  groupName: string
  rateMultiplier: number
  intervals: ModelPlazaPriceInterval[]
}

export interface ModelPlazaPriceInterval {
  label: string
  input: string | null
  output: string | null
  cache: string | null
}

export function modelPlazaProviderForGroup(
  group: Pick<ModelPlazaGroup, 'platform'>,
): string {
  return normalizeProvider(group.platform)
}

export function buildModelPlazaEntries(
  groups: ModelPlazaGroup[],
): ModelPlazaEntry[] {
  return groups.flatMap((group) => {
    const seen = new Set<string>()
    return group.models.flatMap((model) => {
      const id = model.name.trim()
      if (seen.has(id)) return []
      seen.add(id)
      const official = model.official_pricing
      return [{
        id,
        provider: modelPlazaProviderForGroup(group),
        input: perMillion(official?.input_price),
        output: perMillion(official?.output_price),
        cache: perMillion(official?.cache_read_price ?? official?.cache_write_price),
        groupId: group.id,
        groupName: group.name,
        rateMultiplier: group.user_rate_multiplier ?? group.rate_multiplier,
        intervals: (official?.intervals ?? [])
          .filter((interval) => (
            interval.input_price != null
            || interval.output_price != null
            || interval.cache_read_price != null
            || interval.cache_write_price != null
          ))
          .sort((a, b) => a.min_tokens - b.min_tokens)
          .map((interval) => ({
            label: interval.tier_label || formatTokenInterval(interval.min_tokens, interval.max_tokens),
            input: perMillion(interval.input_price),
            output: perMillion(interval.output_price),
            cache: perMillion(interval.cache_read_price ?? interval.cache_write_price),
          })),
      }]
    })
  })
}

export function formatActualModelPrice(value: string | number | null, rateMultiplier: string | number): string {
  if (value == null) return '-'
  try {
    return formatDecimal(new Decimal(value).mul(rateMultiplier), 2, true)
  } catch {
    return '-'
  }
}

function formatTokenInterval(min: number, max: number | null): string {
  if (max == null) return `>${formatTokenCount(min)}`
  if (min === 0) return `≤${formatTokenCount(max)}`
  return `${formatTokenCount(min)}–${formatTokenCount(max)}`
}

function formatTokenCount(value: number): string {
  if (value >= 1_000_000) return `${trimNumber(value / 1_000_000)}M`
  if (value >= 1_000) return `${trimNumber(value / 1_000)}K`
  return String(value)
}

function trimNumber(value: number): string {
  return value.toFixed(2).replace(/\.0+$|(?<=\.[0-9])0+$/, '')
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
