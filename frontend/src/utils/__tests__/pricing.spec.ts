import { describe, expect, it } from 'vitest'
import { formatDecimal, formatScaled } from '../pricing'

describe('decimal pricing helpers', () => {
  it('scales and multiplies prices without native floating-point artifacts', () => {
    expect(formatScaled('0.000000000123456789', 1_000_000, 2, '0.123456789'))
      .toBe('$0.000015241578750190521')
  })

  it('keeps every meaningful decimal while padding short values', () => {
    expect(formatDecimal('0.000000000001', 2)).toBe('0.000000000001')
    expect(formatDecimal('5', 2)).toBe('5.00')
    expect(formatDecimal('1234567.890123456789', 2, true)).toBe('1,234,567.890123456789')
  })
})
