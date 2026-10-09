/** Clip / virtualize helpers for studio status faces. */

export type StudioBox = {
  left: number
  right: number
  top: number
  bottom: number
}

/**
 * Left edge of the uncovered faces viewport.
 * When the KPI card overlaps the scroller (joined side-by-side layout),
 * faces tucked under the card are treated as off-screen.
 * A stacked wrap (card above faces, including the small join overlap) is not a clip.
 */
export function studioFaceClipLeft(scroller: StudioBox, card: StudioBox | null): number {
  if (!card) return scroller.left
  const overlapX = Math.min(card.right, scroller.right) - Math.max(card.left, scroller.left)
  const overlapY = Math.min(card.bottom, scroller.bottom) - Math.max(card.top, scroller.top)
  const scrollerHeight = Math.max(0, scroller.bottom - scroller.top)
  const stacked =
    card.right >= scroller.right - 24 || overlapY < Math.min(40, scrollerHeight * 0.35)
  if (stacked || overlapX <= 8 || overlapY <= 8 || card.left > scroller.left + 8) {
    return scroller.left
  }
  return Math.max(scroller.left, card.right)
}

/** True when the face still peeks out from under the card / scroller clip. */
export function studioFaceUncovered(faceRight: number, clipLeft: number): boolean {
  return faceRight > clipLeft + 2
}
