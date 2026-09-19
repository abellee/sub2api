import { computed, onBeforeUnmount, ref, watch, type ComputedRef, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
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
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'
import { isChannelMonitorThroughputHidden } from '@/utils/featureFlags'
import { formatLatencyPrivacy } from '@/features/channel-monitor-v2/monitorFormat'
import {
  asStudioGroupCatalog,
  collectStudioGroupRates,
  formatStudioCacheRate,
  formatStudioMultiplier,
  formatStudioRateNumber,
  formatStudioSuccessRate,
  formatStudioTtft,
  healthTone,
  lookupStudioGroupRate,
  mergeSelectedStudioGroups,
  studioAccentFromState,
  studioCacheTone,
  studioPlatform,
  studioPlatformLabel,
  studioSampleInsufficient,
  studioSuccessTone,
  studioTtftTone,
  type StudioAccent,
  type StudioGroupRateCatalog,
  type StudioGroupRateIndex,
  type StudioTone,
} from './studioFormat'

export type StudioGroupCard = {
  key: string
  label: string
  platform: ReturnType<typeof studioPlatform>
  brandLabel: string
  rateLabel: string
  successLabel: string
  successRate: string
  ttftLabel: string
  ttft: string
  cacheLabel: string
  cacheRate: string
  statusHeading: string
  statusLabel: string
  title: string
  state: StudioTone
  successState: StudioTone
  ttftState: StudioTone
  cacheState: StudioTone
  accent: StudioAccent
  buckets: MonitorMatrixRow['buckets']
  metrics: MonitorMatrixRow['metrics']
  health: MonitorMatrixRow['health']
}

const EMPTY_GROUP_RATES: StudioGroupRateIndex = { byId: new Map(), byName: new Map() }

export function useStudioGroupCards(options: {
  range: Ref<MonitorRange>
  enabled?: Ref<boolean> | ComputedRef<boolean>
  reportError?: boolean
  autoRefresh?: boolean
}) {
  const { range, reportError = true, autoRefresh = true } = options
  const enabled = options.enabled ?? computed(() => true)
  const { t } = useI18n()
  const authStore = useAuthStore()
  const appStore = useAppStore()
  const isAdmin = computed(() => authStore.isAdmin)
  const showThroughput = computed(() => isAdmin.value || !isChannelMonitorThroughputHidden())

  const snapshot = ref<MonitorSnapshot | null>(null)
  const matrix = ref<MonitorMatrixResponse | null>(null)
  const groupRates = ref<StudioGroupRateIndex>(EMPTY_GROUP_RATES)
  const groupCatalog = ref<StudioGroupRateCatalog[]>([])
  const groupCatalogLoaded = ref(false)
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
  const coverage = computed(() => matrix.value?.coverage || snapshot.value?.coverage || null)

  const groupRows = computed(() =>
    mergeSelectedStudioGroups(
      matrix.value?.items,
      snapshot.value?.config?.group_ids,
      groupCatalog.value,
      groupCatalogLoaded.value,
    ),
  )

  const groupCards = computed<StudioGroupCard[]>(() => {
    const rates = groupRates.value
    const thresholds = healthThresholds.value
    return groupRows.value.map((row) => {
      const state = healthTone(row.health?.overall)
      const insufficient = studioSampleInsufficient(row.health)
      const rate = lookupStudioGroupRate(rates, row.group_id, groupTitle(row))
      return {
        key: groupKey(row),
        label: groupTitle(row),
        platform: studioPlatform(row.platform),
        brandLabel: studioPlatformLabel(row.platform),
        rateLabel: formatStudioMultiplier(rate, t('channelMonitorV2.studio.groups.userRate', { n: formatStudioRateNumber(rate) })),
        successLabel: t('channelMonitorV2.metrics.successRate'),
        successRate: formatStudioSuccessRate(row.metrics, row.health),
        ttftLabel: t('channelMonitorV2.metrics.ttft'),
        ttft: formatStudioTtft(row.metrics, row.health),
        cacheLabel: t('channelMonitorV2.metrics.cacheRate'),
        cacheRate: formatStudioCacheRate(row.metrics, row.health),
        statusHeading: t('channelMonitorV2.studio.status.label'),
        statusLabel: t(`channelMonitorV2.studio.status.${state}`),
        title: insufficient
          ? ''
          : formatLatencyPrivacy(
              row.metrics.ttft.p50_ms,
              row.metrics.ttft.p90_ms,
              row.metrics.ttft.avg_ms,
              row.metrics.ttft.p95_ms,
            ),
        state,
        successState: studioSuccessTone(row.metrics, row.health, thresholds),
        ttftState: studioTtftTone(row.metrics, thresholds, row.health),
        cacheState: studioCacheTone(row.metrics, row.health, thresholds),
        accent: studioAccentFromState(state),
        buckets: row.buckets ?? [],
        metrics: row.metrics,
        health: row.health,
      }
    })
  })

  async function loadGroupRates(signal?: AbortSignal) {
    try {
      const [groups, custom, adminGroups] = await Promise.all([
        userGroupsAPI.getAvailable(),
        userGroupsAPI.getUserGroupRates().catch(() => ({}) as Record<number, number>),
        isAdmin.value ? groupsAPI.getAllIncludingInactive().catch(() => []) : Promise.resolve([]),
      ])
      if (signal?.aborted) return
      const catalog = [...asStudioGroupCatalog(groups), ...asStudioGroupCatalog(adminGroups)]
      groupCatalog.value = catalog
      groupRates.value = collectStudioGroupRates(catalog, custom)
      groupCatalogLoaded.value = true
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
    if (autoRefresh) scheduleAutoRefresh()
  }

  async function reload(silent = true) {
    if (!enabled.value) return
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
      if ((error as { name?: string }).name !== 'CanceledError' && reportError) {
        appStore.showError(extractApiErrorMessage(error, t('channelMonitorV2.loadFailed')))
      }
    } finally {
      if (id === sequence) {
        loading.value = false
        refreshing.value = false
      }
    }
  }

  function scheduleAutoRefresh() {
    if (autoRefreshTimer) {
      window.clearInterval(autoRefreshTimer)
      autoRefreshTimer = null
    }
    if (!autoRefresh) return
    const seconds = bootstrapActive.value
      ? 10
      : snapshot.value?.config?.refresh_interval_seconds || 300
    autoRefreshTimer = window.setInterval(() => {
      if (!loading.value && !refreshing.value) {
        void reload(true)
      }
    }, Math.max(bootstrapActive.value ? 10 : 60, seconds) * 1000)
  }

  function stop() {
    controller?.abort()
    if (autoRefreshTimer) {
      window.clearInterval(autoRefreshTimer)
      autoRefreshTimer = null
    }
  }

  watch(range, () => {
    void reload(true)
  })
  watch(
    enabled,
    (on) => {
      if (on) void reload(false)
      else stop()
    },
    { immediate: true },
  )
  onBeforeUnmount(stop)

  return {
    loading,
    refreshing,
    snapshot,
    matrix,
    groupCards,
    groupCatalog,
    healthThresholds,
    coverage,
    bootstrapActive,
    bootstrapPercent,
    showThroughput,
    reload,
    stop,
  }
}

function groupKey(row: MonitorMatrixRow) {
  return `${row.platform}:${row.group_id}`
}

function groupTitle(row: MonitorMatrixRow) {
  return row.group_name || (row.group_id != null ? `#${row.group_id}` : row.platform)
}
