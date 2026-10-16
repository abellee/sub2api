import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAnnouncementStore, whenAnnouncementEntrySettled } from '../announcements'
import { holdEntryPopups, releaseEntryPopups } from '@/composables/entryPopupGate'
import type { UserAnnouncement } from '@/types'

const list = vi.hoisted(() => vi.fn())
vi.mock('@/api', () => ({ announcementsAPI: { list } }))

const notice = (id: number): UserAnnouncement => ({
  id, title: `Notice ${id}`, content: 'Content', notify_mode: 'popup',
  created_at: '2026-09-18T00:00:00Z', updated_at: '2026-09-18T00:00:00Z'
})

function pendingList() {
  let resolve!: (value: UserAnnouncement[]) => void
  let reject!: (reason: Error) => void
  const promise = new Promise<UserAnnouncement[]>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

beforeEach(() => {
  setActivePinia(createPinia())
  releaseEntryPopups()
  vi.resetAllMocks()
  vi.spyOn(console, 'error').mockImplementation(() => {})
})
afterEach(() => {
  releaseEntryPopups()
  vi.restoreAllMocks()
})

describe('announcement fetch ownership', () => {
  it('does not restore announcements or popups after logout resets the store', async () => {
    const store = useAnnouncementStore()
    const old = pendingList()
    list.mockReturnValueOnce(old.promise)
    const request = store.fetchAnnouncements()
    store.reset()
    old.resolve([notice(1)])
    await request
    expect(store.announcements).toEqual([])
    expect(store.currentPopup).toBeNull()
    expect(store.loading).toBe(false)
  })

  it('keeps the new session loading when an old request finishes', async () => {
    const store = useAnnouncementStore()
    const old = pendingList()
    const current = pendingList()
    list.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise)
    const oldRequest = store.fetchAnnouncements()
    store.reset()
    const currentRequest = store.fetchAnnouncements()
    old.resolve([])
    await oldRequest
    expect(store.loading).toBe(true)
    current.resolve([notice(2)])
    await currentRequest
    expect(store.announcements.map(a => a.id)).toEqual([2])
    expect(store.loading).toBe(false)
  })

  it('keeps the latest forced refresh when responses arrive out of order', async () => {
    const store = useAnnouncementStore()
    const old = pendingList()
    list.mockReturnValueOnce(old.promise).mockResolvedValueOnce([notice(2)])
    const oldRequest = store.fetchAnnouncements()
    await store.fetchAnnouncements(true)
    old.resolve([notice(1)])
    await oldRequest
    expect(store.announcements.map(a => a.id)).toEqual([2])
    expect(store.currentPopup?.id).toBe(2)
  })

  it('does not clear the new session throttle when an old request rejects', async () => {
    const store = useAnnouncementStore()
    const old = pendingList()
    list.mockReturnValueOnce(old.promise).mockResolvedValue([notice(2)])
    const oldRequest = store.fetchAnnouncements()
    store.reset()
    await store.fetchAnnouncements()
    old.reject(new Error('old request failed'))
    await oldRequest
    await store.fetchAnnouncements()
    expect(list).toHaveBeenCalledTimes(2)
    expect(store.announcements.map(a => a.id)).toEqual([2])
  })

  it('still retries a failed current request', async () => {
    const store = useAnnouncementStore()
    list.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce([notice(1)])
    await store.fetchAnnouncements()
    await store.fetchAnnouncements()
    expect(list).toHaveBeenCalledTimes(2)
    expect(store.currentPopup?.id).toBe(1)
  })

  it('keeps a popup announcement queued until entry guides release it', async () => {
    holdEntryPopups()
    const store = useAnnouncementStore()
    list.mockResolvedValueOnce([notice(3)])
    await store.fetchAnnouncements(true)
    expect(store.currentPopup).toBeNull()
    releaseEntryPopups()
    expect(store.currentPopup?.id).toBe(3)
  })

  it('lets the first tour start only after the last popup is closed', async () => {
    const store = useAnnouncementStore()
    let started = false
    whenAnnouncementEntrySettled(() => { started = true })
    list.mockResolvedValueOnce([notice(3), notice(4)])
    await store.fetchAnnouncements(true)
    expect(started).toBe(false)
    expect(store.currentPopup?.id).toBe(3)

    vi.useFakeTimers()
    try {
      store.dismissPopup()
      expect(started).toBe(false)
      await vi.advanceTimersByTimeAsync(300)
      expect(store.currentPopup?.id).toBe(4)
      expect(started).toBe(false)
      store.dismissPopup()
      expect(started).toBe(true)
    } finally {
      vi.useRealTimers()
    }
  })

  it('does not release the first tour from a throttled fetch while the request is still open', async () => {
    const store = useAnnouncementStore()
    let started = false
    whenAnnouncementEntrySettled(() => { started = true })
    const pending = pendingList()
    list.mockReturnValueOnce(pending.promise)
    const request = store.fetchAnnouncements()
    await store.fetchAnnouncements()
    expect(started).toBe(false)
    pending.resolve([notice(1)])
    await request
    expect(store.currentPopup?.id).toBe(1)
    expect(started).toBe(false)
    store.dismissPopup()
    expect(started).toBe(true)
  })

  it('starts the first tour as soon as a fetch has no popup', async () => {
    const store = useAnnouncementStore()
    let started = false
    whenAnnouncementEntrySettled(() => { started = true })
    list.mockResolvedValueOnce([])
    await store.fetchAnnouncements()
    expect(started).toBe(true)
  })

  it('does not let an old empty fetch release the next session tour', async () => {
    const store = useAnnouncementStore()
    const old = pendingList()
    list.mockReturnValueOnce(old.promise)
    const oldRequest = store.fetchAnnouncements()
    store.reset()
    let started = false
    whenAnnouncementEntrySettled(() => { started = true })
    old.resolve([])
    await oldRequest
    expect(started).toBe(false)
  })

  it('still throttles ordinary concurrent fetches', async () => {
    const store = useAnnouncementStore()
    const current = pendingList()
    list.mockReturnValueOnce(current.promise)
    const request = store.fetchAnnouncements()
    await store.fetchAnnouncements()
    expect(list).toHaveBeenCalledTimes(1)
    expect(store.loading).toBe(true)
    current.resolve([])
    await request
    expect(store.loading).toBe(false)
  })
})
