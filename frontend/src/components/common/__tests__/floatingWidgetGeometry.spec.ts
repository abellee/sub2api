import { describe, expect, it } from 'vitest'
import {
  FLOATING_WIDGET_EDGE_GAP,
  fitFloatingWidgetSize,
  getVisibleMainBounds,
  settleFloatingWidgetPosition
} from '../floatingWidgetGeometry'

const bounds = {
  left: 200,
  top: 64,
  right: 1200,
  bottom: 800
}

const size = {
  width: 220,
  height: 48
}

describe('floating widget geometry', () => {
  it('uses only the visible part of main as movement bounds', () => {
    expect(
      getVisibleMainBounds(
        { left: 200, top: -40, right: 1400, bottom: 1000 },
        1280,
        720
      )
    ).toEqual({ left: 200, top: 0, right: 1280, bottom: 720 })
  })

  it('never grows beyond half of main or the visible safe area', () => {
    expect(
      fitFloatingWidgetSize(
        { width: 900, height: 500 },
        { width: 1000, height: 600 },
        bounds
      )
    ).toEqual({ width: 500, height: 300 })
  })

  it('always snaps horizontally to twenty pixels from the right', () => {
    const settled = settleFloatingWidgetPosition({ x: 300, y: 300 }, size, bounds, 'free')

    expect(settled.position).toEqual({
      x: bounds.right - size.width - FLOATING_WIDGET_EDGE_GAP,
      y: 300
    })
    expect(settled.verticalAnchor).toBe('free')
  })

  it('snaps vertically when released within twenty pixels of an edge', () => {
    const nearTop = settleFloatingWidgetPosition({ x: 300, y: 70 }, size, bounds, 'free')
    const nearBottom = settleFloatingWidgetPosition({ x: 300, y: 745 }, size, bounds, 'free')

    expect(nearTop.position.y).toBe(bounds.top + FLOATING_WIDGET_EDGE_GAP)
    expect(nearTop.verticalAnchor).toBe('top')
    expect(nearBottom.position.y).toBe(bounds.bottom - size.height - FLOATING_WIDGET_EDGE_GAP)
    expect(nearBottom.verticalAnchor).toBe('bottom')
  })

  it('keeps an anchored widget visible after viewport changes', () => {
    const smallerBounds = { left: 72, top: 64, right: 700, bottom: 480 }
    const settled = settleFloatingWidgetPosition(
      { x: 960, y: 732 },
      size,
      smallerBounds,
      'bottom'
    )

    expect(settled.position).toEqual({
      x: smallerBounds.right - size.width - FLOATING_WIDGET_EDGE_GAP,
      y: smallerBounds.bottom - size.height - FLOATING_WIDGET_EDGE_GAP
    })
  })
})
