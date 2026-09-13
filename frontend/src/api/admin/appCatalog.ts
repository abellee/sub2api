import { apiClient } from '../client'

export interface CatalogApp {
  id: number
  official_url: string
  github_url: string
  name: string
  icon_url: string
  description: string
  download_page_url: string
  screenshots: string[]
  changelog: string
  version: string
  created_at: string
  updated_at: string
  last_fetched_at?: string | null
  fetch_error?: string
}

export interface CatalogAppDraft {
  official_url?: string
  github_url?: string
  name?: string
  icon_url?: string
  description?: string
  download_page_url?: string
  screenshots?: string[]
  changelog?: string
  version?: string
}

export interface AppCatalogAgentHealth {
  enabled: boolean
  reason?: string
  base_url: string
  status?: string
  version?: string
  uptime_seconds?: number
}

export async function getHealth(): Promise<AppCatalogAgentHealth> {
  const { data } = await apiClient.get<AppCatalogAgentHealth>('/admin/app-catalog/agent/health')
  return data
}

export async function list(): Promise<{ items: CatalogApp[]; total: number }> {
  const { data } = await apiClient.get<{ items: CatalogApp[]; total: number }>('/admin/app-catalog')
  return { items: data?.items || [], total: data?.total || 0 }
}

export async function create(payload: CatalogAppDraft): Promise<CatalogApp> {
  const { data } = await apiClient.post<CatalogApp>('/admin/app-catalog', payload, { timeout: 90000 })
  return data
}

export async function update(id: number, payload: CatalogAppDraft): Promise<CatalogApp> {
  const { data } = await apiClient.put<CatalogApp>(`/admin/app-catalog/${id}`, payload, { timeout: 90000 })
  return data
}

export async function fetchPreview(payload: CatalogAppDraft): Promise<CatalogAppDraft> {
  const { data } = await apiClient.post<CatalogAppDraft>('/admin/app-catalog/fetch', payload, { timeout: 90000 })
  return data || {}
}

export async function remove(id: number): Promise<void> {
  await apiClient.delete(`/admin/app-catalog/${id}`)
}

const appCatalogAPI = { getHealth, list, create, update, fetchPreview, remove }
export default appCatalogAPI
