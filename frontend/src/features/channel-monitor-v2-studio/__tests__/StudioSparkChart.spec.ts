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
import StudioSparkChart from '../StudioSparkChart.vue'
import type { MonitorCoverage, MonitorHealth, MonitorMetric } from '@/api/channelMonitorV2'

function metric(partial: Partial<MonitorMetric> = {}): MonitorMetric {
  return {
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
    ...partial,
  }
}

function health(partial: Partial<MonitorHealth> = {}): MonitorHealth {
  return {
    overall: 'healthy',
    error_rate: 'healthy',
    ttft: 'healthy',
    score: 92,
    error_rate_score: 92,
    ttft_score: 90,
    cache_score: 80,
    minimum_sample: 20,
    ...partial,
  }
}

const coverage: MonitorCoverage = {
  requested_start: '2026-09-10T10:00:00.000Z',
  requested_end: '2026-09-10T10:10:00.000Z',
  coverage_start: '2026-09-10T10:00:00.000Z',
  data_through: '2026-09-10T10:10:00.000Z',
  computed_at: '2026-09-10T10:10:00.000Z',
  aggregation_lag_seconds: 0,
  coverage_complete: true,
  bucket_seconds: 300,
}

describe('StudioSparkChart', () => {
  it('renders a dot per aligned time slot and shows the period tooltip on hover', async () => {
    const wrapper = mount(StudioSparkChart, {
      props: {
        coverage,
        label: '默认组',
        buckets: [
          {
            bucket_start: '2026-09-10T10:00:00.000Z',
            metrics: metric(),
            health: health(),
          },
        ],
      },
      attachTo: document.body,
    })

    const hits = wrapper.findAll('.studio-spark-hit')
    expect(hits).toHaveLength(2)
    expect(wrapper.find('.studio-spark').exists()).toBe(true)
    expect(wrapper.find('.studio-spark-line').exists()).toBe(true)
    expect(wrapper.findAll('.studio-spark-dot')).toHaveLength(2)
    expect(wrapper.find('.studio-spark-dot--current').exists()).toBe(true)
    expect(wrapper.find('.studio-spark-ping').exists()).toBe(true)
    expect(wrapper.find('.studio-spark-hit--current').exists()).toBe(true)

    await hits[0].trigger('pointerenter')
    const tooltip = document.querySelector('.studio-spark-tooltip')
    expect(tooltip?.textContent).toContain('channelMonitorV2.matrix.scoreLine')
    expect(tooltip?.textContent).toContain('channelMonitorV2.metrics.successRateValue')
    wrapper.unmount()
  })

  it('plots health score so the line moves when error_rate is flat', () => {
    const wrapper = mount(StudioSparkChart, {
      props: {
        coverage,
        showThroughput: true,
        buckets: [
          {
            bucket_start: '2026-09-10T10:00:00.000Z',
            metrics: metric({ error_rate: 0, rpm: 8, success_rate: 1 }),
            health: health({ score: 62 }),
          },
          {
            bucket_start: '2026-09-10T10:05:00.000Z',
            metrics: metric({ error_rate: 0, rpm: 40, success_rate: 1 }),
            health: health({ score: 96 }),
          },
        ],
      },
    })
    const dots = wrapper.findAll('.studio-spark-dot')
    expect(dots).toHaveLength(2)
    const hits = wrapper.findAll('.studio-spark-hit')
    const y0 = Number.parseFloat(hits[0].attributes('style')?.match(/top:\s*([\d.]+)px/)?.[1] || '0')
    const y1 = Number.parseFloat(hits[1].attributes('style')?.match(/top:\s*([\d.]+)px/)?.[1] || '0')
    expect(y0).toBeGreaterThan(y1)
    wrapper.unmount()
  })

  it('keeps a unique fill per chart so status colors do not leak across cards', () => {
    const teal = mount(StudioSparkChart, {
      props: { coverage, accent: 'teal', buckets: [{ bucket_start: '2026-09-10T10:00:00.000Z', metrics: metric(), health: health() }] },
    })
    const coral = mount(StudioSparkChart, {
      props: { coverage, accent: 'coral', buckets: [{ bucket_start: '2026-09-10T10:05:00.000Z', metrics: metric(), health: health({ overall: 'critical', score: 20 }) }] },
    })
    const tealFill = teal.find('.studio-spark-fill-top')
    const coralFill = coral.find('.studio-spark-fill-top')
    expect(teal.find('linearGradient').attributes('id')).not.toBe(coral.find('linearGradient').attributes('id'))
    expect(tealFill.attributes('stop-color')).toBe('#14b8a6')
    expect(coralFill.attributes('stop-color')).toBe('#e11d48')
    expect(teal.find('.studio-spark').attributes('style')).toContain('#14b8a6')
    expect(coral.find('.studio-spark').attributes('style')).toContain('#e11d48')
    teal.unmount()
    coral.unmount()
  })
})
