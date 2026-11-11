import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import GroupNotificationsPanel from '../GroupNotificationsPanel.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'

const mocks = vi.hoisted(() => ({ getGroupNotificationConfig: vi.fn(), saveGroupNotificationConfig: vi.fn(), sendGroupNotification: vi.fn(), listGroupNotifications: vi.fn(), showError: vi.fn(), showSuccess: vi.fn() }))
vi.mock('@/api/pushNotifications', () => mocks)
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))
const config = { enabled: true, baseURL: 'http://astrbot:6185', platformID: 'napcat', groupIDs: ['764163343'], configID: 'default', keyConfigured: true }

beforeEach(() => {
  vi.resetAllMocks()
  mocks.getGroupNotificationConfig.mockResolvedValue({ ...config, groupIDs: [...config.groupIDs] })
  mocks.listGroupNotifications.mockResolvedValue([])
  mocks.saveGroupNotificationConfig.mockImplementation(async input => ({ ...config, ...input, apiKey: undefined }))
  mocks.sendGroupNotification.mockResolvedValue({ group: { delivered: 1, failed: 0 } })
})

async function render() {
  const wrapper = mount(GroupNotificationsPanel, { global: { stubs: { ConfirmDialog: true } } })
  await flushPromises()
  return wrapper
}

describe('group notification panel', () => {
  it('keeps the stored key secret, parses recipients, and blocks sends until settings are saved', async () => {
    const wrapper = await render()
    expect((wrapper.get('#group-api-key').element as HTMLInputElement).value).toBe('')
    expect(wrapper.find('#group-site-url').exists()).toBe(false)
    expect(wrapper.find('#group-url').exists()).toBe(false)
    expect((wrapper.get('#group-config-id').element as HTMLInputElement).value).toBe('default')
    await wrapper.get('#group-title').setValue('提醒')
    await wrapper.get('#group-body').setValue('0点结束。')
    const sendButton = wrapper.findAll('button[type="submit"]')[1]
    expect(sendButton.attributes('disabled')).toBeUndefined()
    await wrapper.get('#group-ids').setValue('764163343，1132213496\n764163343')
    expect(sendButton.attributes('disabled')).toBeDefined()
    await wrapper.findAll('form')[0].trigger('submit')
    await flushPromises()
    expect(mocks.saveGroupNotificationConfig).toHaveBeenCalledWith(expect.objectContaining({ groupIDs: ['764163343', '1132213496'] }))
    expect(mocks.saveGroupNotificationConfig.mock.calls[0][0]).not.toHaveProperty('apiKey')
    wrapper.unmount()
  })

  it('requires confirmation, prevents double sends, and clears text after success', async () => {
    let resolve!: (value: any) => void
    mocks.sendGroupNotification.mockImplementation(() => new Promise(done => { resolve = done }))
    const wrapper = await render()
    await wrapper.get('#group-title').setValue('活动提醒')
    await wrapper.get('#group-body').setValue('双11充值活动0点结束。需要的抓紧。')
    await wrapper.findAll('form')[1].trigger('submit')
    expect(mocks.sendGroupNotification).not.toHaveBeenCalled()
    const dialog = wrapper.getComponent(ConfirmDialog)
    expect(dialog.props('show')).toBe(true)
    dialog.vm.$emit('confirm')
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(mocks.sendGroupNotification).toHaveBeenCalledTimes(1)
    expect(mocks.sendGroupNotification).toHaveBeenCalledWith(expect.objectContaining({ title: '活动提醒', requestID: expect.any(String) }))
    resolve({ group: { delivered: 1, failed: 0 } })
    await flushPromises()
    expect((wrapper.get('#group-body').element as HTMLTextAreaElement).value).toBe('')
    expect(mocks.listGroupNotifications).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('retains content and request ID on failure so a retry does not duplicate delivered messages', async () => {
    mocks.sendGroupNotification.mockRejectedValue(new Error('结果不确定，请核对群消息'))
    const wrapper = await render()
    await wrapper.get('#group-title').setValue('Test')
    await wrapper.get('#group-body').setValue('Message')
    for (let n = 0; n < 2; n++) {
      await wrapper.findAll('form')[1].trigger('submit')
      wrapper.getComponent(ConfirmDialog).vm.$emit('confirm')
      await flushPromises()
    }
    expect(mocks.sendGroupNotification.mock.calls[0][0].requestID).toBe(mocks.sendGroupNotification.mock.calls[1][0].requestID)
    expect((wrapper.get('#group-body').element as HTMLTextAreaElement).value).toBe('Message')
    expect(mocks.showError).toHaveBeenCalledWith('结果不确定，请核对群消息')
    wrapper.unmount()
  })

  it('rejects invalid groups, replaces a key only when supplied, and supports recovering a failed load', async () => {
    mocks.getGroupNotificationConfig.mockRejectedValueOnce(new Error('Unavailable'))
    const wrapper = await render()
    expect(wrapper.find('#group-api-key').exists()).toBe(false)
    await wrapper.get('button').trigger('click')
    await flushPromises()
    await wrapper.get('#group-ids').setValue('invalid')
    await wrapper.findAll('form')[0].trigger('submit')
    expect(mocks.saveGroupNotificationConfig).not.toHaveBeenCalled()
    await wrapper.get('#group-ids').setValue('764163343')
    await wrapper.get('#group-api-key').setValue('replacement-key')
    await wrapper.findAll('form')[0].trigger('submit')
    await flushPromises()
    expect(mocks.saveGroupNotificationConfig.mock.calls[0][0].apiKey).toBe('replacement-key')
    expect((wrapper.get('#group-api-key').element as HTMLInputElement).value).toBe('')
    wrapper.unmount()
  })
})
