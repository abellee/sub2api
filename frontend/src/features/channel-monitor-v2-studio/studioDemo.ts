import type { MonitorCoverage, MonitorHealth, MonitorMetric } from '@/api/channelMonitorV2'
import { coverageBucketStarts, limitStudioStarts, studioSlotCap, type StudioBucketPoint } from './studioBuckets'
import { STUDIO_DEFAULT_THRESHOLDS, type StudioThresholds } from './studioFormat'

function resolveThresholds(thresholds?: StudioThresholds | null) {
  return {
    warning_error_rate: thresholds?.warning_error_rate ?? STUDIO_DEFAULT_THRESHOLDS.warning_error_rate,
    critical_error_rate: thresholds?.critical_error_rate ?? STUDIO_DEFAULT_THRESHOLDS.critical_error_rate,
    warning_ttft_ms: thresholds?.warning_ttft_ms ?? STUDIO_DEFAULT_THRESHOLDS.warning_ttft_ms,
    critical_ttft_ms: thresholds?.critical_ttft_ms ?? STUDIO_DEFAULT_THRESHOLDS.critical_ttft_ms,
    warning_cache_rate: thresholds?.warning_cache_rate ?? STUDIO_DEFAULT_THRESHOLDS.warning_cache_rate,
    critical_cache_rate: thresholds?.critical_cache_rate ?? STUDIO_DEFAULT_THRESHOLDS.critical_cache_rate,
  }
}

/** joy / happy / smile / neutral / worried / sad / blank */
const DEMO_SCORES: Array<number | null> = [96, 88, 72, 55, 40, 12, null]

function hasFiniteScore(bucket?: { health?: { score?: number | null } }): boolean {
  const score = bucket?.health?.score
  return score != null && Number.isFinite(Number(score))
}

const demoOrigins = new WeakSet<object>()

function slotKey(start: string): string {
  const time = new Date(start).getTime()
  return Number.isFinite(time) ? new Date(time).toISOString() : start
}

function indexByStart(buckets: StudioBucketPoint[]): Map<string, StudioBucketPoint> {
  const map = new Map<string, StudioBucketPoint>()
  for (const bucket of buckets) {
    map.set(slotKey(bucket.bucket_start), bucket)
  }
  return map
}

function isLiveBucket(bucket?: StudioBucketPoint | null): bucket is StudioBucketPoint {
  if (!bucket || demoOrigins.has(bucket)) return false
  return hasFiniteScore(bucket) || (bucket.metrics?.request_count || 0) > 0
}

function hashSeed(text: string): number {
  let hash = 2166136261
  for (let i = 0; i < text.length; i += 1) {
    hash ^= text.charCodeAt(i)
    hash = Math.imul(hash, 16777619)
  }
  return hash >>> 0
}

function demoMetric(score: number, thresholds?: StudioThresholds | null): MonitorMetric {
  const t = resolveThresholds(thresholds)
  const errorRate =
    score >= 80
      ? t.warning_error_rate * 0.2
      : score >= 50
        ? (t.warning_error_rate + t.critical_error_rate) / 2
        : Math.min(0.95, t.critical_error_rate * 1.6)
  const cacheRate =
    score >= 80
      ? Math.min(0.99, t.warning_cache_rate + 0.05)
      : score >= 50
        ? (t.warning_cache_rate + t.critical_cache_rate) / 2
        : Math.max(0, t.critical_cache_rate * 0.5)
  const ttft =
    score >= 80
      ? Math.max(80, t.warning_ttft_ms * 0.15)
      : score >= 50
        ? (t.warning_ttft_ms + t.critical_ttft_ms) / 2
        : t.critical_ttft_ms * 1.25
  return {
    success_requests: Math.round((1 - errorRate) * 100),
    error_requests: Math.round(errorRate * 100),
    request_count: 100,
    token_count: 1200,
    rpm: 1.6,
    tpm: 80,
    error_rate: errorRate,
    cache_rate: cacheRate,
    cache_rate_numerator: Math.round(cacheRate * 100),
    cache_rate_denominator: 100,
    ttft: {
      sample_count: 80,
      p50_ms: ttft,
      p90_ms: ttft * 1.35,
      p95_ms: ttft * 1.55,
      avg_ms: ttft * 1.12,
    },
    duration: {
      sample_count: 80,
      p50_ms: ttft * 2.1,
      p90_ms: ttft * 3,
      p95_ms: ttft * 3.4,
      avg_ms: ttft * 2.3,
    },
  }
}

function demoHealth(score: number): MonitorHealth {
  // Keep a healthy-but-not-hot band so smile faces still appear (scoreTone treats <80 as warning).
  const overall = score >= 70 ? 'healthy' : score >= 40 ? 'warning' : 'critical'
  const state = overall
  return {
    overall: state,
    error_rate: state,
    ttft: state,
    cache: state,
    score,
    error_rate_score: score,
    ttft_score: score,
    cache_score: score,
    minimum_sample: 1,
  }
}

/** Studio-only test injector: a new time slot appears on this interval. */
export const STUDIO_TEST_SLOT_INTERVAL_MS = 10_000

/** Horizontal squeeze / chart slide duration. Pop starts as soon as this ends. */
export const STUDIO_FACE_MOVE_MS = 500

/** Face pop / scale-in duration. */
export const STUDIO_FACE_POP_MS = 500

/** Top-to-bottom delay so rows do not all animate in the same frame. */
export const STUDIO_FACE_STAGGER_MS = 120

/** Minimum pixel gap between K-line dots on small / wide cards. */
export const STUDIO_CHART_MIN_GAP = 40

/** Dots that should stay visible on a non-small (18rem) KPI card. */
export const STUDIO_CHART_VISIBLE_DOTS = 5

/** Run after the current data commit has painted, so motion starts on a later frame. */
export function studioAfterPaint(fn: () => void): () => void {
  let inner = 0
  const outer = requestAnimationFrame(() => {
    inner = requestAnimationFrame(fn)
  })
  return () => {
    cancelAnimationFrame(outer)
    if (inner) cancelAnimationFrame(inner)
  }
}

export function studioPageHidden(): boolean {
  return typeof document !== 'undefined' && document.visibilityState === 'hidden'
}

/** Subscribe to document visibility. Does not fire immediately. */
export function subscribeStudioVisibility(onChange: (hidden: boolean) => void): () => void {
  if (typeof document === 'undefined') return () => {}
  const handler = () => onChange(document.visibilityState === 'hidden')
  document.addEventListener('visibilitychange', handler)
  return () => document.removeEventListener('visibilitychange', handler)
}

/** Whole slots that elapsed while the page was hidden or the timer lagged. */
export function studioMissedSlots(elapsedMs: number, intervalMs = STUDIO_TEST_SLOT_INTERVAL_MS): number {
  if (!Number.isFinite(elapsedMs) || !Number.isFinite(intervalMs) || intervalMs <= 0 || elapsedMs < intervalMs) {
    return 0
  }
  return Math.floor(elapsedMs / intervalMs)
}

/** Keys that arrived after the first paint; empty when the row is still priming or fully replaced. */
export function studioIncomingKeys(seen: Iterable<string>, nextKeys: string[]): string[] {
  const seenSet = seen instanceof Set ? seen : new Set(seen)
  if (seenSet.size === 0) return []
  if (!nextKeys.some((key) => seenSet.has(key))) return []
  return nextKeys.filter((key) => !seenSet.has(key))
}

/** Only a single appended slot should pop; range switches and catch-up batches settle in place. */
export function studioPopIncomingKeys(seen: Iterable<string>, nextKeys: string[]): string[] {
  if (studioPageHidden()) return []
  const incoming = studioIncomingKeys(seen, nextKeys)
  return incoming.length === 1 ? incoming : []
}

/** Stretch coverage forward so `withDemoBuckets` fills extra trailing slots. */
export function extendCoverage(
  coverage: MonitorCoverage | null,
  extraSlots: number,
): MonitorCoverage | null {
  if (!coverage || extraSlots <= 0) return coverage
  const step = Math.max(60, coverage.bucket_seconds || 60) * 1000
  const extraMs = extraSlots * step
  const shift = (value?: string | null) => {
    if (!value) return value || undefined
    const time = new Date(value).getTime()
    if (!Number.isFinite(time)) return value
    return new Date(time + extraMs).toISOString()
  }
  return {
    ...coverage,
    requested_end: shift(coverage.requested_end || coverage.data_through) || coverage.requested_end,
    data_through: shift(coverage.data_through) || coverage.data_through,
  }
}

export function demoBucket(start: string, score: number, thresholds?: StudioThresholds | null): StudioBucketPoint {
  const point: StudioBucketPoint = {
    bucket_start: start,
    metrics: demoMetric(score, thresholds),
    health: demoHealth(score),
  }
  demoOrigins.add(point)
  return point
}

export function rowHasActivity(row: {
  metrics?: { request_count?: number; rpm?: number }
  buckets?: Array<{ metrics?: { request_count?: number }; health?: { score?: number | null } }>
}): boolean {
  if ((row.metrics?.request_count || 0) > 0 || (row.metrics?.rpm || 0) > 0) return true
  return (row.buckets || []).some(
    (bucket) => (bucket.metrics?.request_count || 0) > 0 || hasFiniteScore(bucket),
  )
}

export type StudioShowcaseRow = {
  key: string
  platform: string
  labelKey: 'healthy' | 'warning' | 'critical'
  rate: number
  metrics: MonitorMetric
  health: MonitorHealth
  buckets: StudioBucketPoint[]
}

const SHOWCASE = [
  { key: 'demo:openai', platform: 'openai', labelKey: 'healthy' as const, score: 96, rate: 0.08 },
  { key: 'demo:anthropic', platform: 'anthropic', labelKey: 'warning' as const, score: 55, rate: 0.12 },
  { key: 'demo:gemini', platform: 'gemini', labelKey: 'critical' as const, score: 12, rate: 0.35 },
]

/** Stable empty list so test-slot appends can reuse the previous fill cache. */
export const EMPTY_STUDIO_BUCKETS: StudioBucketPoint[] = []

/** A short mixed-state sample used only when the window has no live groups. */
export function studioShowcaseRows(
  coverage: MonitorCoverage | null,
  thresholds?: StudioThresholds | null,
): StudioShowcaseRow[] {
  return SHOWCASE.map((item) => ({
    key: item.key,
    platform: item.platform,
    labelKey: item.labelKey,
    rate: item.rate,
    metrics: demoMetric(item.score, thresholds),
    health: demoHealth(item.score),
    buckets: withDemoBuckets(item.key, EMPTY_STUDIO_BUCKETS, coverage, thresholds),
  }))
}

function coverageStepMs(coverage?: MonitorCoverage | null): number {
  return Math.max(60, coverage?.bucket_seconds || 60) * 1000
}

/** Position on the timeline, not in the 18-slot window, so FIFO does not collapse demo variety. */
function demoTimelineIndex(start: string, stepMs: number, fallback: number): number {
  const time = new Date(start).getTime()
  if (!Number.isFinite(time) || !Number.isFinite(stepMs) || stepMs <= 0) return fallback
  return Math.round(time / stepMs)
}

function fillDemoSlot(
  offset: number,
  start: string,
  index: number,
  bucket: StudioBucketPoint | undefined,
  thresholds?: StudioThresholds | null,
): StudioBucketPoint | null {
  if (isLiveBucket(bucket)) return bucket
  const score = DEMO_SCORES[(index + offset) % DEMO_SCORES.length]
  if (score == null) return null
  return demoBucket(start, score, thresholds)
}

type DemoFillCache = {
  filled: StudioBucketPoint[]
}

const demoFillCache = new Map<string, DemoFillCache>()

/** Fill empty slots with demo faces. Live API buckets replace demo in place and are never overwritten. */
export function withDemoBuckets(
  seed: string,
  buckets: StudioBucketPoint[] | undefined,
  coverage: MonitorCoverage | null,
  thresholds?: StudioThresholds | null,
): StudioBucketPoint[] {
  const existing = buckets ?? EMPTY_STUDIO_BUCKETS
  const stepMs = coverageStepMs(coverage)
  const starts = coverage
    ? coverageBucketStarts(coverage)
    : limitStudioStarts(
        existing.map((bucket) => slotKey(bucket.bucket_start)),
        studioSlotCap(stepMs / 1000),
      )
  const cacheKey = `${seed}|${stepMs}`
  const prev = demoFillCache.get(cacheKey)
  const offset = hashSeed(seed) % DEMO_SCORES.length
  const liveByStart = indexByStart(existing)
  const prevByStart = prev ? indexByStart(prev.filled) : new Map<string, StudioBucketPoint>()
  const filled: StudioBucketPoint[] = []
  starts.forEach((start, index) => {
    const live = liveByStart.get(start)
    if (isLiveBucket(live)) {
      filled.push(live)
      return
    }
    const kept = prevByStart.get(start)
    if (kept) {
      filled.push(kept)
      return
    }
    const next = fillDemoSlot(offset, start, demoTimelineIndex(start, stepMs, index), live, thresholds)
    if (next) filled.push(next)
  })
  if (prev && prev.filled.length === filled.length && prev.filled.every((bucket, index) => bucket === filled[index])) {
    return prev.filled
  }
  demoFillCache.set(cacheKey, { filled })
  return filled
}
