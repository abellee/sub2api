import { afterEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import PageNotificationStack from '../PageNotificationStack.vue'
import { dismissPageNotification, showPageNotification, usePageNotifications } from '@/composables/usePageNotifications'

function clearNotices() {
  for (const item of [...usePageNotifications().items.value]) {
    dismissPageNotification(item.id)
  }
}

describe('PageNotificationStack', () => {
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
