<template>
  <section
    class="card flex h-full min-h-[280px] flex-col !rounded-3xl !border-0 !p-6 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700"
  >
    <div class="card-header mb-4 flex shrink-0 items-start justify-between gap-3 !border-0 !p-0">
      <div class="min-w-0">
        <h2 class="text-sm font-bold text-gray-900 dark:text-white">
          {{ t('channelMonitorV2.studio.health.title') }}
        </h2>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
          {{ t('channelMonitorV2.studio.health.description') }}
        </p>
      </div>
    </div>

    <div class="card-body flex flex-1 flex-col items-center justify-center gap-6 !p-0 lg:flex-row lg:items-center">
      <div class="relative h-40 w-40 shrink-0">
        <svg class="h-full w-full" viewBox="0 0 120 120" aria-hidden="true">
          <circle
            cx="60"
            cy="60"
            r="48"
            fill="none"
            stroke="currentColor"
            class="text-gray-100 dark:text-dark-700"
            stroke-width="12"
          />
          <circle
            cx="60"
            cy="60"
            r="48"
            fill="none"
            :stroke="stroke"
            stroke-width="12"
            stroke-linecap="round"
            pathLength="100"
            :stroke-dasharray="`${ringValue} 100`"
            transform="rotate(-90 60 60)"
          />
        </svg>
        <div class="absolute inset-0 flex flex-col items-center justify-center">
          <strong class="text-3xl font-black tabular-nums text-gray-900 dark:text-white">{{ scoreLabel }}</strong>
          <span class="mt-0.5 max-w-[6.5rem] text-center text-[10px] font-bold uppercase tracking-[0.12em] text-gray-400">
            {{ score == null ? t('channelMonitorV2.studio.health.unknown') : t('channelMonitorV2.studio.health.score') }}
          </span>
        </div>
      </div>

      <ul class="w-full min-w-0 flex-1 space-y-3">
        <li v-for="row in rows" :key="row.key">
          <div class="mb-1 flex items-center justify-between gap-2 text-xs">
            <span class="font-medium text-gray-600 dark:text-gray-300">{{ row.label }}</span>
            <span class="tabular-nums text-gray-900 dark:text-white">{{ row.value }}</span>
          </div>
          <div class="h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
            <i
              class="block h-full rounded-full"
              :class="row.barClass"
              :style="{ width: `${row.percent}%` }"
            ></i>
          </div>
        </li>
      </ul>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed } from 'vue'
import type { MonitorHealth, MonitorMetric } from '@/api/channelMonitorV2'
import {
  formatLatencyPrivacy,
  formatMonitorPercent,
  ttftDisplayState,
} from '@/features/channel-monitor-v2/monitorFormat'
import { scoreStroke } from './studioFormat'

const props = defineProps<{
  metrics: MonitorMetric
  health: MonitorHealth
}>()

const { t } = useI18n()

const score = computed(() => {
  const raw = props.health.score
  return raw == null || Number.isNaN(raw) ? null : Math.max(0, Math.min(100, raw))
})
const scoreLabel = computed(() => (score.value == null ? '—' : `${Math.round(score.value)}`))
const ringValue = computed(() => score.value ?? 0)
const stroke = computed(() => scoreStroke(score.value))

const rows = computed(() => {
  const ttftState = ttftDisplayState(props.health.ttft, props.metrics.ttft)
  const ttftScore = ttftState === 'unknown' ? null : props.health.ttft_score
  return [
    {
      key: 'success',
      label: t('channelMonitorV2.metrics.successRate'),
      value: formatMonitorPercent(1 - (props.metrics.error_rate || 0)),
      percent: Math.max(2, (1 - (props.metrics.error_rate || 0)) * 100),
      barClass: 'bg-teal-500',
    },
    {
      key: 'ttft',
      label: t('channelMonitorV2.metrics.ttftP50'),
      value: formatLatencyPrivacy(
        props.metrics.ttft.p50_ms,
        props.metrics.ttft.p90_ms,
        props.metrics.ttft.avg_ms,
        props.metrics.ttft.p95_ms,
      ),
      percent: ttftScore == null ? 0 : Math.max(2, Math.min(100, ttftScore)),
      barClass: 'bg-orange-400',
    },
    {
      key: 'cache',
      label: t('channelMonitorV2.metrics.cacheRate'),
      value: formatMonitorPercent(props.metrics.cache_rate || 0),
      percent: Math.max(2, (props.metrics.cache_rate || 0) * 100),
      barClass: 'bg-amber-400',
    },
  ]
})
</script>
