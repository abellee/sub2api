import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import { FeatureFlags, isChannelMonitorVisibleToUser, isFeatureFlagEnabled, makeSidebarFlag, resolveFeatureFlag } from '@/utils/featureFlags'
import type { PublicSettings } from '@/types'

const authStore = vi.hoisted(() => ({
  isAdmin: false,
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('@/api/admin/system', () => ({
  checkUpdates: vi.fn(),
}))

vi.mock('@/api/auth', () => ({
  getPublicSettings: vi.fn(),
}))

describe('FeatureFlags.subscription', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    delete (window as any).__APP_CONFIG__
  })

  it('reads subscription_enabled as an opt-out flag: visible before settings load', () => {
    expect(FeatureFlags.subscription.key).toBe('subscription_enabled')
    expect(FeatureFlags.subscription.mode).toBe('opt-out')
    expect(useAppStore().cachedPublicSettings).toBeNull()
    expect(isFeatureFlagEnabled(FeatureFlags.subscription)).toBe(true)
  })

  it('hides only when the backend explicitly sends false', () => {
    const store = useAppStore()
    const sidebarFlag = makeSidebarFlag(FeatureFlags.subscription)

    store.cachedPublicSettings = { subscription_enabled: false } as PublicSettings
    expect(sidebarFlag()).toBe(false)

    store.cachedPublicSettings = { subscription_enabled: true } as PublicSettings
    expect(sidebarFlag()).toBe(true)

    store.cachedPublicSettings = {} as PublicSettings
    expect(sidebarFlag()).toBe(true)
  })
})

describe('resolveFeatureFlag', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('reads an explicit boolean from the given settings object', () => {
    expect(resolveFeatureFlag({ subscription_enabled: false } as PublicSettings, FeatureFlags.subscription)).toBe(false)
    expect(resolveFeatureFlag({ subscription_enabled: true } as PublicSettings, FeatureFlags.subscription)).toBe(true)
    expect(resolveFeatureFlag({ available_channels_enabled: true } as PublicSettings, FeatureFlags.availableChannels)).toBe(true)
  })

  it('falls back to the declared mode when settings are missing or the key is absent', () => {
    expect(resolveFeatureFlag(undefined, FeatureFlags.subscription)).toBe(true)
    expect(resolveFeatureFlag(null, FeatureFlags.subscription)).toBe(true)
    expect(resolveFeatureFlag({} as PublicSettings, FeatureFlags.subscription)).toBe(true)
    expect(resolveFeatureFlag({} as PublicSettings, FeatureFlags.availableChannels)).toBe(false)
  })

  it('backs isFeatureFlagEnabled with the same resolution', () => {
    useAppStore().cachedPublicSettings = { subscription_enabled: false } as PublicSettings
    expect(isFeatureFlagEnabled(FeatureFlags.subscription)).toBe(false)
  })
})

describe('isChannelMonitorVisibleToUser', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    authStore.isAdmin = false
  })

  it('fails closed when settings are missing or the visibility field is absent', () => {
    expect(isChannelMonitorVisibleToUser()).toBe(false)
    useAppStore().cachedPublicSettings = { channel_monitor_enabled: true } as PublicSettings
    expect(isChannelMonitorVisibleToUser()).toBe(false)
  })

  it('lets admins through when old backends omit visibility fields', () => {
    authStore.isAdmin = true
    useAppStore().cachedPublicSettings = { channel_monitor_enabled: true } as PublicSettings
    expect(isChannelMonitorVisibleToUser()).toBe(true)
  })

  it('hides in selected mode even when the per-caller flag has not arrived', () => {
    useAppStore().cachedPublicSettings = {
      channel_monitor_enabled: true,
      channel_monitor_visibility: 'selected',
    } as PublicSettings
    expect(isChannelMonitorVisibleToUser()).toBe(false)
  })

  it('shows the user surface for an allow-listed caller in selected mode', () => {
    useAppStore().cachedPublicSettings = {
      channel_monitor_enabled: true,
      channel_monitor_visibility: 'selected',
      channel_monitor_visible: true,
    } as PublicSettings
    expect(isChannelMonitorVisibleToUser()).toBe(true)
  })

  it('shows the user surface only when visibility is explicitly all', () => {
    useAppStore().cachedPublicSettings = {
      channel_monitor_enabled: true,
      channel_monitor_visibility: 'all',
    } as PublicSettings
    expect(isChannelMonitorVisibleToUser()).toBe(true)
  })

  it('hides the user surface when the caller is not visible, even if the feature is on', () => {
    useAppStore().cachedPublicSettings = {
      channel_monitor_enabled: true,
      channel_monitor_visible: false,
    } as PublicSettings
    expect(isFeatureFlagEnabled(FeatureFlags.channelMonitor)).toBe(true)
    expect(isChannelMonitorVisibleToUser()).toBe(false)
  })

  it('hides the user surface when the global feature is off', () => {
    useAppStore().cachedPublicSettings = {
      channel_monitor_enabled: false,
      channel_monitor_visible: true,
    } as PublicSettings
    expect(isChannelMonitorVisibleToUser()).toBe(false)
  })
})
