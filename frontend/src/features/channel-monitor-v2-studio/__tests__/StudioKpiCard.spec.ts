vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key,
      locale: { value: 'zh' },
    }),
  }
})

import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import StudioBrandIcon from '../StudioBrandIcon.vue'
import StudioKpiCard from '../StudioKpiCard.vue'
import StudioSparkChart from '../StudioSparkChart.vue'

const base = {
  label: '默认组',
  platform: 'openai' as const,
  brandLabel: 'OpenAI',
  successLabel: '成功率',
  successRate: '98.2%',
  ttftLabel: '首 TOKEN',
  ttft: '400ms',
  cacheLabel: '缓存率',
  cacheRate: '12.3%',
  statusHeading: '当前状态',
  statusLabel: '正常',
  state: 'healthy' as const,
  successState: 'healthy' as const,
  ttftState: 'healthy' as const,
  cacheState: 'critical' as const,
  accent: 'teal' as const,
}

describe('StudioKpiCard', () => {
  it('renders group name, brand tag, logo, and metrics with status top-right', () => {
    const wrapper = mount(StudioKpiCard, {
      props: { ...base },
    })

    expect(wrapper.classes().join(' ') + wrapper.html()).toContain('stat-card')
    expect(wrapper.find('.studio-kpi-name').text()).toBe('默认组')
    expect(wrapper.text()).toContain('默认组')
    expect(wrapper.find('.studio-kpi-brand').text()).toBe('OpenAI')
    expect(wrapper.find('.studio-kpi-name').classes().join(' ')).toContain('studio-kpi-name')
    expect(wrapper.find('.studio-kpi-brand').attributes('style')).toContain('--studio-brand-color')
    expect(wrapper.find('.studio-kpi-rate').exists()).toBe(false)
    expect(wrapper.findComponent(StudioBrandIcon).exists()).toBe(true)
    expect(wrapper.text()).toContain('成功率')
    expect(wrapper.text()).toContain('98.2%')
    expect(wrapper.text()).toContain('首 TOKEN')
    expect(wrapper.text()).toContain('400ms')
    expect(wrapper.text()).toContain('缓存率')
    expect(wrapper.text()).toContain('12.3%')
    expect(wrapper.find('.studio-kpi-status').text()).toBe('正常')
    expect(wrapper.find('.studio-kpi-status').attributes('aria-label')).toContain('当前状态')
    expect(wrapper.find('.studio-kpi-status').classes().join(' ')).toMatch(/ml-auto/)
    expect(wrapper.find('.studio-kpi-status').classes().join(' ')).toMatch(/text-right/)
    expect(wrapper.find('.studio-kpi-metrics').classes().join(' ')).toMatch(/grid/)
    expect(wrapper.find('.stat-value').classes().join(' ')).toMatch(/emerald/)
    expect(wrapper.findAll('.studio-kpi-metric-label')).toHaveLength(3)
  })

  it('shows eight Chinese-character widths and treats English letters as half', () => {
    const wrapper = mount(StudioKpiCard, {
      props: { ...base, label: 'GPT线路测试分组名称超长' },
    })
    expect(wrapper.find('.studio-kpi-name').text()).toBe('GPT线路测试分组…')
    expect(wrapper.find('.studio-kpi-name').attributes('title')).toBe('GPT线路测试分组名称超长')
  })

  it('shows the group multiplier beside the brand name', () => {
    const wrapper = mount(StudioKpiCard, {
      props: { ...base, brandLabel: 'Anthropic', rateLabel: '用户倍率0.08x' },
    })
    expect(wrapper.find('.studio-kpi-rate').text()).toBe('用户倍率0.08x')
    expect(wrapper.find('.studio-kpi-brand').text()).toBe('Anthropic')
    expect(wrapper.find('.studio-kpi-brand-row').text()).toContain('用户倍率0.08x')
    expect(wrapper.find('.studio-kpi-brand-row').classes().join(' ')).toMatch(/flex/)
    expect(wrapper.find('.studio-kpi-name-row').find('.studio-kpi-status').exists()).toBe(true)
    expect(wrapper.classes().join(' ')).not.toContain('studio-kpi--active')
  })

  it('paints card wash from status accent', () => {
    expect(mount(StudioKpiCard, { props: { ...base, accent: 'teal' } }).classes().join(' ')).toContain('studio-kpi--teal')
    expect(mount(StudioKpiCard, { props: { ...base, accent: 'amber', state: 'warning', statusLabel: '降级' } }).classes().join(' ')).toContain('studio-kpi--amber')
    expect(mount(StudioKpiCard, { props: { ...base, accent: 'coral', state: 'critical', statusLabel: '失败' } }).classes().join(' ')).toContain('studio-kpi--coral')
    expect(mount(StudioKpiCard, { props: { ...base, accent: 'slate', state: 'unknown', statusLabel: '样本不足' } }).classes().join(' ')).toContain('studio-kpi--slate')
  })

  it('does not paint missing first-token dash as critical red', () => {
    const wrapper = mount(StudioKpiCard, {
      props: {
        ...base,
        successRate: '-',
        ttft: '-',
        ttftState: 'critical',
        state: 'critical',
        statusLabel: '失败',
        accent: 'coral',
      },
    })
    const values = wrapper.findAll('dd')
    const ttft = values[1]
    expect(ttft.classes().join(' ')).not.toMatch(/rose|red/)
    expect(ttft.classes().join(' ')).toMatch(/gray|dark/)
  })

  it('maps warning and critical metric states to distinct colors', () => {
    const warning = mount(StudioKpiCard, {
      props: {
        ...base,
        successRate: '90%',
        successState: 'warning',
        state: 'warning',
        statusLabel: '降级',
        accent: 'coral',
      },
    })
    expect(warning.find('.stat-value').classes().join(' ')).toMatch(/amber/)
    expect(warning.find('.studio-kpi-status').text()).toBe('降级')

    const critical = mount(StudioKpiCard, {
      props: {
        ...base,
        successRate: '50%',
        successState: 'critical',
        state: 'critical',
        statusLabel: '失败',
        accent: 'coral',
      },
    })
    expect(critical.find('.stat-value').classes().join(' ')).toMatch(/red/)
    expect(critical.find('.studio-kpi-status').text()).toBe('失败')
  })

  it('renders a bucket-synced chart with a dot per time slot', () => {
    const wrapper = mount(StudioKpiCard, {
      props: {
        ...base,
        buckets: [
          {
            bucket_start: '2026-09-10T10:00:00.000Z',
            metrics: {
              success_requests: 98,
              error_requests: 2,
              request_count: 100,
              token_count: 1000,
              rpm: 1,
              tpm: 10,
              error_rate: 0.02,
              cache_rate: 0.4,
              cache_rate_numerator: 40,
              cache_rate_denominator: 100,
              ttft: { sample_count: 100, p50_ms: 200, p90_ms: 300, p95_ms: 400, avg_ms: 250 },
              duration: { sample_count: 100, p50_ms: 500, p90_ms: 700, p95_ms: 900, avg_ms: 600 },
            },
            health: {
              overall: 'healthy',
              error_rate: 'healthy',
              ttft: 'healthy',
              score: 92,
              error_rate_score: 92,
              ttft_score: 90,
              cache_score: 80,
              minimum_sample: 20,
            },
          },
        ],
        coverage: {
          requested_start: '2026-09-10T10:00:00.000Z',
          requested_end: '2026-09-10T10:10:00.000Z',
          coverage_start: '2026-09-10T10:00:00.000Z',
          data_through: '2026-09-10T10:10:00.000Z',
          computed_at: '2026-09-10T10:10:00.000Z',
          aggregation_lag_seconds: 0,
          coverage_complete: true,
          bucket_seconds: 300,
        },
      },
    })
    expect(wrapper.findComponent(StudioSparkChart).exists()).toBe(true)
    expect(wrapper.findComponent(StudioSparkChart).props('accent')).toBe('teal')
    expect(wrapper.findAll('.studio-spark-dot')).toHaveLength(2)
  })
})
