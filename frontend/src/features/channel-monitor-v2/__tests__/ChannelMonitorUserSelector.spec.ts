import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ChannelMonitorUserSelector from '../ChannelMonitorUserSelector.vue'

const messages: Record<string, string> = {
  'admin.channelMonitor.visibility.userDeleted': '(deleted)',
  'admin.channelMonitor.visibility.userIdFallback': 'User #{id}',
  'admin.channelMonitor.visibility.removeUser': 'Remove user',
  'admin.channelMonitor.visibility.userSearchPlaceholder': 'Search users',
  'admin.channelMonitor.visibility.userSearchEmpty': 'No users found',
  'common.loading': 'Loading',
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      const message = messages[key] ?? key
      return params
        ? Object.entries(params).reduce(
            (value, [name, replacement]) => value.replace(`{${name}}`, String(replacement)),
            message,
          )
        : message
    },
  }),
}))

const mockSearchUsers = vi.fn()
const mockGetUserById = vi.fn()

vi.mock('@/api/admin', () => ({
  adminAPI: {
    usage: {
      searchUsers: (...args: unknown[]) => mockSearchUsers(...args),
    },
    users: {
      getById: (...args: unknown[]) => mockGetUserById(...args),
    },
  },
}))

describe('ChannelMonitorUserSelector', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockSearchUsers.mockReset()
    mockGetUserById.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('hydrates selected ids from the sub2api user list', async () => {
    mockGetUserById.mockResolvedValue({
      id: 7,
      email: 'existing@example.com',
      deleted_at: null,
    })

    const wrapper = mount(ChannelMonitorUserSelector, {
      props: { modelValue: [7] },
      global: { stubs: { Icon: true } },
    })
    await flushPromises()

    expect(mockGetUserById).toHaveBeenCalledWith(7, true)
    expect(wrapper.text()).toContain('existing@example.com')
    expect(wrapper.text()).toContain('#7')
  })

  it('searches sub2api users and adds the selected user id', async () => {
    mockSearchUsers.mockResolvedValue([
      { id: 8, email: 'found@example.com', deleted: false },
    ])

    const wrapper = mount(ChannelMonitorUserSelector, {
      props: { modelValue: [7] },
      global: { stubs: { Icon: true } },
    })
    await flushPromises()

    await wrapper.get('input').setValue('found')
    await wrapper.get('input').trigger('input')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()

    expect(mockSearchUsers).toHaveBeenCalledWith('found')
    await wrapper.get('button.flex').trigger('click')
    const emitted = wrapper.emitted('update:modelValue')
    expect(emitted?.[emitted.length - 1]).toEqual([[7, 8]])
  })
})
