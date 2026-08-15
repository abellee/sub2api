import type { ModelPlazaGroup, PlazaOfficialPricing } from '@/api/modelPlaza'

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

export function canonicalModelId(modelId: string): string {
  return modelId.trim().replace(/-(?:19|20)\d{2}(?:-?\d{2}){2}$/, '')
}

export function buildModelPlazaEntries(
  groups: ModelPlazaGroup[],
): ModelPlazaEntry[] {
  return groups.flatMap((group) => {
    const seen = new Set<string>()
    return group.models.flatMap((model) => {
      const id = canonicalModelId(model.name)
      if (seen.has(id)) return []
      seen.add(id)
      const official = model.official_pricing
      if (!official || !hasPrice(official)) return []
      return [{
        id,
        provider: normalizeProvider(model.platform || group.platform),
        input: perMillion(official.input_price),
        output: perMillion(official.output_price),
        cache: perMillion(official.cache_read_price ?? official.cache_write_price),
        groupId: group.id,
        groupName: group.name,
        rateMultiplier: group.user_rate_multiplier ?? group.rate_multiplier,
      }]
    })
  })
}

function hasPrice(pricing: PlazaOfficialPricing) {
  return [pricing.input_price, pricing.output_price, pricing.cache_read_price, pricing.cache_write_price]
    .some((value) => typeof value === 'number' && value > 0)
}

function perMillion(value: number | null | undefined) {
  return typeof value === 'number' ? value * 1_000_000 : null
}

function normalizeProvider(provider: string) {
  const value = provider.toLowerCase()
  return value === 'google' ? 'gemini' : value
}
