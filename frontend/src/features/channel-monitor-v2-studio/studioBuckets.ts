import type { MonitorCoverage, MonitorHealth, MonitorMatrixBucket, MonitorMetric } from '@/api/channelMonitorV2'

export type StudioBucketPoint = {
  bucket_start: string
  metrics: MonitorMetric
  health: MonitorHealth
}

export type StudioAlignedSlot = {
  start: string
  bucket?: MonitorMatrixBucket
}

let startsCacheKey = ''
let startsCache: string[] = []

/** Extra appended slots FIFO once a range is already at its native length. */
export const STUDIO_MAX_SLOTS = 18

/** Native face counts for 90m / 24h / 7d / 30d. FIFO must not shrink a range below this. */
export function studioNativeSlotCount(bucketSeconds: number): number {
  const step = Math.max(60, bucketSeconds)
  if (step <= 300) return 18
  if (step <= 3600) return 24
  if (step <= 12 * 3600) return 14
  return 30
}

export function studioSlotCap(bucketSeconds: number): number {
  return Math.max(STUDIO_MAX_SLOTS, studioNativeSlotCount(bucketSeconds))
}

export function limitStudioStarts(starts: string[], max = STUDIO_MAX_SLOTS): string[] {
  if (!Number.isFinite(max) || max <= 0 || starts.length <= max) return starts
  return starts.slice(-max)
}

const UNIX_EPOCH_MS = Date.UTC(1970, 0, 2)

function validTime(value?: string | null): number {
  if (!value) return NaN
  const time = new Date(value).getTime()
  if (!Number.isFinite(time) || time < UNIX_EPOCH_MS) return NaN
  return time
}

/** Exclusive end of the in-progress bucket. Exact boundaries keep that instant. */
function exclusiveEndFromNow(step: number): number {
  return Math.ceil(Date.now() / step) * step
}

function finitePositive(value: number | null | undefined): boolean {
  return value != null && Number.isFinite(value) && value > 0
}

function finitePresent(value: number | null | undefined): boolean {
  return value != null && Number.isFinite(value)
}

/**
 * User snapshots zero request_count / sample_count. Treat a bucket as live when
 * any remaining rate, latency, throughput, or health signal is present.
 */
export function studioMetricsHaveActivity(
  metrics?: MonitorMetric | null,
  health?: MonitorHealth | null,
): boolean {
  if (!metrics && !health) return false
  if ((metrics?.request_count || 0) > 0) return true
  if ((metrics?.success_requests || 0) > 0 || (metrics?.error_requests || 0) > 0) return true
  if (finitePositive(metrics?.rpm) || finitePositive(metrics?.tpm)) return true
  if (finitePresent(metrics?.ttft?.p50_ms) || finitePresent(metrics?.duration?.p50_ms)) return true
  if (finitePresent(health?.score)) return true
  if (health?.overall && health.overall !== 'unknown') return true
  if (finitePositive(metrics?.error_rate) || finitePositive(metrics?.cache_rate)) return true
  if (finitePositive(metrics?.success_rate)) return true
  return false
}

/** True when the slot has no traffic and should render as a preset empty interval. */
export function isStudioBucketEmpty(bucket?: StudioBucketPoint | null): boolean {
  if (!bucket) return true
  return !studioMetricsHaveActivity(bucket.metrics, bucket.health)
}

/** K-line Y value: health score (0–100). Empty slots stay null. */
export function studioSparkValue(bucket?: StudioBucketPoint | null): number | null {
  if (!bucket || isStudioBucketEmpty(bucket)) return null
  const score = bucket.health?.score
  if (score != null && Number.isFinite(score)) return score
  const success = bucket.metrics?.success_rate
  if (success != null && Number.isFinite(success)) return success * 100
  const rate = 1 - (bucket.metrics?.error_rate || 0)
  return Number.isFinite(rate) ? rate * 100 : null
}

export function coverageBucketStarts(
  coverage: Pick<MonitorCoverage, 'requested_start' | 'requested_end' | 'data_through' | 'bucket_seconds'>,
): string[] {
  const step = Math.max(60, coverage.bucket_seconds) * 1000
  const native = studioNativeSlotCount(coverage.bucket_seconds)
  const cap = studioSlotCap(coverage.bucket_seconds)
  const requestedStart = validTime(coverage.requested_start)
  const requestedEnd = validTime(coverage.requested_end)
  const hasRequestedEnd =
    Number.isFinite(requestedEnd) && (!Number.isFinite(requestedStart) || requestedEnd > requestedStart)

  let end = hasRequestedEnd ? requestedEnd : exclusiveEndFromNow(step)
  let start = requestedStart
  if (!Number.isFinite(start) || start >= end) {
    start = end - native * step
  }
  // Missing requested_end used to fall back to data_through, which trims trailing
  // empty slots. Pad back to the native preset length (18 / 24 / 14 / 30).
  if (!hasRequestedEnd) {
    const minStart = end - native * step
    if (start > minStart) start = minStart
  }

  const nowBucket = hasRequestedEnd ? 0 : Math.floor(Date.now() / step)
  const cacheKey = `${start}|${end}|${step}|${cap}|${nowBucket}`
  if (cacheKey === startsCacheKey) return startsCache

  const first = Math.floor(start / step) * step
  const last = first + Math.floor((end - first - 1) / step) * step
  if (![first, last].every(Number.isFinite) || last < first) {
    startsCacheKey = cacheKey
    startsCache = []
    return startsCache
  }
  const from = Math.max(first, last - (cap - 1) * step)
  const starts: string[] = []
  for (let cursor = from; cursor <= last; cursor += step) {
    starts.push(new Date(cursor).toISOString())
  }
  startsCacheKey = cacheKey
  startsCache = starts
  return starts
}

export function alignBuckets(starts: string[], buckets: StudioBucketPoint[] | undefined): StudioAlignedSlot[] {
  if (!starts.length) {
    return (buckets || []).map((bucket) => ({
      start: new Date(bucket.bucket_start).toISOString(),
      bucket: bucket as MonitorMatrixBucket,
    }))
  }
  const indexByStart = new Map(starts.map((start, index) => [start, index]))
  const slots: StudioAlignedSlot[] = starts.map((start) => ({ start }))
  const first = new Date(starts[0]).getTime()
  const step = starts.length > 1 ? new Date(starts[1]).getTime() - first : 0
  for (const bucket of buckets || []) {
    const time = new Date(bucket.bucket_start).getTime()
    if (!Number.isFinite(time)) continue
    let index = indexByStart.get(new Date(time).toISOString())
    if (index == null && step > 0) {
      const snapped = Math.floor((time - first) / step + 1e-6)
      if (snapped >= 0 && snapped < starts.length && time < first + (snapped + 1) * step) {
        index = snapped
      }
    }
    if (index != null) slots[index] = { start: starts[index], bucket: bucket as MonitorMatrixBucket }
  }
  return slots
}
