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
  /** Development-only fallback payload marker. */
  demo?: boolean
}

export async function getModelPlaza(options?: { signal?: AbortSignal }): Promise<ModelPlazaResponse> {
  try {
    const { data } = await apiClient.get<ModelPlazaResponse>('/model-plaza/public', {
      signal: options?.signal
    })
    if (!import.meta.env.DEV || data.groups.length > 0) return data
  } catch (error) {
    if (!import.meta.env.DEV || options?.signal?.aborted) throw error
  }

  const { createDevModelPlazaResponse } = await import('./modelPlazaMock')
  return createDevModelPlazaResponse()
}

export const modelPlazaAPI = { getModelPlaza }

export default modelPlazaAPI
