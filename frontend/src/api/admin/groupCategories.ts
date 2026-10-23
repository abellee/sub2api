import { apiClient } from '../client'

export interface GroupCategory {
  id: string
  name: string
  description?: string
  sort_order?: number
  created_at: string
  updated_at: string
}

export interface GroupCategorySnapshot {
  categories: GroupCategory[]
  assignments: Record<string, string>
  /** 分组上限倍率:分组 ID(十进制字符串)→ 倍率;与分类同库存储。 */
  group_caps?: Record<string, number>
}

export async function list(): Promise<GroupCategorySnapshot> {
  const { data } = await apiClient.get<GroupCategorySnapshot>('/admin/group-categories')
  return {
    categories: data?.categories ?? [],
    assignments: data?.assignments ?? {},
    group_caps: data?.group_caps ?? {}
  }
}

export async function create(payload: {
  name: string
  description?: string
}): Promise<GroupCategory> {
  const { data } = await apiClient.post<GroupCategory>('/admin/group-categories', payload)
  return data
}

export async function update(
  id: string,
  payload: { name?: string; description?: string }
): Promise<GroupCategory> {
  const { data } = await apiClient.put<GroupCategory>(`/admin/group-categories/${id}`, payload)
  return data
}

export async function remove(id: string): Promise<void> {
  await apiClient.delete(`/admin/group-categories/${id}`)
}

export async function assign(groupId: number, categoryId: string | null): Promise<void> {
  await apiClient.put(`/admin/group-categories/assignments/${groupId}`, {
    category_id: categoryId
  })
}

export async function setGroupCap(groupId: number, capRate: number | null): Promise<void> {
  await apiClient.put(`/admin/group-categories/group-caps/${groupId}`, {
    cap_rate: capRate
  })
}

export async function reorder(ids: string[]): Promise<void> {
  await apiClient.put('/admin/group-categories/sort-order', { ids })
}

const groupCategoriesAPI = {
  list,
  create,
  update,
  remove,
  assign,
  setGroupCap,
  reorder
}

export default groupCategoriesAPI
