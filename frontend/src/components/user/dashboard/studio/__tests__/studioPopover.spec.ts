import { describe, expect, it } from 'vitest'
import { positionStudioPopover } from '../studioPopover'

describe('positionStudioPopover', () => {
  const anchor = { left: 26, top: 900, right: 62, bottom: 934, width: 36, height: 34 }
  const viewport = { width: 1440, height: 1000 }
  it('aligns a collapsed popup with its trigger outside the sidebar', () => {
    expect(positionStudioPopover(anchor, { width: 280, height: 360 }, { left: 18, top: 850 }, viewport, 'right', 88)).toEqual({ left: 82, top: -276 })
  })
  it('places expanded tools outside the right edge of the sidebar', () => {
    expect(positionStudioPopover(anchor, { width: 280, height: 360 }, { left: 18, top: 850 }, viewport, 'right', 248)).toEqual({ left: 242, top: -276 })
  })
  it('clamps large popups to the viewport instead of crossing its top or right edges', () => {
    const point = positionStudioPopover({ ...anchor, left: 320, right: 356, top: 100, bottom: 134 }, { width: 280, height: 900 }, { left: 18, top: 650 }, { width: 390, height: 844 }, 'right', 256)
    expect(point).toEqual({ left: 80, top: -638 })
  })
})
