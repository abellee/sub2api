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

function waitPaint() {
  return new Promise<void>((resolve) => {
    requestAnimationFrame(() => requestAnimationFrame(() => resolve()))
  })
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
  })

  it('renders SVG faces for time buckets and shows pulse-cell tooltip on hover', async () => {
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
    expect(wrapper.find('.studio-faces-scroller').classes()).toContain('overflow-x-auto')
    expect(wrapper.find('svg.studio-face-svg').exists()).toBe(true)
    expect(wrapper.find('.studio-face--current').exists()).toBe(true)
    expect(wrapper.text()).not.toMatch(/[\u{1F600}-\u{1F64F}]/u)

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

  it('maps Shift+vertical wheel to scrollLeft and leaves trackpad deltaX unprevented', async () => {
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

    const el = wrapper.find('.studio-faces-scroller').element as HTMLElement
    Object.defineProperty(el, 'scrollWidth', { configurable: true, value: 800 })
    Object.defineProperty(el, 'clientWidth', { configurable: true, value: 200 })
    el.scrollLeft = 0

    const shiftWheel = new WheelEvent('wheel', {
      deltaX: 0,
      deltaY: 40,
      shiftKey: true,
      cancelable: true,
      bubbles: true,
    })
    el.dispatchEvent(shiftWheel)
    expect(shiftWheel.defaultPrevented).toBe(true)
    expect(el.scrollLeft).toBe(40)

    const trackpad = new WheelEvent('wheel', {
      deltaX: 30,
      deltaY: 1,
      shiftKey: false,
      cancelable: true,
      bubbles: true,
    })
    el.dispatchEvent(trackpad)
    expect(trackpad.defaultPrevented).toBe(false)
    expect(el.scrollLeft).toBe(40)
  })

  it('scrolls to the last face when buckets change', async () => {
    const wrapper = mount(StudioStatusFaces, {
      props: {
        coverage,
        buckets: [
          { bucket_start: '2026-09-10T10:00:00.000Z', metrics: metric(), health: health() },
        ],
      },
    })
    await flushPromises()
    const el = wrapper.find('.studio-faces-scroller').element as HTMLElement
    Object.defineProperty(el, 'scrollWidth', { configurable: true, value: 800 })
    Object.defineProperty(el, 'clientWidth', { configurable: true, value: 200 })
    el.scrollLeft = 600

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
    expect(el.scrollLeft).toBe(800)
  })

  it('auto-scrolls new faces to the end when the user has not panned the row', async () => {
    const wrapper = mount(StudioStatusFaces, {
      props: {
        coverage,
        buckets: [
          { bucket_start: '2026-09-10T10:00:00.000Z', metrics: metric(), health: health() },
        ],
      },
    })
    await flushPromises()
    const el = wrapper.find('.studio-faces-scroller').element as HTMLElement
    Object.defineProperty(el, 'scrollWidth', { configurable: true, value: 800 })
    Object.defineProperty(el, 'clientWidth', { configurable: true, value: 200 })
    el.scrollLeft = 0

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
    expect(el.scrollLeft).toBe(800)
  })

  it('does not steal scroll when the user has panned away from the end', async () => {
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
    const scroller = wrapper.find('.studio-faces-scroller')
    const el = scroller.element as HTMLElement
    Object.defineProperty(el, 'scrollWidth', { configurable: true, value: 800 })
    Object.defineProperty(el, 'clientWidth', { configurable: true, value: 200 })
    el.scrollLeft = 120
    await scroller.trigger('scroll')

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
    expect(el.scrollLeft).toBe(120)
  })

  it('keeps the scroller pinned to the end when the window shrinks', async () => {
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
    const el = wrapper.find('.studio-faces-scroller').element as HTMLElement
    Object.defineProperty(el, 'scrollWidth', { configurable: true, value: 800 })
    Object.defineProperty(el, 'clientWidth', { configurable: true, value: 200 })
    el.scrollLeft = 600
    Object.defineProperty(el, 'clientWidth', { configurable: true, value: 120 })
    window.dispatchEvent(new Event('resize'))
    await flushPromises()
    expect(el.scrollLeft).toBe(800)
  })

  it('keeps historical faces smaller than the current time slot', async () => {
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
    const buttons = wrapper.findAll('button.studio-face')
    expect(buttons).toHaveLength(2)
    expect(buttons[0].classes()).toContain('studio-face--past')
    expect(buttons[1].classes()).toContain('studio-face--current')
  })

  it('colors faces from V2 thresholds when API overall is unknown', () => {
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

  it('does not keep a green smile when true error rate is 100%', () => {
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
    expect(buttons[0].classes()).not.toContain('studio-face--pending')
    expect(buttons[1].classes()).not.toContain('studio-face--enter')
    expect(buttons[1].classes()).not.toContain('studio-face--pending')
    expect(buttons[1].classes()).toContain('studio-face--current')
    expect(buttons[2].classes()).toContain('studio-face--pending')
    expect(buttons[2].classes()).toContain('studio-face--collapse')
    expect(
      buttons[2].classes().includes('studio-face--pending') || buttons[2].classes().includes('studio-face--enter'),
    ).toBe(true)
    expect(wrapper.find('.studio-faces').classes()).toContain('items-center')
    expect(wrapper.find('.studio-faces-axis').exists()).toBe(true)
    expect(wrapper.find('.studio-faces-canvas').exists()).toBe(true)
    expect(wrapper.find('.studio-faces-scroller').classes()).toContain('overflow-x-auto')
    expect(wrapper.find('.studio-faces-fade').exists()).toBe(true)
    expect(wrapper.find('.studio-faces-nav--prev').exists()).toBe(true)
    expect(wrapper.find('.studio-faces-nav--next').exists()).toBe(true)
    expect(wrapper.find('.studio-faces-root').exists()).toBe(true)
    expect(wrapper.find('.studio-faces-wrap').exists()).toBe(true)
  })

  it('enables the next arrow after scrolling away from the end', async () => {
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
    const el = wrapper.find('.studio-faces-scroller').element as HTMLElement
    Object.defineProperty(el, 'scrollWidth', { configurable: true, value: 800 })
    Object.defineProperty(el, 'clientWidth', { configurable: true, value: 200 })
    el.scrollLeft = 800
    el.dispatchEvent(new Event('scroll'))
    await flushPromises()
    expect(wrapper.find('.studio-faces-nav--next').attributes('disabled')).toBeDefined()

    el.scrollLeft = 120
    el.dispatchEvent(new Event('scroll'))
    await flushPromises()
    expect(wrapper.find('.studio-faces-nav--next').attributes('disabled')).toBeUndefined()
    expect(wrapper.find('.studio-faces-nav--prev').attributes('disabled')).toBeUndefined()
  })

  it('keeps every face visible after the time window is replaced', async () => {
    const OriginalIO = globalThis.IntersectionObserver
    globalThis.IntersectionObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    } as typeof IntersectionObserver

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
    await wrapper.setProps({
      coverage: {
        ...coverage,
        requested_start: '2026-09-09T21:00:00.000Z',
        requested_end: '2026-09-10T00:00:00.000Z',
        coverage_start: '2026-09-09T21:00:00.000Z',
        data_through: '2026-09-10T00:00:00.000Z',
        bucket_seconds: 3600,
      },
      buckets: [
        { bucket_start: '2026-09-09T21:00:00.000Z', metrics: metric(), health: health({ score: 88 }) },
        { bucket_start: '2026-09-09T22:00:00.000Z', metrics: metric(), health: health({ score: 55 }) },
        { bucket_start: '2026-09-09T23:00:00.000Z', metrics: metric(), health: health({ score: 12 }) },
      ],
    })
    await flushPromises()
    await waitPaint()
    const scroller = wrapper.find('.studio-faces-scroller').element as HTMLElement
    scroller.dispatchEvent(new Event('scroll'))
    await flushPromises()
    const buttons = wrapper.findAll('button.studio-face')
    expect(buttons).toHaveLength(3)
    expect(wrapper.find('.studio-face--collapse').exists()).toBe(false)
    expect(wrapper.findAll('svg.studio-face-svg')).toHaveLength(buttons.length)
    wrapper.unmount()
    globalThis.IntersectionObserver = OriginalIO
  })

  it('keeps the native 24h faces after switching from 90m even when scroll tries to cull', async () => {
    const OriginalIO = globalThis.IntersectionObserver
    globalThis.IntersectionObserver = class {
      constructor(private cb: IntersectionObserverCallback) {}
      observe(node: Element) {
        this.cb(
          [{
            target: node,
            isIntersecting: false,
            intersectionRatio: 0,
            boundingClientRect: node.getBoundingClientRect(),
            intersectionRect: node.getBoundingClientRect(),
            rootBounds: null,
            time: 0,
          } as IntersectionObserverEntry],
          this as unknown as IntersectionObserver,
        )
      }
      unobserve() {}
      disconnect() {}
    } as typeof IntersectionObserver

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
    const scroller = wrapper.find('.studio-faces-scroller').element as HTMLElement
    Object.defineProperty(scroller, 'scrollWidth', { configurable: true, value: 800 })
    Object.defineProperty(scroller, 'clientWidth', { configurable: true, value: 200 })
    scroller.scrollLeft = 800
    scroller.dispatchEvent(new Event('scroll'))
    await wrapper.setProps({
      coverage: hourCoverage,
      buckets: [
        { bucket_start: '2026-09-10T00:00:00.000Z', metrics: metric(), health: health({ score: 88 }) },
        { bucket_start: '2026-09-10T10:00:00.000Z', metrics: metric(), health: health({ score: 55 }) },
        { bucket_start: '2026-09-10T23:00:00.000Z', metrics: metric(), health: health({ score: 12 }) },
      ],
    })
    await flushPromises()
    await waitPaint()
    scroller.dispatchEvent(new Event('scroll'))
    await flushPromises()
    const buttons = wrapper.findAll('button.studio-face')
    expect(buttons).toHaveLength(24)
    expect(wrapper.find('.studio-face--collapse').exists()).toBe(false)
    expect(wrapper.findAll('svg.studio-face-svg')).toHaveLength(24)
    wrapper.unmount()
    globalThis.IntersectionObserver = OriginalIO
  })

  it('does not cull replaced faces when intersection data is still incomplete', async () => {
    const OriginalIO = globalThis.IntersectionObserver
    globalThis.IntersectionObserver = class {
      constructor(private cb: IntersectionObserverCallback) {}
      observe(node: Element) {
        this.cb(
          [{
            target: node,
            isIntersecting: (node as HTMLElement).dataset.faceKey?.endsWith('00:00.000Z') === true,
            intersectionRatio: 1,
            boundingClientRect: node.getBoundingClientRect(),
            intersectionRect: node.getBoundingClientRect(),
            rootBounds: null,
            time: 0,
          } as IntersectionObserverEntry],
          this as unknown as IntersectionObserver,
        )
      }
      unobserve() {}
      disconnect() {}
    } as typeof IntersectionObserver

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
    await wrapper.setProps({
      coverage: {
        ...coverage,
        requested_start: '2026-09-09T21:00:00.000Z',
        requested_end: '2026-09-10T00:00:00.000Z',
        coverage_start: '2026-09-09T21:00:00.000Z',
        data_through: '2026-09-10T00:00:00.000Z',
        bucket_seconds: 3600,
      },
      buckets: [
        { bucket_start: '2026-09-09T21:00:00.000Z', metrics: metric(), health: health({ score: 88 }) },
        { bucket_start: '2026-09-09T22:00:00.000Z', metrics: metric(), health: health({ score: 55 }) },
        { bucket_start: '2026-09-09T23:00:00.000Z', metrics: metric(), health: health({ score: 12 }) },
      ],
    })
    await flushPromises()
    await waitPaint()
    const buttons = wrapper.findAll('button.studio-face')
    expect(buttons).toHaveLength(3)
    expect(wrapper.findAll('svg.studio-face-svg')).toHaveLength(buttons.length)
    wrapper.unmount()
    globalThis.IntersectionObserver = OriginalIO
  })

  it('reuses one tooltip panel and only updates its copy when moving between faces', async () => {
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
