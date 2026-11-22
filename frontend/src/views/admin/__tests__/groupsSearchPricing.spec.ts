import { describe, expect, it } from 'vitest'
import { searchPricePerCall, searchPricePer1k, searchPricePreview } from '../groupsSearchPricing'

describe('Google Search group pricing', () => {
  it('round trips per-call dollars and previews the 1.75 group rate', () => {
    expect(searchPricePer1k('0.019600')).toBe(19.6)
    expect(searchPricePerCall(19.6)).toBe('0.019600')
    expect(searchPricePreview(19.6, 1.75)).toBe('$0.034300')
  })
  it('preserves explicit free prices and clears an omitted price', () => {
    expect(searchPricePer1k('0')).toBe(0)
    expect(searchPricePerCall(0)).toBe('0.000000')
    expect(searchPricePer1k('')).toBe(null)
    expect(searchPricePerCall(null)).toBe('')
    expect(searchPricePreview('', 1.75)).toBe('—')
  })
})
