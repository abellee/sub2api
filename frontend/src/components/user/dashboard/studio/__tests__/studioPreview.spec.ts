import { describe, expect, it } from 'vitest'
import { createStudioPreview } from '../studioPreview'

describe('studio preview', () => {
  it('follows the selected range and granularity with consistent token totals', () => {
    const daily = createStudioPreview('2026-10-26', '2026-11-01', 'day')
    expect(daily.trend).toHaveLength(7)
    expect(daily.trend[0].date).toBe('2026-10-26')
    expect(daily.trend[6].date).toBe('2026-11-01')
    const hourly = createStudioPreview('2026-11-01', '2026-11-01', 'hour')
    expect(hourly.trend).toHaveLength(24)
    expect(hourly.trend[23].date).toBe('2026-11-01 23:00')
    for (const row of [...daily.trend, ...daily.models]) {
      expect(row.total_tokens).toBe(row.input_tokens + row.output_tokens + row.cache_creation_tokens + row.cache_read_tokens)
      expect(row.actual_cost).toBeLessThan(row.cost)
    }
    expect(createStudioPreview('2026-11-02', '2026-11-01', 'day').trend).toEqual([])
  })
})
