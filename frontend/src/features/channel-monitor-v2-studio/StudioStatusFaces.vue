<template>
  <div
    ref="rootRef"
    class="studio-faces-root"
    :class="{ 'is-hover': wrapHover, 'is-page-hidden': pageHidden }"
    @mouseenter="wrapHover = true"
    @mouseleave="wrapHover = false"
  >
    <div ref="wrapRef" class="studio-faces-wrap">
    <div
      v-if="faces.length"
      ref="scrollerRef"
      class="studio-faces-scroller overflow-x-auto"
      @scroll.passive="onScroll"
    >
      <div class="studio-faces-sizer">
      <div ref="canvasRef" class="studio-faces-canvas">
        <div class="studio-faces flex items-center gap-2 py-0" role="list">
          <button
            v-for="(face, index) in faces"
            :key="face.key"
            type="button"
            role="listitem"
            class="studio-face"
            draggable="false"
            :class="[
              face.toneClass,
              face.empty ? 'studio-face--empty' : '',
              isCurrentFace(index, face.key) ? 'studio-face--current' : 'studio-face--past',
              pendingKeys.has(face.key) ? 'studio-face--pending' : '',
              pendingKeys.has(face.key) && !slotReady ? 'studio-face--collapse' : '',
              enteringKeys.has(face.key) ? 'studio-face--enter' : '',
              shrinkingKey === face.key ? 'studio-face--shrink' : '',
              hotKey === face.key ? 'studio-face--hot' : '',
            ]"
            :data-face-key="face.key"
            :aria-label="face.aria"
            @mouseenter="onFaceEnter($event, face)"
            @mousemove="move"
            @mouseleave="onFaceLeave"
            @focus="onFaceEnter($event, face)"
            @blur="onFaceLeave"
          >
            <span
              class="studio-face-svg-wrap"
              aria-hidden="true"
              @animationend="clearEnter(face.key)"
            >
              <StudioFaceSvg
                v-if="isFaceOnScreen(face.key)"
                :mood="face.mood"
                :tone="face.tone"
                :score="face.score"
              />
            </span>
          </button>
        </div>
        <div class="studio-faces-axis" aria-hidden="true">
          <strong
            v-for="(face, index) in faces"
            :key="`time-${face.key}`"
            class="studio-face-title"
            :class="[
              isCurrentFace(index, face.key) ? 'studio-face-title--current' : 'studio-face-title--past',
              pendingKeys.has(face.key) ? 'studio-face-title--pending' : '',
              pendingKeys.has(face.key) && !slotReady ? 'studio-face-title--collapse' : '',
              enteringKeys.has(face.key) ? 'studio-face-title--enter' : '',
            ]"
            :data-face-key="face.key"
          >{{ face.time }}</strong>
        </div>
      </div>
      </div>
    </div>
    <p v-else class="py-6 text-sm text-gray-400">
      {{ t('channelMonitorV2.studio.faces.empty') }}
    </p>
    <div v-if="faces.length" class="studio-faces-fade" aria-hidden="true" />
    </div>
    <button
      v-if="faces.length"
      type="button"
      class="studio-faces-nav studio-faces-nav--prev"
      :disabled="!canPrev"
      :aria-label="t('channelMonitorV2.studio.faces.prev')"
      @pointerdown.stop
      @click.stop="nudge(-1)"
    >
      <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M15.5 5.5 8.5 12l7 6.5" /></svg>
    </button>
    <button
      v-if="faces.length"
      type="button"
      class="studio-faces-nav studio-faces-nav--next"
      :disabled="!canNext"
      :aria-label="t('channelMonitorV2.studio.faces.next')"
      @pointerdown.stop
      @click.stop="nudge(1)"
    >
      <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M8.5 5.5 15.5 12l-7 6.5" /></svg>
    </button>

    <Teleport v-if="isHost" to="body">
      <div
        v-show="state.visible"
        ref="tooltipRef"
        class="studio-face-tooltip"
        :style="{ left: `${state.x}px`, top: `${state.y}px` }"
        role="tooltip"
      >
        <span
          v-for="(line, index) in state.lines"
          :key="index"
          class="studio-face-tooltip-line"
          :class="index === 0 ? 'studio-face-tooltip-title' : ''"
        >
          {{ line }}
        </span>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { MonitorCoverage } from '@/api/channelMonitorV2'
import { healthModeScore } from '@/features/channel-monitor-v2/monitorFormat'
import StudioFaceSvg from './StudioFaceSvg.vue'
import { STUDIO_FACE_MOVE_MS, STUDIO_FACE_POP_MS, STUDIO_FACE_STAGGER_MS, studioAfterPaint, studioPageHidden, studioPopIncomingKeys, subscribeStudioVisibility } from './studioDemo'
import { alignBuckets, coverageBucketStarts, isStudioBucketEmpty, type StudioBucketPoint } from './studioBuckets'
import { overallToneFromRow, statusFace, type StudioThresholds } from './studioFormat'
import { bucketTooltipLines, emptyTooltipLines, formatSlotTime } from './studioTooltip'
import { studioFaceClipLeft, studioFaceUncovered } from './studioFaceVisible'
import { useDragInertiaPan } from './useDragInertiaPan'
import { useShiftWheelPan } from './useShiftWheelPan'
import { useStudioHoverTooltip } from './useStudioHoverTooltip'

const props = withDefaults(
  defineProps<{
    buckets: StudioBucketPoint[]
    coverage: MonitorCoverage | null
    showThroughput?: boolean
    thresholds?: StudioThresholds | null
    rowIndex?: number
  }>(),
  { showThroughput: true, thresholds: null, rowIndex: 0 },
)

const { t, locale } = useI18n()
const rootRef = ref<HTMLElement | null>(null)
const wrapRef = ref<HTMLElement | null>(null)
const scrollerRef = ref<HTMLElement | null>(null)
const canvasRef = ref<HTMLElement | null>(null)
useShiftWheelPan(scrollerRef)
const { stopInertia } = useDragInertiaPan(scrollerRef)
const { tooltipRef, state, show, patch, move, hide, isHost } = useStudioHoverTooltip('faces')

const canVirtualize = typeof IntersectionObserver !== 'undefined'
const visibleKeys = ref<Set<string>>(new Set())
const facesObserved = ref(!canVirtualize)
const intersectingKeys = new Set<string>()
const measuredKeys = new Set<string>()
const observedFaceNodes = new Set<HTMLElement>()
let faceObserver: IntersectionObserver | null = null
let layoutReady = !canVirtualize
let visibilityGen = 0
let cancelSettle: (() => void) | null = null
const pendingKeys = ref<Set<string>>(new Set())
const enteringKeys = ref<Set<string>>(new Set())
const seenKeys = ref<Set<string>>(new Set())
const primed = ref(false)
const hotKey = ref('')
const shrinkingKey = ref('')
const slotReady = ref(false)
const wrapHover = ref(false)
const canPrev = ref(false)
const canNext = ref(false)
const pageHidden = ref(studioPageHidden())
const cullEnabled = ref(false)
let flipping = false
let settling = false
let stopVisibility: (() => void) | null = null
let lastCurrentKey = ''
let lastBucketSeconds = 0
let flipClearTimer = 0
let enterClearTimer = 0
let popDelayTimer = 0
let shrinkClearTimer = 0
let slidePlayTimer = 0
let navFrame = 0
let cancelPaint: (() => void) | null = null
let cancelResizePin: (() => void) | null = null
let resizeObserver: ResizeObserver | null = null
let stickToEnd = true
let pinLock = 0

const bucketSeconds = computed(() => Math.max(60, props.coverage?.bucket_seconds || 60))

const slots = computed(() => {
  const starts = props.coverage ? coverageBucketStarts(props.coverage) : []
  return alignBuckets(starts, props.buckets)
})

function toneClass(tone: ReturnType<typeof statusFace>['tone']) {
  if (tone === 'healthy') return 'studio-face--healthy'
  if (tone === 'warning') return 'studio-face--warning'
  if (tone === 'critical') return 'studio-face--critical'
  return 'studio-face--unknown'
}

function toneLabel(tone: ReturnType<typeof statusFace>['tone']) {
  return t(`channelMonitorV2.studio.faces.${tone}`)
}

const faces = computed(() =>
  slots.value.map((slot) => {
    const empty = isStudioBucketEmpty(slot.bucket)
    const score = empty ? null : healthModeScore(slot.bucket!.health, 'overall')
    const tone = empty
      ? 'unknown'
      : overallToneFromRow(slot.bucket!.metrics, slot.bucket!.health, props.thresholds)
    const face = statusFace(score, tone)
    const time = formatSlotTime(slot.start, locale.value)
    return {
      key: slot.start,
      time,
      mood: face.mood,
      tone: face.tone,
      score,
      toneClass: toneClass(face.tone),
      empty,
      bucket: empty ? undefined : slot.bucket,
      aria: `${time} · ${toneLabel(face.tone)}`,
    }
  }),
)

const currentKey = computed(() => faces.value[faces.value.length - 1]?.key || '')

function isFaceOnScreen(key: string) {
  if (pendingKeys.value.has(key) || enteringKeys.value.has(key) || key === currentKey.value) return true
  if (!cullEnabled.value) return true
  if (!canVirtualize || !facesObserved.value) return true
  if (!measuredKeys.has(key)) return true
  return visibleKeys.value.has(key)
}

function isFaceAnimating() {
  return flipping || slotReady.value || pendingKeys.value.size > 0 || enteringKeys.value.size > 0
}

function isCurrentFace(index: number, key: string) {
  const list = faces.value
  const last = list.length - 1
  if (last < 0) return false
  if (index === last) return !pendingKeys.value.has(key) || slotReady.value
  if (
    index === last - 1
    && pendingKeys.value.has(list[last]?.key)
    && !slotReady.value
  ) {
    return true
  }
  return false
}

function firstFaceLeft() {
  const node = scrollerRef.value?.querySelector('.studio-face:not(.studio-face--collapse)') as HTMLElement | null
  return node?.getBoundingClientRect().left ?? 0
}

function canvasInverted(canvas: HTMLElement) {
  const transform = canvas.style.transform
  return Boolean(transform) && transform !== 'none' && transform !== 'translate3d(0, 0, 0)' && transform !== 'translate3d(0px, 0, 0)'
}

function sameKeySet(left: Set<string>, right: Set<string>) {
  if (left.size !== right.size) return false
  for (const key of left) {
    if (!right.has(key)) return false
  }
  return true
}

function cardBox() {
  const card = rootRef.value?.previousElementSibling as HTMLElement | null
  if (!card) return null
  const rect = card.getBoundingClientRect()
  return { left: rect.left, right: rect.right, top: rect.top, bottom: rect.bottom }
}

function liveFaceKeys() {
  return faces.value.map((face) => face.key)
}

function hasMeasuredAllLiveFaces() {
  const live = liveFaceKeys()
  return live.length > 0 && live.every((key) => measuredKeys.has(key))
}

function refreshVisibleKeys() {
  const root = scrollerRef.value
  if (!root) return
  if (!cullEnabled.value || !layoutReady || !hasMeasuredAllLiveFaces() || intersectingKeys.size === 0) {
    if (facesObserved.value) facesObserved.value = !canVirtualize
    return
  }
  const scrollerRect = root.getBoundingClientRect()
  const clipLeft = studioFaceClipLeft(
    { left: scrollerRect.left, right: scrollerRect.right, top: scrollerRect.top, bottom: scrollerRect.bottom },
    cardBox(),
  )
  const next = new Set<string>()
  root.querySelectorAll<HTMLElement>('button[data-face-key]').forEach((node) => {
    const key = node.dataset.faceKey
    if (!key) return
    if (pendingKeys.value.has(key) || enteringKeys.value.has(key) || key === currentKey.value) {
      next.add(key)
      return
    }
    if (!intersectingKeys.has(key)) return
    if (studioFaceUncovered(node.getBoundingClientRect().right, clipLeft)) next.add(key)
  })
  facesObserved.value = true
  if (sameKeySet(visibleKeys.value, next)) return
  visibleKeys.value = next
}

function snapshotScroller() {
  const el = scrollerRef.value
  if (!el) return { width: 0, left: 0, pinned: true }
  const max = el.scrollWidth - el.clientWidth
  return {
    width: el.scrollWidth,
    left: el.scrollLeft,
    pinned: max <= 2 || max - el.scrollLeft <= 2,
  }
}

function observeOnScreenFaces() {
  const root = scrollerRef.value
  if (!root) return
  const live = new Set(liveFaceKeys())
  for (const key of [...intersectingKeys]) {
    if (!live.has(key)) intersectingKeys.delete(key)
  }
  for (const key of [...measuredKeys]) {
    if (!live.has(key)) measuredKeys.delete(key)
  }
  if (!canVirtualize) {
    visibleKeys.value = live
    return
  }
  if (!faceObserver) {
    faceObserver = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          const key = (entry.target as HTMLElement).dataset.faceKey
          if (!key) continue
          measuredKeys.add(key)
          if (entry.isIntersecting) intersectingKeys.add(key)
          else intersectingKeys.delete(key)
        }
        if (!settling && !isFaceAnimating()) refreshVisibleKeys()
      },
      { root, rootMargin: '48px', threshold: 0 },
    )
  }
  const liveNodes = new Set<HTMLElement>()
  root.querySelectorAll<HTMLElement>('button[data-face-key]').forEach((node) => {
    liveNodes.add(node)
    if (observedFaceNodes.has(node)) return
    observedFaceNodes.add(node)
    faceObserver?.observe(node)
  })
  for (const node of [...observedFaceNodes]) {
    if (liveNodes.has(node)) continue
    observedFaceNodes.delete(node)
    faceObserver?.unobserve(node)
  }
}

function restartFaceObserver() {
  faceObserver?.disconnect()
  faceObserver = null
  observedFaceNodes.clear()
  observeOnScreenFaces()
}

function updateNav() {
  const el = scrollerRef.value
  if (!el) {
    canPrev.value = false
    canNext.value = false
    return
  }
  const max = Math.max(0, el.scrollWidth - el.clientWidth)
  canPrev.value = el.scrollLeft > 2
  canNext.value = max - el.scrollLeft > 2
}

function isPinnedEnd(el: HTMLElement) {
  const max = el.scrollWidth - el.clientWidth
  return max <= 2 || max - el.scrollLeft <= 2
}

function pinToEnd() {
  const node = scrollerRef.value
  if (!node) return
  pinLock += 1
  node.scrollLeft = node.scrollWidth
  updateNav()
  pinLock = Math.max(0, pinLock - 1)
}

function scheduleNav() {
  if (navFrame) return
  navFrame = requestAnimationFrame(() => {
    navFrame = 0
    updateNav()
  })
}

function nudge(direction: number) {
  const el = scrollerRef.value
  if (!el) return
  stopInertia()
  const distance = Math.max(180, Math.min(320, Math.round(el.clientWidth * 0.42))) * direction
  el.scrollBy({ left: distance, behavior: 'smooth' })
  scheduleNav()
  const onEnd = () => {
    el.removeEventListener('scrollend', onEnd)
    updateNav()
  }
  el.addEventListener('scrollend', onEnd, { once: true })
}

function playCanvasSlide(dx: number, onDone: (moved: boolean) => void) {
  const canvas = canvasRef.value
  const inverted = canvas ? canvasInverted(canvas) : false
  if (!canvas || (dx < 0.5 && !inverted)) {
    onDone(false)
    return
  }
  flipping = true
  let settled = false
  const pinAfter = stickToEnd && dx >= 0.5 && !inverted
  const target = inverted ? 'translate3d(0, 0, 0)' : `translate3d(${-dx}px, 0, 0)`
  const settle = () => {
    if (settled) return
    settled = true
    window.clearTimeout(flipClearTimer)
    canvas.style.willChange = ''
    canvas.style.transition = ''
    canvas.style.transform = ''
    if (pinAfter) pinToEnd()
    onDone(true)
  }
  const play = () => {
    canvas.style.willChange = 'transform'
    canvas.style.transition = `transform ${STUDIO_FACE_MOVE_MS}ms cubic-bezier(0.22, 1, 0.36, 1)`
    canvas.style.transform = target
  }
  const stagger = Math.max(0, props.rowIndex) * STUDIO_FACE_STAGGER_MS
  window.clearTimeout(slidePlayTimer)
  if (stagger) slidePlayTimer = window.setTimeout(play, stagger)
  else play()
  const onEnd = (event: TransitionEvent) => {
    if (event.propertyName !== 'transform') return
    canvas.removeEventListener('transitionend', onEnd)
    settle()
  }
  canvas.addEventListener('transitionend', onEnd)
  window.clearTimeout(flipClearTimer)
  flipClearTimer = window.setTimeout(settle, stagger + STUDIO_FACE_MOVE_MS + 48)
}

function startPop(keys: string[]) {
  if (!keys.length) {
    finishMotion()
    return
  }
  const apply = () => {
    pendingKeys.value = new Set()
    enteringKeys.value = new Set(keys)
    window.clearTimeout(enterClearTimer)
    enterClearTimer = window.setTimeout(() => {
      enteringKeys.value = new Set()
      finishMotion()
    }, STUDIO_FACE_POP_MS + 24)
  }
  requestAnimationFrame(apply)
}

function finishMotion() {
  flipping = false
  slotReady.value = false
  if (stickToEnd) pinToEnd()
  observeOnScreenFaces()
  if (!canVirtualize || intersectingKeys.size > 0) refreshVisibleKeys()
}

function finishIncoming(keys: string[], moved: boolean) {
  window.clearTimeout(popDelayTimer)
  const delay = moved ? 0 : Math.max(0, props.rowIndex) * STUDIO_FACE_STAGGER_MS
  if (delay) popDelayTimer = window.setTimeout(() => startPop(keys), delay)
  else startPop(keys)
}

function keysDiffer(seen: Set<string>, keys: string[]) {
  if (seen.size !== keys.length) return true
  for (const key of keys) {
    if (!seen.has(key)) return true
  }
  return false
}

function resetFaceVisibility() {
  intersectingKeys.clear()
  measuredKeys.clear()
  visibleKeys.value = new Set()
  facesObserved.value = !canVirtualize
  layoutReady = !canVirtualize
}

function cancelInflightMotion() {
  cancelPaint?.()
  cancelPaint = null
  window.clearTimeout(flipClearTimer)
  window.clearTimeout(enterClearTimer)
  window.clearTimeout(popDelayTimer)
  window.clearTimeout(shrinkClearTimer)
  window.clearTimeout(slidePlayTimer)
  flipping = false
  slotReady.value = false
  pendingKeys.value = new Set()
  enteringKeys.value = new Set()
  shrinkingKey.value = ''
  const canvas = canvasRef.value
  if (!canvas) return
  canvas.style.willChange = ''
  canvas.style.transition = 'none'
  canvas.style.transform = ''
  void canvas.offsetWidth
  canvas.style.transition = ''
}

function settleFaces() {
  cancelInflightMotion()
  settling = true
  cullEnabled.value = false
  resetFaceVisibility()
  const gen = ++visibilityGen
  cancelSettle?.()
  cancelSettle = null
  void nextTick(() => {
    if (gen !== visibilityGen) return
    stopInertia()
    if (stickToEnd) pinToEnd()
    observeOnScreenFaces()
    cancelSettle = studioAfterPaint(() => {
      cancelSettle = null
      if (gen !== visibilityGen) return
      if (stickToEnd) pinToEnd()
      resetFaceVisibility()
      layoutReady = true
      restartFaceObserver()
      settling = false
      cullEnabled.value = false
      facesObserved.value = !canVirtualize
    })
  })
}

function markPending(keys: string[]): string[] {
  const seen = seenKeys.value
  if (!primed.value || seen.size === 0) {
    seenKeys.value = new Set(keys)
    primed.value = true
    pendingKeys.value = new Set()
    enteringKeys.value = new Set()
    return []
  }
  const incoming = studioPopIncomingKeys(seen, keys)
  seenKeys.value = new Set(keys)
  if (!incoming.length) {
    pendingKeys.value = new Set()
    enteringKeys.value = new Set()
    return incoming
  }
  pendingKeys.value = new Set(incoming)
  enteringKeys.value = new Set()
  return incoming
}

function clearEnter(key: string) {
  if (!enteringKeys.value.has(key)) return
  const next = new Set(enteringKeys.value)
  next.delete(key)
  enteringKeys.value = next
}

function faceLines(face: { key: string; empty: boolean; bucket?: StudioBucketPoint }) {
  return face.bucket
    ? bucketTooltipLines(face.bucket, t, {
        bucketSeconds: bucketSeconds.value,
        showThroughput: props.showThroughput,
        locale: locale.value,
      })
    : emptyTooltipLines(face.key, bucketSeconds.value, t, locale.value)
}

function onFaceEnter(event: MouseEvent | FocusEvent, face: { key: string; empty: boolean; bucket?: StudioBucketPoint }) {
  hotKey.value = face.key
  show(event, faceLines(face))
}

function onFaceLeave() {
  hotKey.value = ''
  hide()
}

watch(
  faces,
  (next) => {
    const nextKeys = next.map((face) => face.key)
    const step = bucketSeconds.value
    const rangeSwitched = lastBucketSeconds !== 0 && lastBucketSeconds !== step
    lastBucketSeconds = step
    if (rangeSwitched) {
      primed.value = false
      seenKeys.value = new Set()
      stickToEnd = true
      lastCurrentKey = ''
      cancelInflightMotion()
    }
    const prevSeen = seenKeys.value
    const isFirst = !primed.value || prevSeen.size === 0
    const replaced = keysDiffer(prevSeen, nextKeys)
    const incoming = markPending(nextKeys)
    const prevCurrent = lastCurrentKey
    lastCurrentKey = next[next.length - 1]?.key || ''
    if (hotKey.value) {
      const hovered = next.find((face) => face.key === hotKey.value)
      if (hovered) patch(faceLines(hovered))
    }
    if (!isFirst && !incoming.length) {
      if (replaced) settleFaces()
      return
    }
    slotReady.value = false
    cancelPaint?.()
    cancelPaint = null
    void nextTick(() => {
      const el = scrollerRef.value
      if (!el) return
      if (isFirst) {
        flipping = false
        settleFaces()
        return
      }
      stopInertia()
      flipping = true
      const followEnd = stickToEnd
      if (followEnd) pinToEnd()
      cancelPaint = studioAfterPaint(() => {
        const snap = snapshotScroller()
        const beforeLeft = firstFaceLeft()
        slotReady.value = true
        if (incoming.length && prevCurrent && prevCurrent !== lastCurrentKey) {
          shrinkingKey.value = prevCurrent
          window.clearTimeout(shrinkClearTimer)
          shrinkClearTimer = window.setTimeout(() => {
            if (shrinkingKey.value === prevCurrent) shrinkingKey.value = ''
          }, STUDIO_FACE_MOVE_MS + Math.max(0, props.rowIndex) * STUDIO_FACE_STAGGER_MS + 32)
        }
        void nextTick(() => {
          const canvas = canvasRef.value
          const grow = el.scrollWidth - snap.width
          const shift = firstFaceLeft() - beforeLeft
          let dx = 0
          if (followEnd && grow > 0.5) {
            dx = grow
          } else if (shift < -0.5 && canvas) {
            canvas.style.willChange = 'transform'
            canvas.style.transition = 'none'
            canvas.style.transform = `translate3d(${-shift}px, 0, 0)`
            void canvas.offsetWidth
          }
          playCanvasSlide(dx, (moved) => finishIncoming(incoming, moved))
        })
      })
    })
  },
  { flush: 'pre', immediate: true },
)

function onScroll() {
  updateNav()
  if (pinLock) return
  if (!settling) {
    if (!cullEnabled.value) cullEnabled.value = true
    if (!isFaceAnimating()) refreshVisibleKeys()
  }
  const el = scrollerRef.value
  if (el) stickToEnd = isPinnedEnd(el)
}

function onResize() {
  if (isFaceAnimating()) return
  stopInertia()
  if (stickToEnd) {
    pinToEnd()
    cancelResizePin?.()
    cancelResizePin = studioAfterPaint(() => {
      if (stickToEnd && !isFaceAnimating()) pinToEnd()
    })
  } else {
    updateNav()
  }
  observeOnScreenFaces()
  refreshVisibleKeys()
}

function onPageHidden(hidden: boolean) {
  pageHidden.value = hidden
  if (hidden) {
    cancelInflightMotion()
    if (stickToEnd) pinToEnd()
    return
  }
  void nextTick(() => {
    if (stickToEnd) pinToEnd()
    observeOnScreenFaces()
    if (!canVirtualize || intersectingKeys.size > 0) refreshVisibleKeys()
  })
}

onMounted(() => {
  window.addEventListener('resize', onResize)
  const wrap = wrapRef.value
  if (typeof ResizeObserver !== 'undefined' && wrap) {
    resizeObserver = new ResizeObserver(onResize)
    resizeObserver.observe(wrap)
  }
  stopVisibility = subscribeStudioVisibility(onPageHidden)
  pageHidden.value = studioPageHidden()
  updateNav()
})

onBeforeUnmount(() => {
  faceObserver?.disconnect()
  observedFaceNodes.clear()
  resizeObserver?.disconnect()
  stopVisibility?.()
  window.removeEventListener('resize', onResize)
  cancelPaint?.()
  cancelResizePin?.()
  cancelSettle?.()
  if (navFrame) cancelAnimationFrame(navFrame)
  window.clearTimeout(flipClearTimer)
  window.clearTimeout(enterClearTimer)
  window.clearTimeout(popDelayTimer)
  window.clearTimeout(shrinkClearTimer)
  window.clearTimeout(slidePlayTimer)
})
</script>

<style scoped>
.studio-faces-root {
  --studio-faces-bg: rgb(241 245 249);
  --studio-kline-inset: 1rem;
  --studio-face-pop-ms: 0.5s;
  --studio-face-move-ms: 0.5s;
  position: relative;
  z-index: auto;
  flex: 1 1 100%;
  align-self: stretch;
  width: 100%;
  min-width: 0;
  min-height: 9.5rem;
  margin-top: -0.85rem;
}
@container studio-row (min-width: 34rem) {
  .studio-faces-root {
    flex: 1 1 16rem;
    width: auto;
    min-width: min(16rem, 100%);
    margin-top: 0;
    margin-left: -1.35rem;
    min-height: 8.5rem;
  }
}
.dark .studio-faces-root {
  --studio-faces-bg: rgb(30 41 59);
}
.studio-faces-root.is-page-hidden .studio-face-svg-wrap,
.studio-faces-root.is-page-hidden .studio-face-title,
.studio-faces-root.is-page-hidden .studio-faces-canvas {
  animation-play-state: paused !important;
}
.studio-faces-wrap {
  position: absolute;
  inset: 0;
  z-index: 0;
  overflow: hidden;
  background: var(--studio-faces-bg);
  contain: layout style;
}
.studio-faces-fade {
  pointer-events: none;
  position: absolute;
  z-index: 2;
  inset: 0 auto auto 0;
  width: 100%;
  height: 2.25rem;
  background: linear-gradient(180deg, var(--studio-faces-bg) 0%, transparent 100%);
}
@container studio-row (min-width: 34rem) {
  .studio-faces-fade {
    width: 3.25rem;
    height: 100%;
    background: linear-gradient(90deg, var(--studio-faces-bg) 0%, transparent 100%);
  }
}
.studio-faces-scroller {
  position: absolute;
  inset: 0;
  z-index: 0;
  width: auto;
  min-width: 0;
  overflow-x: auto;
  overflow-y: hidden;
  overflow-anchor: none;
  cursor: grab;
  user-select: none;
  scrollbar-width: none;
  -ms-overflow-style: none;
  overscroll-behavior-x: contain;
  touch-action: pan-x;
}
.studio-faces-sizer {
  position: relative;
  box-sizing: border-box;
  width: max-content;
  min-width: 100%;
  height: 100%;
  min-height: 100%;
  overflow: hidden;
  overflow-anchor: none;
}
.studio-faces-scroller.is-dragging {
  cursor: grabbing;
}
.studio-faces-scroller::-webkit-scrollbar {
  display: none;
  width: 0;
  height: 0;
}
.studio-faces-canvas {
  position: relative;
  box-sizing: border-box;
  width: max-content;
  min-width: 100%;
  height: 100%;
  min-height: 100%;
  padding: 1.15rem 2.35rem 0;
}
@container studio-row (min-width: 34rem) {
  .studio-faces-canvas {
    padding: 0.5rem 2.5rem 0 2.35rem;
  }
}
.studio-faces {
  position: relative;
  z-index: 0;
  width: max-content;
  min-width: 100%;
  height: 100%;
  flex-wrap: nowrap;
  justify-content: flex-end;
}
.studio-face {
  position: relative;
  display: flex;
  flex: 0 0 auto;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 3.05rem;
  height: 100%;
  padding: 0;
  background: transparent;
  text-align: center;
  cursor: help;
}
.studio-face--past {
  width: 3.05rem;
}
.studio-face--current {
  width: 4.75rem;
}
.studio-face:focus-visible {
  outline: 2px solid rgb(20 184 166 / 0.55);
  outline-offset: 2px;
  border-radius: 9999px;
}
.studio-face--empty {
  opacity: 0.55;
}
.studio-face-svg-wrap {
  display: block;
  width: 2.35rem;
  height: 2.35rem;
  overflow: visible;
  transform-origin: 50% 90%;
  backface-visibility: hidden;
  transition: transform 0.18s cubic-bezier(0.22, 1, 0.36, 1);
}
.studio-face-svg-wrap :deep(.studio-face-svg) {
  display: block;
  width: 100%;
  height: 100%;
}
.studio-face:hover:not(.studio-face--enter):not(.studio-face--pending):not(.studio-face--shrink) .studio-face-svg-wrap,
.studio-face--hot:not(.studio-face--enter):not(.studio-face--pending):not(.studio-face--shrink) .studio-face-svg-wrap,
.studio-face:focus-visible:not(.studio-face--enter):not(.studio-face--pending):not(.studio-face--shrink) .studio-face-svg-wrap {
  transform: scale(1.12);
}
.studio-face--pending .studio-face-svg-wrap,
.studio-face--enter .studio-face-svg-wrap,
.studio-face--shrink .studio-face-svg-wrap {
  transition: none;
}
.studio-face--collapse,
.studio-face-title--collapse {
  width: 0 !important;
  min-width: 0 !important;
  margin-left: -0.5rem;
  overflow: hidden;
  opacity: 0;
  pointer-events: none;
}
.studio-face--pending .studio-face-svg-wrap {
  opacity: 0;
  transform: translate3d(0, 110%, 0) scale(0);
  pointer-events: none;
}
.studio-face--enter .studio-face-svg-wrap {
  animation: studio-face-pop var(--studio-face-pop-ms, 1s) cubic-bezier(0.33, 1, 0.68, 1) both;
  will-change: transform;
}
@keyframes studio-face-shrink {
  from {
    width: 3.5rem;
    height: 3.5rem;
    transform: scale(1);
  }
  to {
    width: 2.35rem;
    height: 2.35rem;
    transform: scale(1);
  }
}
@keyframes studio-face-pop {
  0% {
    transform: translate3d(0, 110%, 0) scale(0);
    opacity: 0;
  }
  58% {
    transform: translate3d(0, -5%, 0) scale(1.08);
    opacity: 1;
  }
  78% {
    transform: translate3d(0, 1.6%, 0) scale(0.985);
    opacity: 1;
  }
  100% {
    transform: translate3d(0, 0, 0) scale(1);
    opacity: 1;
  }
}
.studio-face--past .studio-face-svg-wrap {
  width: 2.35rem;
  height: 2.35rem;
}
.studio-face--current .studio-face-svg-wrap {
  width: 3.5rem;
  height: 3.5rem;
}
.studio-face.studio-face--shrink .studio-face-svg-wrap {
  width: 3.5rem;
  height: 3.5rem;
  transform: scale(1);
  animation: studio-face-shrink var(--studio-face-move-ms, 1s) cubic-bezier(0.22, 1, 0.36, 1) both;
}
.studio-faces-axis {
  pointer-events: none;
  position: absolute;
  right: 0;
  bottom: var(--studio-kline-inset);
  left: 0;
  z-index: 1;
  display: flex;
  align-items: flex-end;
  justify-content: flex-end;
  gap: 0.5rem;
  box-sizing: border-box;
  padding: 0 2.35rem;
}
@container studio-row (min-width: 34rem) {
  .studio-faces-axis {
    padding: 0 2.5rem 0 2.35rem;
  }
}
.studio-face-title {
  flex: 0 0 auto;
  width: 3.05rem;
  font-size: 11px;
  font-weight: 500;
  line-height: 1;
  font-variant-numeric: tabular-nums;
  text-align: center;
  color: rgb(156 163 175);
}
.studio-face-title--past {
  width: 3.05rem;
}
.studio-face-title--current {
  width: 4.75rem;
}
.studio-face-title--pending {
  opacity: 0;
}
.studio-face-title--enter {
  animation: studio-face-title-in 0.35s ease both;
}
@keyframes studio-face-title-in {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}
.dark .studio-face-title {
  color: rgb(148 163 184);
}
.studio-faces-nav {
  position: absolute;
  top: 50%;
  z-index: 6;
  display: flex;
  width: 1.85rem;
  height: 1.85rem;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 9999px;
  background: rgb(255 255 255 / 0.92);
  color: rgb(71 85 105);
  box-shadow: 0 8px 20px -12px rgb(15 23 42 / 0.45);
  opacity: 0;
  pointer-events: none;
  transform: translateY(-50%);
  transition: opacity 0.16s ease, background 0.16s ease;
}
.studio-faces-nav svg {
  width: 0.95rem;
  height: 0.95rem;
}
.studio-faces-nav--prev {
  left: 0.45rem;
}
.studio-faces-nav--next {
  right: 0.45rem;
}
.studio-faces-root.is-hover .studio-faces-nav,
.studio-faces-root:hover .studio-faces-nav,
.studio-faces-root:focus-within .studio-faces-nav {
  opacity: 1;
  pointer-events: auto;
}
.studio-faces-root.is-hover .studio-faces-nav:disabled,
.studio-faces-root:hover .studio-faces-nav:disabled,
.studio-faces-root:focus-within .studio-faces-nav:disabled {
  opacity: 0.38;
  pointer-events: none;
}
.studio-faces-nav:not(:disabled):hover {
  background: rgb(255 255 255);
  color: rgb(15 23 42);
}
.dark .studio-faces-nav {
  background: rgb(30 41 59 / 0.92);
  color: rgb(203 213 225);
}
.studio-face-tooltip {
  pointer-events: none;
  position: fixed;
  z-index: 9999;
  min-width: 11.5rem;
  max-width: min(18rem, calc(100vw - 1.5rem));
  border-radius: 0.9rem;
  border: 1px solid rgb(229 231 235);
  background: rgb(255 255 255);
  padding: 0.5rem 0.625rem;
  box-shadow: 0 18px 40px -12px rgb(0 0 0 / 0.28);
  overflow-wrap: anywhere;
  white-space: normal;
}
.dark .studio-face-tooltip {
  border-color: rgb(55 65 81);
  background: rgb(17 24 39);
  color: rgb(229 231 235);
}
.studio-face-tooltip-line {
  display: block;
  font-size: 11px;
  line-height: 1.45;
  color: rgb(75 85 99);
}
.dark .studio-face-tooltip-line {
  color: rgb(209 213 219);
}
.studio-face-tooltip-title {
  margin-bottom: 0.2rem;
  font-weight: 600;
  color: rgb(17 24 39);
}
.dark .studio-face-tooltip-title {
  color: rgb(243 244 246);
}
</style>
