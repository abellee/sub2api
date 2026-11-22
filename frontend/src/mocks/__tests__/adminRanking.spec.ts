import { describe, expect, it } from 'vitest'
import { getMockUserBreakdown } from '../adminRanking'

describe('admin ranking mock data', () => {
  it('scales values with the selected time range and sorts by the selected metric', () => {
    const shortRange = getMockUserBreakdown({
      start_date: '2026-08-24',
      end_date: '2026-08-24',
      sort_by: 'total_tokens',
      limit: 0,
    })
    const longRange = getMockUserBreakdown({
      start_date: '2026-08-18',
      end_date: '2026-08-24',
      sort_by: 'actual_cost',
      limit: 0,
    })

    expect(longRange.users[0].actual_cost).toBeGreaterThan(shortRange.users[0].actual_cost)
    expect(longRange.users[0].actual_cost).toBeGreaterThanOrEqual(longRange.users[1].actual_cost)
    expect(longRange.users).toHaveLength(5)
  })
})
