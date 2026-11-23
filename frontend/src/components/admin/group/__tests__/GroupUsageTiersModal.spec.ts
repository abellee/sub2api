import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import GroupUsageTiersModal from '../GroupUsageTiersModal.vue'
import type { AdminGroup } from '@/types'

const mocks = vi.hoisted(() => ({ getUsageTiers: vi.fn(), saveUsageTiers: vi.fn(), getAll: vi.fn(), showError: vi.fn(), showSuccess: vi.fn() }))
vi.mock('@/api/admin/groups', () => ({ getUsageTiers: mocks.getUsageTiers, saveUsageTiers: mocks.saveUsageTiers }))
vi.mock('@/api/admin', () => ({ adminAPI: { groups: { getAllIncludingInactive: mocks.getAll } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const config = () => ({ enabled: true, tiers: [{ daily_cost: 10, consecutive_days: 2, max_missed_days: 3, rate_multiplier: 0.8 }], linked_group_ids: [2], excluded_user_ids: [3] })
const render = () => mount(GroupUsageTiersModal, {
  props: { show: true, group: { id: 1, name: 'Source', rate_multiplier: 1 } as AdminGroup },
  global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' } } },
})

describe('group usage tier settings', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    mocks.getUsageTiers.mockResolvedValue(config())
    mocks.getAll.mockResolvedValue([{ id: 1, name: 'Source' }, { id: 2, name: 'Linked' }])
    mocks.saveUsageTiers.mockResolvedValue(undefined)
  })
  it('loads links and saves exclusions with the complete rule', async () => {
    const wrapper = render()
    await flushPromises()
    expect(wrapper.findAll('[data-test="tier-row"]')).toHaveLength(1)
    await wrapper.get('#tier-excluded-users').setValue('3, 4\n4')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.saveUsageTiers).toHaveBeenCalledWith(1, { ...config(), excluded_user_ids: [3, 4] })
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })
  it('rejects invalid IDs and keeps the dialog open on a failed save', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('#tier-excluded-users').setValue('wrong')
    await wrapper.get('form').trigger('submit')
    expect(mocks.saveUsageTiers).not.toHaveBeenCalled()
    await wrapper.get('#tier-excluded-users').setValue('3')
    mocks.saveUsageTiers.mockRejectedValue(new Error('offline'))
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.emitted('close')).toBeUndefined()
    expect(mocks.showError).toHaveBeenCalled()
    wrapper.unmount()
  })
  it('prevents an older request from replacing a newly selected group', async () => {
    let resolveOld!: (value: ReturnType<typeof config>) => void
    mocks.getUsageTiers.mockReturnValueOnce(new Promise(resolve => { resolveOld = resolve }))
    const wrapper = render()
    await wrapper.setProps({ group: { id: 2, name: 'New' } as AdminGroup })
    await flushPromises()
    resolveOld({ ...config(), excluded_user_ids: [999] })
    await flushPromises()
    expect((wrapper.get('#tier-excluded-users').element as HTMLTextAreaElement).value).toBe('3')
    wrapper.unmount()
  })
})
