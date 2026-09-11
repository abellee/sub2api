import { onBeforeUnmount, watch, type Ref } from 'vue'
import { applyShiftWheelPan } from './shiftWheelPan'

/** Non-passive wheel listener so Shift+wheel can preventDefault without blocking trackpad deltaX. */
export function useShiftWheelPan(target: Ref<HTMLElement | null | undefined>) {
  function onWheel(event: WheelEvent) {
    applyShiftWheelPan(event, target.value)
  }

  watch(
    target,
    (el, prev) => {
      prev?.removeEventListener('wheel', onWheel)
      el?.addEventListener('wheel', onWheel, { passive: false })
    },
    { flush: 'post', immediate: true },
  )

  onBeforeUnmount(() => {
    target.value?.removeEventListener('wheel', onWheel)
  })
}
