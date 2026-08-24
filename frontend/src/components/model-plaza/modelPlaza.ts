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
