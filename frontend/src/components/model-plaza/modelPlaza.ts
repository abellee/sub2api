import type { ModelPlazaGroup } from '@/api/modelPlaza'

export interface ModelPlazaEntry {
  id: string
  provider: string
  input: number | null
  output: number | null
  cache: number | null
  groupId: number
  groupName: string
  rateMultiplier: number
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
        provider: normalizeProvider(model.platform || group.platform),
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

function perMillion(value: number | null | undefined) {
  return typeof value === 'number' ? value * 1_000_000 : null
}

function normalizeProvider(provider: string) {
  const value = provider.toLowerCase()
  return value === 'google' ? 'gemini' : value
}
