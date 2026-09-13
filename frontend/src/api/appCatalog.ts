import { apiClient } from './client'
import type { CatalogApp } from './admin/appCatalog'

export type { CatalogApp }

export async function listCatalogApps(): Promise<{ items: CatalogApp[]; total: number }> {
  const { data } = await apiClient.get<{ items: CatalogApp[]; total: number }>('/app-catalog')
  return { items: data?.items || [], total: data?.total || 0 }
}
