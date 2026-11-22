import { apiClient } from '../client'

export interface LatencyScope { user_id: number; group_id: number; account_id: number }
export interface LatencySession extends LatencyScope {
  sequence: number
  id: string
  created_by: number
  started_at: string
  stopped_at: string | null
  stop_reason: string
  record_count: number
}
export interface LatencyEvent { name: string; at_ms: number; duration_ms?: number; attempt?: number }
export interface LatencyAttempt {
  number: number; account_id: number; transport: string; request_bytes: number
  status?: number; failed: boolean; reused?: boolean
}
export interface LatencyRecord {
  id: number; collection_id: string; request_id: string; client_request_id: string
  started_at: string; status: number; request_bytes: number; duration_ms: number
  events: LatencyEvent[]; attempts: LatencyAttempt[]; truncated: boolean
}
export interface LatencyHealth {
  queue_depth: number; dropped: number; write_failed: number; refresh_failed: number; ready: boolean
}
interface Page<T> { items: T[]; next_before: number }
const path = '/admin/ops/latency-collections'
export const latencyCollectionsAPI = {
  async list(collection_id = '', before = 0) {
    return (await apiClient.get<Page<LatencySession> & { health: LatencyHealth }>(path, { params: { collection_id, before } })).data
  },
  async create(scope: LatencyScope) { return (await apiClient.post<LatencySession>(path, scope)).data },
  async stop(id: string) { await apiClient.post(`${path}/${encodeURIComponent(id)}/stop`) },
  async records(id: string, request_id = '', before = 0) {
    return (await apiClient.get<Page<LatencyRecord>>(`${path}/${encodeURIComponent(id)}/records`, { params: { request_id, before } })).data
  }
}
