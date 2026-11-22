import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch, type Ref } from 'vue'
import {
  STUDIO_TOOLTIP_HIDE_MS,
  clampTooltipBox,
  eventTooltipPoint,
  type StudioTooltipPoint,
} from './studioTooltipUi'

type TooltipState = {
  visible: boolean
  x: number
  y: number
  lines: string[]
}

type TooltipBucket = {
  state: TooltipState
  tooltipRef: Ref<HTMLElement | null>
  hideTimer: number
  lastPoint: StudioTooltipPoint
  hosts: number
  owner: Ref<number>
  nextId: number
}

const sharedBuckets = new Map<string, TooltipBucket>()

function createBucket(): TooltipBucket {
  return {
    state: reactive({
      visible: false,
      x: 0,
      y: 0,
      lines: [] as string[],
    }),
    tooltipRef: ref<HTMLElement | null>(null),
    hideTimer: 0,
    lastPoint: { x: 0, y: 0 },
    hosts: 0,
    owner: ref(0),
    nextId: 0,
  }
}

/** One floating panel: moving between faces/dots updates copy instead of remounting. */
export function useStudioHoverTooltip(sharedKey?: string) {
  const bucket = sharedKey
    ? sharedBuckets.get(sharedKey) || sharedBuckets.set(sharedKey, createBucket()).get(sharedKey)!
    : createBucket()
  const myId = ++bucket.nextId
  if (!sharedKey) bucket.owner.value = myId

  function place(point: StudioTooltipPoint) {
    bucket.lastPoint = point
    const el = bucket.tooltipRef.value
    const box = clampTooltipBox(point, el?.offsetWidth || 184, el?.offsetHeight || 80)
    bucket.state.x = box.x
    bucket.state.y = box.y
  }

  function reflow() {
    if (!bucket.state.visible) return
    place(bucket.lastPoint)
  }

  function show(event: MouseEvent | PointerEvent | FocusEvent, lines: string[]) {
    window.clearTimeout(bucket.hideTimer)
    bucket.state.lines = lines
    bucket.state.visible = true
    place(eventTooltipPoint(event))
    void nextTick(() => {
      place(bucket.lastPoint)
      requestAnimationFrame(reflow)
    })
  }

  function patch(lines: string[]) {
    if (!bucket.state.visible) return
    bucket.state.lines = lines
    void nextTick(() => {
      place(bucket.lastPoint)
      requestAnimationFrame(reflow)
    })
  }

  function move(event: MouseEvent | PointerEvent) {
    if (!bucket.state.visible) return
    place(eventTooltipPoint(event))
  }

  function hide() {
    window.clearTimeout(bucket.hideTimer)
    bucket.hideTimer = window.setTimeout(() => {
      bucket.state.visible = false
    }, STUDIO_TOOLTIP_HIDE_MS)
  }

  function hideNow() {
    window.clearTimeout(bucket.hideTimer)
    bucket.state.visible = false
  }

  let mounted = false

  onMounted(() => {
    mounted = true
    bucket.hosts += 1
    if (!bucket.owner.value) bucket.owner.value = myId
    window.addEventListener('resize', reflow)
  })

  watch(bucket.owner, (id) => {
    if (!mounted || id || !bucket.hosts) return
    bucket.owner.value = myId
  })

  onBeforeUnmount(() => {
    mounted = false
    window.removeEventListener('resize', reflow)
    bucket.hosts -= 1
    if (bucket.owner.value === myId) bucket.owner.value = 0
    if (bucket.hosts <= 0) hideNow()
  })

  const isHost = computed(() => bucket.owner.value === myId)

  return { tooltipRef: bucket.tooltipRef, state: bucket.state, show, patch, move, hide, hideNow, isHost }
}
