/** Keep the hover panel inside the viewport and reuse it across faces/dots. */

export const STUDIO_TOOLTIP_HIDE_MS = 90
export const STUDIO_TOOLTIP_PAD = 10

export type StudioTooltipPoint = { x: number; y: number }

export function eventTooltipPoint(event: MouseEvent | PointerEvent | FocusEvent): StudioTooltipPoint {
  if ('clientX' in event) return { x: event.clientX, y: event.clientY }
  const rect = (event.target as HTMLElement | null)?.getBoundingClientRect()
  if (!rect) return { x: STUDIO_TOOLTIP_PAD, y: STUDIO_TOOLTIP_PAD }
  return { x: rect.left + rect.width / 2, y: rect.top }
}

/** Place the panel above the cursor when possible; shift so it never leaves the window. */
export function clampTooltipBox(
  point: StudioTooltipPoint,
  width: number,
  height: number,
  pad = STUDIO_TOOLTIP_PAD,
): { x: number; y: number } {
  const viewport = typeof window === 'undefined' ? null : window.visualViewport
  const vw = viewport?.width || (typeof window === 'undefined' ? 1280 : window.innerWidth)
  const vh = viewport?.height || (typeof window === 'undefined' ? 720 : window.innerHeight)
  const w = Math.min(Math.max(width || 184, 1), Math.max(1, vw - pad * 2))
  const h = Math.max(height || 72, 1)
  let x = point.x - w / 2
  let y = point.y - 12 - h
  if (y < pad) y = point.y + 16
  if (y + h > vh - pad) y = Math.max(pad, vh - pad - h)
  if (x < pad) x = pad
  if (x + w > vw - pad) x = Math.max(pad, vw - pad - w)
  return { x, y }
}
