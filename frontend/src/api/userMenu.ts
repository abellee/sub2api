import { apiClient } from './client'

export interface UserMenuSection {
  id: string
  title: string
  items: string[]
}

export interface UserMenuConfig {
  sections: UserMenuSection[]
}

export async function getUserMenu(admin = false): Promise<UserMenuConfig> {
  const { data } = await apiClient.get<UserMenuConfig>(admin ? '/admin/user-menu' : '/user/menu')
  return { sections: data.sections ?? [] }
}

export async function saveUserMenu(config: UserMenuConfig): Promise<UserMenuConfig> {
  const { data } = await apiClient.put<UserMenuConfig>('/admin/user-menu', config)
  return data
}
