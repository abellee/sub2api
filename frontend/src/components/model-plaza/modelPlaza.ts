import type { ModelPlazaGroup, OfficialModelPrice } from '@/api/modelPlaza'

export interface ModelPlazaEntry extends OfficialModelPrice {
  id: string
  groupId: number
  groupName: string
  rateMultiplier: number
}

export function canonicalModelId(modelId: string): string {
  return modelId.trim().replace(/-(?:19|20)\d{2}(?:-?\d{2}){2}$/, '')
}

export function buildModelPlazaEntries(
  groups: ModelPlazaGroup[],
  officialModels: Record<string, OfficialModelPrice>,
): ModelPlazaEntry[] {
  const officialById = new Map(
    Object.entries(officialModels).map(([id, model]) => [canonicalModelId(id), model]),
  )

  return groups.flatMap((group) => {
    const seen = new Set<string>()
    return (group.models_list_config.models ?? []).flatMap((configuredId) => {
      const id = canonicalModelId(configuredId)
      if (seen.has(id)) return []
      seen.add(id)
      const official = officialById.get(id)
      if (!official || official.provider !== group.platform) return []
      return [{ ...official, id, groupId: group.id, groupName: group.name, rateMultiplier: group.rate_multiplier }]
    })
  })
}
