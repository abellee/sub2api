import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import DashboardStudioView from '../DashboardStudioView.vue'

enableAutoUnmount(afterEach)
const api = vi.hoisted(() => ({ stats: vi.fn(), trend: vi.fn(), models: vi.fn(), refreshUser: vi.fn(), user: { balance: 12.5 } }))
vi.mock('@/api/usage', () => ({ usageAPI: { getDashboardStats: api.stats, getDashboardTrend: api.trend, getDashboardModels: api.models } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: api.user, isSimpleMode: false, refreshUser: api.refreshUser }) }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
const stats = { total_api_keys: 0, active_api_keys: 0, total_requests: 44, total_input_tokens: 100, total_output_tokens: 200, total_cache_creation_tokens: 300, total_cache_read_tokens: 400, total_tokens: 1000, total_cost: 4, total_actual_cost: 3, today_requests: 7, today_input_tokens: 10, today_output_tokens: 20, today_cache_creation_tokens: 30, today_cache_read_tokens: 40, today_tokens: 100, today_cost: 2, today_actual_cost: 1, average_duration_ms: 1250, rpm: 2, tpm: 30 }
const stubs = {
  DashboardStudioLayout: { template: '<main><slot name="filters" /><slot /></main>' },
  Icon: true, LoadingSpinner: true,
  RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
  DateRangePicker: { name: 'DateRangePicker', props: ['startDate', 'endDate'], template: '<button />' },
  Select: { name: 'Select', props: ['modelValue', 'options'], template: '<button />' },
  ModelPieScene: { name: 'ModelPieScene', props: ['models'], template: '<div />' },
  StudioTokenTrend: { name: 'StudioTokenTrend', props: ['trendData', 'loading'], template: '<div />' }
}
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>(r => { resolve = r }); return { promise, resolve } }

beforeEach(() => {
  vi.resetAllMocks()
  api.user.balance = 12.5
  api.refreshUser.mockResolvedValue(undefined)
  api.stats.mockResolvedValue(stats)
  api.trend.mockResolvedValue({ trend: [] })
  api.models.mockResolvedValue({ models: [] })
})

describe('DashboardStudioView', () => {
  it('shows recharge below 10 and links to the existing purchase page', async () => {
    for (const amount of [0, 9.99, 10, 10.01]) {
      api.user.balance = amount
      const wrapper = mount(DashboardStudioView, { global: { stubs } })
      await flushPromises()
      const link = wrapper.find('.balance-recharge')
      expect(link.exists()).toBe(amount < 10)
      if (amount < 10) expect(link.attributes('href')).toBe('/purchase')
      wrapper.unmount()
    }
  })
  it('keeps seven daily request bars including today independent of chart filters', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-11-01T12:00:00'))
    try {
      api.trend.mockResolvedValueOnce({ trend: [] }).mockResolvedValueOnce({ trend: [
        { date: '2026-10-26', requests: 3 }, { date: '2026-11-01', requests: 7 }
      ] }).mockResolvedValue({ trend: [] })
      const wrapper = mount(DashboardStudioView, { global: { stubs } })
      await flushPromises()
      expect(api.trend).toHaveBeenNthCalledWith(2, { start_date: '2026-10-26', end_date: '2026-11-01', granularity: 'day' })
      const titles = wrapper.findAll('.request-bar title').map(item => item.text())
      expect(titles).toHaveLength(7)
      expect(titles[0]).toBe('2026-10-26 · dashboard.requests: 3')
      expect(titles[1]).toBe('2026-10-27 · dashboard.requests: 0')
      expect(titles[6]).toBe('2026-11-01 · dashboard.requests: 7')
      wrapper.findComponent({ name: 'Select' }).vm.$emit('update:modelValue', 'hour')
      await flushPromises()
      expect(wrapper.findAll('.request-bar title').map(item => item.text())).toEqual(titles)
      expect(wrapper.find('.request-card .metric-value').text()).toBe('7')
    } finally { vi.useRealTimers() }
  })
  it('does not turn a failed recent request lookup into seven zero bars and retries on refresh', async () => {
    api.trend.mockResolvedValueOnce({ trend: [] }).mockRejectedValueOnce(new Error('offline'))
    const wrapper = mount(DashboardStudioView, { global: { stubs } })
    await flushPromises()
    expect(wrapper.findAll('.request-bar')).toHaveLength(0)
    expect(wrapper.find('.request-card .chart-caption').text()).toBe('dashboard.studio.chartError')
    await wrapper.find('.refresh-button').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.request-bar')).toHaveLength(7)
  })
  it('isolates development sample data from live APIs and offers a return to live data', async () => {
    const previous = window.location.href
    window.history.replaceState({}, '', '/dashboard?studioPreview=1')
    try {
      const wrapper = mount(DashboardStudioView, { global: { stubs } })
      await vi.waitFor(() => expect(wrapper.find('.studio-preview').exists()).toBe(true))
      expect(wrapper.find('.balance-card .metric-value').text()).toBe('$286.50')
      expect(wrapper.find('.studio-preview a').attributes('href')).toBe('/dashboard')
      expect(wrapper.findComponent({ name: 'ModelPieScene' }).props('models')).toHaveLength(6)
      expect(wrapper.findComponent({ name: 'StudioTokenTrend' }).props('trendData')).toHaveLength(7)
      expect(api.stats).not.toHaveBeenCalled()
      expect(api.trend).not.toHaveBeenCalled()
      expect(api.refreshUser).not.toHaveBeenCalled()
    } finally { window.history.replaceState({}, '', previous) }
  })
  it('preserves native balances, billing and token totals with only the requested panels', async () => {
    const wrapper = mount(DashboardStudioView, { global: { stubs } })
    await flushPromises()
    expect(wrapper.find('.balance-card .metric-value').text()).toBe('$12.50')
    expect(wrapper.find('.cost-card .metric-value').text()).toBe('$1.0000')
    expect(wrapper.find('.cost-card .metric-note').text()).toContain('$2.0000')
    expect(wrapper.find('.today-token-card .metric-value').text()).toBe('100')
    expect(wrapper.find('.total-card .metric-value').text()).toBe('1.0K')
    expect(wrapper.find('.response-card strong').text()).toBe('1.25s')
    expect(wrapper.text()).not.toContain('dashboard.quickActions')
    expect(wrapper.text()).not.toContain('dashboard.recentUsage')
    expect(api.trend).toHaveBeenCalledTimes(2)
    expect(api.models).toHaveBeenCalledTimes(1)
  })

  it('does not let an earlier range response overwrite the latest selection', async () => {
    const oldTrend = deferred<{ trend: unknown[] }>()
    const oldModels = deferred<{ models: unknown[] }>()
    api.trend.mockReturnValueOnce(oldTrend.promise).mockResolvedValueOnce({ trend: [] }).mockResolvedValueOnce({ trend: [{ date: 'new', requests: 10 }] })
    api.models.mockReturnValueOnce(oldModels.promise).mockResolvedValueOnce({ models: [{ model: 'new' }] })
    const wrapper = mount(DashboardStudioView, { global: { stubs } })
    wrapper.findComponent({ name: 'Select' }).vm.$emit('update:modelValue', 'hour')
    await flushPromises()
    expect(api.trend.mock.calls[2][0].granularity).toBe('hour')
    oldTrend.resolve({ trend: [{ date: 'old', requests: 5 }] })
    oldModels.resolve({ models: [{ model: 'old' }] })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'StudioTokenTrend' }).props('trendData')).toEqual([{ date: 'new', requests: 10 }])
    expect(wrapper.findComponent({ name: 'ModelPieScene' }).props('models')).toEqual([{ model: 'new' }])
    expect(wrapper.find('.request-card .metric-value').text()).toBe('7')
    expect(wrapper.findAll('.request-card .request-bar')).toHaveLength(7)
    expect(wrapper.find('.request-card .chart-caption').text()).toContain('dashboard.studio.lastSevenDays')
  })

  it('passes dates from the new filter component to both native chart endpoints', async () => {
    const wrapper = mount(DashboardStudioView, { global: { stubs } })
    await flushPromises()
    const picker = wrapper.findComponent({ name: 'DateRangePicker' })
    picker.vm.$emit('update:startDate', '2026-10-01')
    picker.vm.$emit('update:endDate', '2026-10-31')
    picker.vm.$emit('change')
    await flushPromises()
    expect(api.trend).toHaveBeenLastCalledWith({ start_date: '2026-10-01', end_date: '2026-10-31', granularity: 'day' })
    expect(api.models).toHaveBeenLastCalledWith({ start_date: '2026-10-01', end_date: '2026-10-31' })
    expect(wrapper.find('.request-card .metric-value').text()).toBe('7')
  })

  it('shows a chart error independently and recovers on retry', async () => {
    api.models.mockRejectedValueOnce(new Error('offline'))
    api.trend.mockResolvedValue({ trend: [{ date: 'ok', requests: 8 }] })
    const wrapper = mount(DashboardStudioView, { global: { stubs } })
    await flushPromises()
    expect(wrapper.find('.model-card [role="alert"]').exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'StudioTokenTrend' }).props('trendData')).toEqual([{ date: 'ok', requests: 8 }])
    await wrapper.find('.model-card button').trigger('click')
    await flushPromises()
    expect(wrapper.find('.model-card [role="alert"]').exists()).toBe(false)
  })

  it('keeps charts available when overview fails and allows retry', async () => {
    api.stats.mockRejectedValueOnce(new Error('offline'))
    const wrapper = mount(DashboardStudioView, { global: { stubs } })
    await flushPromises()
    expect(wrapper.find('.studio-error').exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'ModelPieScene' }).exists()).toBe(true)
    await wrapper.find('.studio-error button').trigger('click')
    await flushPromises()
    expect(wrapper.find('.studio-error').exists()).toBe(false)
    expect(wrapper.find('.summary-grid').exists()).toBe(true)
  })
})
