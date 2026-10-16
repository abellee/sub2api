import { describe, expect, it } from 'vitest'
import { studioFaceClipLeft, studioFaceUncovered } from '../studioFaceVisible'

describe('studioFaceVisible', () => {
  it('clips faces tucked under the overlapping KPI card', () => {
    const scroller = { left: 200, right: 800, top: 40, bottom: 180 }
    const card = { left: 16, right: 248, top: 40, bottom: 180 }
    expect(studioFaceClipLeft(scroller, card)).toBe(248)
    expect(studioFaceUncovered(240, 248)).toBe(false)
    expect(studioFaceUncovered(260, 248)).toBe(true)
  })

  it('does not treat a stacked mobile card as a horizontal clip', () => {
    const scroller = { left: 16, right: 400, top: 200, bottom: 340 }
    const card = { left: 16, right: 400, top: 40, bottom: 180 }
    expect(studioFaceClipLeft(scroller, card)).toBe(16)
    expect(studioFaceUncovered(80, 16)).toBe(true)
  })

  it('does not clip faces when the stacked card only overlaps the join strip', () => {
    const scroller = { left: 16, right: 400, top: 166, bottom: 318 }
    const card = { left: 16, right: 400, top: 40, bottom: 180 }
    expect(studioFaceClipLeft(scroller, card)).toBe(16)
    expect(studioFaceUncovered(80, 16)).toBe(true)
    expect(studioFaceUncovered(390, 16)).toBe(true)
  })
})
