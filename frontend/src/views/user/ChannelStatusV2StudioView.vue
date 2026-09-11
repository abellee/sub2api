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
          <span class="ml-auto hidden shrink-0 text-right text-[11px] leading-tight text-gray-400 sm:inline">
            {{ t('channelMonitorV2.studio.faces.panHint') }}
          </span>
        </div>
      </section>

      <div
        class="studio-content space-y-10"
        :aria-label="t('channelMonitorV2.studio.groups.title')"
      >
        <template v-if="brandSections.length">
          <section
            v-for="section in brandSections"
            :key="section.key"
            class="studio-brand-section"
            :aria-label="t('channelMonitorV2.studio.brands.sectionAria', { label: section.brandLabel })"
          >
            <header class="studio-brand-heading mb-4 flex items-center gap-3 px-0.5">
              <span
                class="studio-brand-heading-mark"
                :style="{ '--studio-brand-color': studioBrandFill(section.platform) }"
              >
                <StudioBrandIcon :platform="section.platform" size="lg" />
              </span>
              <h2 class="text-base font-extrabold tracking-tight text-gray-900 dark:text-white">
                {{ section.brandLabel }}
              </h2>
            </header>
            <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
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
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import StudioBrandIcon from '@/features/channel-monitor-v2-studio/StudioBrandIcon.vue'
import StudioKpiCard from '@/features/channel-monitor-v2-studio/StudioKpiCard.vue'
import StudioStatusFaces from '@/features/channel-monitor-v2-studio/StudioStatusFaces.vue'
import { rowHasActivity } from '@/features/channel-monitor-v2-studio/studioDemo'
import { studioBrandFill } from '@/features/channel-monitor-v2-studio/studioBrand'
import {
  asStudioGroupCatalog,
  cacheTone,
  collectStudioGroupRates,
  errorRateTone,
  formatStudioMultiplier,
  formatStudioRateNumber,
  groupStudioCardsByBrand,
  lookupStudioGroupRate,
  overallToneFromRow,
  studioPlatform,
  studioPlatformLabel,
  ttftTone,
  type StudioAccent,
  type StudioGroupRateIndex,
} from '@/features/channel-monitor-v2-studio/studioFormat'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { isChannelMonitorThroughputHidden } from '@/utils/featureFlags'
import * as api from '@/api/channelMonitorV2'
import { groupsAPI } from '@/api/admin/groups'
import { userGroupsAPI } from '@/api/groups'
import type {
  MonitorFilter,
  MonitorMatrixResponse,
  MonitorMatrixRow,
  MonitorRange,
  MonitorSnapshot,
} from '@/api/channelMonitorV2'
import {
  formatLatencyPrivacy,
  formatMonitorMs,
  formatMonitorPercent,
} from '@/features/channel-monitor-v2/monitorFormat'

const ACCENTS: StudioAccent[] = ['teal', 'coral', 'indigo', 'amber', 'sky']

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()
const { t, locale } = useI18n()
const isAdmin = computed(() => authStore.isAdmin)
const showThroughput = computed(() => isAdmin.value || !isChannelMonitorThroughputHidden())

const ranges = computed(() => [
  { value: '90m' as MonitorRange, label: t('channelMonitorV2.ranges.90m') },
  { value: '24h' as MonitorRange, label: t('channelMonitorV2.ranges.24h') },
  { value: '7d' as MonitorRange, label: t('channelMonitorV2.ranges.7d') },
  { value: '30d' as MonitorRange, label: t('channelMonitorV2.ranges.30d') },
])

const range = ref<MonitorRange>(parseRange(route.query.range))
const snapshot = ref<MonitorSnapshot | null>(null)
const matrix = ref<MonitorMatrixResponse | null>(null)
const EMPTY_GROUP_RATES: StudioGroupRateIndex = { byId: new Map(), byName: new Map() }
const groupRates = ref<StudioGroupRateIndex>(EMPTY_GROUP_RATES)
const loading = ref(false)
const refreshing = ref(false)
let controller: AbortController | null = null
let sequence = 0
let autoRefreshTimer: number | null = null

const emptyFilter = computed<MonitorFilter>(() => ({
  range: range.value,
  platforms: [],
  groupIds: [],
  models: [],
}))

const bootstrapActive = computed(() => Boolean(snapshot.value?.coverage?.bootstrap?.active))
const bootstrapPercent = computed(() => {
  const raw = snapshot.value?.coverage?.bootstrap?.progress_percent
  if (typeof raw !== 'number' || Number.isNaN(raw)) return 0
  return Math.min(100, Math.max(0, Math.round(raw)))
})

const healthThresholds = computed(() => snapshot.value?.config?.health_thresholds || null)
const faceCoverage = computed(() => matrix.value?.coverage || snapshot.value?.coverage || null)

const groupRows = computed(() => {
  const items = matrix.value?.items || []
  return [...items]
    .filter((row) => row.group_id != null && Number(row.group_id) > 0)
    .sort((a, b) => (b.metrics.rpm || 0) - (a.metrics.rpm || 0))
})

const groupCards = computed(() => {
  const thresholds = healthThresholds.value
  const rates = groupRates.value
  const source = groupRows.value.filter(rowHasActivity).map((row) => ({
    key: groupKey(row),
    platform: row.platform,
    groupId: row.group_id,
    label: groupTitle(row),
    rate: undefined as number | undefined,
    metrics: row.metrics,
    health: row.health,
    buckets: row.buckets ?? [],
  }))
  return source.map((row, index) => {
    const state = overallToneFromRow(row.metrics, row.health, thresholds)
    const rate = lookupStudioGroupRate(rates, row.groupId, row.label, row.rate)
    return {
      key: row.key,
      label: row.label,
      platform: studioPlatform(row.platform),
      brandLabel: studioPlatformLabel(row.platform),
      rateLabel: formatStudioMultiplier(rate, t('channelMonitorV2.studio.groups.userRate', { n: formatStudioRateNumber(rate) })),
      successLabel: t('channelMonitorV2.metrics.successRate'),
      successRate: formatMonitorPercent(1 - (row.metrics.error_rate || 0)),
      ttftLabel: t('channelMonitorV2.metrics.ttft'),
      ttft: formatMonitorMs(row.metrics.ttft.p50_ms),
      cacheLabel: t('channelMonitorV2.metrics.cacheRate'),
      cacheRate: formatMonitorPercent(row.metrics.cache_rate),
      statusHeading: t('channelMonitorV2.studio.status.label'),
      statusLabel: t(`channelMonitorV2.studio.status.${state}`),
      title: formatLatencyPrivacy(
        row.metrics.ttft.p50_ms,
        row.metrics.ttft.p90_ms,
        row.metrics.ttft.avg_ms,
        row.metrics.ttft.p95_ms,
      ),
      state,
      successState: errorRateTone(row.metrics.error_rate, thresholds),
      ttftState: ttftTone(row.metrics.ttft.p50_ms, thresholds, row.metrics.ttft),
      cacheState: cacheTone(row.metrics.cache_rate, thresholds),
      accent: ACCENTS[index % ACCENTS.length],
      buckets: row.buckets,
    }
  })
})

const brandSections = computed(() => {
  let rowIndex = 0
  return groupStudioCardsByBrand(groupCards.value).map((section) => ({
    ...section,
    cards: section.cards.map((card) => ({ ...card, rowIndex: rowIndex++ })),
  }))
})

function parseRange(value: unknown): MonitorRange {
  return ['90m', '24h', '7d', '30d'].includes(String(value)) ? (value as MonitorRange) : '90m'
}

function groupKey(row: MonitorMatrixRow) {
  return `${row.platform}:${row.group_id}`
}

function groupTitle(row: MonitorMatrixRow) {
  return row.group_name || (row.group_id != null ? `#${row.group_id}` : row.platform)
}

function syncQuery() {
  void router.replace({
    query: {
      range: range.value,
    },
  })
}

async function loadGroupRates(signal?: AbortSignal) {
  try {
    const [groups, custom, adminGroups] = await Promise.all([
      userGroupsAPI.getAvailable(),
      userGroupsAPI.getUserGroupRates().catch(() => ({}) as Record<number, number>),
      isAdmin.value ? groupsAPI.getAll().catch(() => []) : Promise.resolve([]),
    ])
    if (signal?.aborted) return
    groupRates.value = collectStudioGroupRates(
      [...asStudioGroupCatalog(groups), ...asStudioGroupCatalog(adminGroups)],
      custom,
    )
  } catch {
    /* keep last known rates */
  }
}

async function loadMetrics(signal?: AbortSignal, id = sequence) {
  const snapshotPromise = api.getSnapshot(emptyFilter.value, isAdmin.value, signal)
  const matrixPromise = api.getMatrix(emptyFilter.value, 'platform_group', isAdmin.value, signal)
  const ratesPromise = loadGroupRates(signal)
  const [nextSnapshot, nextMatrix] = await Promise.all([snapshotPromise, matrixPromise])
  await ratesPromise
  if (id !== sequence) return
  snapshot.value = nextSnapshot
  matrix.value = nextMatrix
  scheduleAutoRefresh()
}

async function reload(silent = true) {
  controller?.abort()
  const request = new AbortController()
  controller = request
  const id = ++sequence
  if (!silent) {
    loading.value = true
    refreshing.value = true
  }
  try {
    await loadMetrics(request.signal, id)
  } catch (error) {
    if ((error as { name?: string }).name !== 'CanceledError') {
      appStore.showError(extractApiErrorMessage(error, t('channelMonitorV2.loadFailed')))
    }
  } finally {
    if (id === sequence) {
      loading.value = false
      refreshing.value = false
    }
  }
}

function setRange(value: MonitorRange) {
  range.value = value
}

function scheduleAutoRefresh() {
  if (autoRefreshTimer) {
    window.clearInterval(autoRefreshTimer)
    autoRefreshTimer = null
  }
  const seconds = bootstrapActive.value
    ? 10
    : snapshot.value?.config?.refresh_interval_seconds || 300
  autoRefreshTimer = window.setInterval(() => {
    if (!loading.value && !refreshing.value) {
      void reload(true)
    }
  }, Math.max(bootstrapActive.value ? 10 : 60, seconds) * 1000)
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
  void reload(true)
})
onMounted(() => {
  void reload(false)
})
onBeforeUnmount(() => {
  controller?.abort()
  if (autoRefreshTimer) window.clearInterval(autoRefreshTimer)
})
</script>

<style scoped>
.studio-brand-heading-mark {
  display: inline-flex;
  width: 2.5rem;
  height: 2.5rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: color-mix(in srgb, var(--studio-brand-color, #14b8a6) 16%, white);
}
.dark .studio-brand-heading-mark {
  background: color-mix(in srgb, var(--studio-brand-color, #14b8a6) 24%, rgb(15 23 42));
}
.studio-model-row {
  --studio-faces-bg: rgb(241 245 249);
  position: relative;
  display: flex;
  flex-wrap: wrap;
  align-items: stretch;
  background: var(--studio-faces-bg);
  container: studio-row / inline-size;
  contain: layout style;
}
.studio-model-card {
  position: relative;
  z-index: 2;
  width: 100%;
  flex: 1 1 100%;
}
@container studio-row (min-width: 34rem) {
  .studio-model-card {
    flex: 0 0 18rem;
    width: 18rem;
  }
}
.dark .studio-model-row {
  --studio-faces-bg: rgb(30 41 59);
}
</style>
