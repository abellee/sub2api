import type { LatencyMetric, MonitorHealth, MonitorMetric } from '@/api/channelMonitorV2'
import {
  formatLatencyPrivacy,
  formatMonitorThroughput,
  formatMonitorTokensPerSecond,
  healthModeScore,
} from '@/features/channel-monitor-v2/monitorFormat'
import { formatStudioCacheRate, formatStudioErrorRate, formatStudioSuccessRate } from './studioFormat'

type Translate = (key: string, params?: Record<string, unknown>) => string

const axisFormats = new Map<string, Intl.DateTimeFormat>()
const hourFormats = new Map<string, Intl.DateTimeFormat>()

function cachedFormat(
  store: Map<string, Intl.DateTimeFormat>,
  locale: string | undefined,
  options: Intl.DateTimeFormatOptions,
) {
  const key = locale || ''
  let fmt = store.get(key)
  if (!fmt) {
    fmt = new Intl.DateTimeFormat(locale || undefined, options)
    store.set(key, fmt)
  }
  return fmt
}

function formatAxisTime(value: string, locale?: string) {
  return cachedFormat(axisFormats, locale, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

export function formatBucketRange(start: string, bucketSeconds: number, locale?: string): string {
  const from = new Date(start)
  const to = new Date(from.getTime() + Math.max(60, bucketSeconds) * 1000)
  return `${formatAxisTime(from.toISOString(), locale)} - ${cachedFormat(hourFormats, locale, {
    hour: '2-digit',
    minute: '2-digit',
  }).format(to)}`
}

export function formatSlotTime(start: string, locale?: string): string {
  return cachedFormat(hourFormats, locale, {
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(start))
}

function successRate(metrics: MonitorMetric, health: MonitorHealth, showThroughput: boolean): string {
  const noCount = (metrics.request_count || 0) <= 0 && (metrics.success_requests || 0) <= 0
  const noTP = (metrics.rpm || 0) <= 0 && (metrics.tpm || 0) <= 0
  if (noCount && noTP && showThroughput && metrics.success_rate == null) return '-'
  return formatStudioSuccessRate(metrics, health)
}

function formatScore(health: MonitorHealth): string {
  const score = healthModeScore(health, 'overall')
  if (score == null) return '—'
  return `${Math.round(score)}`
}

function latencyPrivacy(metric: LatencyMetric) {
  return formatLatencyPrivacy(metric.p50_ms, metric.p90_ms, metric.avg_ms, metric.p95_ms)
}

export function emptyTooltipLines(start: string, bucketSeconds: number, t: Translate, locale?: string): string[] {
  return [formatBucketRange(start, bucketSeconds, locale), t('channelMonitorV2.matrix.noTraffic')]
}

export function bucketTooltipLines(
  bucket: { bucket_start: string; metrics: MonitorMetric; health: MonitorHealth },
  t: Translate,
  options: { bucketSeconds: number; showThroughput: boolean; locale?: string },
): string[] {
  const metrics = bucket.metrics
  const lines = [
    formatBucketRange(bucket.bucket_start, options.bucketSeconds, options.locale),
    t('channelMonitorV2.matrix.scoreLine', { score: formatScore(bucket.health) }),
    t('channelMonitorV2.metrics.successRateValue', { value: successRate(metrics, bucket.health, options.showThroughput) }),
    t('channelMonitorV2.metrics.ttftValue', { value: latencyPrivacy(metrics.ttft) }),
  ]
  if (options.showThroughput) {
    lines.push(t('channelMonitorV2.metrics.tpsValue', { value: formatMonitorTokensPerSecond(metrics.tpm) }))
  }
  lines.push(
    t('channelMonitorV2.metrics.cacheRateValue', { value: formatStudioCacheRate(metrics, bucket.health) }),
    t('channelMonitorV2.metrics.errorRateValue', { value: formatStudioErrorRate(metrics, bucket.health) }),
  )
  if (options.showThroughput) {
    lines.push(t('channelMonitorV2.metrics.rpmValue', { value: formatMonitorThroughput(metrics.rpm) }))
  }
  lines.push(t('channelMonitorV2.metrics.durationValue', { value: latencyPrivacy(metrics.duration) }))
  return lines
}
