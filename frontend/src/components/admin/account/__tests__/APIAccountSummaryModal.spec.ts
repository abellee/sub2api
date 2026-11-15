import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, shallowMount } from '@vue/test-utils'
import { ref } from 'vue'
import APIAccountSummaryModal from '../APIAccountSummaryModal.vue'

const { getSummary } = vi.hoisted(() => ({ getSummary: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: { getAPIAccountSummary: getSummary } } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: ref('en') }) }))
enableAutoUnmount(afterEach)

const totals = { requests: 2, tokens: 120, user_cost: 2.1, account_cost: 0.6, standard_cost: 10, margin: 1.5 }
const data = {
  base_url: 'https://api.example.com/v1', start_date: '2026-11-01', end_date: '2026-11-15',
  timezone: 'Asia/Shanghai', granularity: 'day', summary: totals,
  accounts: [{ account_id: 1, name: 'account one', ...totals }],
  periods: [{ period: '2026-11-01', ...totals }]
}
function mountModal() {
  return shallowMount(APIAccountSummaryModal, {
    props: { show: true },
    global: { stubs: { BaseDialog: { template: '<div><slot /></div>' } } }
  })
}

describe('API account summary', () => {
  beforeEach(() => vi.resetAllMocks())

  it('queries the whole API address and renders spending, cost and both breakdowns', async () => {
    getSummary.mockResolvedValue(data)
    const wrapper = mountModal()
    expect(getSummary).not.toHaveBeenCalled()
    await wrapper.find('input').setValue('https://api.example.com/v1')
    await wrapper.findComponent({ name: 'DateRangePicker' }).vm.$emit('update:startDate', '2026-11-01')
    await wrapper.findComponent({ name: 'DateRangePicker' }).vm.$emit('update:endDate', '2026-11-15')
    await wrapper.findComponent({ name: 'Select' }).vm.$emit('update:modelValue', 'month')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(getSummary).toHaveBeenCalledWith({
      base_url: 'https://api.example.com/v1', start_date: '2026-11-01', end_date: '2026-11-15',
      granularity: 'month', timezone: Intl.DateTimeFormat().resolvedOptions().timeZone
    }, expect.any(AbortSignal))
    expect(wrapper.text()).toContain('$2.10')
    expect(wrapper.text()).toContain('$0.60')
    expect(wrapper.text()).toContain('$1.50')
    const tables = wrapper.findAllComponents({ name: 'DataTable' })
    expect(tables[0].props('data')).toEqual(data.periods)
    expect(tables[1].props('data')).toEqual(data.accounts)
  })

  it.each(['edit', 'close'] as const)('discards a pending response after %s', async action => {
    let resolve!: (value: typeof data) => void
    getSummary.mockReturnValue(new Promise<typeof data>(done => { resolve = done }))
    const wrapper = mountModal()
    await wrapper.find('input').setValue('https://api.example.com/v1')
    await wrapper.find('form').trigger('submit')
    const signal = getSummary.mock.calls[0][1] as AbortSignal
    if (action === 'edit') await wrapper.find('input').setValue('https://other.example.com')
    else await wrapper.setProps({ show: false })
    expect(signal.aborted).toBe(true)
    resolve(data)
    await flushPromises()
    expect(wrapper.findAllComponents({ name: 'DataTable' })).toHaveLength(0)
  })

  it('shows query errors and distinguishes an unmatched address', async () => {
    getSummary.mockRejectedValueOnce({ message: 'date range too long' }).mockResolvedValueOnce({ ...data, accounts: [], periods: [] })
    const wrapper = mountModal()
    await wrapper.find('input').setValue('https://api.example.com/v1')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toBe('date range too long')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.accounts.apiSummary.noAccounts')
  })

  it.each([
    { status: 400, message: 'Invalid account ID' },
    { status: 404, message: 'Not Found' }
  ])('explains when the backend lacks the summary route ($status)', async error => {
    getSummary.mockRejectedValue(error)
    const wrapper = mountModal()
    await wrapper.find('input').setValue('https://api.example.com/v1')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toBe('admin.accounts.apiSummary.backendUnavailable')
    expect(wrapper.text()).not.toContain('Invalid account ID')
  })
})
