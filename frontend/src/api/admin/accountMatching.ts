import { apiClient } from '../client'
import usersAPI from './users'

export interface AccountMatchingRule {
  id: number
  user_id: number
  email: string
  group_ids: number[]
  account_ids: number[]
  created_at: string
  updated_at: string
}
export interface AccountMatchingUser { id: number; email: string }
export type AccountMatchingInput = Pick<AccountMatchingRule, 'user_id' | 'group_ids' | 'account_ids'>

export const accountMatchingAPI = {
  async list() {
    return (await apiClient.get<AccountMatchingRule[]>('/admin/account-matching')).data
  },
  async searchUsers(email: string, signal?: AbortSignal) {
    const search = email.trim()
    if (!search) return []
    const users: AccountMatchingUser[] = []
    for (let page = 1; ; page++) {
      const data = await usersAPI.list(page, 100, {
        search, include_subscriptions: false, sort_by: 'email', sort_order: 'asc'
      }, { signal })
      // The existing endpoint also matches usernames; this picker only matches email.
      for (const user of data.items) {
        if (user.email.toLowerCase().includes(search.toLowerCase())) users.push({ id: user.id, email: user.email })
        if (users.length === 30) return users
      }
      if (page >= data.pages || data.items.length === 0) return users
    }
  },
  async save(input: AccountMatchingInput, id?: number) {
    return id
      ? (await apiClient.put<AccountMatchingRule>(`/admin/account-matching/${id}`, input)).data
      : (await apiClient.post<AccountMatchingRule>('/admin/account-matching', input)).data
  },
  async delete(id: number) { await apiClient.delete(`/admin/account-matching/${id}`) }
}
