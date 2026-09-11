/**
 * Display helpers for the channel-monitor V2 studio shell.
 * Keep original monitorFormat untouched; this file only serves new components.
 */

import type { HealthState, MonitorHealth, MonitorMatrixRow, MonitorMetric } from '@/api/channelMonitorV2'
import { GROUP_PLATFORM_OPTIONS } from '@/constants/platforms'
import {
  formatMonitorPercent,
  formatMonitorSuccessRate,
  formatMonitorSuccessRateFromError,
  healthModeScore,
  isTtftUnavailable,
  type HealthDisplayMode,
} from '@/features/channel-monitor-v2/monitorFormat'
import type { GroupPlatform } from '@/types'
import { studioMetricsHaveActivity } from './studioBuckets'

export type StudioAccent = 'teal' | 'coral' | 'indigo' | 'amber' | 'sky' | 'slate'
export type StudioTone = 'healthy' | 'warning' | 'critical' | 'unknown'

export type StudioSparkPaint = {
  stroke: string
  deep: string
  fill: string
}

export const STUDIO_SPARK_PAINT: Record<StudioAccent, StudioSparkPaint> = {
  teal: { stroke: '#14b8a6', deep: '#0f766e', fill: '#14b8a6' },
  coral: { stroke: '#e11d48', deep: '#be123c', fill: '#e11d48' },
  indigo: { stroke: '#6366f1', deep: '#4338ca', fill: '#6366f1' },
  amber: { stroke: '#d97706', deep: '#b45309', fill: '#d97706' },
  sky: { stroke: '#0284c7', deep: '#0369a1', fill: '#0284c7' },
  slate: { stroke: '#64748b', deep: '#475569', fill: '#64748b' },
}

/** Stroke + fill for the KPI K-line. Keep this out of CSS vars so each card keeps its own paint. */
export function studioSparkPaint(accent?: StudioAccent | string | null): StudioSparkPaint {
  if (accent && accent in STUDIO_SPARK_PAINT) return STUDIO_SPARK_PAINT[accent as StudioAccent]
  return STUDIO_SPARK_PAINT.teal
}

let studioSparkSeq = 0

/** Module-level IDs. A counter in `<script setup>` resets per instance and collides every chart. */
export function allocStudioSparkIds(): { fillId: string; fadeId: string; maskId: string } {
  const n = ++studioSparkSeq
  return {
    fillId: `studio-spark-fill-${n}`,
    fadeId: `studio-spark-fade-${n}`,
    maskId: `studio-spark-mask-${n}`,
  }
}

/** Card wash follows status, not card index. */
export function studioAccentFromState(state?: StudioTone | string | null): StudioAccent {
  if (state === 'warning') return 'amber'
  if (state === 'critical') return 'coral'
  if (state === 'unknown') return 'slate'
  return 'teal'
}

/** Mirrors V2 settings defaults (MonitorSettingsPanel). */
export type StudioThresholds = {
  warning_error_rate?: number
  critical_error_rate?: number
  warning_ttft_ms?: number
  critical_ttft_ms?: number
  warning_cache_rate?: number
  critical_cache_rate?: number
}

export const STUDIO_DEFAULT_THRESHOLDS: Required<StudioThresholds> = {
  warning_error_rate: 0.05,
  critical_error_rate: 0.2,
  warning_ttft_ms: 3000,
  critical_ttft_ms: 10000,
  warning_cache_rate: 0.85,
  critical_cache_rate: 0.6,
}

/** KPI group names show 8 Chinese-character widths; ASCII letters count as half. */
export const STUDIO_GROUP_NAME_WIDTH = 8

export function studioCharDisplayWidth(char: string): number {
  const code = char.codePointAt(0) || 0
  if (code <= 0x7f) return 0.5
  if (code >= 0xff61 && code <= 0xff9f) return 0.5
  return 1
}

export function studioTextDisplayWidth(value: string): number {
  let width = 0
  for (const char of value) width += studioCharDisplayWidth(char)
  return width
}

export function truncateStudioGroupName(value: string, max = STUDIO_GROUP_NAME_WIDTH): string {
  const text = String(value || '')
  if (max <= 0) return ''
  if (studioTextDisplayWidth(text) <= max) return text
  let width = 0
  let out = ''
  for (const char of text) {
    const next = studioCharDisplayWidth(char)
    if (width + next > max) break
    out += char
    width += next
  }
  return `${out}…`
}

export function clamp01(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.min(1, Math.max(0, value))
}

/** 0–100 progress for the KPI ring. Null when the metric has no bounded scale. */
export function metricRingProgress(
  kind: 'success' | 'ttft' | 'cache' | 'tps' | 'rpm',
  metrics: MonitorMetric,
  health?: MonitorHealth,
): number | null {
  if (kind === 'success') return clamp01(1 - (metrics.error_rate || 0)) * 100
  if (kind === 'cache') return clamp01(metrics.cache_rate || 0) * 100
  if (kind === 'ttft') {
    const score = health?.ttft_score
    return score == null || Number.isNaN(score) ? null : clamp01(score / 100) * 100
  }
  return null
}

export function sparklineValues(
  trend: Array<{ metrics: MonitorMetric }> | undefined,
  pick: (metrics: MonitorMetric) => number | null | undefined,
): Array<number | null> {
  return (trend || []).map((point) => {
    const value = pick(point.metrics)
    if (value == null || Number.isNaN(Number(value))) return null
    return Number(value)
  })
}

/** SVG polyline points in a 100×28 viewBox. */
export function sparklinePoints(values: Array<number | null>, width = 100, height = 28, pad = 2): string {
  if (values.length < 2) return ''
  const present = values.filter((value): value is number => value != null && Number.isFinite(value))
  if (present.length < 2) return ''
  const min = Math.min(...present)
  const max = Math.max(...present)
  const span = max - min || 1
  const step = width / (values.length - 1)
  let last = present[0]
  return values
    .map((value, index) => {
      const yVal = value == null || !Number.isFinite(value) ? last : value
      last = yVal
      const x = index * step
      const y = pad + (height - pad * 2) * (1 - (yVal - min) / span)
      return `${x.toFixed(2)},${y.toFixed(2)}`
    })
    .join(' ')
}

export type StudioChartDot = {
  index: number
  x: number
  y: number
  value: number | null
}

/** Line + dot layout for the KPI chart. Every slot gets a point, including empty ones. */
export function studioChartDots(
  values: Array<number | null>,
  width = 100,
  height = 36,
  padX = 6,
  padY = 7,
): StudioChartDot[] {
  const n = values.length
  if (!n) return []
  const present = values.filter((value): value is number => value != null && Number.isFinite(value))
  const min = present.length ? Math.min(...present) : 0
  const max = present.length ? Math.max(...present) : 1
  const span = max - min || 1
  const inner = Math.max(1, n - 1)
  let last = present[0] ?? 0
  return values.map((value, index) => {
    const hasValue = value != null && Number.isFinite(value)
    const yVal = hasValue ? (value as number) : last
    if (hasValue) last = value as number
    const x = n === 1 ? width / 2 : padX + (index * (width - padX * 2)) / inner
    const y = padY + (height - padY * 2) * (1 - (yVal - min) / span)
    return { index, x, y, value: hasValue ? (value as number) : null }
  })
}

export function studioChartLine(dots: StudioChartDot[]): string {
  return dots.map((dot) => `${dot.x.toFixed(2)},${dot.y.toFixed(2)}`).join(' ')
}

export function studioChartArea(dots: StudioChartDot[], height = 36, width = 0): string {
  if (dots.length < 2) return ''
  const first = dots[0]
  const last = dots[dots.length - 1]
  const right = width > last.x ? width : last.x
  return `0,${height} 0,${first.y.toFixed(2)} ${studioChartLine(dots)} ${right.toFixed(2)},${last.y.toFixed(2)} ${right.toFixed(2)},${height}`
}

export function healthTone(state?: HealthState): StudioTone {
  if (state === 'healthy' || state === 'warning' || state === 'critical') return state
  return 'unknown'
}

export function scoreTone(score: number | null | undefined): StudioTone {
  if (score == null || Number.isNaN(score)) return 'unknown'
  if (score >= 80) return 'healthy'
  if (score >= 50) return 'warning'
  return 'critical'
}

function pickThreshold(value: number | undefined, fallback: number): number {
  return value != null && Number.isFinite(value) ? value : fallback
}

/** Success coloring uses the inverse of the V2 error-rate thresholds. */
export function errorRateTone(
  errorRate: number | null | undefined,
  thresholds?: StudioThresholds | null,
): StudioTone {
  if (errorRate == null || Number.isNaN(Number(errorRate))) return 'unknown'
  const critical = pickThreshold(thresholds?.critical_error_rate, STUDIO_DEFAULT_THRESHOLDS.critical_error_rate)
  const warning = pickThreshold(thresholds?.warning_error_rate, STUDIO_DEFAULT_THRESHOLDS.warning_error_rate)
  if (errorRate >= critical) return 'critical'
  if (errorRate >= warning) return 'warning'
  return 'healthy'
}

export function ttftTone(
  p50Ms: number | null | undefined,
  thresholds?: StudioThresholds | null,
  ttft?: { p50_ms?: number | null; sample_count?: number } | null,
): StudioTone {
  if (ttft ? isTtftUnavailable(ttft) : p50Ms == null || Number.isNaN(Number(p50Ms))) return 'unknown'
  const value = Number(p50Ms)
  const critical = pickThreshold(thresholds?.critical_ttft_ms, STUDIO_DEFAULT_THRESHOLDS.critical_ttft_ms)
  const warning = pickThreshold(thresholds?.warning_ttft_ms, STUDIO_DEFAULT_THRESHOLDS.warning_ttft_ms)
  if (value >= critical) return 'critical'
  if (value >= warning) return 'warning'
  return 'healthy'
}

/** Cache rate is higher-is-better: below critical is failed, below warning is degraded. */
export function cacheTone(
  cacheRate: number | null | undefined,
  thresholds?: StudioThresholds | null,
): StudioTone {
  if (cacheRate == null || Number.isNaN(Number(cacheRate))) return 'unknown'
  const critical = pickThreshold(thresholds?.critical_cache_rate, STUDIO_DEFAULT_THRESHOLDS.critical_cache_rate)
  const warning = pickThreshold(thresholds?.warning_cache_rate, STUDIO_DEFAULT_THRESHOLDS.warning_cache_rate)
  if (cacheRate < critical) return 'critical'
  if (cacheRate < warning) return 'warning'
  return 'healthy'
}

export function worstTone(tones: StudioTone[]): StudioTone {
  if (tones.includes('critical')) return 'critical'
  if (tones.includes('warning')) return 'warning'
  if (tones.includes('healthy')) return 'healthy'
  return 'unknown'
}

function hasStudioCacheSignal(metrics?: MonitorMetric): boolean {
  return (metrics?.cache_rate_denominator || 0) > 0 || (metrics?.cache_rate || 0) > 0
}

/** Prefer API health when the backend already applied thresholds; otherwise compare metrics to V2 settings. */
export function overallToneFromRow(
  metrics: MonitorMetric | undefined,
  health?: MonitorHealth,
  thresholds?: StudioThresholds | null,
): StudioTone {
  if (!metrics || !studioMetricsHaveActivity(metrics, health)) return 'unknown'
  const observed = errorRateTone(displayedErrorRate(metrics), thresholds)
  const api = healthTone(health?.overall || health?.error_rate)
  if (api !== 'unknown') return worstTone([api, observed])
  const tones: StudioTone[] = [
    observed,
    errorRateTone(metrics.error_rate, thresholds),
    ttftTone(metrics.ttft?.p50_ms, thresholds, metrics.ttft),
  ]
  if (hasStudioCacheSignal(metrics)) tones.push(cacheTone(metrics.cache_rate, thresholds))
  return worstTone(tones)
}

export function metricTextClass(tone: StudioTone | undefined, missing = false): string {
  if (missing || !tone || tone === 'unknown') return 'text-gray-500 dark:text-dark-400'
  if (tone === 'healthy') return 'text-emerald-600 dark:text-emerald-400'
  if (tone === 'warning') return 'text-amber-600 dark:text-amber-400'
  return 'text-red-600 dark:text-red-400'
}

export function studioPlatformLabel(platform: string): string {
  return GROUP_PLATFORM_OPTIONS.find((item) => item.value === platform)?.label || platform
}

/** OpenAI → Anthropic → Grok, then the rest of the group-platform catalog. */
const STUDIO_BRAND_HEAD = ['openai', 'anthropic', 'grok'] as const

export const STUDIO_BRAND_ORDER: string[] = [
  ...STUDIO_BRAND_HEAD,
  ...GROUP_PLATFORM_OPTIONS.map((item) => item.value).filter(
    (value) => !STUDIO_BRAND_HEAD.includes(value as (typeof STUDIO_BRAND_HEAD)[number]),
  ),
]

export function studioBrandOrderIndex(platform: string | null | undefined): number {
  const key = String(platform || '')
  const head = STUDIO_BRAND_HEAD.indexOf(key as (typeof STUDIO_BRAND_HEAD)[number])
  if (head >= 0) return head
  const rest = GROUP_PLATFORM_OPTIONS.findIndex((item) => item.value === key)
  return rest >= 0 ? STUDIO_BRAND_HEAD.length + rest : Number.MAX_SAFE_INTEGER
}

export type StudioBrandSection<T extends { platform?: string | null; brandLabel?: string }> = {
  key: string
  platform?: T['platform']
  brandLabel: string
  cards: T[]
}

export const STUDIO_ACTIVE_GROUP_LIMIT = 4
const STUDIO_ACTIVE_RECENT_SLOTS = 2

export type StudioActivitySource = {
  metrics?: MonitorMetric | null
  health?: MonitorHealth | null
  buckets?: Array<{ metrics?: MonitorMetric | null; health?: MonitorHealth | null }> | null
}

/** Higher = currently busier. 0 means idle and should stay out of 活跃分组. */
export function studioGroupActivityScore(card: StudioActivitySource): number {
  const buckets = card.buckets || []
  const recent = buckets.slice(-STUDIO_ACTIVE_RECENT_SLOTS)
  const recentLive = recent.filter((bucket) => studioMetricsHaveActivity(bucket.metrics, bucket.health))
  const rowLive = studioMetricsHaveActivity(card.metrics, card.health)
  if (!recentLive.length && !rowLive) return 0

  const latestLive = recentLive[recentLive.length - 1]
  const currentRpm = Number(latestLive?.metrics?.rpm || card.metrics?.rpm) || 0
  const rpm = Number(card.metrics?.rpm) || 0
  const tpm = Number(card.metrics?.tpm) || 0
  const requests = Number(card.metrics?.request_count) || 0
  const liveCount = buckets.filter((bucket) => studioMetricsHaveActivity(bucket.metrics, bucket.health)).length
  return (
    currentRpm * 1_000_000 +
    rpm * 10_000 +
    tpm * 10 +
    requests +
    recentLive.length * 1_000 +
    liveCount * 10 +
    (rowLive ? 1 : 0)
  )
}

/** Top currently-busy group cards, newest/highest RPM first. Idle rows are dropped. */
export function pickStudioActiveGroups<T extends StudioActivitySource>(
  cards: T[],
  limit = STUDIO_ACTIVE_GROUP_LIMIT,
): T[] {
  const cap = Number.isFinite(limit) ? Math.max(0, Math.floor(limit)) : STUDIO_ACTIVE_GROUP_LIMIT
  return cards
    .map((card, index) => ({ card, index, score: studioGroupActivityScore(card) }))
    .filter((item) => item.score > 0)
    .sort((a, b) => b.score - a.score || a.index - b.index)
    .slice(0, cap)
    .map((item) => item.card)
}

const STUDIO_STATUS_RANK: Record<StudioTone, number> = {
  healthy: 0,
  warning: 1,
  critical: 2,
  unknown: 3,
}

export function studioStatusSortRank(state?: StudioTone | string | null): number {
  if (state && state in STUDIO_STATUS_RANK) return STUDIO_STATUS_RANK[state as StudioTone]
  return 4
}

/** 正常 → 降级 → 失败 → 样本不足; same-status cards keep their original order. */
export function sortStudioCardsByStatus<T extends { state?: StudioTone | string | null }>(cards: T[]): T[] {
  return cards
    .map((card, index) => ({ card, index }))
    .sort((a, b) => studioStatusSortRank(a.card.state) - studioStatusSortRank(b.card.state) || a.index - b.index)
    .map((item) => item.card)
}

/** Cluster group cards by model brand. Brand sections follow STUDIO_BRAND_ORDER. */
export function groupStudioCardsByBrand<T extends { platform?: string | null; brandLabel?: string; state?: StudioTone | string | null }>(
  cards: T[],
): StudioBrandSection<T>[] {
  const sections: StudioBrandSection<T>[] = []
  const indexByKey = new Map<string, number>()
  for (const card of cards) {
    const key = String(card.platform || card.brandLabel || 'unknown')
    const existing = indexByKey.get(key)
    if (existing == null) {
      indexByKey.set(key, sections.length)
      sections.push({
        key,
        platform: card.platform,
        brandLabel: card.brandLabel || studioPlatformLabel(key),
        cards: [card],
      })
      continue
    }
    sections[existing].cards.push(card)
  }
  return sections
    .map((section) => ({ ...section, cards: sortStudioCardsByStatus(section.cards) }))
    .sort((a, b) => studioBrandOrderIndex(a.key) - studioBrandOrderIndex(b.key))
}

export function studioPlatform(platform: string): GroupPlatform | undefined {
  return GROUP_PLATFORM_OPTIONS.some((item) => item.value === platform)
    ? (platform as GroupPlatform)
    : undefined
}

export function scoreStroke(score: number | null | undefined): string {
  const tone = scoreTone(score)
  if (tone === 'healthy') return '#14b8a6'
  if (tone === 'warning') return '#f59e0b'
  if (tone === 'critical') return '#f43f5e'
  return '#9ca3af'
}

export type StudioFaceMood = 'joy' | 'happy' | 'smile' | 'neutral' | 'worried' | 'sad' | 'blank'

export type StudioFacePalette = {
  skin: string
  blush: string
  ink: string
}

const FACE_TONE_FALLBACK: Record<StudioTone, StudioFacePalette> = {
  healthy: { skin: '#6EE7B7', blush: '#34D399', ink: '#065F46' },
  warning: { skin: '#FDBA74', blush: '#FB923C', ink: '#9A3412' },
  critical: { skin: '#FCA5A5', blush: '#F87171', ink: '#7F1D1D' },
  unknown: { skin: '#E5E7EB', blush: '#D1D5DB', ink: '#4B5563' },
}

const FACE_TONE_RANGE: Record<StudioTone, { light: StudioFacePalette; dark: StudioFacePalette }> = {
  healthy: {
    light: { skin: '#D1FAE5', blush: '#6EE7B7', ink: '#047857' },
    dark: { skin: '#10B981', blush: '#34D399', ink: '#065F46' },
  },
  warning: {
    light: { skin: '#FFEDD5', blush: '#FDBA74', ink: '#C2410C' },
    dark: { skin: '#F97316', blush: '#FB923C', ink: '#9A3412' },
  },
  critical: {
    light: { skin: '#FEE2E2', blush: '#FCA5A5', ink: '#B91C1C' },
    dark: { skin: '#F87171', blush: '#EF4444', ink: '#991B1B' },
  },
  unknown: {
    light: FACE_TONE_FALLBACK.unknown,
    dark: FACE_TONE_FALLBACK.unknown,
  },
}

/** Map a score onto 0–1 within its status band so higher data reads clearly darker. */
export function studioFaceDepth(tone: StudioTone, score: number): number {
  if (tone === 'healthy') return clamp01((score - 80) / 20)
  if (tone === 'warning') return clamp01((score - 50) / 30)
  if (tone === 'critical') return clamp01(score / 50)
  return 0
}

function parseHex(color: string): [number, number, number] {
  const hex = color.replace('#', '')
  return [parseInt(hex.slice(0, 2), 16), parseInt(hex.slice(2, 4), 16), parseInt(hex.slice(4, 6), 16)]
}

function toHex(value: number): string {
  return Math.max(0, Math.min(255, Math.round(value))).toString(16).padStart(2, '0').toUpperCase()
}

/** Mix two #RRGGBB colors. t=0 keeps `from`, t=1 is `to`. */
export function mixHexColor(from: string, to: string, t: number): string {
  const u = clamp01(t)
  const a = parseHex(from)
  const b = parseHex(to)
  return `#${toHex(a[0] + (b[0] - a[0]) * u)}${toHex(a[1] + (b[1] - a[1]) * u)}${toHex(a[2] + (b[2] - a[2]) * u)}`
}

export function hexLuminance(color: string): number {
  const [r, g, b] = parseHex(color)
  return (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255
}

/** Higher score → darker skin within the status hue. Missing scores keep the light fallback. */
export function studioFacePalette(tone: StudioTone = 'unknown', score?: number | null): StudioFacePalette {
  if (tone === 'unknown' || score == null || Number.isNaN(Number(score))) {
    return FACE_TONE_FALLBACK[tone] || FACE_TONE_FALLBACK.unknown
  }
  const range = FACE_TONE_RANGE[tone] || FACE_TONE_RANGE.unknown
  const t = 0.04 + 0.96 * studioFaceDepth(tone, Number(score))
  return {
    skin: mixHexColor(range.light.skin, range.dark.skin, t),
    blush: mixHexColor(range.light.blush, range.dark.blush, t),
    ink: mixHexColor(range.light.ink, range.dark.ink, t),
  }
}

/** 0–1 mix toward the dark accent; higher values render darker chart dots. */
export function studioValueShade(value: number | null, min: number, max: number): number {
  if (value == null || !Number.isFinite(value)) return 0.22
  const span = max - min || 1
  return 0.12 + 0.5 * clamp01((value - min) / span)
}

/** Numeric part of the group multiplier, e.g. 0.08 */
export function formatStudioRateNumber(value: number | string | null | undefined): string {
  if (value == null || value === '') return ''
  const numeric = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(numeric)) return ''
  const abs = Math.abs(numeric)
  const digits = abs >= 100 ? 0 : abs >= 10 ? 2 : 4
  return `${Number(numeric.toFixed(digits))}`
}

/** Group billing multiplier next to the brand pill, e.g. 用户倍率0.08x */
export function formatStudioMultiplier(
  value: number | string | null | undefined,
  pattern = '用户倍率{n}x',
): string {
  const n = formatStudioRateNumber(value)
  if (!n) return ''
  return pattern.includes('{n}') ? pattern.replace(/\{n\}/g, n) : pattern
}

export type StudioGroupRateCatalog = {
  id?: number | string | null
  name?: string | null
  platform?: string | null
  rate_multiplier?: number | string | null
}

export type StudioGroupRateIndex = {
  byId: Map<number, number>
  byName: Map<string, number>
}

export function asStudioGroupCatalog(value: unknown): StudioGroupRateCatalog[] {
  if (Array.isArray(value)) return value
  if (value && typeof value === 'object') {
    const items = (value as { items?: unknown }).items
    if (Array.isArray(items)) return items
    const data = (value as { data?: unknown }).data
    if (Array.isArray(data)) return data
  }
  return []
}

function readStudioRate(value: unknown): number | undefined {
  const numeric = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(numeric) ? numeric : undefined
}

/** Index group rates by id and name; custom `/groups/rates` overrides win. */
export function collectStudioGroupRates(
  catalog: Iterable<StudioGroupRateCatalog> | StudioGroupRateCatalog[] | null | undefined,
  custom?: Record<string, number | string> | Record<number, number | string> | null,
): StudioGroupRateIndex {
  const byId = new Map<number, number>()
  const byName = new Map<string, number>()
  const list = Array.isArray(catalog)
    ? catalog
    : catalog && typeof catalog === 'object' && Symbol.iterator in Object(catalog)
      ? [...catalog]
      : []
  for (const group of list) {
    const id = Number(group?.id)
    const rate = readStudioRate(group?.rate_multiplier)
    if (Number.isFinite(id) && id > 0 && rate != null && !byId.has(id)) byId.set(id, rate)
    const name = String(group?.name || '').trim()
    if (name && rate != null && !byName.has(name)) byName.set(name, rate)
  }
  for (const [id, rate] of Object.entries(custom || {})) {
    const groupId = Number(id)
    const value = readStudioRate(rate)
    if (Number.isFinite(groupId) && groupId > 0 && value != null) byId.set(groupId, value)
  }
  return { byId, byName }
}

export function lookupStudioGroupRate(
  rates: StudioGroupRateIndex | null | undefined,
  groupId?: number | string | null,
  groupName?: string | null,
  fallback?: number | string | null,
): number | undefined {
  const id = groupId == null || groupId === '' ? NaN : Number(groupId)
  if (Number.isFinite(id) && rates?.byId.has(id)) return rates.byId.get(id)
  const name = String(groupName || '').trim()
  if (name && rates?.byName.has(name)) return rates.byName.get(name)
  return readStudioRate(fallback)
}

export function emptyStudioMetrics(): MonitorMetric {
  return {
    success_requests: 0,
    error_requests: 0,
    request_count: 0,
    token_count: 0,
    rpm: 0,
    tpm: 0,
    error_rate: 0,
    success_rate: 0,
    cache_rate: 0,
    cache_rate_numerator: 0,
    cache_rate_denominator: 0,
    ttft: { sample_count: 0, p50_ms: null, p95_ms: null, avg_ms: null },
    duration: { sample_count: 0, p50_ms: null, p95_ms: null, avg_ms: null },
  }
}

/** Prefer API success_rate so user-redacted counts still show a real percentage. */
export function formatStudioSuccessRate(
  metrics?: MonitorMetric | null,
  health?: MonitorHealth | null,
): string {
  if (!metrics) return '-'
  if (metrics.success_rate != null && Number.isFinite(metrics.success_rate)) {
    if (!studioMetricsHaveActivity(metrics, health) && metrics.success_rate <= 0) return '-'
    return formatMonitorPercent(metrics.success_rate)
  }
  if ((metrics.request_count || 0) > 0) {
    return formatMonitorSuccessRate(metrics.success_requests, metrics.request_count)
  }
  if (studioMetricsHaveActivity(metrics, health)) {
    return formatMonitorSuccessRateFromError(metrics.error_rate)
  }
  return '-'
}

/** True failure share, including ignored categories. */
export function displayedErrorRate(metrics?: MonitorMetric | null): number | null {
  if (!metrics) return null
  if (metrics.success_rate != null && Number.isFinite(metrics.success_rate)) {
    return Math.max(0, Math.min(1, 1 - metrics.success_rate))
  }
  if ((metrics.request_count || 0) > 0) {
    return (metrics.error_requests || 0) / metrics.request_count
  }
  if (metrics.error_rate != null && Number.isFinite(metrics.error_rate)) return metrics.error_rate
  return null
}

/**
 * Displayed error share is 1 − success_rate (true failures, including ignored
 * categories). Faces and card status use this so a 100% fail bucket cannot stay green.
 */
export function formatStudioErrorRate(
  metrics?: MonitorMetric | null,
  health?: MonitorHealth | null,
): string {
  if (!metrics || !studioMetricsHaveActivity(metrics, health)) return '-'
  const rate = displayedErrorRate(metrics)
  if (rate == null) return '-'
  return formatMonitorPercent(rate)
}

export function formatStudioCacheRate(
  metrics?: MonitorMetric | null,
  health?: MonitorHealth | null,
): string {
  if (!metrics) return '-'
  if ((metrics.cache_rate_denominator || 0) > 0 || studioMetricsHaveActivity(metrics, health)) {
    return formatMonitorPercent(metrics.cache_rate || 0)
  }
  return '-'
}

export function emptyStudioHealth(): MonitorHealth {
  return {
    overall: 'unknown',
    error_rate: 'unknown',
    ttft: 'unknown',
    score: null,
    minimum_sample: 0,
  }
}

function emptyStudioGroupRow(
  groupId: number,
  catalog?: StudioGroupRateCatalog | null,
): MonitorMatrixRow {
  const name = String(catalog?.name || '').trim()
  return {
    platform: String(catalog?.platform || '').trim() || 'unknown',
    group_id: groupId,
    group_name: name || `#${groupId}`,
    metrics: emptyStudioMetrics(),
    health: emptyStudioHealth(),
    buckets: [],
  }
}

function studioGroupId(row: MonitorMatrixRow): number {
  return Number(row.group_id)
}

/** Keep every selected monitor group on 渠道状态, even when the matrix omitted idle rows. */
export function mergeSelectedStudioGroups(
  items: MonitorMatrixRow[] | null | undefined,
  selectedIds: Array<number | string> | null | undefined,
  catalog: Iterable<StudioGroupRateCatalog> | StudioGroupRateCatalog[] | null | undefined,
): MonitorMatrixRow[] {
  const rows = (items || []).filter((row) => Number.isFinite(studioGroupId(row)) && studioGroupId(row) > 0)
  const selected = [...new Set((selectedIds || []).map((id) => Number(id)).filter((id) => Number.isFinite(id) && id > 0))]
  if (!selected.length) {
    return [...rows].sort((a, b) => (b.metrics?.rpm || 0) - (a.metrics?.rpm || 0))
  }

  const catalogById = new Map<number, StudioGroupRateCatalog>()
  const list = Array.isArray(catalog)
    ? catalog
    : catalog && typeof catalog === 'object' && Symbol.iterator in Object(catalog)
      ? [...catalog]
      : []
  for (const group of list) {
    const id = Number(group?.id)
    if (Number.isFinite(id) && id > 0 && !catalogById.has(id)) catalogById.set(id, group)
  }

  const rowsByGroupId = new Map<number, MonitorMatrixRow[]>()
  for (const row of rows) {
    const id = studioGroupId(row)
    const current = rowsByGroupId.get(id)
    if (current) current.push(row)
    else rowsByGroupId.set(id, [row])
  }

  const merged: MonitorMatrixRow[] = []
  const seen = new Set<number>()
  for (const id of selected) {
    seen.add(id)
    const existing = rowsByGroupId.get(id)
    if (existing?.length) merged.push(...existing)
    else merged.push(emptyStudioGroupRow(id, catalogById.get(id)))
  }
  for (const [id, extra] of rowsByGroupId) {
    if (!seen.has(id)) merged.push(...extra)
  }
  return merged
}

/** Shrink the gap only when a short card cannot already show `visibleDots` at `minGap`. */
export function studioChartGap(
  containerWidth: number,
  minGap = 40,
  padX = 10,
  visibleDots = 5,
): number {
  const inner = Math.max(0, containerWidth - padX * 2)
  if (visibleDots <= 1) return minGap
  const visibleWithMin = minGap > 0 ? inner / minGap + 1 : visibleDots
  if (visibleWithMin + 0.05 >= visibleDots) return minGap
  return Math.max(8, inner / (visibleDots - 1))
}

/** Track width so dots keep at least `minGap` px and still fill a short container. */
export function studioChartTrackWidth(
  count: number,
  containerWidth: number,
  minGap = 40,
  padX = 10,
): number {
  if (count <= 1) return Math.max(containerWidth, padX * 2)
  const packed = padX * 2 + (count - 1) * minGap
  return Math.max(containerWidth, packed)
}

export function statusFace(
  score: number | null | undefined,
  state?: HealthState | StudioTone,
): {
  mood: StudioFaceMood
  tone: StudioTone
} {
  const tone = healthTone(state) !== 'unknown' ? healthTone(state) : scoreTone(score)
  if (tone === 'unknown') return { mood: 'blank', tone }
  if (tone === 'critical') return { mood: 'sad', tone }
  if (tone === 'warning') {
    if (score != null && Number.isFinite(score) && score >= 50) return { mood: 'neutral', tone }
    return { mood: 'worried', tone }
  }
  if (score != null && Number.isFinite(score) && score >= 90) return { mood: 'joy', tone }
  if (score != null && Number.isFinite(score) && score >= 80) return { mood: 'happy', tone }
  return { mood: 'smile', tone }
}

export function rowHealthScore(health: MonitorHealth, mode: HealthDisplayMode): number | null {
  return healthModeScore(health, mode)
}
