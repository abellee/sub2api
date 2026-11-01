import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import StudioTokenTrend from '../StudioTokenTrend.vue'

enableAutoUnmount(afterEach)
vi.mock('vue-i18n', async () => ({ ...await vi.importActual('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-chartjs', () => ({ Line: { name: 'Line', props: ['data', 'options'], template: '<div class="chart-data">{{ JSON.stringify(data) }}</div>' } }))

describe('StudioTokenTrend', () => {
  it('keeps all four native token series and the native cache hit rate formula', () => {
    const wrapper = mount(StudioTokenTrend, { props: { trendData: [{ date: '2026-11-01', requests: 1, input_tokens: 100, output_tokens: 20, cache_creation_tokens: 50, cache_read_tokens: 150, total_tokens: 320, cost: 0.2, actual_cost: 0.1 }] } })
    const data = JSON.parse(wrapper.find('.chart-data').text())
    expect(data.datasets.map((dataset: { data: number[] }) => dataset.data[0])).toEqual([100, 20, 50, 150, 50])
  })
  it('toggles a native series through the aligned HTML legend', async () => {
    const wrapper = mount(StudioTokenTrend, { props: { trendData: [{ date: '2026-11-01', requests: 1, input_tokens: 100, output_tokens: 20, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 120, cost: 0.2, actual_cost: 0.1 }] } })
    const button = wrapper.find('.trend-legend button')
    await button.trigger('click')
    expect(button.attributes('aria-pressed')).toBe('false')
    expect(wrapper.findComponent({ name: 'Line' }).props('data').datasets[0].hidden).toBe(true)
    await button.trigger('click')
    expect(wrapper.findComponent({ name: 'Line' }).props('data').datasets[0].hidden).toBe(false)
  })
  it('updates chart colors when the theme changes and handles a zero prompt count', async () => {
    const wrapper = mount(StudioTokenTrend, { props: { trendData: [{ date: '2026-11-01', requests: 0, input_tokens: 0, output_tokens: 0, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 0, cost: 0, actual_cost: 0 }] } })
    const chart = wrapper.findComponent({ name: 'Line' })
    const previous = chart.props('options').scales.x.ticks.color
    const wasDark = document.documentElement.classList.contains('dark')
    window.dispatchEvent(new CustomEvent('tokenfree:theme-change', { detail: wasDark ? 'light' : 'dark' }))
    await nextTick()
    expect(chart.props('options').scales.x.ticks.color).not.toBe(previous)
    expect(chart.props('data').datasets[4].data).toEqual([0])
    window.dispatchEvent(new CustomEvent('tokenfree:theme-change', { detail: wasDark ? 'dark' : 'light' }))
  })
})
