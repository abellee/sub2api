import { apiClient } from './client'

export interface DocumentationCategory { id: string; name: string }
export interface DocumentationAgent {
  enabled: boolean
  title: string
  description: string
  template: string
  base_url: string
  key_mode: 'all' | 'groups' | 'manual' | 'none'
  group_ids: number[]
}
export interface DocumentationArticle {
  id: string
  name: string
  logo: string
  category_id: string
  content: string
  published: boolean
  agent: DocumentationAgent
}
export interface DocumentationSnapshot {
  revision: number
  categories: DocumentationCategory[]
  articles: DocumentationArticle[]
}
export async function getDocumentation(category?: string): Promise<DocumentationSnapshot> {
  const { data } = await apiClient.get<DocumentationSnapshot>('/documentation', { params: { category } })
  return data
}
export async function getAdminDocumentation(): Promise<DocumentationSnapshot> {
  const { data } = await apiClient.get<DocumentationSnapshot>('/admin/documentation')
  return data
}
export async function getDocumentationCategories(): Promise<DocumentationCategory[]> {
  const { data } = await apiClient.get<DocumentationCategory[]>('/documentation/categories')
  return data
}
export async function downloadDocumentationSkill(id: string): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(`/documentation/${encodeURIComponent(id)}/skill`, { responseType: 'blob' })
  return data
}
export async function saveDocumentation(snapshot: DocumentationSnapshot): Promise<DocumentationSnapshot> {
  const { data } = await apiClient.put<DocumentationSnapshot>('/admin/documentation', snapshot)
  return data
}
