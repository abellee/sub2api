import { apiClient } from './client'
import { i18n } from '@/i18n'

export interface UserMenuSection {
  id: string
  title: string
  items: string[]
}

export interface UserMenuConfig {
  sections: UserMenuSection[]
  hidden_items?: string[]
}

export async function getUserMenu(admin = false): Promise<UserMenuConfig> {
  const { data } = await apiClient.get<UserMenuConfig>(admin ? '/admin/user-menu' : '/user/menu')
  return { sections: data.sections ?? [], hidden_items: data.hidden_items ?? [] }
}

export async function saveUserMenu(config: UserMenuConfig): Promise<UserMenuConfig> {
  const { data } = await apiClient.put<UserMenuConfig>('/admin/user-menu', config)
  const requested = new Set(config.hidden_items ?? [])
  const saved = new Set(data.hidden_items ?? [])
  if (requested.size !== saved.size || [...requested].some(path => !saved.has(path))) {
    throw new Error(i18n.global.t('admin.menuManagement.visibilityNotSaved'))
  }
  return data
}
