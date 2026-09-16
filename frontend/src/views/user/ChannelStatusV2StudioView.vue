<template>
  <AppLayout>
    <div class="space-y-6 pb-12">
      <section
        class="card sticky top-0 z-20 !rounded-3xl !border-0 p-0 shadow-sm ring-1 ring-gray-900/5 backdrop-blur-sm dark:!bg-dark-800 dark:ring-dark-700 supports-[backdrop-filter]:bg-white/95 dark:supports-[backdrop-filter]:bg-dark-800/95"
      >
        <header class="page-header mb-0 flex flex-wrap items-start justify-between gap-4 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:px-6">
          <div class="min-w-0">
            <h1 class="page-title flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
              <span class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-blue-50 text-blue-500 dark:bg-blue-900/30 dark:text-blue-400">
                <Icon name="chart" size="sm" />
              </span>
              {{ t('channelMonitorV2.title') }}
            </h1>
            <div class="page-description mt-1.5 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
              <span class="relative flex h-2 w-2 shrink-0">
                <span
                  class="relative inline-flex h-2 w-2 rounded-full"
                  :class="loading || refreshing ? 'bg-gray-400' : 'bg-green-500'"
                ></span>
              </span>
              <span v-if="refreshing" class="inline-flex items-center gap-1 text-primary-600 dark:text-primary-300">
                <LoadingSpinner size="sm" />
                {{ t('channelMonitorV2.updating') }}
              </span>
              <span v-else-if="snapshot?.coverage.data_through">
                {{ t('channelMonitorV2.updatedTo', { time: formatTime(snapshot.coverage.data_through) }) }}
              </span>
              <span v-else class="text-gray-400">{{ t('common.loading') }}</span>
              <span
                v-if="snapshot && !snapshot.coverage.coverage_complete && !bootstrapActive"
                class="badge badge-warning"
              >
                {{ t('channelMonitorV2.partialCoverage') }}
              </span>
              <span
                v-if="bootstrapActive"
                class="badge badge-primary inline-flex items-center gap-1"
              >
                <LoadingSpinner size="sm" />
                {{ t('channelMonitorV2.bootstrap.progress', { percent: bootstrapPercent }) }}
              </span>
            </div>
          </div>
          <button
            class="btn btn-secondary btn-icon flex h-8 w-8 items-center justify-center rounded-lg bg-gray-100 text-gray-500 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600"
            type="button"
            :title="t('common.refresh')"
            :disabled="loading"
            @click="reload(false)"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          </button>
        </header>

        <div
          v-if="bootstrapActive"
          class="border-b border-blue-100 bg-blue-50/90 px-5 py-3 dark:border-blue-900/40 dark:bg-blue-950/40 sm:px-6"
          role="status"
          aria-live="polite"
        >
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <p class="text-sm font-semibold text-blue-900 dark:text-blue-100">
                {{ t('channelMonitorV2.bootstrap.title') }}
              </p>
              <p class="mt-0.5 text-xs text-blue-800/80 dark:text-blue-200/80">
                {{ t('channelMonitorV2.bootstrap.description') }}
              </p>
            </div>
            <span class="shrink-0 text-xs font-medium tabular-nums text-blue-700 dark:text-blue-300">
              {{ t('channelMonitorV2.bootstrap.progress', { percent: bootstrapPercent }) }}
            </span>
          </div>
          <div
            class="mt-2.5 h-1.5 overflow-hidden rounded-full bg-blue-200/80 dark:bg-blue-900/60"
            role="progressbar"
            :aria-valuenow="bootstrapPercent"
            aria-valuemin="0"
            aria-valuemax="100"
            :aria-label="t('channelMonitorV2.bootstrap.working')"
          >
            <div
              class="h-full rounded-full bg-blue-500 transition-[width] duration-500 ease-out dark:bg-blue-400"
              :style="{ width: `${bootstrapPercent}%` }"
            />
          </div>
        </div>

        <div class="monitor-toolbar flex flex-nowrap items-center gap-3 px-4 py-3 sm:px-5">
          <div class="min-w-0 flex-1 overflow-x-auto">
            <div
              class="tabs inline-flex"
              role="group"
              :aria-label="t('channelMonitorV2.timeRange')"
            >
              <button
                v-for="option in ranges"
                :key="option.value"
                type="button"
                class="tab !px-2 !py-1 text-xs sm:!px-2.5"
                :class="range === option.value ? 'tab-active' : ''"
                @click="setRange(option.value)"
              >
                {{ option.label }}
              </button>
            </div>
          </div>
        </div>
      </section>

      <div
        class="studio-content space-y-10"
        :aria-label="t('channelMonitorV2.studio.groups.title')"
      >
        <template v-if="pageSections.length">
          <section
            v-for="section in pageSections"
            :key="section.key"
            :class="section.kind === 'active' ? 'studio-active-section' : 'studio-brand-section'"
            :aria-label="
              section.kind === 'active'
                ? t('channelMonitorV2.studio.groups.activeAria')
                : t('channelMonitorV2.studio.brands.sectionAria', { label: section.brandLabel })
            "
          >
            <header class="studio-brand-heading mb-4 flex items-center gap-2 px-0.5">
              <span
                class="studio-brand-heading-mark"
                :style="
                  section.kind === 'brand'
                    ? { '--studio-brand-color': studioBrandFill(section.platform) }
                    : undefined
                "
              >
                <Icon
                  v-if="section.kind === 'active'"
                  name="radar"
                  size="xs"
                  class="studio-active-mark-icon"
                />
                <StudioBrandIcon v-else :platform="section.platform" size="sm" />
              </span>
              <h2 class="studio-brand-heading-title text-sm font-extrabold tracking-tight text-gray-900 dark:text-white">
                {{ section.brandLabel }}
              </h2>
            </header>
            <div class="studio-card-grid">
              <div
                v-for="card in section.cards"
                :key="card.key"
                class="studio-model-row overflow-hidden rounded-3xl shadow-sm ring-1 ring-gray-900/5 dark:ring-dark-700"
              >
                <div class="studio-model-card">
                  <StudioKpiCard
                    joined
                    :label="card.label"
                    :platform="card.platform"
                    :brand-label="card.brandLabel"
                    :rate-label="card.rateLabel"
                    :success-label="card.successLabel"
                    :success-rate="card.successRate"
                    :ttft-label="card.ttftLabel"
                    :ttft="card.ttft"
                    :cache-label="card.cacheLabel"
                    :cache-rate="card.cacheRate"
                    :status-heading="card.statusHeading"
                    :status-label="card.statusLabel"
                    :status-note="card.statusNote"
                    :title="card.title"
                    :state="card.state"
                    :success-state="card.successState"
                    :ttft-state="card.ttftState"
                    :cache-state="card.cacheState"
                    :accent="card.accent"
                    :buckets="card.buckets"
                    :coverage="faceCoverage"
                    :show-throughput="showThroughput"
                  />
                </div>
                <StudioStatusFaces
                  :buckets="card.buckets"
                  :coverage="faceCoverage"
                  :show-throughput="showThroughput"
                  :thresholds="healthThresholds"
                  :row-index="card.rowIndex"
                />
              </div>
            </div>
          </section>
        </template>
        <section
          v-else-if="loading"
          class="flex w-full flex-col gap-3"
          aria-hidden="true"
        >
          <div
            v-for="i in 4"
            :key="i"
            class="h-28 animate-pulse rounded-3xl bg-gray-50 dark:bg-dark-900/30"
          />
        </section>
        <div
          v-else
          class="card flex min-h-[8.25rem] items-center justify-center !rounded-3xl !border-0 text-sm text-gray-400 shadow-sm ring-1 ring-gray-900/5 dark:ring-dark-700"
        >
          {{ t('channelMonitorV2.studio.groups.empty') }}
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import StudioBrandIcon from '@/features/channel-monitor-v2-studio/StudioBrandIcon.vue'
import StudioKpiCard from '@/features/channel-monitor-v2-studio/StudioKpiCard.vue'
import StudioStatusFaces from '@/features/channel-monitor-v2-studio/StudioStatusFaces.vue'
import { studioBrandFill } from '@/features/channel-monitor-v2-studio/studioBrand'
import {
  groupStudioCardsByBrand,
  pickStudioActiveGroups,
  sortStudioCardsByStatus,
} from '@/features/channel-monitor-v2-studio/studioFormat'
import { useStudioGroupCards } from '@/features/channel-monitor-v2-studio/useStudioGroupCards'
import type { MonitorRange } from '@/api/channelMonitorV2'

const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()

const ranges = computed(() => [
  { value: '90m' as MonitorRange, label: t('channelMonitorV2.ranges.90m') },
  { value: '24h' as MonitorRange, label: t('channelMonitorV2.ranges.24h') },
  { value: '7d' as MonitorRange, label: t('channelMonitorV2.ranges.7d') },
  { value: '30d' as MonitorRange, label: t('channelMonitorV2.ranges.30d') },
])

const range = ref<MonitorRange>(parseRange(route.query.range))
const {
  loading,
  refreshing,
  snapshot,
  groupCards,
  healthThresholds,
  coverage: faceCoverage,
  bootstrapActive,
  bootstrapPercent,
  showThroughput,
  reload,
} = useStudioGroupCards({ range })

const pageSections = computed(() => {
  let rowIndex = 0
  const withRowIndex = (cards: typeof groupCards.value) =>
    cards.map((card) => ({ ...card, rowIndex: rowIndex++ }))
  const activeCards = withRowIndex(sortStudioCardsByStatus(pickStudioActiveGroups(groupCards.value)))
  const brandSections = groupStudioCardsByBrand(groupCards.value).map((section) => ({
    ...section,
    kind: 'brand' as const,
    cards: withRowIndex(section.cards),
  }))
  if (!activeCards.length) return brandSections
  return [
    {
      key: 'active',
      kind: 'active' as const,
      brandLabel: t('channelMonitorV2.studio.groups.active'),
      cards: activeCards,
    },
    ...brandSections,
  ]
})

function parseRange(value: unknown): MonitorRange {
  return ['90m', '24h', '7d', '30d'].includes(String(value)) ? (value as MonitorRange) : '90m'
}

function syncQuery() {
  void router.replace({
    query: {
      range: range.value,
    },
  })
}

function setRange(value: MonitorRange) {
  range.value = value
}

function formatTime(value: string) {
  return new Intl.DateTimeFormat(locale.value || undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

watch(range, () => {
  syncQuery()
})
</script>

<style scoped>
.studio-brand-heading-mark {
  display: inline-flex;
  width: 1.25rem;
  height: 1.25rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  color: var(--studio-brand-color, #14b8a6);
  background: color-mix(in srgb, var(--studio-brand-color, #14b8a6) 16%, white);
}
.dark .studio-brand-heading-mark {
  background: color-mix(in srgb, var(--studio-brand-color, #14b8a6) 24%, rgb(15 23 42));
}
.studio-active-mark-icon {
  color: inherit;
}
.studio-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 20rem), 1fr));
  gap: 1.5rem;
}
.studio-model-row {
  --studio-faces-bg: rgb(241 245 249);
  position: relative;
  display: flex;
  width: 100%;
  min-width: 0;
  flex-direction: column;
  align-items: stretch;
  background: var(--studio-faces-bg);
  contain: layout style;
}
.studio-model-card {
  position: relative;
  z-index: 2;
  width: 100%;
}
.dark .studio-model-row {
  --studio-faces-bg: rgb(30 41 59);
}
</style>
