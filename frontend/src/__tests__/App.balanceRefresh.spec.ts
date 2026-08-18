import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  afterEachCallback: null as null | (() => void),
  removeAfterEach: vi.fn(),
  refreshUser: vi.fn(),
  fetchAnnouncements: vi.fn(),
  fetchPublicSettings: vi.fn(),
  getSetupStatus: vi.fn(),
  replace: vi.fn(),
  route: {
    path: '/admin/usage',
    fullPath: '/admin/usage',
    meta: {},
  },
}))

vi.mock('vue-router', () => ({
  RouterView: { template: '<div />' },
  useRoute: () => mocks.route,
  useRouter: () => ({
    afterEach: (callback: () => void) => {
      mocks.afterEachCallback = callback
      return mocks.removeAfterEach
    },
    replace: mocks.replace,
  }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    siteLogo: '',
    siteName: 'Sub2API',
    cachedPublicSettings: { custom_menu_items: [] },
    fetchPublicSettings: mocks.fetchPublicSettings,
  }),
  useAuthStore: () => ({
    isAuthenticated: true,
    isAdmin: false,
    refreshUser: mocks.refreshUser,
  }),
  useSubscriptionStore: () => ({
    fetchActiveSubscriptions: vi.fn().mockResolvedValue(undefined),
    startPolling: vi.fn(),
    clear: vi.fn(),
  }),
  useAnnouncementStore: () => ({
    fetchAnnouncements: mocks.fetchAnnouncements,
    reset: vi.fn(),
  }),
  useAdminComplianceStore: () => ({
    fetchStatus: vi.fn().mockResolvedValue(undefined),
    requireAcknowledgement: vi.fn(),
    reset: vi.fn(),
  }),
  useAdminSettingsStore: () => ({ customMenuItems: [] }),
}))

vi.mock('@/api/setup', () => ({
  getSetupStatus: mocks.getSetupStatus,
}))

vi.mock('@/router/title', () => ({
  resolveRouteDocumentTitle: () => 'Sub2API',
}))

vi.mock('@/utils/branding', () => ({
  updateFavicon: vi.fn(),
}))

import App from '@/App.vue'

describe('App navigation user refresh', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-18T08:00:00Z'))
    mocks.afterEachCallback = null
    mocks.removeAfterEach.mockReset()
    mocks.refreshUser.mockReset().mockResolvedValue(undefined)
    mocks.fetchAnnouncements.mockReset().mockResolvedValue(undefined)
    mocks.fetchPublicSettings.mockReset().mockResolvedValue(undefined)
    mocks.getSetupStatus.mockReset().mockResolvedValue({ needs_setup: false })
    mocks.replace.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('refreshes on navigation at most once every five seconds', async () => {
    const wrapper = mount(App, {
      global: {
        stubs: {
          NavigationProgress: true,
          PersistentRemoteWidgets: true,
          Toast: true,
          AnnouncementPopup: true,
          AdminComplianceDialog: true,
        },
      },
    })
    await flushPromises()

    expect(mocks.afterEachCallback).not.toBeNull()

    mocks.afterEachCallback!()
    await flushPromises()
    expect(mocks.refreshUser).toHaveBeenCalledTimes(1)

    mocks.afterEachCallback!()
    await vi.advanceTimersByTimeAsync(4999)
    mocks.afterEachCallback!()
    await flushPromises()
    expect(mocks.refreshUser).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(1)
    mocks.afterEachCallback!()
    await flushPromises()
    expect(mocks.refreshUser).toHaveBeenCalledTimes(2)

    wrapper.unmount()
    expect(mocks.removeAfterEach).toHaveBeenCalledOnce()
  })
})
