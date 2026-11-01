import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { nextTick, reactive } from 'vue'
import DashboardStudioLayout from '../DashboardStudioLayout.vue'

const state = vi.hoisted(() => ({ toggleMobileSidebar: vi.fn(), replayTour: vi.fn() }))
const route = reactive({ name: 'APIKeys', params: { id: '' }, meta: { titleKey: 'keys.title', title: 'API Keys' } })
const settings = reactive({ subscription_enabled: true, payment_balance_disabled: false, custom_menu_items: [{ id: 'guide', label: '使用指南' }] })
const auth = reactive<{ user: { role: string } | null; isSimpleMode: boolean }>({ user: null, isSimpleMode: false })

vi.mock('vue-router', () => ({ useRoute: () => route }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ cachedPublicSettings: settings, toggleMobileSidebar: state.toggleMobileSidebar }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => ({ customMenuItems: [{ id: 'admin-guide', label: '管理指南' }] }) }))
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => ({ setReplayCallback: vi.fn() }) }))
vi.mock('@/stores/announcements', () => ({ whenAnnouncementEntrySettled: vi.fn() }))
vi.mock('@/composables/useOnboardingTour', () => ({ useOnboardingTour: () => ({ replayTour: state.replayTour }), onboardingTourStorageKey: vi.fn() }))
vi.mock('../StudioSidebar.vue', () => ({ default: { template: '<aside class="new-sidebar" />' } }))

enableAutoUnmount(afterEach)
beforeEach(() => {
  vi.clearAllMocks()
  route.name = 'APIKeys'
  route.params.id = ''
  route.meta = { titleKey: 'keys.title', title: 'API Keys' }
  settings.subscription_enabled = true
  settings.payment_balance_disabled = false
  auth.user = null
})

describe('DashboardStudioLayout shared user frame', () => {
  it('updates the header with the route and keeps content and dashboard filters', async () => {
    const wrapper = mount(DashboardStudioLayout, { slots: { default: '<section>Page content</section>', filters: '<button>Range</button>' } })
    expect(wrapper.find('h1').text()).toBe('keys.title')
    expect(wrapper.find('.new-sidebar').exists()).toBe(true)
    expect(wrapper.find('main').text()).toBe('Page content')
    expect(wrapper.find('header').text()).toContain('Range')
    route.meta = { titleKey: 'usage.title', title: 'Usage Records' }
    await nextTick()
    expect(wrapper.find('h1').text()).toBe('usage.title')
    route.meta = { titleKey: '', title: 'Custom title' }
    await nextTick()
    expect(wrapper.find('h1').text()).toBe('Custom title')
    await wrapper.find('.studio-menu-toggle').trigger('click')
    expect(state.toggleMobileSidebar).toHaveBeenCalledOnce()
  })

  it('uses the configured custom menu label and reserves admin labels for admins', async () => {
    route.name = 'CustomPage'
    route.params.id = 'guide'
    const wrapper = mount(DashboardStudioLayout)
    expect(wrapper.find('h1').text()).toBe('使用指南')
    route.params.id = 'admin-guide'
    await nextTick()
    expect(wrapper.find('h1').text()).toBe('keys.title')
    auth.user = { role: 'admin' }
    await nextTick()
    expect(wrapper.find('h1').text()).toBe('管理指南')
  })

  it('follows the site billing mode for the purchase title', async () => {
    route.name = 'PurchaseSubscription'
    route.meta.titleKey = 'nav.buySubscription'
    const wrapper = mount(DashboardStudioLayout)
    expect(wrapper.find('h1').text()).toBe('nav.buySubscription')
    settings.subscription_enabled = false
    await nextTick()
    expect(wrapper.find('h1').text()).toBe('nav.recharge')
    settings.subscription_enabled = true
    settings.payment_balance_disabled = true
    await nextTick()
    expect(wrapper.find('h1').text()).toBe('nav.subscribe')
  })
})
