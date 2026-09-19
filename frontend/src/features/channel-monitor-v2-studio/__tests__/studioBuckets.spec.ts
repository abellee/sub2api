import { afterEach, describe, expect, it, vi } from 'vitest'
import { alignBuckets, coverageBucketStarts, isStudioBucketEmpty, studioSparkValue, STUDIO_MAX_SLOTS, studioNativeSlotCount, studioSlotCap } from '../studioBuckets'
import type { MonitorMetric, MonitorHealth } from '@/api/channelMonitorV2'

const metric: MonitorMetric = {
  success_requests: 1,
  error_requests: 0,
  request_count: 1,
  token_count: 1,
  rpm: 1,
  tpm: 1,
  error_rate: 0,
  cache_rate: 0,
  cache_rate_numerator: 0,
  cache_rate_denominator: 1,
  ttft: { sample_count: 1, p50_ms: 10, p95_ms: 20, avg_ms: 12 },
  duration: { sample_count: 1, p50_ms: 10, p95_ms: 20, avg_ms: 12 },
}

const health: MonitorHealth = {
  overall: 'healthy',
  error_rate: 'healthy',
  ttft: 'healthy',
  score: 90,
  minimum_sample: 1,
}

describe('studioBuckets', () => {
  it('builds aligned empty slots across the requested window', () => {
    const starts = coverageBucketStarts({
      requested_start: '2026-09-10T10:00:00.000Z',
      requested_end: '2026-09-10T10:10:00.000Z',
      data_through: '2026-09-10T10:10:00.000Z',
      bucket_seconds: 300,
    })
    expect(starts).toHaveLength(2)
    const slots = alignBuckets(starts, [
      {
        bucket_start: starts[1],
        metrics: metric,
        health,
      },
    ])
    expect(slots[0].bucket).toBeUndefined()
    expect(slots[1].bucket?.health.score).toBe(90)
  })

  it('keeps only the newest 18 slots in a longer window', () => {
    expect(STUDIO_MAX_SLOTS).toBe(18)
    const starts = coverageBucketStarts({
      requested_start: '2026-09-10T10:00:00.000Z',
      requested_end: '2026-09-10T12:00:00.000Z',
      data_through: '2026-09-10T12:00:00.000Z',
      bucket_seconds: 300,
    })
    expect(starts).toHaveLength(18)
    expect(starts[0]).toBe('2026-09-10T10:30:00.000Z')
    expect(starts.at(-1)).toBe('2026-09-10T11:55:00.000Z')
  })

  it('keeps each time range at its native slot count instead of shrinking to 18', () => {
    expect(studioNativeSlotCount(300)).toBe(18)
    expect(studioNativeSlotCount(3600)).toBe(24)
    expect(studioNativeSlotCount(12 * 3600)).toBe(14)
    expect(studioNativeSlotCount(24 * 3600)).toBe(30)
    expect(studioSlotCap(3600)).toBe(24)
    expect(studioSlotCap(24 * 3600)).toBe(30)

    const hourly = coverageBucketStarts({
      requested_start: '2026-09-10T00:00:00.000Z',
      requested_end: '2026-09-11T00:00:00.000Z',
      data_through: '2026-09-11T00:00:00.000Z',
      bucket_seconds: 3600,
    })
    expect(hourly).toHaveLength(24)
    expect(hourly[0]).toBe('2026-09-10T00:00:00.000Z')
    expect(hourly.at(-1)).toBe('2026-09-10T23:00:00.000Z')

    const weekly = coverageBucketStarts({
      requested_start: '2026-09-03T00:00:00.000Z',
      requested_end: '2026-09-10T00:00:00.000Z',
      data_through: '2026-09-10T00:00:00.000Z',
      bucket_seconds: 12 * 3600,
    })
    expect(weekly).toHaveLength(14)

    const monthly = coverageBucketStarts({
      requested_start: '2026-08-11T00:00:00.000Z',
      requested_end: '2026-09-10T00:00:00.000Z',
      data_through: '2026-09-10T00:00:00.000Z',
      bucket_seconds: 24 * 3600,
    })
    expect(monthly).toHaveLength(30)
  })

  it('keeps empty-range coverage even when the API sent no requested_end', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-10T12:00:00.000Z'))
    const starts = coverageBucketStarts({
      requested_start: '2026-09-10T10:30:00.000Z',
      data_through: '2026-09-10T11:00:00.000Z',
      bucket_seconds: 300,
    })
    expect(starts).toHaveLength(18)
    expect(starts[0]).toBe('2026-09-10T10:30:00.000Z')
    expect(starts.at(-1)).toBe('2026-09-10T11:55:00.000Z')
    const slots = alignBuckets(starts, [
      { bucket_start: '2026-09-10T11:00:00.000Z', metrics: metric, health },
    ])
    expect(slots.filter((slot) => !slot.bucket)).toHaveLength(17)
    expect(isStudioBucketEmpty(slots[6].bucket)).toBe(false)
  })

  it('snaps late-arriving native hourly buckets onto the 24h coverage grid', () => {
    const starts = coverageBucketStarts({
      requested_start: '2026-09-10T00:00:00.000Z',
      requested_end: '2026-09-11T00:00:00.000Z',
      data_through: '2026-09-11T00:00:00.000Z',
      bucket_seconds: 3600,
    })
    const slots = alignBuckets(starts, [
      { bucket_start: '2026-09-10T10:07:00.000Z', metrics: metric, health },
    ])
    expect(slots).toHaveLength(24)
    expect(slots[10].bucket?.health.score).toBe(90)
    expect(slots[9].bucket).toBeUndefined()
  })

  it('includes the current in-progress bucket when now is past a boundary', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-10T12:03:00.000Z'))
    const starts = coverageBucketStarts({
      requested_start: '2026-09-10T10:30:00.000Z',
      bucket_seconds: 300,
    })
    expect(starts).toHaveLength(18)
    expect(starts[0]).toBe('2026-09-10T10:35:00.000Z')
    expect(starts.at(-1)).toBe('2026-09-10T12:00:00.000Z')
  })

  it('treats zero-traffic buckets as empty preset slots', () => {
    expect(isStudioBucketEmpty(undefined)).toBe(true)
    expect(
      isStudioBucketEmpty({
        bucket_start: '2026-09-10T10:00:00.000Z',
        metrics: {
          ...metric,
          request_count: 0,
          success_requests: 0,
          rpm: 0,
          tpm: 0,
          error_rate: 0,
          success_rate: 0,
          cache_rate: 0,
          ttft: { sample_count: 0, p50_ms: null, p95_ms: null, avg_ms: null },
          duration: { sample_count: 0, p50_ms: null, p95_ms: null, avg_ms: null },
        },
        health: { ...health, overall: 'unknown', score: null },
      }),
    ).toBe(true)
  })

  it('keeps user-redacted buckets with rates or health as live slots', () => {
    const redacted = {
      bucket_start: '2026-09-10T10:00:00.000Z',
      metrics: {
        ...metric,
        request_count: 0,
        success_requests: 0,
        error_requests: 0,
        rpm: 4.2,
        tpm: 0,
        error_rate: 0,
        success_rate: 0.92,
        cache_rate: 0.3,
        cache_rate_denominator: 0,
        ttft: { sample_count: 0, p50_ms: 180, p95_ms: 400, avg_ms: 220 },
      },
      health: { ...health, overall: 'healthy', score: 88 },
    }
    expect(isStudioBucketEmpty(redacted)).toBe(false)
    expect(studioSparkValue(redacted)).toBe(88)
    expect(studioSparkValue({ ...redacted, health: { ...redacted.health, score: 72 } })).toBe(72)
  })

  it('plots health score so the K-line moves independently of error_rate', () => {
    const quiet = {
      bucket_start: '2026-09-10T10:00:00.000Z',
      metrics: { ...metric, error_rate: 0, rpm: 12 },
      health: { ...health, score: 70 },
    }
    const busy = {
      bucket_start: '2026-09-10T10:05:00.000Z',
      metrics: { ...metric, error_rate: 0, rpm: 40 },
      health: { ...health, score: 94 },
    }
    expect(studioSparkValue(quiet)).toBe(70)
    expect(studioSparkValue(busy)).toBe(94)
    expect(studioSparkValue(quiet)).not.toBe(studioSparkValue(busy))
  })
})

afterEach(() => {
  vi.useRealTimers()
})
