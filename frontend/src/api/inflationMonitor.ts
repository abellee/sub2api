import { apiClient } from './client'
export type InflationAggregation = 'mean' | 'median'
export interface InflationPlan {
  group_id: number
  enabled: boolean
  interval_minutes: number
  account_ids: number[]
  model: string
  prompt: string
  encoding: string
  reasoning_effort: string
  aggregation: InflationAggregation
  yellow_threshold: number
  red_threshold: number
}
export interface InflationSample {
  model: string
  input_tokens: number | null
  output_tokens: number | null
  text_tokens: number
  reasoning_tokens: number | null
  rate: number | null
  error?: string
}
export interface InflationResult {
  measured_at: string
  prompt: string
  encoding: string
  reasoning_effort: string
  samples: InflationSample[]
}
export interface InflationSummary {
  group_id: number
  rate: number | null
  aggregation: InflationAggregation
  yellow_threshold: number
  red_threshold: number
  measured_at: string | null
}
export interface InflationHistory extends Omit<InflationSummary, 'rate' | 'measured_at'> {
  results: InflationResult[]
}
export const inflationMonitorAPI = {
  async plans() { return (await apiClient.get<InflationPlan[]>('/admin/inflation-monitor')).data },
  async save(plans: InflationPlan[]) { await apiClient.put('/admin/inflation-monitor', plans) },
  async summaries(signal?: AbortSignal) { return (await apiClient.get<InflationSummary[]>('/inflation-monitor', { signal })).data },
  async history(keyId: number, signal?: AbortSignal) { return (await apiClient.get<InflationHistory>(`/keys/${keyId}/inflation-monitor`, { signal })).data },
}
export function inflationAggregate(samples: InflationSample[], mode: InflationAggregation): number | null {
  const values = samples.filter(s => !s.error && s.rate !== null && Number.isFinite(s.rate)).map(s => s.rate!).sort((a, b) => a - b)
  if (!values.length) return null
  if (mode === 'mean') return values.reduce((a, b) => a + b, 0) / values.length
  const middle = Math.floor(values.length / 2)
  return values.length % 2 ? values[middle]! : (values[middle - 1]! + values[middle]!) / 2
}
export function inflationTone(rate: number | null, yellow: number, red: number): 'unknown' | 'healthy' | 'warning' | 'critical' {
  return rate === null ? 'unknown' : rate >= red ? 'critical' : rate >= yellow ? 'warning' : 'healthy'
}
export function inflationCandles(history: InflationHistory) {
  let previous: number | null = null
  return history.results.map(result => {
    const close = inflationAggregate(result.samples, history.aggregation)
    if (close === null) return null
    const open = previous ?? close
    const rates = result.samples.filter(s => !s.error && s.rate !== null).map(s => s.rate!)
    previous = close
    return { open, close, high: Math.max(open, close, ...rates), low: Math.min(open, close, ...rates) }
  })
}
