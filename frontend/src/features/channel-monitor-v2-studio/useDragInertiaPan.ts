import { onBeforeUnmount, watch, type Ref } from 'vue'
import {
  STUDIO_INERTIA_MIN_VELOCITY,
  dragScrollLeft,
  inertiaStep,
  inertiaVelocity,
  shouldStartDrag,
} from './dragInertiaPan'

/** Mouse/pen grab-to-scroll on an overflow-x scroller. Touch is left to native pan. */
export function useDragInertiaPan(target: Ref<HTMLElement | null | undefined>) {
  let pointerId = -1
  let dragging = false
  let moved = false
  let lastX = 0
  let lastT = 0
  let originX = 0
  let originScroll = 0
  let velocity = 0
  let inertiaRaf = 0

  function stopInertia() {
    if (inertiaRaf) cancelAnimationFrame(inertiaRaf)
    inertiaRaf = 0
  }

  function clampScroll(el: HTMLElement, next: number) {
    const max = Math.max(0, el.scrollWidth - el.clientWidth)
    el.scrollLeft = Math.max(0, Math.min(max, next))
  }

  function runInertia(el: HTMLElement, startV: number, startT: number) {
    stopInertia()
    let v = startV
    let prev = startT
    const tick = (now: number) => {
      const dt = Math.min(32, now - prev)
      prev = now
      v = inertiaStep(v, dt)
      if (Math.abs(v) < STUDIO_INERTIA_MIN_VELOCITY) {
        inertiaRaf = 0
        return
      }
      clampScroll(el, el.scrollLeft + v * dt)
      if (el.scrollLeft <= 0 || el.scrollLeft >= el.scrollWidth - el.clientWidth - 0.5) {
        inertiaRaf = 0
        return
      }
      inertiaRaf = requestAnimationFrame(tick)
    }
    inertiaRaf = requestAnimationFrame(tick)
  }

  function onPointerDown(event: PointerEvent) {
    const el = target.value
    if (!el || event.pointerType === 'touch') return
    if ((event.target as HTMLElement | null)?.closest('button.studio-faces-nav')) return
    stopInertia()
    pointerId = event.pointerId
    dragging = true
    moved = false
    lastX = event.clientX
    originX = event.clientX
    originScroll = el.scrollLeft
    lastT = event.timeStamp
    velocity = 0
    el.classList.add('is-dragging')
    try {
      el.setPointerCapture(event.pointerId)
    } catch {
      /* jsdom */
    }
  }

  function onPointerMove(event: PointerEvent) {
    const el = target.value
    if (!el || !dragging || event.pointerId !== pointerId) return
    const dx = event.clientX - lastX
    const dt = event.timeStamp - lastT || 16
    if (!moved && shouldStartDrag(event.clientX - originX)) moved = true
    if (!moved) return
    event.preventDefault()
    const next = dragScrollLeft(originScroll, event.clientX - originX)
    const prevScroll = el.scrollLeft
    clampScroll(el, next)
    velocity = inertiaVelocity(el.scrollLeft - prevScroll, dt) || inertiaVelocity(-dx, dt)
    lastX = event.clientX
    lastT = event.timeStamp
  }

  function onPointerUp(event: PointerEvent) {
    const el = target.value
    if (!dragging || event.pointerId !== pointerId) return
    dragging = false
    pointerId = -1
    el?.classList.remove('is-dragging')
    if (el && moved && Math.abs(velocity) >= STUDIO_INERTIA_MIN_VELOCITY) {
      runInertia(el, velocity, event.timeStamp)
    }
    if (moved) {
      const block = (click: Event) => {
        click.preventDefault()
        click.stopPropagation()
        el?.removeEventListener('click', block, true)
      }
      el?.addEventListener('click', block, true)
      window.setTimeout(() => el?.removeEventListener('click', block, true), 0)
    }
    moved = false
  }

  function bind(el: HTMLElement | null | undefined, prev?: HTMLElement | null) {
    prev?.removeEventListener('pointerdown', onPointerDown)
    prev?.removeEventListener('pointermove', onPointerMove)
    prev?.removeEventListener('pointerup', onPointerUp)
    prev?.removeEventListener('pointercancel', onPointerUp)
    el?.addEventListener('pointerdown', onPointerDown)
    el?.addEventListener('pointermove', onPointerMove)
    el?.addEventListener('pointerup', onPointerUp)
    el?.addEventListener('pointercancel', onPointerUp)
  }

  watch(
    target,
    (el, prev) => bind(el, prev || undefined),
    { flush: 'post', immediate: true },
  )

  onBeforeUnmount(() => {
    stopInertia()
    bind(null, target.value)
  })

  return { stopInertia }
}
