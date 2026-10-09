import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { announcementsAPI } from '@/api'
import { entryPopupsHeld, whenEntryPopupsReleased } from '@/composables/entryPopupGate'
import type { UserAnnouncement } from '@/types'

const THROTTLE_MS = 20 * 60 * 1000 // 20 minutes

// 本次登录的公告弹窗处理完之前，首次漫游引导先不开始。
let announcementEntrySettled = false
const announcementEntryWaiters: Array<() => void> = []

function resetAnnouncementEntryGate() {
  announcementEntrySettled = false
  announcementEntryWaiters.length = 0
}

function markAnnouncementEntrySettled() {
  if (announcementEntrySettled) return
  announcementEntrySettled = true
  const pending = announcementEntryWaiters.splice(0, announcementEntryWaiters.length)
  for (const waiter of pending) waiter()
}

/** 公告弹窗已关闭或这次没有要弹的公告时执行。返回取消函数。 */
export function whenAnnouncementEntrySettled(fn: () => void): () => void {
  if (announcementEntrySettled) {
    fn()
    return () => {}
  }
  announcementEntryWaiters.push(fn)
  return () => {
    const index = announcementEntryWaiters.indexOf(fn)
    if (index >= 0) announcementEntryWaiters.splice(index, 1)
  }
}

export const useAnnouncementStore = defineStore('announcements', () => {
  resetAnnouncementEntryGate()

  // State
  const announcements = ref<UserAnnouncement[]>([])
  const loading = ref(false)
  const lastFetchTime = ref(0)
  const popupQueue = ref<UserAnnouncement[]>([])
  const currentPopup = ref<UserAnnouncement | null>(null)

  // Session-scoped dedup set — not reactive, used as plain lookup only
  let shownPopupIds = new Set<number>()
  let fetchGeneration = 0
  let fetchesInFlight = 0
  let popupReleaseArmed = false

  // Getters
  const unreadCount = computed(() =>
    announcements.value.filter((a) => !a.read_at).length
  )

  // Actions
  function settleAnnouncementEntryIfIdle() {
    // 请求还在飞时队列是空的，这时不能放行，否则引导会和随后弹出的公告叠在一起。
    if (fetchesInFlight > 0) return
    if (currentPopup.value || popupQueue.value.length > 0) return
    markAnnouncementEntrySettled()
  }

  async function fetchAnnouncements(force = false) {
    const now = Date.now()
    if (!force && lastFetchTime.value > 0 && now - lastFetchTime.value < THROTTLE_MS) {
      settleAnnouncementEntryIfIdle()
      return
    }

    // Set immediately to prevent concurrent duplicate requests
    lastFetchTime.value = now
    const generation = ++fetchGeneration
    fetchesInFlight++

    try {
      loading.value = true
      const all = await announcementsAPI.list(false)
      if (generation !== fetchGeneration) return
      announcements.value = all.slice(0, 20)
      enqueueNewPopups()
    } catch (err: any) {
      if (generation !== fetchGeneration) return
      // Revert throttle timestamp on failure so retry is allowed
      lastFetchTime.value = 0
      console.error('Failed to fetch announcements:', err)
    } finally {
      fetchesInFlight--
      if (generation === fetchGeneration) {
        loading.value = false
        settleAnnouncementEntryIfIdle()
      }
    }
  }

  function enqueueNewPopups() {
    const newPopups = announcements.value.filter(
      (a) => a.notify_mode === 'popup' && !a.read_at && !shownPopupIds.has(a.id)
    )
    if (newPopups.length === 0) return

    for (const p of newPopups) {
      if (!popupQueue.value.some((q) => q.id === p.id)) {
        popupQueue.value.push(p)
      }
    }

    if (!currentPopup.value) {
      showNextPopup()
    }
  }

  function showNextPopup() {
    if (entryPopupsHeld.value) {
      if (!popupReleaseArmed) {
        popupReleaseArmed = true
        whenEntryPopupsReleased(() => {
          popupReleaseArmed = false
          if (!currentPopup.value) showNextPopup()
        })
      }
      return
    }
    if (popupQueue.value.length === 0) {
      currentPopup.value = null
      return
    }
    currentPopup.value = popupQueue.value.shift()!
    shownPopupIds.add(currentPopup.value.id)
  }

  async function dismissPopup() {
    if (!currentPopup.value) return
    const id = currentPopup.value.id
    currentPopup.value = null

    // Mark as read (fire-and-forget, UI already updated)
    markAsRead(id)

    // Show next popup after a short delay. 全部关完后才允许首次引导。
    if (popupQueue.value.length > 0) {
      setTimeout(() => showNextPopup(), 300)
      return
    }
    settleAnnouncementEntryIfIdle()
  }

  async function markAsRead(id: number) {
    try {
      await announcementsAPI.markRead(id)
      const ann = announcements.value.find((a) => a.id === id)
      if (ann) {
        ann.read_at = new Date().toISOString()
      }
    } catch (err: any) {
      console.error('Failed to mark announcement as read:', err)
    }
  }

  async function markAllAsRead() {
    const unread = announcements.value.filter((a) => !a.read_at)
    if (unread.length === 0) return

    try {
      loading.value = true
      const results = await Promise.allSettled(unread.map(async (a) => {
        await announcementsAPI.markRead(a.id)
        a.read_at = new Date().toISOString()
      }))
      const failure = results.find((result) => result.status === 'rejected')
      if (failure) throw failure.reason
    } catch (err: any) {
      console.error('Failed to mark all as read:', err)
      throw err
    } finally {
      loading.value = false
    }
  }

  function reset() {
    resetAnnouncementEntryGate()
    fetchGeneration++
    announcements.value = []
    lastFetchTime.value = 0
    shownPopupIds = new Set()
    popupQueue.value = []
    currentPopup.value = null
    loading.value = false
  }

  return {
    // State
    announcements,
    loading,
    currentPopup,
    // Getters
    unreadCount,
    // Actions
    fetchAnnouncements,
    dismissPopup,
    markAsRead,
    markAllAsRead,
    reset,
  }
})
