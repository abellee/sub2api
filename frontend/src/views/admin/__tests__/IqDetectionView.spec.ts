import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import IqDetectionView from '../IqDetectionView.vue'
import type { SelectionGroup } from '@/api/admin/iqDetection'

const mocks = vi.hoisted(() => ({
  getSelection: vi.fn(),
  saveSelection: vi.fn(),
  listTestResults: vi.fn(),
  triggerTestRun: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    iqDetection: {
      ...mocks,
      getHealth: vi.fn().mockResolvedValue({ enabled: true, base_url: 'local' }),
    },
    groups: {
      list: vi.fn().mockResolvedValue({ items: [
        { id: 1, name: 'Group One', platform: 'openai', rate_multiplier: 0.5, account_count: 1 },
        { id: 2, name: 'Group Two', platform: 'openai', rate_multiplier: 1, account_count: 1 },
      ] }),
    },
    accounts: {
      list: vi.fn().mockResolvedValue({ items: [
        { id: 42, name: 'Account', platform: 'openai', credentials: {} },
      ] }),
    },
  },
}))
vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: mocks.showError, showSuccess: vi.fn() }),
}))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))

let savedGroups: SelectionGroup[]
let wrapper: ReturnType<typeof mount>

beforeEach(() => {
  vi.clearAllMocks()
  savedGroups = [
    { id: 1, name: 'Group One', rate_multiplier: 0.5, interval_minutes: 30, accounts: [{ id: 42 }] },
    { id: 2, name: 'Group Two', rate_multiplier: 1, interval_minutes: 60, visible: false, accounts: [{ id: 42 }] },
  ]
  mocks.getSelection.mockImplementation(async () => ({
    groups: structuredClone(savedGroups), updated_at: '2026-10-18T00:00:00Z',
  }))
  mocks.saveSelection.mockImplementation(async (groups: SelectionGroup[]) => {
    savedGroups = structuredClone(groups)
    return { groups: savedGroups, updated_at: '2026-10-18T00:01:00Z' }
  })
  mocks.listTestResults.mockResolvedValue({ items: [], total: 0 })
  mocks.triggerTestRun.mockResolvedValue({ triggered: 1, items: [] })
  wrapper = mount(IqDetectionView, {
    global: { stubs: {
      AppLayout: { template: '<main><slot /></main>' },
      BaseDialog: { props: ['show'], template: '<section v-if="show"><slot /></section>' },
      Icon: true,
    } },
  })
})

afterEach(() => wrapper.unmount())

async function focus(groupId: number) {
  await wrapper.get(`[data-testid="group-item-${groupId}"]`).trigger('click')
  await flushPromises()
}

describe('IQ detection group visibility', () => {
  it('defaults legacy groups to visible and restores hidden groups independently', async () => {
    await flushPromises()
    await wrapper.get('[data-testid="open-selector"]').trigger('click')
    await flushPromises()
    await focus(1)
    expect((wrapper.get('[data-testid="group-visible-checkbox"]').element as HTMLInputElement).checked).toBe(true)
    await focus(2)
    expect((wrapper.get('[data-testid="group-visible-checkbox"]').element as HTMLInputElement).checked).toBe(false)
    expect((wrapper.get('[data-testid="group-checkbox-2"]').element as HTMLInputElement).checked).toBe(true)
  })

  it('saves public visibility without changing selected accounts, intervals or other groups', async () => {
    await flushPromises()
    await wrapper.get('[data-testid="open-selector"]').trigger('click')
    await flushPromises()
    await focus(1)
    await wrapper.get('[data-testid="group-visible-checkbox"]').setValue(false)
    await focus(2)
    await wrapper.get('[data-testid="save-selection"]').trigger('click')
    await flushPromises()
    expect(mocks.saveSelection).toHaveBeenCalledOnce()
    expect(savedGroups.map(({ id, interval_minutes, visible, accounts }) => ({
      id, interval_minutes, visible, accountIds: accounts.map(account => account.id),
    }))).toEqual([
      { id: 1, interval_minutes: 30, visible: false, accountIds: [42] },
      { id: 2, interval_minutes: 60, visible: false, accountIds: [42] },
    ])
    await wrapper.get('[data-testid="open-selector"]').trigger('click')
    await flushPromises()
    await focus(1)
    expect((wrapper.get('[data-testid="group-visible-checkbox"]').element as HTMLInputElement).checked).toBe(false)
    expect(mocks.showError).not.toHaveBeenCalled()
  })
})

describe('IQ detection manual test scope', () => {
  it('keeps the all-tests action unfiltered on every tab', async () => {
    await flushPromises()
    await wrapper.get('[data-testid="test-result-tab-2"]').trigger('click')
    await wrapper.get('[data-testid="run-all-tests"]').trigger('click')
    await flushPromises()
    expect(mocks.triggerTestRun).toHaveBeenCalledWith()

    await wrapper.get('[data-testid="test-result-tab-0"]').trigger('click')
    await wrapper.get('[data-testid="run-all-tests"]').trigger('click')
    await flushPromises()
    expect(mocks.triggerTestRun.mock.calls).toEqual([[], []])
  })

  it('tests the active result tab rather than the group focused in settings', async () => {
    await flushPromises()
    await wrapper.get('[data-testid="open-selector"]').trigger('click')
    await flushPromises()
    await focus(1)
    await wrapper.get('[data-testid="test-result-tab-2"]').trigger('click')
    await wrapper.get('[data-testid="run-test"]').trigger('click')
    await flushPromises()
    expect(mocks.triggerTestRun).toHaveBeenLastCalledWith(2)

    await wrapper.get('[data-testid="test-result-tab-1"]').trigger('click')
    await wrapper.get('[data-testid="run-test"]').trigger('click')
    await flushPromises()
    expect(mocks.triggerTestRun.mock.calls).toEqual([[2], [1]])
  })

  it('disables current-group testing for ungrouped history or an empty selection', async () => {
    await flushPromises()
    await wrapper.get('[data-testid="test-result-tab-0"]').trigger('click')
    expect(wrapper.get('[data-testid="run-test"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="run-test"]').trigger('click')
    expect(mocks.triggerTestRun).not.toHaveBeenCalled()

    savedGroups = []
    await wrapper.get('button[title="common.refresh"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="run-test"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="run-all-tests"]').attributes('disabled')).toBeUndefined()
  })

  it('blocks both actions while a request is pending and recovers after failure', async () => {
    await flushPromises()
    await wrapper.get('[data-testid="test-result-tab-1"]').trigger('click')
    let rejectRun!: (error: Error) => void
    mocks.triggerTestRun.mockImplementationOnce(() => new Promise((_, reject) => { rejectRun = reject }))
    await wrapper.get('[data-testid="run-test"]').trigger('click')
    expect(wrapper.get('[data-testid="run-test"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="run-all-tests"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="run-all-tests"]').trigger('click')
    expect(mocks.triggerTestRun).toHaveBeenCalledOnce()

    rejectRun(new Error('test unavailable'))
    await flushPromises()
    expect(mocks.showError).toHaveBeenCalledOnce()
    expect(wrapper.get('[data-testid="run-test"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[data-testid="run-all-tests"]').attributes('disabled')).toBeUndefined()
  })
})
