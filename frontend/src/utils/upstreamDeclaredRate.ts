import type { UpstreamBillingProbeSnapshot } from '@/types'

function parseMinute(value?: string): number | null {
  if (typeof value !== 'string') return null
  const match = /^(\d{2}):(\d{2})$/.exec(value)
  if (!match) return null
  const hour = Number(match[1])
  const minute = Number(match[2])
  return hour < 24 && minute < 60 ? hour * 60 + minute : null
}

function minuteInTimeZone(timestamp: number, timeZone?: string): number | null {
  if (!timeZone) return null
  try {
    const parts = new Intl.DateTimeFormat('en-GB', {
      timeZone,
      hour: '2-digit',
      minute: '2-digit',
      hourCycle: 'h23'
    }).formatToParts(new Date(timestamp))
    const hour = Number(parts.find(part => part.type === 'hour')?.value)
    const minute = Number(parts.find(part => part.type === 'minute')?.value)
    return Number.isInteger(hour) && Number.isInteger(minute) ? hour * 60 + minute : null
  } catch {
    return null
  }
}

/** 与账户页「上游声明倍率」相同：resolved × 当前峰时系数。 */
export function currentUpstreamDeclaredRate(
  snapshot?: UpstreamBillingProbeSnapshot | null,
  now = Date.now()
): number | null {
  const billing = snapshot?.data
  if (!billing) return null
  if (billing.billing_scope !== 'token') return null
  const base = billing.resolved_rate_multiplier
  if (typeof base !== 'number' || !Number.isFinite(base) || base < 0) return null
  if (typeof billing.peak_rate_enabled !== 'boolean') return null
  if (!billing.peak_rate_enabled) return base
  const start = parseMinute(billing.peak_start)
  const end = parseMinute(billing.peak_end)
  const minute = minuteInTimeZone(now, billing.timezone)
  const peak = billing.peak_rate_multiplier
  if (
    start == null ||
    end == null ||
    minute == null ||
    start >= end ||
    typeof peak !== 'number' ||
    !Number.isFinite(peak) ||
    peak < 0
  ) {
    return null
  }
  const value = minute >= start && minute < end ? base * peak : base
  return Number.isFinite(value) ? value : null
}

export function lastDetectedUpstreamRate(
  snapshot?: UpstreamBillingProbeSnapshot | null
): number | null {
  const value = snapshot?.data?.effective_rate_multiplier
  return typeof value === 'number' && Number.isFinite(value) && value >= 0
    ? Number(value.toPrecision(12))
    : null
}

/** 悬停展示：优先当前有效声明倍率，否则退回上次探测到的 effective 值。 */
export function resolveUpstreamDeclaredRate(
  snapshot?: UpstreamBillingProbeSnapshot | null,
  now = Date.now()
): number | null {
  return currentUpstreamDeclaredRate(snapshot, now) ?? lastDetectedUpstreamRate(snapshot)
}

export function upstreamDeclaredRateFromExtra(
  extra?: { upstream_billing_probe?: UpstreamBillingProbeSnapshot } | null,
  now = Date.now()
): number | null {
  return resolveUpstreamDeclaredRate(extra?.upstream_billing_probe, now)
}
