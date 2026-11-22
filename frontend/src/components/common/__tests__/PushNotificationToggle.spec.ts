import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PushNotificationToggle from '../PushNotificationToggle.vue'

const mocks = vi.hoisted(() => ({
  getPushPermissionState: vi.fn(), enablePushNotifications: vi.fn(), disablePushNotifications: vi.fn(),
  showError: vi.fn(), showSuccess: vi.fn(),
}))
vi.mock('@/services/pushNotifications', () => mocks)
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

beforeEach(() => {
  vi.resetAllMocks()
  mocks.getPushPermissionState.mockResolvedValue('disabled')
})

describe('PushNotificationToggle', () => {
  it('shows checking, enabling and disabling, blocking duplicate actions until completion', async () => {
    let finishCheck!: (state: string) => void
    let finishEnable!: (state: string) => void
    let finishDisable!: (state: string) => void
    mocks.getPushPermissionState.mockReturnValueOnce(new Promise(resolve => { finishCheck = resolve }))
    mocks.enablePushNotifications.mockReturnValueOnce(new Promise(resolve => { finishEnable = resolve }))
    mocks.disablePushNotifications.mockReturnValueOnce(new Promise(resolve => { finishDisable = resolve }))
    const wrapper = mount(PushNotificationToggle, { global: { stubs: { Icon: true } } })
    expect(wrapper.text()).toContain('status.checking')
    expect(wrapper.get('[role="switch"]').attributes('disabled')).toBeDefined()
    finishCheck('disabled')
    await flushPromises()

    await wrapper.get('[role="switch"]').trigger('click')
    expect(wrapper.text()).toContain('status.enabling')
    expect(wrapper.get('[role="switch"]').attributes('aria-busy')).toBe('true')
    expect(wrapper.find('svg.animate-spin').exists()).toBe(true)
    await wrapper.trigger('click')
    expect(mocks.enablePushNotifications).toHaveBeenCalledTimes(1)
    finishEnable('enabled')
    await flushPromises()
    expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('true')

    await wrapper.get('[role="switch"]').trigger('click')
    expect(wrapper.text()).toContain('status.disabling')
    finishDisable('disabled')
    await flushPromises()
    expect(wrapper.text()).toContain('status.disabled')
    expect(wrapper.get('[role="switch"]').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })

  it('clears the busy state and keeps the saved setting after a failed toggle', async () => {
    mocks.enablePushNotifications.mockRejectedValueOnce(new Error('offline'))
    const wrapper = mount(PushNotificationToggle, { global: { stubs: { Icon: true } } })
    await flushPromises()
    await wrapper.get('[role="switch"]').trigger('click')
    await flushPromises()
    expect(mocks.showError).toHaveBeenCalledWith('offline')
    expect(wrapper.text()).toContain('status.disabled')
    expect(wrapper.get('[role="switch"]').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
})
