<template>
  <div
    v-if="dots.length"
    ref="rootRef"
    class="studio-spark"
    :class="{ 'is-page-hidden': pageHidden }"
    :style="paintStyle"
    @mouseleave="onChartLeave"
  >
    <div ref="trackRef" class="studio-spark-track" :style="{ width: `${trackWidth}px` }">
      <svg
        class="studio-spark-svg"
        :viewBox="`0 0 ${trackWidth} ${chartH}`"
        :width="trackWidth"
        :height="chartH"
        preserveAspectRatio="none"
        role="img"
        :aria-label="label"
      >
        <defs>
          <linearGradient :id="fillId" x1="0" y1="0" x2="0" y2="1">
            <stop class="studio-spark-fill-top" offset="0%" :stop-color="paint.fill" stop-opacity="0.2" />
            <stop class="studio-spark-fill-bottom" offset="100%" :stop-color="paint.fill" stop-opacity="0" />
          </linearGradient>
          <linearGradient
            :id="fadeId"
            gradientUnits="userSpaceOnUse"
            :x1="visibleLeft"
            y1="0"
            :x2="trackWidth"
            y2="0"
          >
            <stop offset="0%" stop-color="#fff" stop-opacity="0" />
            <stop offset="16%" stop-color="#fff" stop-opacity="1" />
            <stop offset="84%" stop-color="#fff" stop-opacity="1" />
            <stop offset="100%" stop-color="#fff" stop-opacity="0" />
          </linearGradient>
          <mask :id="maskId" maskUnits="userSpaceOnUse">
            <rect :width="trackWidth" :height="chartH" :fill="`url(#${fadeId})`" />
          </mask>
        </defs>
        <polygon
          v-if="area"
          :points="area"
          :fill="`url(#${fillId})`"
          :mask="`url(#${maskId})`"
          class="studio-spark-area"
        />
        <polyline
          v-if="line"
          :points="line"
          fill="none"
          class="studio-spark-line"
          :stroke="paint.stroke"
          stroke-width="1.7"
          stroke-linecap="round"
          stroke-linejoin="round"
          vector-effect="non-scaling-stroke"
        />
      </svg>

      <button
        v-for="(dot, index) in dots"
        :key="dot.key"
        type="button"
        class="studio-spark-hit"
        :class="index === dots.length - 1 ? 'studio-spark-hit--current' : ''"
        :style="{ left: `${dot.x}px`, top: `${dot.y}px` }"
        :aria-label="dot.label"
        @pointerenter="onDotEnter($event, dot)"
        @pointermove="move"
      >
        <span
          class="studio-spark-dot"
          :class="[
            dot.empty ? 'studio-spark-dot--empty' : '',
            index === dots.length - 1 ? 'studio-spark-dot--current' : '',
            pendingKey === dot.key ? 'studio-spark-dot--pending' : '',
            enteringKey === dot.key ? 'studio-spark-dot--enter' : '',
          ]"
          :style="dot.empty ? undefined : { '--studio-dot-depth': `${Math.round(dot.shade * 100)}%` }"
        >
          <span v-if="index === dots.length - 1" class="studio-spark-ping" aria-hidden="true" />
        </span>
      </button>
    </div>

    <Teleport to="body">
      <div
        v-show="state.visible"
        ref="tooltipRef"
        class="studio-spark-tooltip"
        :style="{ left: `${state.x}px`, top: `${state.y}px` }"
        role="tooltip"
      >
        <span
          v-for="(line, index) in state.lines"
          :key="index"
          class="studio-spark-tooltip-line"
          :class="index === 0 ? 'studio-spark-tooltip-title' : ''"
        >
          {{ line }}
        </span>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { MonitorCoverage } from '@/api/channelMonitorV2'
import { STUDIO_CHART_MIN_GAP, STUDIO_CHART_VISIBLE_DOTS, STUDIO_FACE_MOVE_MS, STUDIO_FACE_POP_MS, studioAfterPaint, studioPageHidden, studioPopIncomingKeys, subscribeStudioVisibility } from './studioDemo'
import { alignBuckets, coverageBucketStarts, isStudioBucketEmpty, studioSparkValue, type StudioBucketPoint } from './studioBuckets'
import { allocStudioSparkIds, studioSparkPaint, studioChartArea, studioChartDots, studioChartGap, studioChartLine, studioChartTrackWidth, studioValueShade, type StudioAccent } from './studioFormat'
import { bucketTooltipLines, emptyTooltipLines, formatSlotTime } from './studioTooltip'
import { useStudioHoverTooltip } from './useStudioHoverTooltip'

const chartH = 44

const props = withDefaults(
  defineProps<{
    buckets?: StudioBucketPoint[]
    coverage?: MonitorCoverage | null
    showThroughput?: boolean
    label?: string
    accent?: StudioAccent
  }>(),
  { buckets: () => [], coverage: null, showThroughput: true, label: '', accent: 'teal' },
)

const { t, locale } = useI18n()
const { fillId, fadeId, maskId } = allocStudioSparkIds()
const paint = computed(() => studioSparkPaint(props.accent))
const paintStyle = computed(() => ({
  color: paint.value.stroke,
  '--studio-spark': paint.value.stroke,
  '--studio-spark-deep': paint.value.deep,
  '--studio-spark-fill': paint.value.fill,
}))
const { tooltipRef, state, show, patch, move, hide } = useStudioHoverTooltip()

const rootRef = ref<HTMLElement | null>(null)
const trackRef = ref<HTMLElement | null>(null)
const containerWidth = ref(240)
const pendingKey = ref('')
const enteringKey = ref('')
const hotDotKey = ref('')
const pageHidden = ref(studioPageHidden())
let enterTimer = 0
let primed = false
let seenKeys = new Set<string>()
let resizeObserver: ResizeObserver | null = null
let cancelPaint: (() => void) | null = null
let cancelPop: (() => void) | null = null
let stopVisibility: (() => void) | null = null

const bucketSeconds = computed(() => Math.max(60, props.coverage?.bucket_seconds || 60))

const slots = computed(() => {
  const starts = props.coverage ? coverageBucketStarts(props.coverage) : []
  return alignBuckets(starts, props.buckets)
})

const values = computed(() => slots.value.map((slot) => studioSparkValue(slot.bucket)))

const sparkPadX = 14

const chartGap = computed(() =>
  studioChartGap(containerWidth.value, STUDIO_CHART_MIN_GAP, sparkPadX, STUDIO_CHART_VISIBLE_DOTS),
)

const trackWidth = computed(() =>
  studioChartTrackWidth(values.value.length, containerWidth.value, chartGap.value, sparkPadX),
)

const layout = computed(() => studioChartDots(values.value, trackWidth.value, chartH, sparkPadX, 8))
const line = computed(() => studioChartLine(layout.value))
const area = computed(() => studioChartArea(layout.value, chartH, trackWidth.value))
const visibleLeft = computed(() => Math.max(0, trackWidth.value - containerWidth.value))

const dots = computed(() => {
  const present = layout.value
    .map((dot) => dot.value)
    .filter((value): value is number => value != null && Number.isFinite(value))
  const min = present.length ? Math.min(...present) : 0
  const max = present.length ? Math.max(...present) : 1
  return layout.value.map((dot, index) => {
    const slot = slots.value[index]
    return {
      key: slot?.start || `dot-${index}`,
      x: dot.x,
      y: dot.y,
      empty: dot.value == null,
      shade: studioValueShade(dot.value, min, max),
      start: slot?.start || '',
      label: formatSlotTime(slot?.start || '', locale.value),
      bucket: isStudioBucketEmpty(slot?.bucket) ? undefined : slot?.bucket,
    }
  })
})

function dotLines(dot: { start: string; bucket?: StudioBucketPoint }) {
  return dot.bucket
    ? bucketTooltipLines(dot.bucket, t, {
        bucketSeconds: bucketSeconds.value,
        showThroughput: props.showThroughput,
        locale: locale.value,
      })
    : emptyTooltipLines(dot.start, bucketSeconds.value, t, locale.value)
}

function onDotEnter(
  event: PointerEvent,
  dot: { start: string; bucket?: StudioBucketPoint; key?: string },
) {
  hotDotKey.value = dot.key || dot.start
  show(event, dotLines(dot))
}

function onChartLeave() {
  hotDotKey.value = ''
  hide()
}

function settleDots() {
  pendingKey.value = ''
  enteringKey.value = ''
  window.clearTimeout(enterTimer)
  cancelPop?.()
  cancelPop = null
  cancelPaint?.()
  cancelPaint = null
  const track = trackRef.value
  if (!track) return
  track.style.willChange = ''
  track.style.transition = 'none'
  track.style.transform = ''
}

function slideTrack(prevWidth: number, nextWidth: number) {
  const track = trackRef.value
  const dx = nextWidth - prevWidth
  if (!track || dx <= 0.5 || pageHidden.value) return
  if (dx > chartGap.value * 1.5) return
  track.style.transition = 'none'
  track.style.transform = `translate3d(${dx}px, 0, 0)`
  cancelPaint?.()
  cancelPaint = studioAfterPaint(() => {
    track.style.willChange = 'transform'
    track.style.transition = `transform ${STUDIO_FACE_MOVE_MS}ms cubic-bezier(0.22, 1, 0.36, 1)`
    track.style.transform = 'translate3d(0, 0, 0)'
  })
}

watch(
  dots,
  (next) => {
    const keys = next.map((dot) => dot.key)
    if (!primed) {
      seenKeys = new Set(keys)
      primed = true
      return
    }
    const incoming = studioPopIncomingKeys(seenKeys, keys)
    const changed = keys.length !== seenKeys.size || keys.some((key) => !seenKeys.has(key))
    seenKeys = new Set(keys)
    if (hotDotKey.value) {
      const hovered = next.find((dot) => dot.key === hotDotKey.value)
      if (hovered) patch(dotLines(hovered))
    }
    const nextKey = incoming[0] || ''
    if (!nextKey) {
      if (changed) settleDots()
      return
    }
    window.clearTimeout(enterTimer)
    cancelPop?.()
    cancelPop = null
    pendingKey.value = nextKey
    enteringKey.value = ''
    cancelPop = studioAfterPaint(() => {
      enterTimer = window.setTimeout(() => {
        pendingKey.value = ''
        enteringKey.value = nextKey
        enterTimer = window.setTimeout(() => {
          enteringKey.value = ''
        }, STUDIO_FACE_POP_MS)
      }, STUDIO_FACE_MOVE_MS)
    })
  },
  { flush: 'pre', immediate: true },
)

watch(trackWidth, (next, prev) => {
  if (pageHidden.value) return
  if (prev == null || next <= prev + 0.5) return
  slideTrack(prev, next)
})

function measure() {
  const width = rootRef.value?.clientWidth
  if (width && Math.abs(width - containerWidth.value) > 0.5) containerWidth.value = width
}

onMounted(() => {
  measure()
  if (typeof ResizeObserver !== 'undefined' && rootRef.value) {
    resizeObserver = new ResizeObserver(measure)
    resizeObserver.observe(rootRef.value)
  }
  pageHidden.value = studioPageHidden()
  stopVisibility = subscribeStudioVisibility((hidden) => {
    pageHidden.value = hidden
    if (hidden) settleDots()
  })
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  stopVisibility?.()
  cancelPaint?.()
  cancelPop?.()
  window.clearTimeout(enterTimer)
})
</script>

<style scoped>
.studio-spark {
  position: relative;
  z-index: 1;
  margin-top: auto;
  height: 2.75rem;
  width: 100%;
  color: var(--studio-spark, currentColor);
  overflow: hidden;
  -webkit-mask-image: linear-gradient(
    90deg,
    transparent 0,
    #000 2.75rem,
    #000 calc(100% - 0.35rem),
    transparent 100%
  );
  mask-image: linear-gradient(
    90deg,
    transparent 0,
    #000 2.75rem,
    #000 calc(100% - 0.35rem),
    transparent 100%
  );
  mask-repeat: no-repeat;
  mask-size: 100% 100%;
  -webkit-mask-repeat: no-repeat;
  -webkit-mask-size: 100% 100%;
}
.studio-spark-track {
  position: absolute;
  right: 0;
  top: 0;
  height: 100%;
  overflow: visible;
}
.studio-spark-svg {
  display: block;
  height: 100%;
  overflow: visible;
  color: inherit;
}
.studio-spark-fill-top {
  stop-color: var(--studio-spark-fill, currentColor);
  stop-opacity: 0.2;
}
.studio-spark-fill-bottom {
  stop-color: var(--studio-spark-fill, currentColor);
  stop-opacity: 0;
}
.studio-spark-hit {
  position: absolute;
  z-index: 1;
  display: flex;
  width: 18px;
  height: 18px;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 0;
  background: transparent;
  transform: translate(-50%, -50%);
  cursor: help;
}
.studio-spark-dot {
  position: relative;
  display: block;
  width: 8px;
  height: 8px;
  margin: 0;
  flex-shrink: 0;
  border-radius: 50%;
  background: color-mix(
    in oklab,
    currentColor,
    var(--studio-spark-deep, currentColor) var(--studio-dot-depth, 22%)
  );
  box-shadow: 0 0 0 1.15px var(--studio-wash-to, #ffffff);
  transform-origin: 50% 50%;
}
.studio-spark-dot--empty {
  background: var(--studio-wash-to, #ffffff);
  box-shadow: 0 0 0 1.15px color-mix(in oklab, currentColor 55%, var(--studio-wash-to, #ffffff));
  opacity: 0.5;
}
.studio-spark-dot--pending {
  transform: scale(0);
  opacity: 0;
}
.studio-spark-dot--pending .studio-spark-ping {
  display: none;
}
.studio-spark-dot--enter {
  animation: studio-spark-dot-in 0.55s cubic-bezier(0.22, 1.45, 0.32, 1) both;
}
.studio-spark-ping {
  position: absolute;
  left: 50%;
  top: 50%;
  width: 100%;
  height: 100%;
  border-radius: 50%;
  border: 2px solid currentColor;
  opacity: 0.45;
  transform: translate(-50%, -50%) scale(1);
  animation: studio-spark-ping 1.65s cubic-bezier(0, 0, 0.2, 1) infinite;
  pointer-events: none;
}
@keyframes studio-spark-dot-in {
  0% {
    transform: scale(0);
    opacity: 0;
  }
  100% {
    transform: scale(1);
    opacity: 1;
  }
}
@keyframes studio-spark-ping {
  0% {
    transform: translate(-50%, -50%) scale(1);
    opacity: 0.42;
  }
  100% {
    transform: translate(-50%, -50%) scale(2.85);
    opacity: 0;
  }
}
.studio-spark.is-page-hidden .studio-spark-ping,
.studio-spark.is-page-hidden .studio-spark-dot {
  animation: none !important;
}
.dark .studio-spark-dot {
  box-shadow: 0 0 0 1.15px var(--studio-wash-to, rgb(15 23 42));
}
.dark .studio-spark-dot--empty {
  background: color-mix(
    in oklab,
    var(--studio-spark-deep, #64748b) 22%,
    var(--studio-wash-to, rgb(15 23 42))
  );
}
.studio-spark-tooltip {
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
.dark .studio-spark-tooltip {
  border-color: rgb(55 65 81);
  background: rgb(17 24 39);
  color: rgb(229 231 235);
}
.studio-spark-tooltip-line {
  display: block;
  font-size: 11px;
  line-height: 1.45;
  color: rgb(75 85 99);
}
.dark .studio-spark-tooltip-line {
  color: rgb(209 213 219);
}
.studio-spark-tooltip-title {
  margin-bottom: 0.2rem;
  font-weight: 600;
  color: rgb(17 24 39);
}
.dark .studio-spark-tooltip-title {
  color: rgb(243 244 246);
}
</style>
