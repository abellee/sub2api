<template>
  <div
    class="studio-faces-root"
    :class="{ 'is-hover': wrapHover }"
    @mouseenter="wrapHover = true"
    @mouseleave="wrapHover = false"
  >
    <div v-if="faces.length" class="studio-faces-wrap">
      <div class="studio-faces-meta">
        <span>{{ t('channelMonitorV2.studio.faces.recentCount', { count: faces.length }) }}</span>
        <span>{{ bucketLabel }}</span>
      </div>
      <div
        class="studio-faces"
        :style="{ '--studio-face-cols': String(Math.max(faces.length, 1)) }"
        role="list"
      >
        <button
          v-for="(face, index) in faces"
          :key="face.key"
          type="button"
          role="listitem"
          class="studio-face"
          :class="[
            face.toneClass,
            face.empty ? 'studio-face--empty' : '',
            index === faces.length - 1 ? 'studio-face--current' : '',
            enteringKeys.has(face.key) ? 'studio-face--enter' : '',
            hotKey === face.key ? 'studio-face--hot' : '',
          ]"
          :style="{ '--studio-cell': face.color }"
          :data-face-key="face.key"
          :aria-label="face.aria"
          @mouseenter="onFaceEnter($event, face)"
          @mousemove="move"
          @mouseleave="onFaceLeave"
          @focus="onFaceEnter($event, face)"
          @blur="onFaceLeave"
        >
          <span
            class="studio-face-cell"
            aria-hidden="true"
            @animationend="clearEnter(face.key)"
          />
        </button>
      </div>
      <div class="studio-faces-axis">
        <span>{{ t('channelMonitorV2.studio.faces.past') }}</span>
        <span>{{ t('channelMonitorV2.studio.faces.now') }}</span>
      </div>
    </div>
    <p v-else class="py-6 text-sm text-gray-400">
      {{ t('channelMonitorV2.studio.faces.empty') }}
    </p>

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
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import type { MonitorCoverage } from '@/api/channelMonitorV2'
import { healthModeScore } from '@/features/channel-monitor-v2/monitorFormat'
import { STUDIO_FACE_POP_MS, studioPopIncomingKeys } from './studioDemo'
import { alignBuckets, coverageBucketStarts, isStudioBucketEmpty, type StudioBucketPoint } from './studioBuckets'
import { overallToneFromRow, statusFace, studioFacePalette, type StudioThresholds } from './studioFormat'
import { bucketTooltipLines, emptyTooltipLines, formatSlotTime } from './studioTooltip'
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
const { tooltipRef, state, show, patch, move, hide, isHost } = useStudioHoverTooltip('faces')

const enteringKeys = ref<Set<string>>(new Set())
const seenKeys = ref<Set<string>>(new Set())
const primed = ref(false)
const hotKey = ref('')
const wrapHover = ref(false)
let enterClearTimer = 0
let lastBucketSeconds = 0

const bucketSeconds = computed(() => Math.max(60, props.coverage?.bucket_seconds || 60))

const bucketLabel = computed(() => {
  const minutes = bucketSeconds.value / 60
  if (minutes < 60) return t('channelMonitorV2.bucket.minutes', { count: Math.round(minutes) })
  const hours = minutes / 60
  if (hours < 24) return t('channelMonitorV2.bucket.hours', { count: Math.round(hours) })
  return t('channelMonitorV2.bucket.days', { count: Math.round(hours / 24) })
})

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
      tone: face.tone,
      score,
      color: studioFacePalette(face.tone, score).skin,
      toneClass: toneClass(face.tone),
      empty,
      bucket: empty ? undefined : slot.bucket,
      aria: `${time} · ${toneLabel(face.tone)}`,
    }
  }),
)

function markIncoming(keys: string[]): string[] {
  const seen = seenKeys.value
  if (!primed.value || seen.size === 0) {
    seenKeys.value = new Set(keys)
    primed.value = true
    enteringKeys.value = new Set()
    return []
  }
  const incoming = studioPopIncomingKeys(seen, keys)
  seenKeys.value = new Set(keys)
  enteringKeys.value = new Set()
  return incoming
}

function startPop(keys: string[]) {
  if (!keys.length) return
  enteringKeys.value = new Set(keys)
  window.clearTimeout(enterClearTimer)
  enterClearTimer = window.setTimeout(() => {
    enteringKeys.value = new Set()
  }, STUDIO_FACE_POP_MS + 24)
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
    if (lastBucketSeconds !== 0 && lastBucketSeconds !== step) {
      primed.value = false
      seenKeys.value = new Set()
    }
    lastBucketSeconds = step
    const incoming = markIncoming(nextKeys)
    if (hotKey.value) {
      const hovered = next.find((face) => face.key === hotKey.value)
      if (hovered) patch(faceLines(hovered))
    }
    if (incoming.length) startPop(incoming)
  },
  { flush: 'pre', immediate: true },
)

onBeforeUnmount(() => {
  window.clearTimeout(enterClearTimer)
})
</script>

<style scoped>
.studio-faces-root {
  --studio-faces-bg: rgb(241 245 249);
  --studio-face-pop-ms: 0.5s;
  --studio-face-cols: 18;
  position: relative;
  z-index: auto;
  flex: 0 0 auto;
  align-self: stretch;
  width: 100%;
  min-width: 0;
}
.dark .studio-faces-root {
  --studio-faces-bg: rgb(30 41 59);
}
.studio-faces-wrap {
  position: relative;
  z-index: 0;
  box-sizing: border-box;
  padding: 0.15rem 1rem 0.7rem;
  overflow: hidden;
  background: var(--studio-faces-bg);
  contain: layout style;
}
.studio-faces-meta,
.studio-faces-axis {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
  font-size: 11px;
  line-height: 1.3;
  color: rgb(148 163 184);
}
.studio-faces-meta {
  margin-bottom: 0.4rem;
}
.studio-faces-axis {
  margin-top: 0.35rem;
}
.dark .studio-faces-meta,
.dark .studio-faces-axis {
  color: rgb(148 163 184);
}
.studio-faces {
  --studio-face-min: 0.7rem;
  --studio-face-gap: 0.3rem;
  display: grid;
  grid-template-columns: repeat(
    var(--studio-face-cols, 18),
    minmax(
      min(
        var(--studio-face-min),
        calc((100% - (var(--studio-face-cols, 18) - 1) * var(--studio-face-gap)) / var(--studio-face-cols, 18))
      ),
      1fr
    )
  );
  column-gap: var(--studio-face-gap);
  width: 100%;
  align-items: stretch;
}
.studio-face {
  position: relative;
  display: block;
  width: 100%;
  min-width: 0;
  height: auto;
  aspect-ratio: 3 / 4;
  padding: 0;
  border: 0;
  background: transparent;
  cursor: help;
}
.studio-face:focus-visible {
  outline: 2px solid rgb(20 184 166 / 0.55);
  outline-offset: 1px;
  border-radius: 0.22rem;
}
.studio-face-cell {
  display: block;
  width: 100%;
  height: 100%;
  border-radius: 0.16rem;
  background: var(--studio-cell, #e5e7eb);
  transform-origin: 50% 50%;
  transition: transform 0.16s cubic-bezier(0.22, 1, 0.36, 1), box-shadow 0.16s ease;
}
.studio-face--empty .studio-face-cell {
  opacity: 0.45;
}
.studio-face:hover:not(.studio-face--enter) .studio-face-cell,
.studio-face--hot:not(.studio-face--enter) .studio-face-cell,
.studio-face:focus-visible:not(.studio-face--enter) .studio-face-cell {
  transform: scale(1.08);
  box-shadow: 0 0 0 1px rgb(15 23 42 / 0.08);
}
.studio-face--current .studio-face-cell {
  box-shadow: inset 0 0 0 1.5px rgb(255 255 255 / 0.72);
}
.studio-face--enter .studio-face-cell {
  animation: studio-face-pop var(--studio-face-pop-ms, 0.5s) cubic-bezier(0.33, 1, 0.68, 1) both;
}
@keyframes studio-face-pop {
  0% {
    transform: scale(0.35);
    opacity: 0;
  }
  62% {
    transform: scale(1.08);
    opacity: 1;
  }
  100% {
    transform: scale(1);
    opacity: 1;
  }
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
