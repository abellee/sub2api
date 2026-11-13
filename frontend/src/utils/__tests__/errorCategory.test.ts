import { describe, expect, it } from 'vitest'
import { mapErrorCategory } from '../errorCategory'

describe('balance error category', () => {
  it('groups local balance shortage with quota and preserves upstream attribution', () => {
    expect(mapErrorCategory('request', 'insufficient_balance')).toBe('quota')
    expect(mapErrorCategory('upstream', 'insufficient_balance')).toBe('upstream')
  })
})
