/**
 * Horizontal pan for overflow rows:
 * - Mouse: Shift + vertical wheel → scrollLeft
 * - Trackpad (macOS): native deltaX two-finger swipe is left alone
 * - Browsers that already map Shift+wheel to deltaX are also left alone
 */

export function shiftWheelPanDelta(
  event: Pick<WheelEvent, 'deltaX' | 'deltaY' | 'shiftKey'>,
): number | null {
  const absX = Math.abs(event.deltaX)
  const absY = Math.abs(event.deltaY)
  if (absX > absY) return null
  if (!event.shiftKey || event.deltaY === 0) return null
  return event.deltaY
}

export function applyShiftWheelPan(
  event: WheelEvent,
  element: HTMLElement | null | undefined,
): boolean {
  const delta = shiftWheelPanDelta(event)
  if (delta == null || !element) return false
  if (element.scrollWidth <= element.clientWidth + 1) return false
  event.preventDefault()
  element.scrollLeft += delta
  return true
}
