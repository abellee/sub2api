import { describe, expect, it } from 'vitest'
import { inflationAggregate, inflationCandles, inflationTone, type InflationSample, type InflationHistory } from '../inflationMonitor'
const sample = (rate: number | null, error?: string) => ({ rate, error } as InflationSample)
describe('inflation monitor', () => {
  it('excludes failures and supports mean and odd/even median', () => {
    const items = [sample(0), sample(20), sample(100), sample(null), sample(1000, 'failed')]
    expect(inflationAggregate(items, 'mean')).toBe(40)
    expect(inflationAggregate(items, 'median')).toBe(20)
    expect(inflationAggregate(items.slice(0, 2), 'median')).toBe(10)
    expect(inflationAggregate(items.slice(3), 'mean')).toBeNull()
  })
  it('uses group thresholds with inclusive yellow and red boundaries', () => {
    expect(inflationTone(null, 10, 30)).toBe('unknown')
    expect(inflationTone(-5, 10, 30)).toBe('healthy')
    expect(inflationTone(10, 10, 30)).toBe('warning')
    expect(inflationTone(30, 10, 30)).toBe('critical')
  })
  it('builds candles from consecutive statistics and sample extrema, leaving failed rounds empty', () => {
    const history = { aggregation: 'mean', results: [
      { samples: [sample(0), sample(20)] }, { samples: [sample(null)] }, { samples: [sample(30), sample(90)] },
    ] } as InflationHistory
    expect(inflationCandles(history)).toEqual([
      { open: 10, close: 10, low: 0, high: 20 }, null, { open: 10, close: 60, low: 10, high: 90 },
    ])
  })
})
