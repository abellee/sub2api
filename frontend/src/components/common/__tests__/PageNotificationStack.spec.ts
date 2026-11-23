import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import PageNotificationStack from '../PageNotificationStack.vue'
import { dismissPageNotification, showPageNotification, usePageNotifications } from '@/composables/usePageNotifications'
import { listPageNotifications } from '@/api/pushNotifications'

vi.mock('@/api/pushNotifications', () => ({ listPageNotifications: vi.fn() }))

function clearNotices() {
  for (const item of [...usePageNotifications().items.value]) {
    dismissPageNotification(item.id)
  }
}

describe('PageNotificationStack', () => {
  beforeEach(() => { localStorage.clear() })
  afterEach(() => {
    clearNotices()
  })

  it('floats the stack on the body and stacks a new notice under the older one', async () => {
    showPageNotification({ title: '旧通知', body: '先到' })
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    })
    const wrapper = mount(PageNotificationStack, {
      attachTo: document.body,
      global: { plugins: [router] },
    })

    expect(document.body.querySelector('[data-testid="page-notification-test"]')).toBeNull()

    showPageNotification({
      title: '新通知',
      body: '后到',
      image: 'https://example.com/notice.png',
    })
    await wrapper.vm.$nextTick()

    const stack = document.body.querySelector('[data-testid="page-notification-stack"]')
    expect(stack?.parentElement).toBe(document.body)
    expect(stack?.classList.contains('fixed')).toBe(true)
    expect(stack?.classList.contains('bottom-5')).toBe(true)
    expect(stack?.classList.contains('right-5')).toBe(true)
    expect(stack?.classList.contains('relative')).toBe(false)

    const cards = [...document.body.querySelectorAll('[data-testid="page-notification"]')]
    expect(cards).toHaveLength(2)
    expect(cards[0]?.textContent).toContain('旧通知')
    expect(cards[1]?.textContent).toContain('新通知')
    expect(cards[1]?.querySelector('[data-testid="page-notification-image"]')?.getAttribute('src')).toBe('https://example.com/notice.png')
    expect(cards[1]?.querySelector('[data-testid="page-notification-close"]')?.getAttribute('aria-label')).toBe('关闭')

    cards[0]?.querySelector<HTMLButtonElement>('[data-testid="page-notification-close"]')?.click()
    await wrapper.vm.$nextTick()
    expect(usePageNotifications().items.value.map((item) => item.title)).toEqual(['新通知'])
    wrapper.unmount()
  })
})

describe('page notification polling', () => {
  let wrapper: ReturnType<typeof mount> | undefined
  let hidden = false

  async function mountStack() {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    })
    wrapper = mount(PageNotificationStack, { global: { plugins: [router] } })
    await flushPromises()
  }

  beforeEach(() => {
    vi.useFakeTimers()
    localStorage.clear()
    localStorage.setItem('auth_token', 'account-a')
    hidden = false
    vi.spyOn(document, 'hidden', 'get').mockImplementation(() => hidden)
    vi.mocked(listPageNotifications).mockReset().mockResolvedValue({ notices: [], now: 'cursor-1' })
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
    clearNotices()
    localStorage.clear()
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('uses 30-second checks, pauses hidden pages and removes listeners on unmount', async () => {
    await mountStack()
    await vi.advanceTimersByTimeAsync(29_999)
    expect(listPageNotifications).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(listPageNotifications).toHaveBeenLastCalledWith('cursor-1')
    expect(listPageNotifications).toHaveBeenCalledTimes(2)
    hidden = true
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(90_000)
    expect(listPageNotifications).toHaveBeenCalledTimes(2)
    hidden = false
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(listPageNotifications).toHaveBeenCalledTimes(3)
    wrapper!.unmount()
    wrapper = undefined
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(30_000)
    expect(listPageNotifications).toHaveBeenCalledTimes(3)
  })

  it('does not overlap slow requests and retains the cursor after a failure', async () => {
    await mountStack()
    let reject!: (error: Error) => void
    vi.mocked(listPageNotifications).mockReturnValueOnce(new Promise((_resolve, fail) => { reject = fail }))
    await vi.advanceTimersByTimeAsync(30_000)
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(60_000)
    expect(listPageNotifications).toHaveBeenCalledTimes(2)
    reject(new Error('offline'))
    await flushPromises()
    await vi.advanceTimersByTimeAsync(30_000)
    expect(listPageNotifications).toHaveBeenCalledTimes(3)
    expect(listPageNotifications).toHaveBeenLastCalledWith('cursor-1')
  })

  it('discards notices from a previous account and resets its cursor', async () => {
    let resolve!: (value: Awaited<ReturnType<typeof listPageNotifications>>) => void
    vi.mocked(listPageNotifications).mockReturnValueOnce(new Promise((done) => { resolve = done }))
    await mountStack()
    localStorage.setItem('auth_token', 'account-b')
    resolve({ notices: [{ id: 'private-a', title: 'Private A', body: '', image: '', url: '', createdAt: '' }], now: 'private-cursor' })
    await flushPromises()
    expect(usePageNotifications().items.value).toHaveLength(0)
    await vi.advanceTimersByTimeAsync(30_000)
    expect(listPageNotifications).toHaveBeenLastCalledWith('')
  })
})
