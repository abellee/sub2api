import { describe, expect, it } from 'vitest'
import { clampTooltipBox, eventTooltipPoint, STUDIO_TOOLTIP_PAD } from '../studioTooltipUi'

describe('studioTooltipUi', () => {
  it('places the panel above the cursor and keeps it inside the window', () => {
    const box = clampTooltipBox({ x: 40, y: 40 }, 200, 80, STUDIO_TOOLTIP_PAD)
    expect(box.x).toBeGreaterThanOrEqual(STUDIO_TOOLTIP_PAD)
    expect(box.y).toBeGreaterThanOrEqual(STUDIO_TOOLTIP_PAD)
    expect(box.x + 200).toBeLessThanOrEqual(window.innerWidth - STUDIO_TOOLTIP_PAD)
    expect(box.y + 80).toBeLessThanOrEqual(window.innerHeight - STUDIO_TOOLTIP_PAD)
  })

  it('shifts left when the cursor is at the right edge', () => {
    const width = 220
    const box = clampTooltipBox({ x: window.innerWidth - 8, y: 200 }, width, 72)
    expect(box.x + width).toBeLessThanOrEqual(window.innerWidth - STUDIO_TOOLTIP_PAD)
    expect(box.x).toBeGreaterThanOrEqual(STUDIO_TOOLTIP_PAD)
  })

  it('reads client coordinates from pointer events', () => {
    const event = new MouseEvent('mouseenter', { clientX: 120, clientY: 80 })
    expect(eventTooltipPoint(event)).toEqual({ x: 120, y: 80 })
  })
})
