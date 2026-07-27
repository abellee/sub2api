<template>
  <div
    v-show="sideVisible && (!sidebarCollapsed || sideHasIcon)"
    class="fixed bottom-[108px] left-3 z-[45]"
    :class="[
      mobileOpen ? 'block' : 'hidden lg:block',
      sidebarCollapsed ? (preview ? 'h-16 w-[232px]' : 'h-10 w-12') : 'h-16 w-[232px]'
    ]"
    @mouseenter="showPreview"
    @mouseleave="hidePreview"
  >
    <iframe
      ref="iframeElement"
      class="block h-full w-full border-0 bg-transparent transition-[width,height] duration-200"
      :class="preview ? 'rounded-lg shadow-xl' : ''"
      :src="WIDGET_URL"
      title="Sidebar widget"
      scrolling="no"
      sandbox="allow-scripts allow-same-origin allow-popups"
      @load="handleLoad"
    ></iframe>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useAppStore, useRemoteWidgetsStore } from '@/stores'
import {
  closeFloatingWidget,
  parseFloatingWidgetConfig,
  readFloatingWidgetRecord,
  shouldShowFloatingWidget,
  type FloatingWidgetConfig,
  type FloatingWidgetStorageRecord
} from './floatingWidgetVisibility'

const WIDGET_URL = `https://widget.llmfree.work/side.html?t=${Date.now()}`
const WIDGET_ORIGIN = new URL(WIDGET_URL).origin

const appStore = useAppStore()
const remoteWidgetsStore = useRemoteWidgetsStore()
const { sidebarCollapsed, mobileOpen } = storeToRefs(appStore)
const { sideVisible, sideHasIcon } = storeToRefs(remoteWidgetsStore)
const iframeElement = ref<HTMLIFrameElement | null>(null)
const preview = ref(false)
const config = ref<FloatingWidgetConfig | null>(null)
const storageRecord = ref<FloatingWidgetStorageRecord | null>(null)

function sendMode(): void {
  iframeElement.value?.contentWindow?.postMessage(
    { type: 'side-mode', mode: sidebarCollapsed.value && !preview.value ? 'icon' : 'full' },
    WIDGET_ORIGIN
  )
}

function handleLoad(): void {
  iframeElement.value?.contentWindow?.postMessage({ type: 'side-config-request' }, WIDGET_ORIGIN)
  sendMode()
}

function showPreview(): void {
  if (!sidebarCollapsed.value || !sideHasIcon.value) return
  preview.value = true
  void nextTick(sendMode)
}

function hidePreview(): void {
  if (!preview.value) return
  preview.value = false
  void nextTick(sendMode)
}

function handlePointerMove(event: PointerEvent): void {
  if (!preview.value || !iframeElement.value) return
  const rect = iframeElement.value.getBoundingClientRect()
  if (
    event.clientX < rect.left ||
    event.clientX > rect.right ||
    event.clientY < rect.top ||
    event.clientY > rect.bottom
  ) {
    hidePreview()
  }
}

function handleMessage(event: MessageEvent): void {
  if (event.origin !== WIDGET_ORIGIN || event.source !== iframeElement.value?.contentWindow) return

  if (event.data?.type === 'side-close') {
    if (!config.value?.closable || !storageRecord.value) return
    storageRecord.value = closeFloatingWidget(window.localStorage, config.value, storageRecord.value)
    preview.value = false
    remoteWidgetsStore.setSideState(false, sideHasIcon.value)
    return
  }

  if (event.data?.type === 'side-hover') {
    if (event.data.hovered === true) showPreview()
    else hidePreview()
    return
  }

  if (event.data?.type !== 'side-config') return
  const nextConfig = parseFloatingWidgetConfig(event.data.config)
  if (!nextConfig) {
    config.value = null
    storageRecord.value = null
    remoteWidgetsStore.setSideState(false, false)
    return
  }

  const nextRecord = readFloatingWidgetRecord(window.localStorage, nextConfig)
  const hasIcon = event.data.hasIcon === true ||
    (typeof event.data.icon === 'string' && event.data.icon.trim().length > 0)
  config.value = nextConfig
  storageRecord.value = nextRecord
  remoteWidgetsStore.setSideState(shouldShowFloatingWidget(nextConfig, nextRecord), hasIcon)
  sendMode()
}

watch(sidebarCollapsed, () => {
  preview.value = false
  void nextTick(sendMode)
})

onMounted(() => {
  window.addEventListener('message', handleMessage)
  window.addEventListener('pointermove', handlePointerMove)
})

onBeforeUnmount(() => {
  window.removeEventListener('message', handleMessage)
  window.removeEventListener('pointermove', handlePointerMove)
  remoteWidgetsStore.setSideState(false, false)
})
</script>
