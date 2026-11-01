interface Rect { left: number; top: number; right: number; bottom: number; width: number; height: number }

export function positionStudioPopover(anchor: Rect, panel: { width: number; height: number }, parent: { left: number; top: number }, viewport: { width: number; height: number }, side: 'top' | 'right', sidebarRight: number) {
  const margin = 12
  const width = Math.min(panel.width, viewport.width - margin * 2)
  const height = Math.min(panel.height, viewport.height - margin * 2)
  const x = side === 'top' ? anchor.left + anchor.width / 2 - width / 2 : sidebarRight + margin
  const y = side === 'top' ? anchor.top - height - margin : anchor.bottom - height
  return {
    left: Math.max(margin, Math.min(x, viewport.width - width - margin)) - parent.left,
    top: Math.max(margin, Math.min(y, viewport.height - height - margin)) - parent.top
  }
}
