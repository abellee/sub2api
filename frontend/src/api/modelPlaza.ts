import { apiClient } from './client'
import type { UserPricingInterval, UserSupportedModelPricing } from './channels'

/** 官方参考价（USD per token，与计费目录同源；字段缺失 = 目录未覆盖）。 */
export interface PlazaOfficialPricing {
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price?: number | null
  cache_read_price: number | null
  intervals?: UserPricingInterval[]
}

/** 分时倍率时段：配置时区当天 [start_time, end_time) 内整单实付乘 multiplier。 */
export interface PlazaTimePricingPeriod {
  start_time: string
  end_time: string
  multiplier: number
}

/** 计费会生效的分时倍率（仅倍率 ≠ 1 的时段，已按开始时间升序）。 */
export interface PlazaTimePricing {
  /** IANA 时区名，如 Asia/Shanghai。 */
  timezone: string
  /** true 时时段仅周一至周五生效，周末整天按标准价计费。 */
  weekdays_only?: boolean
  periods: PlazaTimePricingPeriod[]
}

export interface PlazaModel {
  name: string
  platform: string
  /** 实收口径的基础展示定价；均为标准时段价。 */
  pricing: UserSupportedModelPricing | null
  official_pricing: PlazaOfficialPricing | null
  long_context_basis?: 'whole_request' | 'marginal'
  /** 仅配置了分时倍率的模型返回。 */
  time_pricing?: PlazaTimePricing
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
  video_rate_independent: boolean
  video_rate_multiplier: number
  models: PlazaModel[]
}

export interface ModelPlazaResponse {
  description: string
  groups: ModelPlazaGroup[]
  /** Development-only fallback payload marker. */
  demo?: boolean
}

export async function getModelPlaza(options?: { signal?: AbortSignal }): Promise<ModelPlazaResponse> {
  const useLiveBackend = Boolean(import.meta.env.VITE_DEV_PROXY_TARGET)
  try {
    const { data } = await apiClient.get<ModelPlazaResponse>('/model-plaza/public', {
      signal: options?.signal
    })
    if (useLiveBackend || !import.meta.env.DEV || data.groups.length > 0) return data
  } catch (error) {
    if (useLiveBackend || !import.meta.env.DEV || options?.signal?.aborted) throw error
  }

  const { createDevModelPlazaResponse } = await import('./modelPlazaMock')
  return createDevModelPlazaResponse()
}

export const modelPlazaAPI = { getModelPlaza }

export default modelPlazaAPI
