import { describe, expect, it } from 'vitest'
import { applyShiftWheelPan, shiftWheelPanDelta } from '../shiftWheelPan'

function scroller(scrollWidth = 400, clientWidth = 120, scrollLeft = 0) {
  const el = document.createElement('div')
  Object.defineProperty(el, 'scrollWidth', { configurable: true, value: scrollWidth })
  Object.defineProperty(el, 'clientWidth', { configurable: true, value: clientWidth })
  el.scrollLeft = scrollLeft
  return el
}

describe('shiftWheelPan', () => {
  it('maps Shift + vertical wheel to horizontal delta', () => {
    expect(shiftWheelPanDelta({ deltaX: 0, deltaY: 80, shiftKey: true })).toBe(80)
    expect(shiftWheelPanDelta({ deltaX: 0, deltaY: -24, shiftKey: true })).toBe(-24)
  })

  it('leaves mouse wheel alone when Shift is not held', () => {
    expect(shiftWheelPanDelta({ deltaX: 0, deltaY: 80, shiftKey: false })).toBeNull()
  })

  it('does not intercept trackpad / Shift-remapped horizontal deltaX', () => {
    expect(shiftWheelPanDelta({ deltaX: 40, deltaY: 2, shiftKey: false })).toBeNull()
    expect(shiftWheelPanDelta({ deltaX: 40, deltaY: 2, shiftKey: true })).toBeNull()
    expect(shiftWheelPanDelta({ deltaX: -30, deltaY: 0, shiftKey: true })).toBeNull()
  })

  it('preventDefault and pans only when mapping Shift+vertical wheel', () => {
    const el = scroller()
    const mapped = new WheelEvent('wheel', { deltaX: 0, deltaY: 50, shiftKey: true, cancelable: true })
    expect(applyShiftWheelPan(mapped, el)).toBe(true)
    expect(mapped.defaultPrevented).toBe(true)
    expect(el.scrollLeft).toBe(50)

    const trackpad = new WheelEvent('wheel', { deltaX: 40, deltaY: 1, shiftKey: false, cancelable: true })
    expect(applyShiftWheelPan(trackpad, el)).toBe(false)
    expect(trackpad.defaultPrevented).toBe(false)
    expect(el.scrollLeft).toBe(50)
  })

  it('does nothing when the row does not overflow', () => {
    const el = scroller(120, 120)
    const event = new WheelEvent('wheel', { deltaX: 0, deltaY: 50, shiftKey: true, cancelable: true })
    expect(applyShiftWheelPan(event, el)).toBe(false)
    expect(event.defaultPrevented).toBe(false)
  })
})
