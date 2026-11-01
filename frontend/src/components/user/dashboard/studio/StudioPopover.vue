<template><div ref="panel" class="studio-popover" :style="position"><slot /></div></template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useElementBounding, useElementSize, useEventListener, useRafFn, useWindowSize } from '@vueuse/core'
import { positionStudioPopover } from './studioPopover'

const props = defineProps<{ anchor: HTMLElement | null; side: 'top' | 'right' }>()
const panel = ref<HTMLElement | null>(null)
const anchor = useElementBounding(computed(() => props.anchor))
const parent = useElementBounding(computed(() => panel.value?.parentElement))
const sidebarElement = computed(() => props.anchor?.closest('.sidebar') as HTMLElement | null)
const sidebar = useElementBounding(sidebarElement)
function updateBounds() {
  anchor.update()
  parent.update()
  sidebar.update()
}
// Transforms move the trigger without firing ResizeObserver; follow only while animating.
const { pause, resume } = useRafFn(() => {
  updateBounds()
  if (!sidebarElement.value?.getAnimations({ subtree: true }).some(animation => 'transitionProperty' in animation && animation.playState === 'running')) pause()
})
useEventListener(sidebarElement, 'transitionrun', event => {
  if (['width', 'height', 'transform', 'left', 'bottom'].includes(event.propertyName)) resume()
})
watch(() => props.anchor, resume)
const size = useElementSize(panel, undefined, { box: 'border-box' })
const viewport = useWindowSize()
const position = computed(() => {
  const point = positionStudioPopover(
    { left: anchor.left.value, top: anchor.top.value, right: anchor.right.value, bottom: anchor.bottom.value, width: anchor.width.value, height: anchor.height.value },
    { width: size.width.value, height: size.height.value },
    { left: parent.left.value, top: parent.top.value },
    { width: viewport.width.value, height: viewport.height.value },
    props.side,
    sidebar.right.value || anchor.right.value
  )
  return { left: `${point.left}px`, top: `${point.top}px`, visibility: size.width.value ? 'visible' as const : 'hidden' as const }
})
</script>

<style scoped>
.studio-popover{position:absolute;z-index:90;max-width:calc(100vw - 24px);max-height:calc(100dvh - 24px);overflow-y:auto;border:1px solid #bdc8df;box-shadow:0 16px 40px -8px #17234b33,0 4px 12px #17234b14,inset 0 1px 0 #ffffffb3}
:global(.dark .studio-popover){--tf-console-surface:var(--studio-menu-surface);border-color:#505773;box-shadow:0 20px 48px -8px #0009,0 4px 14px #0006,inset 0 1px 0 #ffffff0d}
</style>
