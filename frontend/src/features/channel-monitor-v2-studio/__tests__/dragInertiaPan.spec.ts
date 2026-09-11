import { describe, expect, it } from 'vitest'
import {
  dragScrollLeft,
  inertiaStep,
  inertiaVelocity,
  shouldStartDrag,
  STUDIO_DRAG_THRESHOLD_PX,
} from '../dragInertiaPan'

describe('dragInertiaPan', () => {
  it('scrolls opposite the pointer so content follows the drag', () => {
    expect(dragScrollLeft(400, 80)).toBe(320)
    expect(dragScrollLeft(400, -50)).toBe(450)
  })

  it('starts a drag only after the movement threshold', () => {
    expect(shouldStartDrag(STUDIO_DRAG_THRESHOLD_PX - 1)).toBe(false)
    expect(shouldStartDrag(STUDIO_DRAG_THRESHOLD_PX)).toBe(true)
  })

  it('decays velocity so a fast fling coasts then stops', () => {
    const v0 = inertiaVelocity(120, 16)
    expect(v0).toBeGreaterThan(0)
    const later = inertiaStep(v0, 200)
    expect(Math.abs(later)).toBeLessThan(Math.abs(v0))
    expect(Math.abs(inertiaStep(v0, 2000))).toBeLessThan(0.01)
  })
})
