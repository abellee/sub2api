import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import StudioSidebar from '../StudioSidebar.vue'
import AppSidebar from '@/components/layout/AppSidebar.vue'
const versionMocks = vi.hoisted(() => ({ isAdmin: false, fetchVersion: vi.fn(), versions: vi.fn(), rollback: vi.fn() }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-router', () => ({ useRoute: () => ({ path: '/dashboard' }), useRouter: () => ({ push: vi.fn() }) }))
vi.mock('@vueuse/core', () => ({ useMediaQuery: () => ref(true) }))
vi.mock('pinia', () => ({ storeToRefs: (store: unknown) => store }))
vi.mock('@/stores', () => ({
  useAppStore: () => ({ siteName: 'TokenFree', siteVersion: '0.2.19', currentVersion: '0.2.19', buildType: 'release', hasUpdate: false, versionLoading: false, fetchVersion: versionMocks.fetchVersion, publicSettingsLoaded: true, sidebarCollapsed: false, sidebarScrollTop: 0, cachedPublicSettings: {}, setMobileOpen: vi.fn() }),
  useAuthStore: () => ({ isAdmin: versionMocks.isAdmin }),
  useAdminSettingsStore: () => ({ fetch: vi.fn(), customMenuItems: [] }), useOnboardingStore: () => ({ isCurrentStep: () => false }),
  useRemoteWidgetsStore: () => ({ sideVisible: ref(false), sideHasIcon: ref(false) })
}))
vi.mock('@/stores/userMenu', () => ({ useUserMenuStore: () => ({ load: () => Promise.resolve(), config: { sections: [{ id: 'account', title: '账户', items: ['/dashboard', '/keys'] }, { id: 'usage', title: '使用', items: ['/usage'] }], hidden_items: [] } }) }))
vi.mock('@/utils/featureFlags', () => ({ FeatureFlags: {}, makeSidebarFlag: () => () => false, isChannelMonitorVisibleToUser: () => false }))
vi.mock('@/utils/siteBillingMode', () => ({ resolveSiteBillingMode: () => 'recharge_only' }))
vi.mock('@/composables/useMenuStatus', () => ({ useMenuStatus: () => ({ lotteryVisible: ref(false), taskVisible: ref(false), lotteryPhase: ref(''), taskPhase: ref('') }) }))
vi.mock('../StudioAccountMenu.vue', () => ({ default: { props: ['expanded'], template: '<button class="account-stub" @click="$emit(\'holdOpen\', true)" />' } }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copied: ref(false), copyToClipboard: vi.fn() }) }))
vi.mock('@/api/admin/system', () => ({ performUpdate: vi.fn(), restartService: vi.fn(), getRollbackVersions: versionMocks.versions, rollback: versionMocks.rollback }))
beforeEach(() => { versionMocks.isAdmin = false; vi.clearAllMocks() })
const wrappers: ReturnType<typeof mount>[] = []
afterEach(() => { wrappers.forEach(wrapper => wrapper.unmount()); wrappers.length = 0; vi.useRealTimers() })
describe('StudioSidebar', () => {
  it.each([['management', AppSidebar], ['studio', StudioSidebar]] as const)(
    'shows the admin personal dashboard alongside the management dashboard in the %s sidebar',
    async (_, component) => {
      versionMocks.isAdmin = true
      const wrapper = mount(component, { global: { stubs: {
        VersionBadge: true,
        RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
      } } })
      wrappers.push(wrapper)
      await flushPromises()
      expect(wrapper.findAll('.sidebar-nav a[href="/admin/dashboard"]')).toHaveLength(1)
      expect(wrapper.findAll('.sidebar-nav a[href="/dashboard"]')).toHaveLength(1)
      expect(wrapper.findAll('.sidebar-nav a[href="/admin/media-service"]')).toHaveLength(1)
      expect(wrapper.findAll('.sidebar-nav a[href="/media-library"]')).toHaveLength(1)
      const personalSection = wrapper.findAll('.sidebar-section').find(section => section.find('a[href="/dashboard"]').exists())
      expect(personalSection?.find('.sidebar-section-title').text()).toBe('账户')
      expect(personalSection?.find('a[href="/dashboard"]').exists()).toBe(true)
    },
  )
  it('keeps the original admin version panel and its rollback version list', async () => {
    versionMocks.isAdmin = true
    versionMocks.versions.mockResolvedValue({ versions: [{ version: '0.2.18', published_at: '2026-10-30' }] })
    const wrapper = mount(StudioSidebar, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    wrappers.push(wrapper)
    await wrapper.find('.sidebar-header').trigger('focusin')
    await wrapper.find('.version-info-trigger').trigger('click')
    expect(wrapper.find('.sidebar-version').text()).toContain('version.rollback')
    const rollbackButton = wrapper.findAll('.sidebar-version button').find(button => button.text().includes('version.rollback'))!
    await rollbackButton.trigger('click')
    await flushPromises()
    expect(versionMocks.versions).toHaveBeenCalledOnce()
    expect(wrapper.find('.sidebar-version').text()).toContain('v0.2.18')
    expect(versionMocks.rollback).not.toHaveBeenCalled()
  })
  it('hides the entire version entry from ordinary users', async () => {
    const wrapper = mount(StudioSidebar, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    wrappers.push(wrapper)
    await wrapper.find('.sidebar-header').trigger('focusin')
    expect(wrapper.find('.sidebar-version').exists()).toBe(false)
    expect(wrapper.find('.version-info-trigger').exists()).toBe(false)
    expect(versionMocks.fetchVersion).not.toHaveBeenCalled()
    expect(wrapper.find('a[href="/admin/media-service"]').exists()).toBe(false)
  })
  it('expands through navigation, keeps utility focus stable and preserves configured sections', async () => {
    vi.useFakeTimers()
    const wrapper = mount(StudioSidebar, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    wrappers.push(wrapper)
    const account = wrapper.findComponent({ name: 'StudioAccountMenu' })
    expect(account.props('expanded')).toBe(false)
    expect(wrapper.findAll('.sidebar-section-title').map(item => item.text())).toContain('账户')
    expect(wrapper.findAll('.sidebar-section-title').map(item => item.text())).toContain('使用')
    await wrapper.find('.account-stub').trigger('focusin')
    expect(account.props('expanded')).toBe(false)
    await wrapper.find('.sidebar-nav').trigger('mouseenter')
    expect(account.props('expanded')).toBe(true)
    expect(wrapper.find('aside').classes()).toContain('studio-sidebar-expanded')
    await wrapper.find('aside').trigger('mouseleave')
    expect(account.props('expanded')).toBe(true)
    await vi.advanceTimersByTimeAsync(100)
    await wrapper.find('aside').trigger('mouseenter')
    await vi.advanceTimersByTimeAsync(200)
    expect(account.props('expanded')).toBe(true)
    await wrapper.find('aside').trigger('mouseleave')
    await vi.advanceTimersByTimeAsync(180)
    expect(account.props('expanded')).toBe(false)
    await wrapper.find('.sidebar-header').trigger('focusin')
    expect(account.props('expanded')).toBe(true)
    await wrapper.find('aside').trigger('focusout', { relatedTarget: null })
    expect(account.props('expanded')).toBe(false)
    account.vm.$emit('holdOpen', true)
    await wrapper.vm.$nextTick()
    expect(account.props('expanded')).toBe(true)
  })
})
