import { apiClient } from '../client'

export type Fields = Record<string, string | number | boolean>
export interface ConfigView {
  settings: Fields
  active: Fields
  restart_required: boolean
  pending_fields: string[]
}
export interface Binding {
  group_id: number
  schema_id: string
  estimate_kind: string
  enabled: boolean
}
export interface DownloadRecord { id: string; user_id: number; task_id: string; file_id: string; model: string; download_url: string; content_type: string; size_bytes: number; created_at: string }
const base = '/admin/media-service'
export const mediaServiceAPI = {
  async records(userID: number): Promise<{ records: DownloadRecord[] }> { return (await apiClient.get(`${base}/records`, { params: { user_id: userID } })).data },
  async config(): Promise<ConfigView> { return (await apiClient.get(`${base}/config`)).data },
  async saveConfig(body: Fields): Promise<ConfigView> { return (await apiClient.put(`${base}/config`, body)).data },
  async bindings(): Promise<{ bindings: Binding[] }> { return (await apiClient.get(`${base}/bindings`)).data },
  async saveBinding(body: Binding): Promise<Binding> { return (await apiClient.put(`${base}/bindings/${body.group_id}`, body)).data },
  async deleteBinding(id: number): Promise<void> { await apiClient.delete(`${base}/bindings/${id}`) },
  async agent<T = Record<string, unknown>>(method: string, path: string, data?: unknown): Promise<T> {
    return (await apiClient.request({ method, url: `${base}/agent${path}`, data, timeout: 610000 })).data
  }
}
