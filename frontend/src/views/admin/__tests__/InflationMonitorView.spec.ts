import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import InflationMonitorView from '../InflationMonitorView.vue'
import { inflationMonitorAPI } from '@/api/inflationMonitor'
const { save, showError } = vi.hoisted(() => ({ save: vi.fn(), showError: vi.fn() }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<slot />' } }))
vi.mock('@/api/inflationMonitor', () => ({ inflationMonitorAPI: { plans: vi.fn().mockResolvedValue([]), save } }))
vi.mock('@/api/admin/groups', () => ({ groupsAPI: { getAllIncludingInactive: vi.fn().mockResolvedValue([{ id: 27, name: 'Group' }]) } }))
vi.mock('@/api/admin/accounts', () => ({ accountsAPI: { list: vi.fn().mockResolvedValue({ items: [{ id: 233, name: 'Account' }], pages: 1 }) } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
describe('InflationMonitorView', () => {
  it('saves an enabled group with minutes, accounts, thresholds and median selection', async () => {
    const wrapper = mount(InflationMonitorView, { global: { stubs: { AppLayout: { template: '<slot />' } } } })
    await flushPromises()
    await wrapper.get('select').setValue(27)
    await wrapper.findAll('button').find(b => b.text() === 'inflationMonitor.addGroup')!.trigger('click')
    await flushPromises()
    const selects = wrapper.findAll('section select')
    await selects[2]!.setValue('median')
    await wrapper.get('section input[maxlength="200"]').setValue('gpt-test')
    await wrapper.get('section input[type="number"]').setValue(30)
    const thresholds = wrapper.findAll('section input[step="any"]')
    await thresholds[0]!.setValue(5); await thresholds[1]!.setValue(15)
    const checkboxes = wrapper.findAll('section input[type="checkbox"]')
    await checkboxes[0]!.setValue(true); await checkboxes[1]!.setValue(true)
    await wrapper.findAll('button').find(b => b.text() === 'inflationMonitor.save')!.trigger('click')
    await flushPromises()
    expect(save).toHaveBeenCalledWith([expect.objectContaining({ group_id: 27, enabled: true, interval_minutes: 30, account_ids: [233], aggregation: 'median', yellow_threshold: 5, red_threshold: 15 })])
    expect(showError).not.toHaveBeenCalled()
    wrapper.unmount()
    expect(inflationMonitorAPI.plans).toHaveBeenCalled()
  })
})
