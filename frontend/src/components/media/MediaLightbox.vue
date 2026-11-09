<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { MediaFile } from '@/api/mediaLibrary'

const props = defineProps<{ file: MediaFile; index: number; total: number }>()
const emit = defineEmits<{ close: []; move: [direction: number] }>()
const { t } = useI18n()
const overlay = ref<HTMLElement>()
const closeButton = ref<HTMLButtonElement>()
const scale = ref(1)
const rotation = ref(0)
const position = ref({ x: 0, y: 0 })
const dragging = ref(false)
const video = computed(() => props.file.content_type.startsWith('video/'))
const transform = computed(() => `translate(${position.value.x}px, ${position.value.y}px) scale(${scale.value}) rotate(${rotation.value}deg)`)
let previousFocus: HTMLElement | null = null
let dragStart = { x: 0, y: 0 }

function reset() {
  scale.value = 1
  rotation.value = 0
  position.value = { x: 0, y: 0 }
  dragging.value = false
}

function zoom(step: number) {
  scale.value = Math.min(4, Math.max(0.25, scale.value + step))
  if (scale.value <= 1) position.value = { x: 0, y: 0 }
}

function startDrag(event: PointerEvent) {
  if (video.value || scale.value <= 1 || event.button !== 0) return
  event.preventDefault()
  dragging.value = true
  dragStart = { x: event.clientX - position.value.x, y: event.clientY - position.value.y }
  const target = event.currentTarget as HTMLElement
  target.setPointerCapture(event.pointerId)
}

function drag(event: PointerEvent) {
  if (dragging.value) position.value = { x: event.clientX - dragStart.x, y: event.clientY - dragStart.y }
}

function keyboard(event: KeyboardEvent) {
  if (event.key === 'Escape') { event.preventDefault(); emit('close'); return }
  if (event.key === 'Tab') {
    const controls = [...(overlay.value?.querySelectorAll<HTMLElement>('button:not(:disabled), video[controls]') || [])]
    const index = controls.indexOf(document.activeElement as HTMLElement)
    if (index === -1 || (event.shiftKey && index === 0) || (!event.shiftKey && index === controls.length - 1)) {
      event.preventDefault()
      controls[event.shiftKey ? controls.length - 1 : 0]?.focus()
    }
    return
  }
  if (event.target instanceof HTMLVideoElement) return
  if (event.key === 'ArrowLeft' && props.index > 0) { event.preventDefault(); emit('move', -1) }
  if (event.key === 'ArrowRight' && props.index < props.total - 1) { event.preventDefault(); emit('move', 1) }
}

watch(() => props.file.id, reset)
onMounted(() => {
  previousFocus = document.activeElement as HTMLElement | null
  document.body.classList.add('media-lightbox-open')
  closeButton.value?.focus()
  document.addEventListener('keydown', keyboard)
})
onUnmounted(() => {
  document.body.classList.remove('media-lightbox-open')
  document.removeEventListener('keydown', keyboard)
  if (previousFocus?.isConnected) previousFocus.focus()
})
</script>

<template>
  <Teleport to="body">
    <div ref="overlay" class="library-lightbox" role="dialog" aria-modal="true" :aria-label="t('mediaLibrary.preview')" @click.self="emit('close')">
      <div class="lightbox-stage" @click.self="emit('close')">
        <div class="lightbox-media" :class="{ 'is-zoomed': !video && scale > 1, 'is-dragging': dragging }" :style="{ transform }" @pointerdown="startDrag" @pointermove="drag" @pointerup="dragging = false" @pointercancel="dragging = false" @lostpointercapture="dragging = false" @click.stop>
          <video v-if="video" :key="file.id" :src="file.download_url" controls playsinline preload="metadata" />
          <img v-else :key="`image-${file.id}`" :src="file.download_url" :alt="file.model || t('mediaLibrary.image')" referrerpolicy="no-referrer" draggable="false" />
        </div>
      </div>
      <span class="lightbox-position" aria-live="polite">{{ index + 1 }} / {{ total }}</span>
      <button ref="closeButton" class="lightbox-button lightbox-close" :aria-label="t('common.close')" :title="t('common.close')" @click="emit('close')"><Icon name="x" size="lg" /></button>
      <button class="lightbox-button lightbox-previous" :disabled="index <= 0" :aria-label="t('mediaLibrary.previous')" @click="emit('move', -1)"><Icon name="chevronLeft" size="lg" /></button>
      <button class="lightbox-button lightbox-next" :disabled="index >= total - 1" :aria-label="t('mediaLibrary.next')" @click="emit('move', 1)"><Icon name="chevronRight" size="lg" /></button>
      <div class="lightbox-tools" role="group" :aria-label="t('mediaLibrary.previewTools')">
        <button class="lightbox-button" :disabled="scale <= 0.25" :aria-label="t('mediaLibrary.zoomOut')" :title="t('mediaLibrary.zoomOut')" @click="zoom(-0.25)"><Icon name="minus" size="md" /></button>
        <button class="lightbox-scale" :aria-label="t('mediaLibrary.resetView')" :title="t('mediaLibrary.resetView')" @click="reset">{{ Math.round(scale * 100) }}%</button>
        <button class="lightbox-button" :disabled="scale >= 4" :aria-label="t('mediaLibrary.zoomIn')" :title="t('mediaLibrary.zoomIn')" @click="zoom(0.25)"><Icon name="plus" size="md" /></button>
        <span class="lightbox-divider" />
        <button class="lightbox-button" :aria-label="t('mediaLibrary.rotate')" :title="t('mediaLibrary.rotate')" @click="rotation = (rotation + 90) % 360"><Icon name="rotate" size="md" /></button>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
:global(body.media-lightbox-open) { overflow: hidden; }
.library-lightbox { position: fixed; inset: 0; z-index: 1000; overflow: hidden; background: rgb(0 0 0 / 90%); color: white; }
.lightbox-stage { position: absolute; inset: 72px 80px 100px; display: flex; align-items: center; justify-content: center; }
.lightbox-media { display: flex; align-items: center; justify-content: center; flex: 0 0 auto; transform-origin: center; }
.lightbox-media img, .lightbox-media video { display: block; max-width: calc(100vw - 160px); max-height: calc(100dvh - 172px); width: auto; height: auto; object-fit: contain; user-select: none; }
.lightbox-media.is-zoomed { cursor: grab; touch-action: none; }
.lightbox-media.is-dragging { cursor: grabbing; }
.lightbox-button { display: grid; place-items: center; width: 44px; height: 44px; border-radius: 50%; background: rgb(255 255 255 / 8%); color: white; }
.lightbox-button:hover:not(:disabled), .lightbox-scale:hover { background: rgb(255 255 255 / 18%); }
.lightbox-button:disabled { opacity: .25; cursor: default; }
.lightbox-button:focus-visible, .lightbox-scale:focus-visible { outline: 2px solid white; outline-offset: 3px; }
.lightbox-position { position: absolute; top: 28px; left: 28px; font-size: 13px; color: #d5dae3; }
.lightbox-close { position: absolute; top: 18px; right: 20px; }
.lightbox-previous, .lightbox-next { position: absolute; top: calc(50% - 22px); }
.lightbox-previous { left: 20px; }
.lightbox-next { right: 20px; }
.lightbox-tools { position: absolute; bottom: max(24px, env(safe-area-inset-bottom)); left: 50%; transform: translateX(-50%); display: flex; align-items: center; gap: 8px; padding: 8px 12px; border-radius: 40px; background: rgb(30 34 42 / 90%); }
.lightbox-tools .lightbox-button { background: transparent; }
.lightbox-tools .lightbox-button:hover:not(:disabled) { background: rgb(255 255 255 / 18%); }
.lightbox-scale { min-width: 60px; height: 40px; border-radius: 20px; font-size: 13px; font-variant-numeric: tabular-nums; }
.lightbox-divider { width: 1px; height: 20px; background: rgb(255 255 255 / 20%); }
@media (max-width: 600px) {
  .lightbox-stage { left: 48px; right: 48px; }
  .lightbox-media img, .lightbox-media video { max-width: calc(100vw - 96px); }
  .lightbox-previous { left: 4px; } .lightbox-next { right: 4px; }
}
</style>
