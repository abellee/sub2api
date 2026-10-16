export const FLOATING_WIDGET_EDGE_GAP = 20

export interface FloatingWidgetPoint {
  x: number
  y: number
}

export interface FloatingWidgetSize {
  width: number
  height: number
}

export interface FloatingWidgetBounds {
  left: number
  top: number
  right: number
  bottom: number
}

export type FloatingWidgetVerticalAnchor = 'top' | 'bottom' | 'free'

function clamp(value: number, min: number, max: number): number {
  if (max < min) return min
  return Math.min(Math.max(value, min), max)
}

export function getVisibleMainBounds(
  mainRect: Pick<DOMRect, 'left' | 'top' | 'right' | 'bottom'>,
  viewportWidth: number,
  viewportHeight: number
): FloatingWidgetBounds {
  return {
    left: Math.max(0, mainRect.left),
    top: Math.max(0, mainRect.top),
    right: Math.min(viewportWidth, mainRect.right),
    bottom: Math.min(viewportHeight, mainRect.bottom)
  }
}

export function fitFloatingWidgetSize(
  reportedSize: FloatingWidgetSize,
  mainRect: Pick<DOMRect, 'width' | 'height'>,
  bounds: FloatingWidgetBounds
): FloatingWidgetSize {
  const visibleWidth = Math.max(1, bounds.right - bounds.left - FLOATING_WIDGET_EDGE_GAP * 2)
  const visibleHeight = Math.max(1, bounds.bottom - bounds.top - FLOATING_WIDGET_EDGE_GAP * 2)

  return {
    width: Math.max(1, Math.min(reportedSize.width, mainRect.width / 2, visibleWidth)),
    height: Math.max(1, Math.min(reportedSize.height, mainRect.height / 2, visibleHeight))
  }
}

function clampFloatingWidgetPosition(
  position: FloatingWidgetPoint,
  size: FloatingWidgetSize,
  bounds: FloatingWidgetBounds
): FloatingWidgetPoint {
  return {
    x: clamp(position.x, bounds.left, bounds.right - size.width),
    y: clamp(position.y, bounds.top, bounds.bottom - size.height)
  }
}

export function settleFloatingWidgetPosition(
  position: FloatingWidgetPoint,
  size: FloatingWidgetSize,
  bounds: FloatingWidgetBounds,
  verticalAnchor: FloatingWidgetVerticalAnchor
): { position: FloatingWidgetPoint; verticalAnchor: FloatingWidgetVerticalAnchor } {
  const rightX = bounds.right - size.width - FLOATING_WIDGET_EDGE_GAP
  const topY = bounds.top + FLOATING_WIDGET_EDGE_GAP
  const bottomY = bounds.bottom - size.height - FLOATING_WIDGET_EDGE_GAP
  const constrained = clampFloatingWidgetPosition(position, size, bounds)

  if (bottomY < topY) {
    return {
      position: {
        x: clamp(rightX, bounds.left, bounds.right - size.width),
        y: clamp(constrained.y, bounds.top, bounds.bottom - size.height)
      },
      verticalAnchor: 'free'
    }
  }

  let nextY = constrained.y
  let nextAnchor = verticalAnchor

  if (verticalAnchor === 'top') {
    nextY = topY
  } else if (verticalAnchor === 'bottom') {
    nextY = bottomY
  } else {
    const topDistance = constrained.y - bounds.top
    const bottomDistance = bounds.bottom - (constrained.y + size.height)

    if (topDistance < FLOATING_WIDGET_EDGE_GAP) {
      nextY = topY
      nextAnchor = 'top'
    } else if (bottomDistance < FLOATING_WIDGET_EDGE_GAP) {
      nextY = bottomY
      nextAnchor = 'bottom'
    }
  }

  return {
    position: {
      x: clamp(rightX, bounds.left, bounds.right - size.width),
      y: clamp(nextY, bounds.top, bounds.bottom - size.height)
    },
    verticalAnchor: nextAnchor
  }
}
