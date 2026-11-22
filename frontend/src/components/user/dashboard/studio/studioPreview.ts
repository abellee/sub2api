import type { UserDashboardStats } from '@/api/usage'
import type { ModelStat, TrendDataPoint } from '@/types'
import { formatDateLocalInput } from '@/utils/format'

// Only loaded by the development-only preview; never changes account data.
export function createStudioPreview(start: string, end: string, granularity: 'day' | 'hour') {
  const first = new Date(`${start}T00:00:00`)
  const last = new Date(`${end}T23:59:59`)
  const step = granularity === 'hour' ? 3600000 : 86400000
  const count = Math.max(0, Math.min(2200, Math.floor((last.getTime() - first.getTime()) / step) + 1))
  const trend: TrendDataPoint[] = Array.from({ length: count }, (_, index) => {
    const date = new Date(first.getTime() + index * step)
    const activity = (1 + Math.sin(index * 1.7) * 0.32 + Math.cos(index * 0.7) * 0.2) * (granularity === 'hour' ? 1 / 24 : 1)
    const input_tokens = Math.round(480000 * activity)
    const output_tokens = Math.round(135000 * activity * (1 + Math.sin(index) * 0.2))
    const cache_creation_tokens = Math.round(65000 * activity)
    const cache_read_tokens = Math.round(780000 * activity * (1 + Math.cos(index) * 0.18))
    const cost = Number((8.7 * activity).toFixed(4))
    return {
      date: granularity === 'day' ? formatDateLocalInput(date) : `${formatDateLocalInput(date)} ${String(date.getHours()).padStart(2, '0')}:00`,
      requests: Math.round(1280 * activity), input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
      total_tokens: input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens,
      cost, actual_cost: Number((cost * 0.64).toFixed(4))
    }
  })
  const sum = (key: Exclude<keyof TrendDataPoint, 'date'>) => trend.reduce((value, point) => value + point[key], 0)
  const modelShares = [
    ['gpt-6.1-sol', 0.32], ['claude-sonnet-4-6', 0.25], ['gemini-2.5-pro', 0.19],
    ['deepseek-chat', 0.12], ['gpt-6-astra', 0.08], ['qwen3-max', 0.04]
  ] as const
  const models: ModelStat[] = modelShares.map(([model, share]) => {
    const input_tokens = Math.floor(sum('input_tokens') * share)
    const output_tokens = Math.floor(sum('output_tokens') * share)
    const cache_creation_tokens = Math.floor(sum('cache_creation_tokens') * share)
    const cache_read_tokens = Math.floor(sum('cache_read_tokens') * share)
    return {
      model, requests: Math.round(sum('requests') * share), input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
      total_tokens: input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens,
      cost: Number((sum('cost') * share).toFixed(4)),
      actual_cost: Number((sum('actual_cost') * share).toFixed(4))
    }
  })
  const stats: UserDashboardStats = {
    total_api_keys: 8, active_api_keys: 6, total_requests: 168420,
    total_input_tokens: 248600000, total_output_tokens: 58400000,
    total_cache_creation_tokens: 31200000, total_cache_read_tokens: 482800000,
    total_tokens: 821000000, total_cost: 2468.72, total_actual_cost: 1580.48,
    today_requests: 1286, today_input_tokens: 486000, today_output_tokens: 138000,
    today_cache_creation_tokens: 64000, today_cache_read_tokens: 792000,
    today_tokens: 1480000, today_cost: 8.764, today_actual_cost: 5.609,
    average_duration_ms: 842, rpm: 24, tpm: 28600
  }
  return { stats, trend, models, balance: 286.5 }
}
