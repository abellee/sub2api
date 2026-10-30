import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PushNotificationsView from '../PushNotificationsView.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'

const mocks = vi.hoisted(() => ({
  broadcastPush: vi.fn(), createPushSchedule: vi.fn(), getPushOverview: vi.fn(),
  listPushMessages: vi.fn(), listPushSubscriptions: vi.fn(), listPushSchedules: vi.fn(),
  deletePushSchedule: vi.fn(), clearPushMessages: vi.fn(), deletePushMessage: vi.fn(), uploadPushImage: vi.fn(),
  showError: vi.fn(), showSuccess: vi.fn(), showPageNotification: vi.fn(),
}))
vi.mock('@/api/pushNotifications', () => mocks)
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('@/composables/usePageNotifications', () => ({ showPageNotification: mocks.showPageNotification }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))

beforeEach(() => {
  vi.resetAllMocks()
  mocks.getPushOverview.mockResolvedValue({ activeSubscriptions: 3, messageCount: 0, lastSentAt: null })
  mocks.listPushMessages.mockResolvedValue([])
  mocks.listPushSchedules.mockResolvedValue([])
  mocks.listPushSubscriptions.mockResolvedValue([
    { id: 'one', user: { id: 7, username: 'ada', email: 'ada@example.com' } },
    { id: 'two', user: { id: 7, username: 'ada', email: 'ada@example.com' } },
    { id: 'three', user: { id: 8, username: 'bob', email: 'bob@example.com' }, enabled: false },
    { id: 'anonymous', user: null },
  ])
  mocks.broadcastPush.mockImplementation(async payload => ({ ...payload, id: 'notice', delivered: 2, failed: 0, createdAt: new Date().toISOString() }))
  mocks.createPushSchedule.mockImplementation(async payload => ({ ...payload, id: 'scheduled', status: 'pending' }))
})

async function render() {
  const wrapper = mount(PushNotificationsView, { global: { stubs: { Icon: true, ConfirmDialog: true } } })
  await flushPromises()
  await wrapper.get('#push-title').setValue('Test')
  await wrapper.get('#push-body').setValue('Message')
  return wrapper
}

describe('targeted notification composer', () => {
  it('explains that an older notification service needs updating without retrying as a broadcast', async () => {
    mocks.broadcastPush.mockRejectedValueOnce(Object.assign(new Error('not found'), { status: 404 }))
    const wrapper = await render()
    await wrapper.get('input[value="selected"]').setValue(true)
    await wrapper.get('input[type="checkbox"][value="7"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    wrapper.findAllComponents(ConfirmDialog)[0].vm.$emit('confirm')
    await flushPromises()
    expect(mocks.showError).toHaveBeenCalledWith('admin.pushNotifications.audience.unavailable')
    expect(mocks.broadcastPush).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
  it('deduplicates users, searches recipients and sends only selected IDs', async () => {
    const wrapper = await render()
    await wrapper.get('input[value="selected"]').setValue(true)
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    expect(wrapper.findAll('input[type="checkbox"]').filter(input => ['7', '8'].includes(input.element.getAttribute('value') || ''))).toHaveLength(2)
    await wrapper.get('#push-recipient-search').setValue('ada@example.com')
    expect(wrapper.find('input[type="checkbox"][value="8"]').exists()).toBe(false)
    await wrapper.get('input[type="checkbox"][value="7"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    const confirm = wrapper.findAllComponents(ConfirmDialog)[0]
    expect(confirm.props('show')).toBe(true)
    expect(confirm.props('message')).toBe('admin.pushNotifications.audience.confirm')
    confirm.vm.$emit('confirm')
    await flushPromises()
    expect(mocks.broadcastPush).toHaveBeenCalledWith(expect.objectContaining({ userIDs: [7], channels: ['browser', 'page'] }))
    expect(mocks.showPageNotification).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('keeps selected recipients for scheduled delivery and omits IDs for broadcasts', async () => {
    const wrapper = await render()
    await wrapper.get('input[value="selected"]').setValue(true)
    await wrapper.get('input[type="checkbox"][value="8"]').setValue(true)
    await wrapper.get('#push-scheduled-at').setValue('2099-01-01T12:00')
    await wrapper.get('form').trigger('submit')
    wrapper.findAllComponents(ConfirmDialog)[0].vm.$emit('confirm')
    await flushPromises()
    expect(mocks.createPushSchedule).toHaveBeenCalledWith(expect.objectContaining({ userIDs: [8], scheduledAt: expect.any(String) }))
    expect(mocks.broadcastPush).not.toHaveBeenCalled()
    await wrapper.get('input[value="all"]').setValue(true)
    await wrapper.get('#push-title').setValue('All')
    await wrapper.get('#push-body').setValue('Everyone')
    await wrapper.get('form').trigger('submit')
    wrapper.findAllComponents(ConfirmDialog)[0].vm.$emit('confirm')
    await flushPromises()
    expect(mocks.broadcastPush.mock.calls[0][0]).not.toHaveProperty('userIDs')
    expect(mocks.showPageNotification).toHaveBeenCalled()
    wrapper.unmount()
  })
})
