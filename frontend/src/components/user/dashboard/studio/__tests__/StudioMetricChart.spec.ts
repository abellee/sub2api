import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import StudioMetricChart from '../StudioMetricChart.vue'
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
describe('StudioMetricChart', () => {
  it('shows unavailable points without inventing an amount or balance percentage', () => {
    const wrapper = mount(StudioMetricChart, { props: { kind: 'balance', value: 75, reference: 25 } })
    expect(wrapper.find('.balance-detail').text()).toBe('common.points—')
    expect(wrapper.find('.balance-detail img').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('$25.00')
    expect(wrapper.find('circle').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('%')
  })
  it('compares actual cost with standard cost without capping actual cost', () => {
    const wrapper = mount(StudioMetricChart, { props: { kind: 'cost', value: 4, reference: 2 } })
    expect(wrapper.findAll('.cost-bars i').map(bar => bar.attributes('style'))).toEqual(['width: 100%;', 'width: 50%;'])
  })
  it('uses only the provided request history and keeps zero data finite', () => {
    const wrapper = mount(StudioMetricChart, { props: { kind: 'requests', value: 0, points: [0, 0] } })
    expect(wrapper.findAll('.request-bar').map(bar => bar.attributes('height'))).toEqual(['0', '0'])
    const populated = mount(StudioMetricChart, { props: { kind: 'requests', value: 6, points: [2, 4], labels: ['2026-10-31', '2026-11-01'], caption: '10-31 — 11-01 · Day' } })
    expect(populated.findAll('.request-bar').map(bar => bar.attributes('height'))).toEqual(['22', '44'])
    expect(populated.findAll('.request-bar title').map(title => title.text())).toEqual(['2026-10-31 · dashboard.requests: 2', '2026-11-01 · dashboard.requests: 4'])
    expect(populated.find('.chart-caption').text()).toBe('10-31 — 11-01 · Day')
    const emptyBalance = mount(StudioMetricChart, { props: { kind: 'balance', value: 0, reference: 0 } })
    expect(emptyBalance.find('.balance-detail strong').text()).toBe('—')
  })
})
