import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AnnouncementsView from '../AnnouncementsView.vue'
import Select from '@/components/common/Select.vue'

const mocks = vi.hoisted(() => ({
  list: vi.fn(), create: vi.fn(), update: vi.fn(), getAll: vi.fn(),
  sendGroupNotification: vi.fn(), showError: vi.fn(), showSuccess: vi.fn(),
}))
vi.mock('@/api/admin', () => ({ adminAPI: { announcements: mocks, groups: mocks } }))
vi.mock('@/api/pushNotifications', () => ({ sendGroupNotification: mocks.sendGroupNotification }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string, values?: { error?: string }) => values?.error ? `${key}: ${values.error}` : key }),
}))

beforeEach(() => {
  vi.resetAllMocks()
  mocks.list.mockResolvedValue({ items: [], page: 1, page_size: 20, total: 0, pages: 0 })
  mocks.getAll.mockResolvedValue([])
  mocks.create.mockImplementation(async payload => ({ ...payload, id: 42, starts_at: '2099-01-01T00:00:00Z' }))
  mocks.sendGroupNotification.mockResolvedValue({ group: { delivered: 1, failed: 0 } })
})

async function render() {
  const wrapper = mount(AnnouncementsView, { global: { stubs: {
    AppLayout: { template: '<main><slot /></main>' },
    TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
    BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
    AnnouncementTargetingEditor: true, AnnouncementReadStatusDialog: true,
    AnnouncementPopup: true, ConfirmDialog: true, DataTable: true, Icon: true,
  } } })
  await flushPromises()
  await wrapper.findAll('button').find(button => button.text() === 'admin.announcements.createAnnouncement')!.trigger('click')
  await wrapper.get('#announcement-form input[type="text"]').setValue('维护公告')
  await wrapper.get('#announcement-form textarea').setValue('凌晨维护，请提前保存。')
  return wrapper
}

async function publish(wrapper: Awaited<ReturnType<typeof render>>) {
  const status = wrapper.findAllComponents(Select).find(component => component.props('options').length === 3 && component.props('options')[0].value === 'draft')!
  status.vm.$emit('update:modelValue', 'active')
  await wrapper.vm.$nextTick()
}

describe('announcement group notification', () => {
  it('leaves group notification unchecked and preserves existing website notification settings', async () => {
    const wrapper = await render()
    expect((wrapper.get('#announcement-group-notify').element as HTMLInputElement).checked).toBe(false)
    expect(wrapper.get('#announcement-group-notify').attributes('disabled')).toBeDefined()
    await publish(wrapper)
    await wrapper.get('#announcement-form').trigger('submit')
    await flushPromises()
    expect(mocks.create).toHaveBeenCalledWith(expect.objectContaining({ status: 'active', notify_mode: 'silent' }))
    expect(mocks.sendGroupNotification).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('passes saved announcement facts and Beijing schedule to the shared group sender', async () => {
    const wrapper = await render()
    await publish(wrapper)
    await wrapper.get('#announcement-group-notify').setValue(true)
    await wrapper.get('#announcement-form').trigger('submit')
    await flushPromises()
    expect(mocks.sendGroupNotification).toHaveBeenCalledTimes(1)
    const payload = mocks.sendGroupNotification.mock.calls[0][0]
    expect(payload.requestID).toBe('announcement-42')
    expect(JSON.parse(payload.body)).toMatchObject({ 类型: '公告', 标题: '维护公告', 内容: '凌晨维护，请提前保存。' })
    expect(JSON.parse(payload.body).生效时间).toContain('08:00:00（北京时间）')
    expect(JSON.parse(payload.body).通知方式).toContain('作为预告')
    expect(payload).not.toHaveProperty('url')
    wrapper.unmount()
  })

  it('retries only group delivery after saving, keeping the same request ID and preventing duplicate creation', async () => {
    mocks.sendGroupNotification.mockRejectedValueOnce(new Error('连接失败'))
    const wrapper = await render()
    await publish(wrapper)
    await wrapper.get('#announcement-group-notify').setValue(true)
    await wrapper.get('#announcement-form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('admin.announcements.groupNotifyPending')
    expect(wrapper.get('fieldset').attributes('disabled')).toBeDefined()
    expect(mocks.showError).toHaveBeenCalledWith('admin.announcements.groupNotifyFailed: 连接失败')
    expect(mocks.showSuccess).not.toHaveBeenCalled()
    await wrapper.get('#announcement-form').trigger('submit')
    await flushPromises()
    expect(mocks.create).toHaveBeenCalledTimes(1)
    expect(mocks.sendGroupNotification).toHaveBeenCalledTimes(2)
    expect(mocks.sendGroupNotification.mock.calls.map(([input]) => input.requestID)).toEqual(['announcement-42', 'announcement-42'])
    expect(wrapper.find('#announcement-form').exists()).toBe(false)
    wrapper.unmount()
  })

  it('does not send drafts even after checking the box and clears the checkbox for the next announcement', async () => {
    const wrapper = await render()
    await publish(wrapper)
    await wrapper.get('#announcement-group-notify').setValue(true)
    const status = wrapper.findAllComponents(Select).find(component => component.props('modelValue') === 'active')!
    status.vm.$emit('update:modelValue', 'draft')
    await wrapper.vm.$nextTick()
    await wrapper.get('#announcement-form').trigger('submit')
    await flushPromises()
    expect(mocks.sendGroupNotification).not.toHaveBeenCalled()
    await wrapper.findAll('button').find(button => button.text() === 'admin.announcements.createAnnouncement')!.trigger('click')
    expect((wrapper.get('#announcement-group-notify').element as HTMLInputElement).checked).toBe(false)
    wrapper.unmount()
  })
})
