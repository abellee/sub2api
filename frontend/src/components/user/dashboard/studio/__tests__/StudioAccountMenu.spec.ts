import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import StudioAccountMenu from '../StudioAccountMenu.vue'
import StudioMetricChart from '../StudioMetricChart.vue'

const mocks = vi.hoisted(() => ({ balance: 286.5, simpleMode: false, logout: vi.fn(), push: vi.fn(), copy: vi.fn(), feature: vi.fn(() => false), mobile: vi.fn(), replay: vi.fn(), setTheme: vi.fn(), info: vi.fn() }))
vi.mock('@/components/user/PointsModal.vue', () => ({ default: { props: ['show', 'userId'], emits: ['close', 'loaded'], template: '<div v-if="show" class="points-modal-stub"><button @click="$emit(\'close\')">close</button></div>' } }))
vi.mock('@/composables/usePushNotificationTour', () => ({ OPEN_USER_MENU_EVENT: 'open-user-menu', PUSH_TOUR_UNLOCK_EVENT: 'push-tour-unlock', OPEN_QQ_GROUP_EVENT: 'open-qq-group', CLOSE_QQ_GROUP_EVENT: 'close-qq-group' }))
vi.mock('@/components/common/AnnouncementBell.vue', () => ({ default: { template: '<button aria-label="announcements" />' } }))
vi.mock('@/components/common/SubscriptionProgressMini.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/common/PushNotificationToggle.vue', () => ({ default: { template: '<button data-tour="push-notification-toggle" />' } }))
vi.mock('../StudioLocaleMenu.vue', () => ({ default: { template: '<button aria-label="language" />' } }))
vi.mock('../StudioPopover.vue', () => ({ default: { props: ['anchor', 'side'], template: '<div><slot /></div>' } }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: mocks.push }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('pinia', () => ({ storeToRefs: (store: unknown) => store }))
vi.mock('@/stores', () => ({
  useAppStore: () => ({ docUrl: 'https://example.com/docs', contactInfo: 'support', setMobileOpen: mocks.mobile, showInfo: mocks.info }),
  useAuthStore: () => ({ user: { id: 2, username: 'Demo', role: 'user', balance: mocks.balance, frozen_balance: 63.5 }, logout: mocks.logout, isAdmin: false, isSimpleMode: mocks.simpleMode }),
  useOnboardingStore: () => ({ replay: mocks.replay }),
  useRemoteWidgetsStore: () => ({ headerConfig: ref({ qq: { groupName: 'QQ', groupNumber: '123', qrUrl: '/qq.png' }, telegram: { groupName: 'Telegram', link: 'https://t.me/demo', qrUrl: '/tg.png' } }) })
}))
vi.mock('@/composables/useLotterySocketStatus', () => ({ useLotterySocketStatus: () => ({ status: ref('idle') }) }))
vi.mock('@/composables/useTheme', () => ({ useTheme: () => ({ isDark: ref(false), setTheme: mocks.setTheme }) }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: mocks.copy }) }))
vi.mock('@/utils/featureFlags', () => ({ FeatureFlags: {}, isFeatureFlagEnabled: mocks.feature }))
const wrappers: ReturnType<typeof mount>[] = []
function render(expanded = true) {
  const wrapper = mount(StudioAccountMenu, { props: { expanded }, attachTo: document.body, global: { stubs: {
    RouterLink: { props: ['to'], template: '<a :href="to" @click.prevent><slot /></a>' }, Icon: true, QqBrandIcon: true,
    StudioPopover: { props: ['anchor', 'side'], template: '<div><slot /></div>' },
    AnnouncementBell: { template: '<button aria-label="announcements" />' },
    StudioLocaleMenu: { template: '<button aria-label="language" />' },
    PushNotificationToggle: { template: '<button data-tour="push-notification-toggle" />' }, SubscriptionProgressMini: true
  } } })
  wrappers.push(wrapper)
  return wrapper
}
beforeEach(() => { vi.clearAllMocks(); mocks.balance = 286.5; mocks.simpleMode = false; window.matchMedia = vi.fn().mockReturnValue({ matches: false }) })
afterEach(() => { wrappers.forEach(wrapper => wrapper.unmount()); wrappers.length = 0 })
describe('StudioAccountMenu', () => {
  it('opens the same points dialog and item notice from the balance card values', async () => {
    const sidebar = render()
    const card = mount(StudioMetricChart, { props: { kind: 'balance', value: 75 }, global: { stubs: { RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' } } } })
    wrappers.push(card)
    await card.find('.balance-points button').trigger('click')
    expect(sidebar.find('.points-modal-stub').exists()).toBe(true)
    expect(sidebar.findAll('.points-modal-stub')).toHaveLength(1)
    await sidebar.find('.points-modal-stub button').trigger('click')
    await card.find('.balance-items button').trigger('click')
    expect(mocks.info).toHaveBeenCalledWith('common.itemsComingSoon')
    expect(sidebar.find('.points-modal-stub').exists()).toBe(false)
    sidebar.unmount()
    mocks.info.mockClear()
    await card.find('.balance-items button').trigger('click')
    expect(mocks.info).not.toHaveBeenCalled()
  })
  it('makes each icon and value one button and opens the points dialog from the icon', async () => {
    const wrapper = render()
    const buttons = wrapper.findAll('.studio-resource-button')
    expect(buttons).toHaveLength(2)
    expect(buttons[0].element.tagName).toBe('BUTTON')
    expect(buttons[0].text()).toBe('—')
    expect(buttons[0].attributes('aria-haspopup')).toBe('dialog')
    expect(wrapper.find('.points-modal-stub').exists()).toBe(false)
    await buttons[0].find('img').trigger('click')
    expect(wrapper.find('.points-modal-stub').exists()).toBe(true)
    await wrapper.find('.points-modal-stub button').trigger('click')
    expect(wrapper.find('.points-modal-stub').exists()).toBe(false)
    await buttons[1].trigger('click')
    expect(mocks.info).toHaveBeenCalledWith('common.itemsComingSoon')
  })
  it.each([0, 4.99, 5, 9.99, 10, 10.01])('colors available balance at the warning thresholds: %s', amount => {
    mocks.balance = amount
    const wrapper = render()
    const recharge = wrapper.find('.wallet-recharge')
    expect(recharge.exists()).toBe(amount < 10)
    if (amount < 10) {
      expect(recharge.attributes('href')).toBe('/purchase')
      expect(wrapper.find('.wallet-heading .wallet-recharge').exists()).toBe(true)
      expect(wrapper.find('.wallet-heading .wallet-label').text()).toBe('common.availableBalance')
    }
    const available = wrapper.find('.wallet-available strong')
    expect(available.classes().includes('balance-warning')).toBe(amount >= 5 && amount < 10)
    expect(available.classes().includes('balance-critical')).toBe(amount < 5)
    expect(wrapper.find('.wallet-frozen strong').classes()).not.toContain('balance-warning')
    expect(wrapper.find('.wallet-frozen strong').classes()).not.toContain('balance-critical')
  })

  it('hides recharge in simple mode', () => {
    mocks.balance = 4
    mocks.simpleMode = true
    expect(render().find('.wallet-recharge').exists()).toBe(false)
  })

  it('keeps recharge inaccessible while collapsed and closes the mobile sidebar on click', async () => {
    mocks.balance = 4
    const wrapper = render(false)
    expect(wrapper.find('.studio-wallet').attributes()).toHaveProperty('inert')
    await wrapper.setProps({ expanded: true })
    expect(wrapper.find('.studio-wallet').attributes()).not.toHaveProperty('inert')
    await wrapper.find('.wallet-recharge').trigger('click')
    expect(mocks.mobile).toHaveBeenCalledWith(false)
  })

  it('closes every flyout immediately when the sidebar collapses, without moving focus', async () => {
    window.matchMedia = vi.fn().mockReturnValue({ matches: true })
    const wrapper = render()
    for (const selector of ['.studio-settings', '[aria-label="加入 QQ 群"]']) {
      await wrapper.setProps({ expanded: true })
      await wrapper.find(selector).trigger('mouseenter')
      expect(wrapper.find('.studio-settings-menu').isVisible() || wrapper.find('.studio-community').exists()).toBe(true)
      const focused = document.activeElement
      await wrapper.trigger('mouseleave')
      await wrapper.setProps({ expanded: false })
      expect(wrapper.find('.studio-settings-menu').isVisible()).toBe(false)
      expect(wrapper.find('.studio-community').exists()).toBe(false)
      expect(wrapper.find('.studio-theme-menu').exists()).toBe(false)
      expect(document.activeElement).toBe(focused)
    }
  })
  it('shows wallet, frozen funds, points and identity directly, and opens only settings on click', async () => {
    const wrapper = render()
    expect(wrapper.find('.studio-user-card').text()).toContain('$286.50')
    expect(wrapper.find('.wallet-frozen').text()).toContain('$63.50')
    expect(wrapper.find('.studio-points').isVisible()).toBe(true)
    expect(wrapper.find('.studio-account-label').text()).toContain('Demo')
    expect(wrapper.find('.studio-settings-menu').isVisible()).toBe(false)
    await wrapper.find('.studio-avatar').trigger('mouseenter')
    expect(wrapper.find('.studio-settings-menu').isVisible()).toBe(false)
    await wrapper.find('.studio-settings').trigger('click')
    expect(wrapper.find('.studio-settings-menu').isVisible()).toBe(true)
    expect(wrapper.findAllComponents({ name: 'SubscriptionProgressMini' })).toHaveLength(0)
    await wrapper.trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('.studio-settings-menu').isVisible()).toBe(false)
  })
  it('keeps collapsed quick access in the requested order with a single announcement entry', async () => {
    const wrapper = render(false)
    const details = ['.studio-wallet', '.studio-points', '.studio-account-label', '.studio-settings']
    for (const selector of details) {
      expect(wrapper.find(selector).attributes('aria-hidden')).toBe('true')
      expect(wrapper.find(selector).attributes()).toHaveProperty('inert')
    }
    const tools = wrapper.find('.studio-utilities').findAll('button, a')
    expect(tools.map(item => item.attributes('aria-label'))).toEqual(['announcements', 'nav.darkMode', '加入 QQ 群', '加入 Telegram', 'language'])
    await wrapper.setProps({ expanded: true })
    for (const selector of details) {
      expect(wrapper.find(selector).attributes('aria-hidden')).toBe('false')
      expect(wrapper.find(selector).attributes()).not.toHaveProperty('inert')
    }
    expect(wrapper.findAll('[aria-label="announcements"]')).toHaveLength(1)
    expect(wrapper.find('.studio-utilities').findAll('button, a').map(item => item.attributes('aria-label'))).toEqual(['announcements', 'nav.darkMode', '加入 QQ 群', '加入 Telegram', 'language'])
  })
  it('keeps community panels reachable and switches theme directly without a flyout', async () => {
    vi.useFakeTimers()
    window.matchMedia = vi.fn().mockReturnValue({ matches: true })
    const wrapper = render(false)
    await wrapper.find('[aria-label="加入 QQ 群"]').trigger('mouseenter')
    expect(wrapper.find('.studio-community').exists()).toBe(true)
    await wrapper.trigger('mouseleave')
    await vi.advanceTimersByTimeAsync(100)
    await wrapper.find('.studio-community').trigger('mouseenter')
    await vi.advanceTimersByTimeAsync(200)
    expect(wrapper.find('.studio-community').exists()).toBe(true)
    await wrapper.find('.studio-avatar').trigger('mouseenter')
    expect(wrapper.find('.studio-settings-menu').isVisible()).toBe(true)
    expect(wrapper.find('.studio-community').exists()).toBe(false)
    await wrapper.find('[aria-label="nav.darkMode"]').trigger('mouseenter')
    expect(wrapper.find('.studio-theme-menu').exists()).toBe(false)
    expect(mocks.setTheme).not.toHaveBeenCalled()
    await wrapper.find('[aria-label="nav.darkMode"]').trigger('click')
    expect(mocks.setTheme).toHaveBeenCalledWith('dark')
    expect(wrapper.find('.studio-settings-menu').isVisible()).toBe(false)
    expect(wrapper.find('.studio-theme-menu').exists()).toBe(false)
    await wrapper.trigger('mouseleave')
    await vi.advanceTimersByTimeAsync(200)
    await wrapper.find('.studio-announcement').trigger('mouseenter')
    expect(wrapper.find('.studio-announcement-preview').exists()).toBe(false)
    expect(wrapper.find('.studio-theme-menu').exists()).toBe(false)
    vi.useRealTimers()
  })
  it.each([false, true])('opens both community panels beside the sidebar when expanded is %s', async (expanded) => {
    window.matchMedia = vi.fn().mockReturnValue({ matches: true })
    const wrapper = render(expanded)
    for (const name of ['加入 QQ 群', '加入 Telegram']) {
      await wrapper.find(`[aria-label="${name}"]`).trigger('mouseenter')
      expect(wrapper.findComponent('.studio-community').props('side')).toBe('right')
    }
  })
  it('links directly to each community and keeps hover details and copying available', async () => {
    window.matchMedia = vi.fn().mockReturnValue({ matches: true })
    const wrapper = render()
    await wrapper.find('[aria-label="加入 QQ 群"]').trigger('mouseenter')
    const qq = wrapper.find('[aria-label="加入 QQ 群"]')
    expect(qq.element.tagName).toBe('A')
    expect(qq.attributes('href')).toBe(wrapper.find('.studio-community-link').attributes('href'))
    expect(qq.attributes('target')).toBe('_blank')
    expect(wrapper.find('.studio-community').text()).toContain('123')
    await wrapper.find('[aria-label="复制QQ群号"]').trigger('click')
    expect(mocks.copy).toHaveBeenCalledWith('123', '群号已复制')
    await wrapper.find('[aria-label="加入 Telegram"]').trigger('mouseenter')
    const telegram = wrapper.find('[aria-label="加入 Telegram"]')
    expect(telegram.element.tagName).toBe('A')
    expect(telegram.attributes('href')).toBe(wrapper.find('.studio-community-link').attributes('href'))
    expect(telegram.attributes('target')).toBe('_blank')
    expect(wrapper.find('.studio-community-link').attributes('href')).toBe('https://t.me/demo')
    expect(wrapper.find('.studio-settings-menu').isVisible()).toBe(false)
    document.body.click()
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.studio-community').exists()).toBe(false)
  })
  it('keeps the notification guide visible and locked until the guide releases it', async () => {
    const wrapper = render()
    window.dispatchEvent(new Event('open-user-menu'))
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('holdOpen')).toEqual([[true]])
    expect(wrapper.find('.studio-settings-menu').isVisible()).toBe(true)
    document.body.click()
    await wrapper.trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('.studio-settings-menu').isVisible()).toBe(true)
    window.dispatchEvent(new Event('push-tour-unlock'))
    document.body.click()
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.studio-settings-menu').isVisible()).toBe(false)
    window.dispatchEvent(new Event('open-qq-group'))
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.studio-community').exists()).toBe(true)
    window.dispatchEvent(new Event('close-qq-group'))
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.studio-community').exists()).toBe(false)
  })
  it('still redirects to login if logout fails', async () => {
    mocks.logout.mockRejectedValueOnce(new Error('offline'))
    const errorLog = vi.spyOn(console, 'error').mockImplementation(() => {})
    const wrapper = render()
    await wrapper.find('.studio-settings').trigger('click')
    await wrapper.find('.studio-logout').trigger('click')
    expect(mocks.logout).toHaveBeenCalledOnce()
    expect(mocks.push).toHaveBeenCalledWith('/login')
    errorLog.mockRestore()
  })
})
