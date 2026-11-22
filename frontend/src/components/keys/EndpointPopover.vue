<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'
import type { CustomEndpoint } from '@/types'

const props = defineProps<{
  apiBaseUrl: string
  customEndpoints: CustomEndpoint[]
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const copiedEndpoint = ref<string | null>(null)

let copiedResetTimer: number | undefined

const allEndpoints = computed(() => {
  const items: Array<{ name: string; endpoint: string; description: string; isDefault: boolean }> = []
  if (props.apiBaseUrl) {
    items.push({
      name: t('keys.endpoints.title'),
      endpoint: props.apiBaseUrl,
      description: '',
      isDefault: true,
    })
  }
  for (const ep of props.customEndpoints) {
    items.push({ ...ep, isDefault: false })
  }
  return items
})

async function copy(url: string) {
  const success = await copyToClipboard(url, t('keys.endpoints.copied'))
  if (!success) return

  copiedEndpoint.value = url
  if (copiedResetTimer !== undefined) {
    window.clearTimeout(copiedResetTimer)
  }
  copiedResetTimer = window.setTimeout(() => {
    if (copiedEndpoint.value === url) {
      copiedEndpoint.value = null
    }
  }, 1800)
}

function tooltipHint(endpoint: string): string {
  return copiedEndpoint.value === endpoint
    ? t('keys.endpoints.copiedHint')
    : t('keys.endpoints.clickToCopy')
}

function speedTestUrl(endpoint: string): string {
  return `https://www.tcptest.cn/http/${encodeURIComponent(endpoint)}`
}

onBeforeUnmount(() => {
  if (copiedResetTimer !== undefined) {
    window.clearTimeout(copiedResetTimer)
  }
})
</script>

<template>
  <div v-if="allEndpoints.length > 0" class="endpoint-list">
    <div
      v-for="(item, index) in allEndpoints"
      :key="index"
      class="endpoint-item"
    >
      <span class="endpoint-name">{{ item.name }}</span>
      <span
        v-if="item.isDefault"
        class="endpoint-default"
      >{{ t('keys.endpoints.default') }}</span>

      <span class="endpoint-divider" aria-hidden="true"></span>

      <div class="group/endpoint endpoint-address relative flex items-center gap-1.5">
        <div
          class="pointer-events-none absolute bottom-full left-1/2 z-20 mb-2 w-max max-w-[24rem] -translate-x-1/2 translate-y-1 rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-left opacity-0 shadow-[0_14px_36px_-20px_rgba(15,23,42,0.35)] ring-1 ring-slate-200/80 transition-all duration-150 group-hover/endpoint:translate-y-0 group-hover/endpoint:opacity-100 group-focus-within/endpoint:translate-y-0 group-focus-within/endpoint:opacity-100 dark:border-slate-700 dark:bg-slate-900 dark:ring-slate-700/70"
        >
          <p
            v-if="item.description"
            class="max-w-[24rem] break-words text-xs leading-5 text-slate-600 dark:text-slate-200"
          >
            {{ item.description }}
          </p>
          <p
            class="flex items-center gap-1.5 text-[11px] leading-4 text-primary-600 dark:text-primary-300"
            :class="item.description ? 'mt-1.5' : ''"
          >
            <span class="h-1.5 w-1.5 rounded-full bg-primary-500 dark:bg-primary-300"></span>
            {{ tooltipHint(item.endpoint) }}
          </p>
          <div class="absolute left-1/2 top-full h-3 w-3 -translate-x-1/2 -translate-y-1/2 rotate-45 border-b border-r border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900"></div>
        </div>

        <code
          class="endpoint-url"
          role="button"
          tabindex="0"
          @click="copy(item.endpoint)"
          @keydown.enter.prevent="copy(item.endpoint)"
          @keydown.space.prevent="copy(item.endpoint)"
        >{{ item.endpoint }}</code>

        <button
          type="button"
          class="rounded p-0.5 transition-colors"
          :class="copiedEndpoint === item.endpoint
            ? 'text-emerald-500 dark:text-emerald-400'
            : 'text-gray-400 hover:text-primary-500 dark:text-gray-500 dark:hover:text-primary-400'"
          :aria-label="tooltipHint(item.endpoint)"
          @click="copy(item.endpoint)"
        >
          <svg v-if="copiedEndpoint === item.endpoint" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
          </svg>
          <svg v-else class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
          </svg>
        </button>

        <a
          :href="speedTestUrl(item.endpoint)"
          target="_blank"
          rel="noopener noreferrer"
          class="rounded p-0.5 text-gray-400 transition-colors hover:text-amber-500 dark:text-gray-500 dark:hover:text-amber-400"
          :title="t('keys.endpoints.speedTest')"
        >
          <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
        </a>
      </div>
    </div>
  </div>
</template>

<style scoped>
.endpoint-list{display:flex;flex-wrap:wrap;gap:8px 16px;min-width:0}
.endpoint-item{display:flex;align-items:center;gap:8px;min-width:0;max-width:100%;padding:8px 10px;border-radius:8px;background:var(--keys-card-surface);font-size:12px;color:var(--studio-muted)}
.endpoint-name{flex-shrink:0;font-weight:500;color:var(--studio-ink)}
.endpoint-default{flex-shrink:0;padding:2px 6px;border-radius:5px;background:var(--studio-haze);color:var(--studio-muted);font-size:10px;line-height:16px}
.endpoint-divider{width:1px;height:12px;flex-shrink:0;background:color-mix(in srgb,var(--studio-muted) 25%,transparent)}
.endpoint-address{min-width:0}.endpoint-url{min-width:0;overflow-wrap:anywhere;font-family:var(--font-mono,monospace);cursor:pointer;text-underline-offset:3px}
.endpoint-url:hover{color:var(--tf-console-active-ink);text-decoration:underline}.endpoint-url:focus-visible{outline:2px solid var(--tf-console-active-ink);outline-offset:3px;border-radius:4px}
.endpoint-address>button,.endpoint-address>a{flex-shrink:0;border-radius:5px;padding:4px;color:var(--studio-muted)}.endpoint-address>button:hover,.endpoint-address>a:hover{background:var(--tf-console-hover);color:var(--tf-console-active-ink)}.endpoint-address>button:focus-visible,.endpoint-address>a:focus-visible{outline:2px solid var(--tf-console-active-ink);outline-offset:2px}
.endpoint-address>button.text-emerald-500{color:#10b981}
</style>
