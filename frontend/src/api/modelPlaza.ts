import { apiClient } from './client'
import type { UserSupportedModelPricing } from './channels'

/** Official reference pricing used by the built-in model plaza. */
export interface PlazaOfficialPricing {
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price?: number | null
  cache_read_price: number | null
}

export interface PlazaModel {
  name: string
  platform: string
  pricing: UserSupportedModelPricing | null
  official_pricing: PlazaOfficialPricing | null
}

export interface ModelPlazaGroup {
  id: number
  name: string
  description: string
  platform: string
  subscription_type: string
  rate_multiplier: number
  user_rate_multiplier?: number
  peak_rate_enabled: boolean
  peak_start: string
  peak_end: string
  peak_rate_multiplier: number
  is_exclusive: boolean
  image_rate_independent: boolean
  image_rate_multiplier: number
  models: PlazaModel[]
}

export interface ModelPlazaResponse {
  description: string
  groups: ModelPlazaGroup[]
}

/** Legacy public model plaza data used by the custom LLM Free home page. */
export interface LegacyModelPlazaGroup {
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

export async function getModelPlaza(options?: { signal?: AbortSignal }): Promise<ModelPlazaResponse> {
  const { data } = await apiClient.get<ModelPlazaResponse>('/model-plaza', {
    signal: options?.signal
  })
  return data
}

export const modelPlazaAPI = { getModelPlaza }

export async function getModelPlazaGroups(): Promise<LegacyModelPlazaGroup[]> {
  const { data } = await apiClient.get<LegacyModelPlazaGroup[]>('/model-plaza/groups')
  return data
}

export async function getOfficialModelPrices(): Promise<OfficialModelPricesResponse> {
  const endpoint = import.meta.env.DEV
    ? 'https://llmfree.work/api/official-model-prices'
    : '/api/official-model-prices'
  const response = await fetch(endpoint, {
    headers: { Accept: 'application/json' },
    cache: 'no-store'
  })
  if (!response.ok) throw new Error(`HTTP ${response.status}`)
  const payload = await response.json() as OfficialModelPricesResponse
  if (!payload.models || typeof payload.models !== 'object') {
    throw new Error('Invalid price response')
  }
  return payload
}

export default modelPlazaAPI
