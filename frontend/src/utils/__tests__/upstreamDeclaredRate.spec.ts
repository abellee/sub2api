import { describe, expect, it } from 'vitest'
import {
  currentUpstreamDeclaredRate,
  resolveUpstreamDeclaredRate,
  upstreamDeclaredRateFromExtra,
} from '../upstreamDeclaredRate'
import type { UpstreamBillingData, UpstreamBillingProbeSnapshot } from '@/types'

const billingData = (overrides: Partial<UpstreamBillingData> = {}): UpstreamBillingData => ({
  object: 'sub2api.key_billing',
  schema_version: 1,
  billing_scope: 'token',
  group_rate_multiplier: 0.8,
  resolved_rate_multiplier: 0.065,
  peak_rate_enabled: false,
  effective_rate_multiplier: 0.065,
  observed_at: '2026-07-13T00:00:00Z',
  ...overrides,
})

const snapshot = (data?: UpstreamBillingData): UpstreamBillingProbeSnapshot => ({
  status: 'ok',
  data,
  last_attempt_at: '2026-07-13T00:00:00Z',
  next_probe_at: '2026-07-13T00:30:00Z',
})

describe('upstreamDeclaredRate', () => {
  it('uses resolved rate when peak is off', () => {
    expect(currentUpstreamDeclaredRate(snapshot(billingData()))).toBe(0.065)
  })

  it('applies peak multiplier inside the peak window', () => {
    const peak = snapshot(billingData({
      peak_rate_enabled: true,
      peak_start: '09:00',
      peak_end: '18:00',
      peak_rate_multiplier: 1.5,
      timezone: 'Asia/Shanghai',
      effective_rate_multiplier: 0.0975,
    }))
    const duringPeak = Date.parse('2026-07-13T04:00:00Z') // 12:00 CST
    expect(currentUpstreamDeclaredRate(peak, duringPeak)).toBeCloseTo(0.0975, 10)
  })

  it('falls back to last detected effective rate when current cannot be computed', () => {
    const stalePeak = snapshot(billingData({
      peak_rate_enabled: true,
      peak_start: '09:00',
      peak_end: '18:00',
      timezone: 'Invalid/Zone',
      effective_rate_multiplier: 0.09,
    }))
    expect(currentUpstreamDeclaredRate(stalePeak)).toBeNull()
    expect(resolveUpstreamDeclaredRate(stalePeak)).toBe(0.09)
  })

  it('reads the probe snapshot from account extra', () => {
    expect(upstreamDeclaredRateFromExtra({
      upstream_billing_probe: snapshot(billingData()),
    })).toBe(0.065)
    expect(upstreamDeclaredRateFromExtra({})).toBeNull()
  })
})
