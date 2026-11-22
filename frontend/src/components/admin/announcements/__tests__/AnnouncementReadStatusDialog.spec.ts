import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AnnouncementReadStatusDialog from '../AnnouncementReadStatusDialog.vue'

const { getReadStatus, showError } = vi.hoisted(() => ({
  getReadStatus: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    announcements: {
      getReadStatus,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: { count?: number }) => params?.count === undefined ? key : `${key}: ${params.count}`,
    }),
  }
})

vi.mock('@/composables/usePersistedPageSize', () => ({
  getPersistedPageSize: () => 20,
}))

const BaseDialogStub = {
  props: ['show', 'title', 'width'],
  emits: ['close'],
  template: '<div><slot /><slot name="footer" /></div>',
}

const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue', 'change'],
  template: `<select :value="modelValue" @change="$emit('update:modelValue', $event.target.value); $emit('change', $event.target.value)">
    <option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option>
  </select>`,
}

const mountDialog = () => mount(AnnouncementReadStatusDialog, {
  props: { show: false, announcementId: 1 },
  global: { stubs: { BaseDialog: BaseDialogStub, DataTable: true, Pagination: true, Icon: true, Select: SelectStub } },
})

describe('AnnouncementReadStatusDialog', () => {
  beforeEach(() => {
    getReadStatus.mockReset()
    showError.mockReset()
    vi.useFakeTimers()
  })

  afterEach(() => vi.useRealTimers())

  it('shows the full read count beside pagination and filters read users from page one', async () => {
    getReadStatus.mockResolvedValue({ items: [], total: 80, page: 1, page_size: 20, pages: 4, read_count: 42 })
    const wrapper = mountDialog()
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(wrapper.findAll('option').map((option) => option.text())).toEqual(['common.all', 'admin.announcements.read'])
    expect(wrapper.find('[aria-live="polite"]').text()).toBe('admin.announcements.readCount: 42')
    wrapper.findComponent({ name: 'Pagination' }).vm.$emit('update:page', 3)
    await flushPromises()
    expect(getReadStatus.mock.lastCall?.[1]).toBe(3)
    getReadStatus.mockResolvedValue({ items: [{ user_id: 2, read_at: '2026-01-01' }], total: 42, page: 1, page_size: 20, pages: 3, read_count: 42 })
    await wrapper.find('select').setValue('read')
    await flushPromises()
    expect(getReadStatus.mock.lastCall?.slice(0, 4)).toEqual([1, 1, 20, { search: '', read_status: 'read', sort_by: 'email', sort_order: 'asc' }])
    expect(wrapper.findComponent({ name: 'Pagination' }).props('total')).toBe(42)
    expect(wrapper.findComponent({ name: 'DataTable' }).props('data')).toEqual([{ user_id: 2, read_at: '2026-01-01' }])

    await wrapper.find('input').setValue('alice')
    vi.advanceTimersByTime(300)
    await flushPromises()
    expect(getReadStatus.mock.lastCall?.[3]).toMatchObject({ search: 'alice', read_status: 'read' })
    wrapper.findComponent({ name: 'DataTable' }).vm.$emit('sort', 'balance', 'desc')
    await flushPromises()
    expect(getReadStatus.mock.lastCall?.[3]).toMatchObject({ search: 'alice', read_status: 'read', sort_by: 'balance', sort_order: 'desc' })
    await wrapper.find('select').setValue('all')
    await flushPromises()
    expect(getReadStatus.mock.lastCall?.[3].read_status).toBe('all')
    wrapper.unmount()
  })

  it('ignores late unfiltered results and keeps the read count visible for empty results', async () => {
    let finishOld!: (value: unknown) => void
    let oldSignal: AbortSignal | undefined
    getReadStatus.mockImplementationOnce((...args: any[]) => {
      oldSignal = args[4].signal
      return new Promise((resolve) => { finishOld = resolve })
    })
    const wrapper = mountDialog()
    await wrapper.setProps({ show: true })
    getReadStatus.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 1, read_count: 0 })
    await wrapper.find('select').setValue('read')
    await flushPromises()
    expect(oldSignal?.aborted).toBe(true)
    finishOld({ items: [{ user_id: 9 }], total: 80, page: 1, page_size: 20, pages: 4, read_count: 40 })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'DataTable' }).props('data')).toEqual([])
    expect(wrapper.find('[aria-live="polite"]').text()).toBe('admin.announcements.readCount: 0')
    expect(wrapper.findComponent({ name: 'Pagination' }).exists()).toBe(false)
    await wrapper.setProps({ show: false })
    expect(wrapper.find('select').element.value).toBe('all')
    wrapper.unmount()
  })

  it('closes by aborting active requests and clearing debounced reloads', async () => {
    let activeSignal: AbortSignal | undefined
    getReadStatus.mockImplementation(async (...args: any[]) => {
      activeSignal = args[4]?.signal
      return new Promise(() => {})
    })

    const wrapper = mount(AnnouncementReadStatusDialog, {
      props: {
        show: false,
        announcementId: 1,
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          DataTable: true,
          Pagination: true,
          Icon: true,
          Select: SelectStub,
        },
      },
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(getReadStatus).toHaveBeenCalledTimes(1)
    expect(activeSignal?.aborted).toBe(false)

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.search = 'alice'
    setupState.handleSearch()

    setupState.handleClose()
    await flushPromises()

    expect(activeSignal?.aborted).toBe(true)
    expect(wrapper.emitted('close')).toHaveLength(1)

    vi.advanceTimersByTime(350)
    await flushPromises()

    expect(getReadStatus).toHaveBeenCalledTimes(1)
  })
})
