import { describe, expect, it } from 'vitest'
import { addCalendarDays, beijingInputToISO, formatBeijingDateTime, isoToBeijingInput } from '../beijingTime'

describe('beijingTime', () => {
  it('shows 18:30 UTC as 02:30 the next Beijing day', () => {
    expect(formatBeijingDateTime('2026-10-07T18:30:00Z')).toBe('2026/10/08 02:30:00')
  })

  it('treats a datetime-local value as Beijing time', () => {
    expect(beijingInputToISO('2026-10-08T18:30')).toBe('2026-10-08T10:30:00.000Z')
    expect(isoToBeijingInput('2026-10-08T10:30:00.000Z')).toBe('2026-10-08T18:30')
  })

  it('adds calendar days without shifting the date', () => {
    expect(addCalendarDays('2026-10-08', 6)).toBe('2026-10-14')
  })
})
