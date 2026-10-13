import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ChannelMonitorVisibilityPanel from '../ChannelMonitorVisibilityPanel.vue'

const { getSettings, updateSettings, showSuccess, showError, fetchPublicSettings } = vi.hoisted(() => ({
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
  fetchPublicSettings: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    settings: {
      getSettings: (...args: unknown[]) => getSettings(...args),
      updateSettings: (...args: unknown[]) => updateSettings(...args),
    },
    usage: {
      searchUsers: vi.fn().mockResolvedValue([]),
    },
    users: {
      getById: vi.fn(),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError, fetchPublicSettings }),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params ? `${key}:${JSON.stringify(params)}` : key,
  }),
}))

describe('ChannelMonitorVisibilityPanel', () => {
  beforeEach(() => {
    getSettings.mockReset()
    updateSettings.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    fetchPublicSettings.mockReset().mockResolvedValue(null)
    getSettings.mockResolvedValue({
      channel_monitor_visible_user_ids: [],
    })
    updateSettings.mockResolvedValue({})
  })

  it('defaults to selected when the visibility field is missing', async () => {
    const wrapper = mount(ChannelMonitorVisibilityPanel, {
      global: {
        stubs: {
          Icon: true,
          ChannelMonitorUserSelector: true,
        },
      },
    })
    await flushPromises()

    const radios = wrapper.findAll('[role="radio"]')
    expect(radios).toHaveLength(2)
    expect(radios[0].attributes('aria-checked')).toBe('false')
    expect(radios[1].attributes('aria-checked')).toBe('true')
    expect(wrapper.text()).toContain('admin.channelMonitor.visibility.emptySelected')
  })

  it('loads all-visible from the backend and saves selected users', async () => {
    getSettings.mockResolvedValue({
      channel_monitor_visibility: 'all',
      channel_monitor_visible_user_ids: [3],
    })
    const wrapper = mount(ChannelMonitorVisibilityPanel, {
      global: {
        stubs: {
          Icon: true,
          ChannelMonitorUserSelector: {
            props: ['modelValue'],
            emits: ['update:modelValue'],
            template:
              '<button data-testid="set-users" @click="$emit(\'update:modelValue\', [7, 11])">users</button>',
          },
        },
      },
    })
    await flushPromises()

    const radios = wrapper.findAll('[role="radio"]')
    expect(radios).toHaveLength(2)
    expect(radios[0].attributes('aria-checked')).toBe('true')

    await radios[1].trigger('click')
    await wrapper.get('[data-testid="set-users"]').trigger('click')
    await wrapper.get('.btn-primary').trigger('click')
    await flushPromises()

    expect(updateSettings).toHaveBeenCalledWith({
      channel_monitor_visibility: 'selected',
      channel_monitor_visible_user_ids: [7, 11],
    })
    expect(getSettings).toHaveBeenCalledTimes(2)
    expect(fetchPublicSettings).toHaveBeenCalledWith(true)
    expect(showError).toHaveBeenCalled()
    expect(showSuccess).not.toHaveBeenCalled()
  })

  it('confirms persistence after save', async () => {
    getSettings
      .mockResolvedValueOnce({
        channel_monitor_visibility: 'all',
        channel_monitor_visible_user_ids: [],
      })
      .mockResolvedValueOnce({
        channel_monitor_visibility: 'selected',
        channel_monitor_visible_user_ids: [7, 11],
      })
    const wrapper = mount(ChannelMonitorVisibilityPanel, {
      global: {
        stubs: {
          Icon: true,
          ChannelMonitorUserSelector: {
            props: ['modelValue'],
            emits: ['update:modelValue'],
            template:
              '<button data-testid="set-users" @click="$emit(\'update:modelValue\', [7, 11])">users</button>',
          },
        },
      },
    })
    await flushPromises()
    await wrapper.findAll('[role="radio"]')[1].trigger('click')
    await wrapper.get('[data-testid="set-users"]').trigger('click')
    await wrapper.get('.btn-primary').trigger('click')
    await flushPromises()
    expect(showSuccess).toHaveBeenCalled()
  })
})
