/** Mouse-drag horizontal pan with decaying inertia. Touch keeps native pan-x. */

export const STUDIO_DRAG_THRESHOLD_PX = 5
export const STUDIO_INERTIA_FRICTION = 0.92
export const STUDIO_INERTIA_MIN_VELOCITY = 0.05

export function dragScrollLeft(startScroll: number, pointerDx: number): number {
  return startScroll - pointerDx
}

export function inertiaVelocity(scrollDelta: number, dtMs: number): number {
  if (dtMs <= 0) return 0
  return scrollDelta / dtMs
}

export function inertiaStep(
  velocity: number,
  dtMs: number,
  friction = STUDIO_INERTIA_FRICTION,
): number {
  return velocity * Math.pow(friction, dtMs / 16.67)
}

export function shouldStartDrag(distance: number, threshold = STUDIO_DRAG_THRESHOLD_PX): boolean {
  return Math.abs(distance) >= threshold
}
