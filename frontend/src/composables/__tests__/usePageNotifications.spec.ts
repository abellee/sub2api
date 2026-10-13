import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { dismissPageNotification, showPageNotification, usePageNotifications } from '../usePageNotifications'

function clearNotices() {
  for (const item of [...usePageNotifications().items.value]) {
    dismissPageNotification(item.id)
  }
}

describe('usePageNotifications', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    clearNotices()
  })

  afterEach(() => {
    clearNotices()
    vi.useRealTimers()
  })

  it('puts a newer notice after the older ones', () => {
    const first = showPageNotification({ title: '第一', body: '先到' })
    const second = showPageNotification({ title: '第二', body: '后到' })
    const titles = usePageNotifications().items.value.map((item) => item.title)

    expect([first, second]).toEqual(usePageNotifications().items.value.map((item) => item.id))
    expect(titles).toEqual(['第一', '第二'])
  })

  it('closes a notice from the close action and cancels the timer', () => {
    const id = showPageNotification({ title: '可关' })
    dismissPageNotification(id)
    vi.advanceTimersByTime(5000)

    expect(usePageNotifications().items.value).toEqual([])
  })

  it('closes a notice five seconds after it appears', () => {
    showPageNotification({ title: '自动' })
    vi.advanceTimersByTime(4999)
    expect(usePageNotifications().items.value).toHaveLength(1)

    vi.advanceTimersByTime(1)
    expect(usePageNotifications().items.value).toEqual([])
  })

  it('keeps the image and ignores a repeated notice key', () => {
    const image = 'https://cdn.example.test/notice.jpg'
    const first = showPageNotification({
      key: 'notice-1',
      title: '带图',
      body: '正文',
      image,
      url: '/lottery',
    })
    const repeat = showPageNotification({ key: 'notice-1', title: '带图', image })
    const notice = usePageNotifications().items.value[0]

    expect(first).toBeGreaterThan(0)
    expect(repeat).toBe(0)
    expect(notice?.image).toBe(image)
    expect(notice?.url).toBe('/lottery')
    expect(usePageNotifications().items.value).toHaveLength(1)
  })

  it('ignores a blank title', () => {
    expect(showPageNotification({ title: '   ', body: '空标题' })).toBe(0)
    expect(usePageNotifications().items.value).toEqual([])
  })
})
