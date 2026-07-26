<template>
  <div
    v-show="isVisible"
    ref="widgetElement"
    class="group fixed z-40 overflow-hidden rounded-xl bg-transparent shadow-xl ring-1 ring-black/10 transition-[left,top,width,height] duration-200 ease-out"
    :style="widgetStyle"
    role="complementary"
    aria-label="LLMFree status widget"
  >
    <button
      v-if="config?.closable"
      type="button"
      class="pointer-events-none absolute right-2 top-2 z-10 flex h-7 w-7 items-center justify-center rounded-md bg-gray-900/80 text-lg leading-none text-white opacity-0 shadow-sm backdrop-blur-sm transition-[color,background-color,opacity] hover:bg-gray-900 group-hover:pointer-events-auto group-hover:opacity-100 focus:pointer-events-auto focus:opacity-100 focus:outline-none focus:ring-2 focus:ring-white/80"
      aria-label="关闭浮窗"
      title="关闭"
      @click="hideWidget"
    >
      <span aria-hidden="true">&times;</span>
    </button>

    <iframe
      ref="iframeElement"
      class="absolute inset-0 block h-full w-full border-0 bg-transparent"
      :src="WIDGET_URL"
      title="LLMFree support widget"
      scrolling="no"
      @load="requestConfig"
    ></iframe>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  fitFloatingWidgetSize,
  getVisibleMainBounds,
  settleFloatingWidgetPosition,
  type FloatingWidgetBounds,
  type FloatingWidgetSize
} from './floatingWidgetGeometry'
import {
  closeFloatingWidget,
  parseFloatingWidgetConfig,
  readFloatingWidgetRecord,
  shouldShowFloatingWidget,
  type FloatingWidgetConfig,
  type FloatingWidgetStorageRecord
} from './floatingWidgetVisibility'

const WIDGET_URL = 'https://widget.llmfree.work/float.html?v=202607270310'
const WIDGET_ORIGIN = new URL(WIDGET_URL).origin
const DEFAULT_SIZE: FloatingWidgetSize = { width: 180, height: 100 }

const widgetElement = ref<HTMLElement | null>(null)
const iframeElement = ref<HTMLIFrameElement | null>(null)
const config = ref<FloatingWidgetConfig | null>(null)
const storageRecord = ref<FloatingWidgetStorageRecord | null>(null)
const isVisible = ref(false)
const reportedSize = reactive<FloatingWidgetSize>({ ...DEFAULT_SIZE })
const size = reactive<FloatingWidgetSize>({ ...DEFAULT_SIZE })
const position = reactive({ x: 0, y: 0 })
let mainElement: HTMLElement | null = null
let mainResizeObserver: ResizeObserver | null = null
let layoutFrame = 0

const widgetStyle = computed(() => ({
  left: `${position.x}px`,
  top: `${position.y}px`,
  width: `${size.width}px`,
  height: `${size.height}px`
}))

function getLayout(): { mainRect: DOMRect; bounds: FloatingWidgetBounds } | null {
  if (!mainElement) return null

  const mainRect = mainElement.getBoundingClientRect()
  return {
    mainRect,
    bounds: getVisibleMainBounds(mainRect, window.innerWidth, window.innerHeight)
  }
}

function updateLayout(): void {
  const layout = getLayout()
  if (!layout) return

  Object.assign(size, fitFloatingWidgetSize(reportedSize, layout.mainRect, layout.bounds))
  Object.assign(
    position,
    settleFloatingWidgetPosition(position, size, layout.bounds, 'bottom').position
  )
}

function scheduleLayout(): void {
  cancelAnimationFrame(layoutFrame)
  layoutFrame = requestAnimationFrame(updateLayout)
}

function applyConfig(value: unknown): void {
  const nextConfig = parseFloatingWidgetConfig(value)
  if (!nextConfig) {
    config.value = null
    storageRecord.value = null
    isVisible.value = false
    return
  }

  const nextRecord = readFloatingWidgetRecord(window.localStorage, nextConfig)
  config.value = nextConfig
  storageRecord.value = nextRecord
  isVisible.value = shouldShowFloatingWidget(nextConfig, nextRecord)
}

function hideWidget(): void {
  if (!config.value?.closable || !storageRecord.value) return

  storageRecord.value = closeFloatingWidget(window.localStorage, config.value, storageRecord.value)
  isVisible.value = false
}

function handleWidgetMessage(event: MessageEvent): void {
  if (event.origin !== WIDGET_ORIGIN || event.source !== iframeElement.value?.contentWindow) return

  if (event.data?.type === 'float-config') {
    applyConfig(event.data.config)
    return
  }

  if (event.data?.type !== 'float-resize') return

  const width = Number(event.data.width)
  const height = Number(event.data.height)
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) return

  reportedSize.width = width
  reportedSize.height = height
  scheduleLayout()
}

function requestConfig(): void {
  iframeElement.value?.contentWindow?.postMessage({ type: 'float-config-request' }, WIDGET_ORIGIN)
}

onMounted(async () => {
  await nextTick()
  mainElement = widgetElement.value?.parentElement ?? null
  if (!mainElement) return

  mainResizeObserver = new ResizeObserver(scheduleLayout)
  mainResizeObserver.observe(mainElement)

  window.addEventListener('message', handleWidgetMessage)
  window.addEventListener('resize', scheduleLayout)
  window.addEventListener('scroll', scheduleLayout, true)
  updateLayout()
})

onBeforeUnmount(() => {
  cancelAnimationFrame(layoutFrame)
  mainResizeObserver?.disconnect()
  window.removeEventListener('message', handleWidgetMessage)
  window.removeEventListener('resize', scheduleLayout)
  window.removeEventListener('scroll', scheduleLayout, true)
})
</script>
