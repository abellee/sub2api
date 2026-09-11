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

export function coverageBucketStarts(
  coverage: Pick<MonitorCoverage, 'requested_start' | 'requested_end' | 'data_through' | 'bucket_seconds'>,
): string[] {
  const step = Math.max(60, coverage.bucket_seconds) * 1000
  const cap = studioSlotCap(coverage.bucket_seconds)
  const cacheKey = `${coverage.requested_start}|${coverage.requested_end || ''}|${coverage.data_through}|${step}|${cap}`
  if (cacheKey === startsCacheKey) return startsCache
  const requestedStart = new Date(coverage.requested_start).getTime()
  const requestedEndRaw = coverage.requested_end ? new Date(coverage.requested_end).getTime() : NaN
  const dataThrough = new Date(coverage.data_through).getTime()
  const end = Number.isFinite(requestedEndRaw) && requestedEndRaw > requestedStart ? requestedEndRaw : dataThrough
  if (![requestedStart, end].every(Number.isFinite) || requestedStart >= end) {
    startsCacheKey = cacheKey
    startsCache = []
    return startsCache
  }
  const first = Math.floor(requestedStart / step) * step
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
  for (const bucket of buckets || []) {
    const key = new Date(bucket.bucket_start).toISOString()
    const index = indexByStart.get(key)
    if (index != null) slots[index] = { start: starts[index], bucket: bucket as MonitorMatrixBucket }
  }
  return slots
}
