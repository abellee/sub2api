/**
 * User Announcements API endpoints
 */

import { apiClient } from './client'
import type { UserAnnouncement } from '@/types'
import type { AnnouncementEmoji, AnnouncementReaction } from '@/components/common/announcementReactions'

export async function list(unreadOnly: boolean = false): Promise<UserAnnouncement[]> {
  const { data } = await apiClient.get<UserAnnouncement[]>('/announcements', {
    params: unreadOnly ? { unread_only: 1 } : {}
  })
  return data
}

export async function markRead(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(`/announcements/${id}/read`)
  return data
}

const announcementsAPI = {
  list,
  markRead,
  async reactions(id: number): Promise<AnnouncementReaction[]> {
    const { data } = await apiClient.get<AnnouncementReaction[]>(`/announcements/${id}/reactions`)
    return data
  },
  async setReaction(id: number, emojiID: AnnouncementEmoji | null): Promise<AnnouncementReaction[]> {
    const path = `/announcements/${id}/reactions`
    const { data } = emojiID
      ? await apiClient.put<AnnouncementReaction[]>(path, { emoji_id: emojiID })
      : await apiClient.delete<AnnouncementReaction[]>(path)
    return data
  }
}

export default announcementsAPI

