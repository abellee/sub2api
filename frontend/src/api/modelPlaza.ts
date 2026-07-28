import { apiClient } from './client'

export interface ModelPlazaGroup {
  id: number
  name: string
  platform: string
  rate_multiplier: number
  models_list_config: {
    enabled: boolean
    models?: string[]
  }
}

export interface OfficialModelPrice {
  provider: string
  input: number
  output: number
  cache: number
}

export interface OfficialModelPricesResponse {
  updated_at?: string
  stale?: boolean
  models: Record<string, OfficialModelPrice>
}

export async function getModelPlazaGroups(): Promise<ModelPlazaGroup[]> {
  const { data } = await apiClient.get<ModelPlazaGroup[]>('/model-plaza/groups')
  return data
}

export async function getOfficialModelPrices(): Promise<OfficialModelPricesResponse> {
  const endpoint = import.meta.env.DEV
    ? 'https://llmfree.work/api/official-model-prices'
    : '/api/official-model-prices'
  const response = await fetch(endpoint, {
    headers: { Accept: 'application/json' },
    cache: 'no-store',
  })
  if (!response.ok) throw new Error(`HTTP ${response.status}`)
  const payload = await response.json() as OfficialModelPricesResponse
  if (!payload.models || typeof payload.models !== 'object') throw new Error('Invalid price response')
  return payload
}
