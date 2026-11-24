import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import PointsModal from '../PointsModal.vue'
import type { PointsSummary } from '@/api/points'

const api = vi.hoisted(() => ({ summary: vi.fn(), history: vi.fn() }))
vi.mock('@/api/points', () => ({ fetchMyPoints: api.summary, fetchMyPointsHistory: api.history }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: ref('en') }) }))
vi.mock('@/utils/format', () => ({ formatDateTime: (value: string) => value }))
const summary = { balance: '120', total_earned: '150', total_spent: '30' }
const rows = [{ id: '1', kind: 'spend', delta: '-30', balance_after: '70', note: 'Used on recharge', created_at: '2026-11-24T12:00:00Z' }]
const wrappers: ReturnType<typeof mount>[] = []
function render(show = true) {
  const wrapper = mount(PointsModal, { props: { show, userId: 2 }, global: { stubs: {
    BaseDialog: { props: ['show'], template: '<div v-if="show" role="dialog"><slot /></div>' }
  } } })
  wrappers.push(wrapper)
  return wrapper
}
beforeEach(() => {
  vi.resetAllMocks()
  api.summary.mockResolvedValue(summary)
  api.history.mockResolvedValue({ items: rows, total: 11, page: 1, page_size: 10 })
})
afterEach(() => { wrappers.forEach(wrapper => wrapper.unmount()); wrappers.length = 0 })

describe('PointsModal', () => {
  it('loads only when opened and shows current points, totals and signed records', async () => {
    const wrapper = render(false)
    expect(api.summary).not.toHaveBeenCalled()
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(api.history).toHaveBeenCalledWith(1, 'all')
    expect(wrapper.find('.points-current strong').text()).toBe('120')
    expect(wrapper.find('.points-summary').text()).toContain('150')
    expect(wrapper.find('.points-summary').text()).toContain('30')
    expect(wrapper.find('.points-record').text()).toContain('Used on recharge')
    expect(wrapper.find('.is-spent').text()).toBe('-30')
    expect(wrapper.emitted('loaded')?.[0]).toEqual([summary])
    await wrapper.find('.points-pagination button:last-child').trigger('click')
    await flushPromises()
    expect(api.history).toHaveBeenLastCalledWith(2, 'all')
    await wrapper.find('select').setValue('spend')
    await flushPromises()
    expect(api.history).toHaveBeenLastCalledWith(1, 'spend')
    expect(wrapper.find('.points-pagination').text()).toContain('1 / 2')
  })

  it('keeps zero and empty history distinct from a failed request, and retries', async () => {
    api.summary.mockRejectedValueOnce(new Error('unavailable'))
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.find('.points-current strong').text()).toBe('—')
    expect(wrapper.find('.points-record').exists()).toBe(false)
    api.summary.mockResolvedValue({ balance: '0', total_earned: '0', total_spent: '0' })
    api.history.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10 })
    await wrapper.find('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.find('.points-current strong').text()).toBe('0')
    expect(wrapper.text()).toContain('common.pointsDetails.empty')
    expect(wrapper.find('.points-pagination').exists()).toBe(false)
  })

  it('does not round bigint points', async () => {
    api.summary.mockResolvedValue({ ...summary, balance: '9007199254740993' })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('.points-current strong').text()).toBe('9,007,199,254,740,993')
  })

  it('ignores an old user request that finishes after the identity changes', async () => {
    let resolveOld!: (value: PointsSummary) => void
    api.summary.mockReturnValueOnce(new Promise<PointsSummary>(resolve => { resolveOld = resolve }))
    const wrapper = render()
    api.summary.mockResolvedValue({ ...summary, balance: '5' })
    await wrapper.setProps({ userId: 3 })
    await flushPromises()
    expect(wrapper.find('.points-current strong').text()).toBe('5')
    resolveOld({ ...summary, balance: '999' })
    await flushPromises()
    expect(wrapper.find('.points-current strong').text()).toBe('5')
    expect(wrapper.emitted('loaded')).toHaveLength(1)
  })

  it('discards responses after closing the dialog', async () => {
    let resolveOld!: (value: PointsSummary) => void
    api.summary.mockReturnValueOnce(new Promise<PointsSummary>(resolve => { resolveOld = resolve }))
    const wrapper = render()
    await wrapper.setProps({ show: false })
    resolveOld(summary)
    await flushPromises()
    expect(wrapper.emitted('loaded')).toBeUndefined()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  })
})
