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

import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import StudioStatusFaces from '../StudioStatusFaces.vue'
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

const hourCoverage: MonitorCoverage = {
  requested_start: '2026-09-10T00:00:00.000Z',
  requested_end: '2026-09-11T00:00:00.000Z',
  coverage_start: '2026-09-10T00:00:00.000Z',
  data_through: '2026-09-11T00:00:00.000Z',
  computed_at: '2026-09-11T00:00:00.000Z',
  aggregation_lag_seconds: 0,
  coverage_complete: true,
  bucket_seconds: 3600,
}

describe('StudioStatusFaces', () => {
  it('renders the native empty grid when a range has no traffic buckets', async () => {
    const wrapper = mount(StudioStatusFaces, {
      props: {
        coverage: hourCoverage,
        buckets: [],
      },
    })
    await flushPromises()
    expect(wrapper.findAll('button.studio-face')).toHaveLength(24)
    expect(wrapper.findAll('.studio-face--empty')).toHaveLength(24)
    expect(wrapper.find('.studio-faces').attributes('style') || '').toContain('--studio-face-cols: 24')
  })

  it('lays 18 slots into one row of cells instead of face SVGs', async () => {
    const wrapper = mount(StudioStatusFaces, {
      props: {
        coverage: {
          ...coverage,
          requested_start: '2026-09-10T09:00:00.000Z',
          requested_end: '2026-09-10T10:30:00.000Z',
          coverage_start: '2026-09-10T09:00:00.000Z',
          data_through: '2026-09-10T10:30:00.000Z',
        },
        buckets: [
          {
            bucket_start: '2026-09-10T10:25:00.000Z',
            metrics: metric(),
            health: health({ score: 95 }),
          },
        ],
      },
    })
    await flushPromises()
    expect(wrapper.findAll('button.studio-face')).toHaveLength(18)
    expect(wrapper.find('.studio-faces').attributes('style') || '').toContain('--studio-face-cols: 18')
    expect(wrapper.text()).toContain('channelMonitorV2.studio.faces.recentCount')
    expect(wrapper.text()).toContain('channelMonitorV2.bucket.minutes')
    expect(wrapper.find('.studio-faces-axis').text()).toContain('channelMonitorV2.studio.faces.past')
    expect(wrapper.find('.studio-faces-axis').text()).toContain('channelMonitorV2.studio.faces.now')
    expect(wrapper.find('svg.studio-face-svg').exists()).toBe(false)
    expect(wrapper.findAll('.studio-face-cell')).toHaveLength(18)
    expect(wrapper.find('.studio-faces-scroller').exists()).toBe(false)
    expect(wrapper.find('.studio-faces-nav').exists()).toBe(false)
  })

  it('renders colored cells for time buckets and shows pulse-cell tooltip on hover', async () => {
    const wrapper = mount(StudioStatusFaces, {
      props: {
        coverage,
        showThroughput: true,
        buckets: [
          {
            bucket_start: '2026-09-10T10:00:00.000Z',
            metrics: metric(),
            health: health({ score: 95 }),
          },
        ],
      },
      attachTo: document.body,
    })

    expect(wrapper.find('.card').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('channelMonitorV2.studio.faces.title')
    expect(wrapper.find('svg.studio-face-svg').exists()).toBe(false)
    expect(wrapper.find('.studio-face-cell').exists()).toBe(true)
    expect(wrapper.find('.studio-face--current').exists()).toBe(true)
    expect(wrapper.text()).not.toMatch(/[\u{1F600}-\u{1F64F}]/u)
    const current = wrapper.find('button.studio-face--current')
    expect(current.attributes('style') || '').toMatch(/--studio-cell:\s*#/i)

    await wrapper.find('button.studio-face').trigger('mouseenter')
    expect(wrapper.find('button.studio-face').classes()).toContain('studio-face--hot')
    const tooltip = document.querySelector('.studio-face-tooltip') as HTMLElement | null
    expect(tooltip?.getAttribute('style') || '').not.toContain('display: none')
    expect(tooltip?.textContent).toContain('channelMonitorV2.matrix.scoreLine')
    expect(tooltip?.textContent).toContain('channelMonitorV2.metrics.successRateValue')
    expect(tooltip?.textContent).toContain('channelMonitorV2.metrics.ttftValue')
    expect(tooltip?.textContent).toContain('channelMonitorV2.metrics.cacheRateValue')
    wrapper.unmount()
  })

  it('colors cells from V2 thresholds when API overall is unknown', () => {
    const wrapper = mount(StudioStatusFaces, {
      props: {
        coverage,
        thresholds: { warning_error_rate: 0.05, critical_error_rate: 0.2, warning_cache_rate: 0.85, critical_cache_rate: 0.6 },
        buckets: [
          {
            bucket_start: '2026-09-10T10:00:00.000Z',
            metrics: metric({
              success_requests: 0,
              error_requests: 0,
              request_count: 0,
              rpm: 0,
              tpm: 0,
              error_rate: 0,
              success_rate: 0,
              cache_rate: 0,
              cache_rate_numerator: 0,
              cache_rate_denominator: 0,
              ttft: { sample_count: 0, p50_ms: null, p90_ms: null, p95_ms: null, avg_ms: null },
              duration: { sample_count: 0, p50_ms: null, p90_ms: null, p95_ms: null, avg_ms: null },
            }),
            health: health({ overall: 'unknown', error_rate: 'unknown', ttft: 'unknown', cache: 'unknown', score: null }),
          },
        ],
      },
    })
    expect(wrapper.find('button.studio-face').classes()).toContain('studio-face--unknown')
  })

  it('does not keep a green cell when true error rate is 100%', () => {
    const wrapper = mount(StudioStatusFaces, {
      props: {
        coverage,
        buckets: [
          {
            bucket_start: '2026-09-10T10:00:00.000Z',
            metrics: metric({ error_rate: 0, success_rate: 0, success_requests: 0, error_requests: 20, request_count: 20 }),
            health: health({ overall: 'healthy', error_rate: 'healthy', score: 92 }),
          },
        ],
      },
    })
    const face = wrapper.find('button.studio-face')
    expect(face.classes()).toContain('studio-face--critical')
    expect(face.classes()).not.toContain('studio-face--healthy')
  })

  it('pops in only newly arrived time slots', async () => {
    const wrapper = mount(StudioStatusFaces, {
      props: {
        coverage,
        buckets: [
          { bucket_start: '2026-09-10T10:00:00.000Z', metrics: metric(), health: health() },
          { bucket_start: '2026-09-10T10:05:00.000Z', metrics: metric(), health: health({ score: 40 }) },
        ],
      },
    })
    await flushPromises()
    expect(wrapper.find('.studio-face--enter').exists()).toBe(false)

    await wrapper.setProps({
      coverage: {
        ...coverage,
        requested_end: '2026-09-10T10:15:00.000Z',
        data_through: '2026-09-10T10:15:00.000Z',
      },
      buckets: [
        { bucket_start: '2026-09-10T10:00:00.000Z', metrics: metric(), health: health() },
        { bucket_start: '2026-09-10T10:05:00.000Z', metrics: metric(), health: health({ score: 40 }) },
        { bucket_start: '2026-09-10T10:10:00.000Z', metrics: metric(), health: health({ score: 88 }) },
      ],
    })
    await flushPromises()
    const buttons = wrapper.findAll('button.studio-face')
    expect(buttons).toHaveLength(3)
    expect(buttons[0].classes()).not.toContain('studio-face--enter')
    expect(buttons[1].classes()).not.toContain('studio-face--enter')
    expect(buttons[2].classes()).toContain('studio-face--enter')
    expect(wrapper.find('.studio-faces').attributes('style') || '').toContain('--studio-face-cols: 3')
    expect(wrapper.find('.studio-faces-root').exists()).toBe(true)
    expect(wrapper.find('.studio-faces-wrap').exists()).toBe(true)
  })

  it('reuses one tooltip panel and only updates its copy when moving between cells', async () => {
    const wrapper = mount(StudioStatusFaces, {
      props: {
        coverage,
        buckets: [
          { bucket_start: '2026-09-10T10:00:00.000Z', metrics: metric(), health: health() },
          { bucket_start: '2026-09-10T10:05:00.000Z', metrics: metric({ error_rate: 0.4 }), health: health({ score: 40, overall: 'critical' }) },
        ],
      },
      attachTo: document.body,
    })
    await flushPromises()
    const buttons = wrapper.findAll('button.studio-face')
    await buttons[0].trigger('mouseenter')
    await flushPromises()
    const tooltip = [...document.querySelectorAll('.studio-face-tooltip')].find((el) =>
      el.textContent?.includes('channelMonitorV2.matrix.scoreLine'),
    )
    expect(tooltip).toBeTruthy()
    const firstText = tooltip?.textContent || ''
    expect(firstText).toContain('channelMonitorV2.matrix.scoreLine')
    await buttons[1].trigger('mouseenter')
    await flushPromises()
    expect(document.body.contains(tooltip as Node)).toBe(true)
    expect(tooltip?.textContent).not.toBe(firstText)
    wrapper.unmount()
  })
})
