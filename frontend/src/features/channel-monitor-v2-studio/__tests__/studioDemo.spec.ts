import { describe, expect, it } from 'vitest'
import type { MonitorCoverage, MonitorHealth, MonitorMetric } from '@/api/channelMonitorV2'
import { coverageBucketStarts, STUDIO_MAX_SLOTS } from '../studioBuckets'
import { statusFace } from '../studioFormat'
import { cacheTone, errorRateTone, ttftTone } from '../studioFormat'
import {
  EMPTY_STUDIO_BUCKETS,
  STUDIO_CHART_MIN_GAP,
  STUDIO_FACE_MOVE_MS,
  STUDIO_FACE_POP_MS,
  STUDIO_FACE_STAGGER_MS,
  STUDIO_TEST_SLOT_INTERVAL_MS,
  extendCoverage,
  studioAfterPaint,
  rowHasActivity,
  studioIncomingKeys,
  studioMissedSlots,
  studioPageHidden,
  studioPopIncomingKeys,
  studioShowcaseRows,
  withDemoBuckets,
} from '../studioDemo'

function metric(partial: Partial<MonitorMetric> = {}): MonitorMetric {
  return {
    success_requests: 10,
    error_requests: 0,
    request_count: 10,
    token_count: 100,
    rpm: 1,
    tpm: 10,
    error_rate: 0,
    cache_rate: 0.2,
    cache_rate_numerator: 2,
    cache_rate_denominator: 10,
    ttft: { sample_count: 10, p50_ms: 120, p90_ms: 180, p95_ms: 200, avg_ms: 140 },
    duration: { sample_count: 10, p50_ms: 300, p90_ms: 400, p95_ms: 450, avg_ms: 320 },
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
    minimum_sample: 1,
    ...partial,
  }
}

const coverage: MonitorCoverage = {
  requested_start: '2026-09-10T10:00:00.000Z',
  requested_end: '2026-09-10T10:35:00.000Z',
  coverage_start: '2026-09-10T10:00:00.000Z',
  data_through: '2026-09-10T10:35:00.000Z',
  computed_at: '2026-09-10T10:35:00.000Z',
  aggregation_lag_seconds: 0,
  coverage_complete: true,
  bucket_seconds: 300,
}

describe('studio demo buckets', () => {
  it('keeps real API buckets and replaces empty no-score slots', () => {
    const realStart = '2026-09-10T10:00:00.000Z'
    const emptyStart = '2026-09-10T10:05:00.000Z'
    const filled = withDemoBuckets(
      'openai:gpt-4',
      [
        { bucket_start: realStart, metrics: metric(), health: health() },
        {
          bucket_start: emptyStart,
          metrics: metric({ request_count: 0, rpm: 0, tpm: 0, error_rate: 0 }),
          health: health({ overall: 'unknown', score: null }),
        },
      ],
      coverage,
    )
    const real = filled.find((bucket) => bucket.bucket_start === realStart)
    expect(real?.health.score).toBe(92)

    const replaced = filled.find((bucket) => bucket.bucket_start === emptyStart)
    expect(replaced?.health.score).not.toBe(92)
    if (replaced) {
      expect(Number.isFinite(replaced.health.score)).toBe(true)
    }
  })

  it('fills a sparse window with mixed healthy / degraded / failed / blank faces', () => {
    const window: MonitorCoverage = {
      ...coverage,
      requested_end: '2026-09-10T11:30:00.000Z',
      data_through: '2026-09-10T11:30:00.000Z',
    }
    const starts = coverageBucketStarts(window)
    const filled = withDemoBuckets('openai:gpt-4', [], window)
    const moods = filled.map((bucket) => statusFace(bucket.health.score, bucket.health.overall).mood)

    expect(starts.length).toBeGreaterThanOrEqual(14)
    expect(moods).toContain('joy')
    expect(moods).toContain('happy')
    expect(moods).toContain('smile')
    expect(moods).toContain('neutral')
    expect(moods).toContain('worried')
    expect(moods).toContain('sad')
    expect(filled.length).toBeLessThan(starts.length)
  })

  it('treats empty groups as inactive and keeps the empty-window showcase to three rows', () => {
    expect(rowHasActivity({ metrics: metric({ request_count: 0, rpm: 0 }), buckets: [] })).toBe(false)
    expect(rowHasActivity({ metrics: metric({ request_count: 12 }), buckets: [] })).toBe(true)
    const thresholds = {
      warning_error_rate: 0.05,
      critical_error_rate: 0.2,
      warning_ttft_ms: 8000,
      critical_ttft_ms: 20000,
      warning_cache_rate: 0.85,
      critical_cache_rate: 0.6,
    }
    const rows = studioShowcaseRows(coverage, thresholds)
    expect(rows).toHaveLength(3)
    expect(rows.map((row) => row.labelKey)).toEqual(['healthy', 'warning', 'critical'])
    expect(rows.map((row) => row.rate)).toEqual([0.08, 0.12, 0.35])
    expect(errorRateTone(rows[0].metrics.error_rate, thresholds)).toBe('healthy')
    expect(ttftTone(rows[0].metrics.ttft.p50_ms, thresholds, rows[0].metrics.ttft)).toBe('healthy')
    expect(cacheTone(rows[0].metrics.cache_rate, thresholds)).toBe('healthy')
    expect(errorRateTone(rows[1].metrics.error_rate, thresholds)).toBe('warning')
    expect(ttftTone(rows[1].metrics.ttft.p50_ms, thresholds, rows[1].metrics.ttft)).toBe('warning')
    expect(cacheTone(rows[1].metrics.cache_rate, thresholds)).toBe('warning')
    expect(errorRateTone(rows[2].metrics.error_rate, thresholds)).toBe('critical')
    expect(ttftTone(rows[2].metrics.ttft.p50_ms, thresholds, rows[2].metrics.ttft)).toBe('critical')
    expect(cacheTone(rows[2].metrics.cache_rate, thresholds)).toBe('critical')
  })

  it('extends coverage by one bucket per test slot without moving the window start', () => {
    expect(STUDIO_TEST_SLOT_INTERVAL_MS).toBe(10_000)
    expect(STUDIO_FACE_MOVE_MS).toBe(500)
    expect(STUDIO_FACE_POP_MS).toBe(500)
    expect(STUDIO_FACE_STAGGER_MS).toBe(120)
    expect(STUDIO_CHART_MIN_GAP).toBe(40)
    const next = extendCoverage(coverage, 1)
    expect(next?.requested_start).toBe(coverage.requested_start)
    expect(coverageBucketStarts(next!).length).toBe(coverageBucketStarts(coverage).length + 1)
    expect(extendCoverage(coverage, 0)).toBe(coverage)
  })

  it('treats only keys that arrived after the first paint as incoming faces', () => {
    expect(studioIncomingKeys(new Set(), ['a', 'b'])).toEqual([])
    expect(studioIncomingKeys(new Set(['a', 'b']), ['a', 'b', 'c'])).toEqual(['c'])
    expect(studioIncomingKeys(new Set(['a', 'b']), ['x', 'y'])).toEqual([])
    expect(studioPopIncomingKeys(new Set(['a', 'b']), ['a', 'b', 'c'])).toEqual(['c'])
    expect(studioPopIncomingKeys(new Set(['a', 'b']), ['a', 'b', 'c', 'd'])).toEqual([])
    expect(studioPopIncomingKeys(new Set(['a', 'b']), ['x', 'y'])).toEqual([])
    expect(studioMissedSlots(0)).toBe(0)
    expect(studioMissedSlots(9_999)).toBe(0)
    expect(studioMissedSlots(10_000)).toBe(1)
    expect(studioMissedSlots(35_000)).toBe(3)
    expect(studioPageHidden()).toBe(typeof document !== 'undefined' && document.visibilityState === 'hidden')
  })

  it('reuses previous demo buckets when coverage only grows by one slot', () => {
    const first = withDemoBuckets('openai:gpt-4', EMPTY_STUDIO_BUCKETS, coverage)
    const second = withDemoBuckets('openai:gpt-4', EMPTY_STUDIO_BUCKETS, extendCoverage(coverage, 1))
    expect(first.length).toBeGreaterThan(0)
    expect(second.length).toBeGreaterThanOrEqual(first.length)
    expect(first.every((bucket, index) => second[index] === bucket)).toBe(true)
  })

  it('drops the oldest demo slot when the 18-slot window slides forward', () => {
    const seed = 'cap-18:gpt-4'
    const long: MonitorCoverage = {
      ...coverage,
      requested_end: '2026-09-10T12:00:00.000Z',
      data_through: '2026-09-10T12:00:00.000Z',
    }
    const firstStarts = coverageBucketStarts(long)
    const first = withDemoBuckets(seed, EMPTY_STUDIO_BUCKETS, long)
    const next = extendCoverage(long, 1)
    const secondStarts = coverageBucketStarts(next!)
    const second = withDemoBuckets(seed, EMPTY_STUDIO_BUCKETS, next)
    expect(firstStarts).toHaveLength(STUDIO_MAX_SLOTS)
    expect(secondStarts).toHaveLength(STUDIO_MAX_SLOTS)
    expect(secondStarts[0]).not.toBe(firstStarts[0])
    expect(secondStarts.at(-1)).not.toBe(firstStarts.at(-1))
    expect(second.length).toBeLessThanOrEqual(STUDIO_MAX_SLOTS)
    expect(first.filter((bucket) => second.includes(bucket)).length).toBeGreaterThan(0)
    expect(secondStarts[0]).toBe(firstStarts[1])
    expect(secondStarts.at(-1)).not.toBe(firstStarts.at(-1))
  })

  it('FIFO extra 24h slots at 24 instead of shrinking the native window to 18', () => {
    const day: MonitorCoverage = {
      ...coverage,
      requested_start: '2026-09-10T00:00:00.000Z',
      requested_end: '2026-09-11T00:00:00.000Z',
      coverage_start: '2026-09-10T00:00:00.000Z',
      data_through: '2026-09-11T00:00:00.000Z',
      bucket_seconds: 3600,
    }
    const firstStarts = coverageBucketStarts(day)
    const next = extendCoverage(day, 1)
    const secondStarts = coverageBucketStarts(next!)
    expect(firstStarts).toHaveLength(24)
    expect(secondStarts).toHaveLength(24)
    expect(secondStarts[0]).toBe(firstStarts[1])
    expect(secondStarts.at(-1)).not.toBe(firstStarts.at(-1))
    const filled = withDemoBuckets('range-24h:gpt-4', EMPTY_STUDIO_BUCKETS, next)
    expect(filled.length).toBeGreaterThan(1)
    expect(filled.length).toBeLessThanOrEqual(24)
  })

  it('keeps mixed demo faces after FIFO slides past the first 18 slots', () => {
    const seed = 'fifo-cycle:gpt-4'
    let window: MonitorCoverage = {
      ...coverage,
      requested_end: '2026-09-10T12:00:00.000Z',
      data_through: '2026-09-10T12:00:00.000Z',
    }
    let filled = withDemoBuckets(seed, EMPTY_STUDIO_BUCKETS, window)
    expect(coverageBucketStarts(window)).toHaveLength(STUDIO_MAX_SLOTS)
    expect(filled.length).toBeGreaterThan(1)
    expect(filled.length).toBeLessThanOrEqual(STUDIO_MAX_SLOTS)
    for (let i = 0; i < STUDIO_MAX_SLOTS + 2; i += 1) {
      window = extendCoverage(window, 1) as MonitorCoverage
      filled = withDemoBuckets(seed, EMPTY_STUDIO_BUCKETS, window)
    }
    expect(coverageBucketStarts(window)).toHaveLength(STUDIO_MAX_SLOTS)
    expect(filled.length).toBeGreaterThan(1)
    expect(filled.length).toBeLessThanOrEqual(STUDIO_MAX_SLOTS)
    const scores = filled.map((bucket) => bucket.health.score)
    expect(new Set(scores).size).toBeGreaterThan(1)
    const moods = filled.map((bucket) => statusFace(bucket.health.score, bucket.health.overall).mood)
    expect(new Set(moods).size).toBeGreaterThan(1)
  })

  it('promotes a demo slot to the live API bucket without rebuilding other faces', () => {
    const seed = 'promote-real:gpt-4'
    const demoFilled = withDemoBuckets(seed, EMPTY_STUDIO_BUCKETS, coverage)
    const start = demoFilled[0]?.bucket_start
    expect(start).toBeTruthy()
    const live = { bucket_start: start, metrics: metric(), health: health({ score: 81 }) }
    const promoted = withDemoBuckets(seed, [live], coverage)
    expect(promoted.find((bucket) => bucket.bucket_start === start)).toBe(live)
    expect(promoted).not.toContain(demoFilled[0])
    expect(demoFilled.slice(1).every((bucket) => promoted.includes(bucket))).toBe(true)
    expect(promoted.map((bucket) => bucket.bucket_start)).toEqual(demoFilled.map((bucket) => bucket.bucket_start))
  })

  it('keeps a live bucket when a later payload has no traffic for that slot', () => {
    const seed = 'keep-real:gpt-4'
    const start = coverageBucketStarts(coverage)[0]
    const live = { bucket_start: start, metrics: metric(), health: health({ score: 77 }) }
    const first = withDemoBuckets(seed, [live], coverage)
    const second = withDemoBuckets(seed, [], coverage)
    expect(first.find((bucket) => new Date(bucket.bucket_start).toISOString() === start)).toBe(live)
    expect(second.find((bucket) => new Date(bucket.bucket_start).toISOString() === start)).toBe(live)
  })

  it('treats request_count as live traffic even when health score is still empty', () => {
    const seed = 'live-count:gpt-4'
    const start = coverageBucketStarts(coverage)[0]
    const live = {
      bucket_start: start,
      metrics: metric({ request_count: 4 }),
      health: health({ overall: 'unknown', score: null }),
    }
    const filled = withDemoBuckets(seed, [live], coverage)
    expect(filled.find((bucket) => new Date(bucket.bucket_start).toISOString() === start)).toBe(live)
  })

  it('runs studioAfterPaint after two animation frames and can cancel', async () => {
    const order: string[] = []
    const waitFrames = () =>
      new Promise<void>((resolve) => {
        requestAnimationFrame(() => requestAnimationFrame(() => resolve()))
      })
    order.push('sync')
    const cancel = studioAfterPaint(() => order.push('paint'))
    expect(order).toEqual(['sync'])
    await waitFrames()
    expect(order).toEqual(['sync', 'paint'])
    const cancelled: string[] = []
    const stop = studioAfterPaint(() => cancelled.push('paint'))
    stop()
    await waitFrames()
    expect(cancelled).toEqual([])
    cancel()
  })
})
