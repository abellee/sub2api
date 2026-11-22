<template>
  <section
    class="card flex min-h-[280px] flex-col overflow-hidden !rounded-3xl !border-0 !p-6 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700"
  >
    <div class="card-header mb-4 flex shrink-0 flex-wrap items-start justify-between gap-3 !border-0 !p-0">
      <div class="min-w-0">
        <h2 class="text-sm font-bold text-gray-900 dark:text-white">
          {{ t('channelMonitorV2.studio.journey.title') }}
        </h2>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
          {{ t('channelMonitorV2.studio.journey.description') }}
        </p>
      </div>
      <span v-if="coverage" class="badge badge-gray shrink-0">{{ bucketLabel }}</span>
    </div>

    <div class="card-body min-h-0 flex-1 !p-0">
      <div
        v-if="stages.length"
        class="studio-journey flex snap-x snap-mandatory gap-3 overflow-x-auto pb-1"
        role="list"
      >
        <button
          v-for="(stage, index) in stages"
          :key="stage.key"
          type="button"
          role="listitem"
          class="studio-stage snap-start"
          :class="stage.active ? 'studio-stage--active' : ''"
          :aria-label="t('channelMonitorV2.studio.journey.selectAria', { label: stage.label })"
          :aria-pressed="stage.active"
          @click="emit('select', stage.row)"
        >
          <span class="studio-stage-accent" :class="stage.toneClass"></span>
          <div class="flex items-center justify-between gap-2">
            <span class="text-[10px] font-bold uppercase tracking-[0.16em] text-gray-400">
              {{ t('channelMonitorV2.studio.journey.stage', { n: String(index + 1).padStart(2, '0') }) }}
            </span>
            <span
              class="inline-flex rounded-full border px-1.5 py-0.5 text-[10px] font-semibold"
              :class="platformBadgeLightClass(stage.row.platform)"
            >{{ stage.row.platform }}</span>
          </div>
          <strong class="mt-2 block truncate text-sm font-semibold text-gray-900 dark:text-white" :title="stage.label">
            {{ stage.title }}
          </strong>
          <p class="mt-0.5 truncate text-[11px] text-gray-400">{{ stage.subtitle }}</p>
          <div class="mt-3 flex items-end justify-between gap-2">
            <div>
              <span class="block text-lg font-black tabular-nums text-gray-900 dark:text-white">{{ stage.success }}</span>
              <small class="text-[10px] text-gray-400">{{ t('channelMonitorV2.metrics.successRate') }}</small>
            </div>
            <div class="text-right">
              <span class="block text-sm font-semibold tabular-nums text-gray-700 dark:text-gray-200">{{ stage.ttft }}</span>
              <small class="text-[10px] text-gray-400">{{ t('channelMonitorV2.metrics.ttftP50') }}</small>
            </div>
          </div>
          <div class="studio-stage-pulse mt-3" aria-hidden="true">
            <i
              v-for="cell in stage.pulse"
              :key="cell.key"
              class="studio-stage-cell"
              :class="cell.className"
            ></i>
          </div>
        </button>
      </div>
      <div v-else class="flex min-h-[200px] items-center justify-center py-8">
        <EmptyState
          :title="t('channelMonitorV2.studio.journey.empty')"
          :description="t('channelMonitorV2.empty.description')"
        />
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed } from 'vue'
import type { MonitorCoverage, MonitorMatrixRow } from '@/api/channelMonitorV2'
import EmptyState from '@/components/common/EmptyState.vue'
import { formatMonitorMs, formatMonitorSuccessRateFromError, healthScoreClass } from '@/features/channel-monitor-v2/monitorFormat'
import { platformBadgeLightClass } from '@/utils/platformColors'
import { scoreTone } from './studioFormat'

const props = defineProps<{
  rows: MonitorMatrixRow[]
  coverage: MonitorCoverage | null
  selectedKey?: string
}>()

const emit = defineEmits<{
  select: [row: MonitorMatrixRow]
}>()

const { t } = useI18n()

const bucketLabel = computed(() => {
  const seconds = props.coverage?.bucket_seconds || 60
  const minutes = seconds / 60
  if (minutes < 60) return t('channelMonitorV2.bucket.minutes', { count: minutes })
  const hours = minutes / 60
  if (hours < 24) return t('channelMonitorV2.bucket.hours', { count: hours })
  return t('channelMonitorV2.bucket.days', { count: hours / 24 })
})

const stages = computed(() =>
  props.rows.map((row, index) => {
    const title = row.group_name || row.model || row.platform
    const subtitleParts = [row.platform]
    if (row.group_name && row.group_name !== title) subtitleParts.push(row.group_name)
    if (row.model && row.model !== title) {
      subtitleParts.push(row.model === '__other__' ? t('channelMonitorV2.otherModels') : row.model)
    }
    const key = [row.platform, row.group_id || 0, row.model || ''].join(':')
    const tone = scoreTone(row.health.score)
    const buckets = row.buckets || []
    const tail = buckets.slice(-16)
    return {
      key,
      row,
      title: title === '__other__' ? t('channelMonitorV2.otherModels') : title,
      subtitle: subtitleParts.filter((part, i, arr) => arr.indexOf(part) === i).join(' · '),
      label: subtitleParts.join(' / '),
      success: formatMonitorSuccessRateFromError(row.metrics.error_rate),
      ttft: formatMonitorMs(row.metrics.ttft.p50_ms),
      active: props.selectedKey === key,
      toneClass:
        tone === 'healthy'
          ? 'bg-teal-400'
          : tone === 'warning'
            ? 'bg-amber-400'
            : tone === 'critical'
              ? 'bg-rose-400'
              : 'bg-gray-300 dark:bg-dark-600',
      pulse: tail.map((bucket, cellIndex) => ({
        key: `${key}:${bucket.bucket_start}:${cellIndex}`,
        className: healthScoreClass(bucket.health, 'overall', bucket.metrics.request_count),
      })),
      index,
    }
  }),
)
</script>

<style scoped>
.studio-stage {
  position: relative;
  display: flex;
  width: min(16.5rem, 78vw);
  flex: none;
  flex-direction: column;
  overflow: hidden;
  border-radius: 1.25rem;
  background: rgb(255 255 255);
  padding: 1rem 1rem 0.9rem;
  text-align: left;
  box-shadow: 0 1px 2px rgb(15 23 42 / 0.04);
  outline: 1px solid rgb(15 23 42 / 0.06);
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}
:global(.dark) .studio-stage {
  background: rgb(17 24 39 / 0.55);
  outline-color: rgb(55 65 81 / 0.8);
}
.studio-stage:hover {
  transform: translateY(-1px);
  box-shadow: 0 10px 24px -16px rgb(15 23 42 / 0.35);
}
.studio-stage--active {
  outline: 2px solid rgb(20 184 166);
}
.studio-stage-accent {
  position: absolute;
  inset: 0 auto auto 0;
  height: 3px;
  width: 100%;
}
.studio-stage-pulse {
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: 1fr;
  gap: 3px;
  height: 10px;
}
.studio-stage-cell {
  display: block;
  height: 100%;
  border-radius: 999px;
  opacity: 0.92;
}
.health-score10 { background: #14b8a6; }
.health-score9  { background: #2dd4bf; }
.health-score8  { background: #5eead4; }
.health-score7  { background: #a3e635; }
.health-score6  { background: #facc15; }
.health-score5  { background: #fbbf24; }
.health-score4  { background: #f59e0b; }
.health-score3  { background: #fb923c; }
.health-score2  { background: #fb7185; }
.health-score1  { background: #f87171; }
.health-score0  { background: #f43f5e; }
.health-healthy  { background: #14b8a6; }
.health-warning  { background: #f59e0b; }
.health-critical { background: #f43f5e; }
.health-unknown  { background: #d1d5db; }
</style>
