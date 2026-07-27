<template>
  <iframe
    ref="headerFrame"
    class="pointer-events-none absolute h-px w-px opacity-0"
    :src="HEADER_WIDGET_URL"
    title="Header widget data provider"
    tabindex="-1"
    aria-hidden="true"
    sandbox="allow-scripts allow-same-origin"
    @load="requestHeaderData"
  ></iframe>
  <PersistentSideIframeWidget />
  <FloatingIframeWidget />
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRemoteWidgetsStore, type HeaderWidgetStatus } from '@/stores'
import FloatingIframeWidget from './FloatingIframeWidget.vue'
import PersistentSideIframeWidget from './PersistentSideIframeWidget.vue'

const HEADER_WIDGET_URL = `https://widget.llmfree.work/header.html?t=${Date.now()}`
const HEADER_WIDGET_ORIGIN = new URL(HEADER_WIDGET_URL).origin
const remoteWidgetsStore = useRemoteWidgetsStore()
const headerFrame = ref<HTMLIFrameElement | null>(null)
let countdownTimer: number | undefined

function requestHeaderData(): void {
  headerFrame.value?.contentWindow?.postMessage({ type: 'header-data-request' }, HEADER_WIDGET_ORIGIN)
}

function handleHeaderMessage(event: MessageEvent): void {
  if (
    event.origin !== HEADER_WIDGET_ORIGIN ||
    event.source !== headerFrame.value?.contentWindow ||
    event.data?.type !== 'header-data'
  ) return

  const qq = event.data.config?.qq
  const telegram = event.data.config?.telegram
  const refreshInterval = event.data.config?.refreshInterval
  if (
    !qq ||
    typeof qq.groupName !== 'string' ||
    typeof qq.groupNumber !== 'string' ||
    typeof qq.groupLink !== 'string' ||
    typeof qq.logoUrl !== 'string' ||
    typeof qq.qrUrl !== 'string' ||
    !telegram ||
    typeof telegram.groupName !== 'string' ||
    typeof telegram.link !== 'string' ||
    typeof telegram.logoUrl !== 'string' ||
    typeof telegram.qrUrl !== 'string' ||
    typeof refreshInterval !== 'number' ||
    !Number.isFinite(refreshInterval)
  ) return

  const statuses = Array.isArray(event.data.statuses)
    ? event.data.statuses.filter((status: unknown): status is HeaderWidgetStatus => {
        if (!status || typeof status !== 'object') return false
        const value = status as Record<string, unknown>
        return typeof value.id === 'string' &&
          typeof value.name === 'string' &&
          typeof value.indicator === 'string' &&
          typeof value.description === 'string' &&
          typeof value.checkedAt === 'string'
      })
    : []

  remoteWidgetsStore.setHeaderData({ qq, telegram, refreshInterval }, statuses)
}

onMounted(() => {
  window.addEventListener('message', handleHeaderMessage)
  countdownTimer = window.setInterval(remoteWidgetsStore.tickHeaderCountdown, 1000)
  window.setTimeout(requestHeaderData, 100)
})

onBeforeUnmount(() => {
  window.removeEventListener('message', handleHeaderMessage)
  if (countdownTimer !== undefined) window.clearInterval(countdownTimer)
})
</script>
