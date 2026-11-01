<template>
  <DashboardStudioLayout>
    <template #filters><StudioFilters v-model:start-date="startDate" v-model:end-date="endDate" v-model:granularity="granularity" :loading="loadingStats || loadingCharts" @range-change="loadCharts" @granularity-change="loadCharts" @refresh="refreshAll" /></template>
    <div class="dashboard-studio">
      <div v-if="preview" class="studio-preview" role="status"><span>{{ t('dashboard.studio.previewData') }}</span><a href="/dashboard">{{ t('dashboard.studio.backToLive') }}<span aria-hidden="true"> ↗</span></a></div>
      <div v-if="statsError" class="studio-error" role="alert">{{ t('dashboard.studio.statsError') }}<button type="button" @click="loadStats">{{ t('common.tryAgain') }}</button></div>
      <div v-if="loadingStats && !stats" class="studio-loading"><LoadingSpinner />{{ t('common.loading') }}</div>
      <div v-if="stats" class="summary-grid">
        <article v-if="!authStore.isSimpleMode" class="studio-card balance-card">
          <div class="metric-title"><h3>{{ t('dashboard.balance') }}</h3><Icon name="creditCard" size="md" /></div>
          <p class="metric-value">${{ balance }}</p><div class="balance-note"><p class="metric-note">{{ t('common.available') }} · USD</p><router-link v-if="availableBalance < 10" to="/purchase" class="balance-recharge">{{ t('nav.recharge') }}<Icon name="arrowRight" size="sm" /></router-link></div>
          <StudioMetricChart kind="balance" :value="availableBalance" />
        </article>
        <article class="studio-card request-card">
          <div class="metric-title"><h3>{{ t('dashboard.todayRequests') }}</h3><Icon name="chart" size="md" /></div>
          <p class="metric-value" :title="formatNumber(stats.today_requests)">{{ formatNumber(stats.today_requests) }}</p>
          <p class="metric-note">{{ t('dashboard.studio.todayOnly') }}</p>
          <StudioMetricChart kind="requests" :value="stats.today_requests" :points="requestTrend.map(point => point.requests)" :labels="requestTrend.map(point => point.date)" :caption="requestError ? t('dashboard.studio.chartError') : `${requestDates[0]?.slice(5)} — ${requestEndDate.slice(5)} · ${t('dashboard.studio.lastSevenDays')}`" />
        </article>
        <article class="studio-card cost-card">
          <div class="metric-title"><h3>{{ t('dashboard.todayCost') }}</h3><Icon name="dollar" size="md" /></div>
          <p class="metric-value">${{ formatCost(stats.today_actual_cost) }}</p>
          <p class="metric-note">{{ t('dashboard.standard') }} <span>${{ formatCost(stats.today_cost) }}</span></p>
          <StudioMetricChart kind="cost" :value="stats.today_actual_cost" :reference="stats.today_cost" />
        </article>
        <article class="studio-card today-token-card">
          <div class="metric-title"><h3>{{ t('dashboard.todayTokens') }}</h3><Icon name="cube" size="md" /></div>
          <p class="metric-value" :title="formatNumber(stats.today_tokens)">{{ formatTokens(stats.today_tokens) }}</p>
          <p class="metric-note token-subtitle" aria-hidden="true">&nbsp;</p>
          <div class="token-visual"><div class="token-plot"><div class="token-strip" role="img" :aria-label="todaySplitLabel"><i v-for="part in todayParts" :key="part.label" :style="{ flexGrow: part.value, background: part.color }" :title="`${part.label}: ${formatNumber(part.value)}`"></i></div></div>
          <p class="metric-note token-caption" :title="todaySplitLabel"><span v-for="part in todayParts" :key="part.label">{{ part.label }} {{ formatTokens(part.value) }}</span></p></div>
        </article>
      </div>

      <div class="analysis-grid" :class="{ 'without-stats': !stats }">
        <section class="studio-card model-card" :aria-busy="loadingCharts">
          <div class="section-heading"><div><span class="eyebrow">MODEL MIX</span><h3>{{ t('dashboard.modelDistribution') }}</h3></div><span class="section-tag">3D / TOKEN</span></div>
          <div v-if="modelError" class="chart-state" role="alert"><p>{{ t('dashboard.studio.chartError') }}</p><button type="button" @click="loadCharts">{{ t('common.tryAgain') }}</button></div>
          <div v-else-if="loadingCharts" class="chart-state"><LoadingSpinner /></div>
          <ModelPieScene v-else :models="models" />
        </section>
        <aside v-if="stats" class="secondary-metrics">
          <article class="studio-card total-card">
            <div class="metric-title"><h3>{{ t('dashboard.totalTokens') }}</h3><span class="micro-label">ALL TIME</span></div>
            <p class="metric-value" :title="formatNumber(stats.total_tokens)">{{ formatTokens(stats.total_tokens) }}</p>
            <div class="total-breakdown">
              <div v-for="part in totalParts" :key="part.label" class="breakdown-row"><span><i :style="{ background: part.color }"></i>{{ part.label }}</span><b>{{ formatTokens(part.value) }}</b><div class="breakdown-track"><i :style="{ width: `${totalPartSum ? part.value / totalPartSum * 100 : 0}%`, background: part.color }"></i></div></div>
            </div>
          </article>
          <div class="live-metrics"><article class="studio-card performance-card">
            <div class="metric-title"><h3>{{ t('dashboard.performance') }}</h3><span class="micro-label">{{ t('dashboard.studio.lastFiveMinutes') }}</span></div>
            <div class="performance-values"><p><strong>{{ formatTokens(stats.rpm) }}</strong><span>RPM</span></p><p><strong>{{ formatTokens(stats.tpm) }}</strong><span>TPM</span></p></div>
          </article>
          <article class="studio-card response-card"><div><h3>{{ t('dashboard.avgResponse') }}</h3><span class="metric-note">{{ t('dashboard.averageTime') }}</span></div><strong>{{ formatDuration(stats.average_duration_ms) }}</strong><Icon name="clock" size="md" /></article></div>
        </aside>
      </div>

      <section class="studio-card trend-card" :aria-busy="loadingCharts">
        <div class="section-heading"><div><span class="eyebrow">TOKEN ACTIVITY</span><h3>{{ t('dashboard.tokenUsageTrend') }}</h3></div><span class="range-label"><span>{{ startDate }} — {{ endDate }}</span><small class="granularity-tag">{{ t(granularity === 'day' ? 'dashboard.day' : 'dashboard.hour') }}</small></span></div>
        <div v-if="trendError" class="chart-state" role="alert"><p>{{ t('dashboard.studio.chartError') }}</p><button type="button" @click="loadCharts">{{ t('common.tryAgain') }}</button></div>
        <StudioTokenTrend v-else :trend-data="trend" :loading="loadingCharts" />
      </section>
    </div>
  </DashboardStudioLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { usageAPI, type UserDashboardStats } from '@/api/usage'
import type { ModelStat, TrendDataPoint } from '@/types'
import { formatDateLocalInput, formatCostFixed as formatCost, formatTokensK as formatTokens, formatNumberLocaleString as formatNumber } from '@/utils/format'
import DashboardStudioLayout from '@/components/user/dashboard/studio/DashboardStudioLayout.vue'
import StudioFilters from '@/components/user/dashboard/studio/StudioFilters.vue'
import StudioMetricChart from '@/components/user/dashboard/studio/StudioMetricChart.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import ModelPieScene from '@/components/user/dashboard/studio/ModelPieScene.vue'
import StudioTokenTrend from '@/components/user/dashboard/studio/StudioTokenTrend.vue'

const { t } = useI18n()
const authStore = useAuthStore()
const liveStats = ref<UserDashboardStats | null>(null)
const liveTrend = ref<TrendDataPoint[]>([])
const liveModels = ref<ModelStat[]>([])
const liveRequestTrend = ref<TrendDataPoint[] | null>(null)
const requestError = ref(false)
const requestEndDate = ref(formatDateLocalInput(new Date()))
const requestDates = computed(() => Array.from({ length: 7 }, (_, index) => {
  const date = new Date(`${requestEndDate.value}T00:00:00`)
  date.setDate(date.getDate() - 6 + index)
  return formatDateLocalInput(date)
}))
const previewRequested = import.meta.env.DEV && new URLSearchParams(window.location.search).get('studioPreview') === '1'
const previewFactory = shallowRef<typeof import('@/components/user/dashboard/studio/studioPreview').createStudioPreview>()
const preview = computed(() => previewFactory.value?.(startDate.value, endDate.value, granularity.value))
const stats = computed(() => preview.value?.stats ?? liveStats.value)
const trend = computed(() => preview.value?.trend ?? liveTrend.value)
const models = computed(() => preview.value?.models ?? liveModels.value)
const requestTrend = computed(() => {
  const source = previewFactory.value?.(requestDates.value[0]!, requestEndDate.value, 'day').trend ?? liveRequestTrend.value
  if (!source) return []
  const counts = new Map(source.map(point => [point.date, point.requests]))
  return requestDates.value.map(date => ({ date, requests: counts.get(date) ?? 0 }))
})
const loadingStats = ref(false)
const loadingCharts = ref(false)
const statsError = ref(false)
const trendError = ref(false)
const modelError = ref(false)
const startDate = ref(formatDateLocalInput(new Date(Date.now() - 6 * 86400000)))
const endDate = ref(formatDateLocalInput(new Date()))
const granularity = ref<'day' | 'hour'>('day')
const availableBalance = computed(() => Math.max(0, preview.value?.balance ?? authStore.user?.balance ?? 0))
const balance = computed(() => new Intl.NumberFormat('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(availableBalance.value))
const split = (input: number, output: number, cache: number) => [
  { label: t('dashboard.input'), value: input, color: '#4d7bfa' },
  { label: t('dashboard.output'), value: output, color: '#ff9657' },
  { label: t('dashboard.cache'), value: cache, color: '#35bfd0' }
]
const todayParts = computed(() => stats.value ? split(stats.value.today_input_tokens, stats.value.today_output_tokens, stats.value.today_cache_creation_tokens + stats.value.today_cache_read_tokens) : [])
const totalParts = computed(() => stats.value ? split(stats.value.total_input_tokens, stats.value.total_output_tokens, stats.value.total_cache_creation_tokens + stats.value.total_cache_read_tokens) : [])
const totalPartSum = computed(() => totalParts.value.reduce((sum, part) => sum + part.value, 0))
const todaySplitLabel = computed(() => todayParts.value.map(part => `${part.label}: ${formatNumber(part.value)}`).join(', '))
const formatDuration = (ms: number) => ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${ms.toFixed(0)}ms`
let statsVersion = 0
let chartVersion = 0
let requestVersion = 0

async function loadStats() {
  if (previewRequested) return
  const version = ++statsVersion
  loadingStats.value = true
  statsError.value = false
  try {
    await authStore.refreshUser()
    const result = await usageAPI.getDashboardStats()
    if (version === statsVersion) liveStats.value = result
  } catch {
    if (version === statsVersion) statsError.value = true
  } finally {
    if (version === statsVersion) loadingStats.value = false
  }
}
async function loadCharts() {
  if (previewRequested) return
  const version = ++chartVersion
  loadingCharts.value = true
  trendError.value = false
  modelError.value = false
  const dates = { start_date: startDate.value, end_date: endDate.value }
  const results = await Promise.allSettled([
    usageAPI.getDashboardTrend({ ...dates, granularity: granularity.value }),
    usageAPI.getDashboardModels(dates)
  ])
  if (version !== chartVersion) return
  if (results[0].status === 'fulfilled') liveTrend.value = results[0].value.trend || []
  else { liveTrend.value = []; trendError.value = true }
  if (results[1].status === 'fulfilled') liveModels.value = results[1].value.models || []
  else { liveModels.value = []; modelError.value = true }
  loadingCharts.value = false
}
async function loadRequestTrend() {
  requestEndDate.value = formatDateLocalInput(new Date())
  if (previewRequested) return
  const version = ++requestVersion
  requestError.value = false
  try {
    const result = await usageAPI.getDashboardTrend({ start_date: requestDates.value[0]!, end_date: requestEndDate.value, granularity: 'day' })
    if (version === requestVersion) liveRequestTrend.value = result.trend || []
  } catch {
    if (version === requestVersion) { liveRequestTrend.value = null; requestError.value = true }
  }
}
function refreshAll() { void loadStats(); void loadCharts(); void loadRequestTrend() }
onMounted(async () => {
  if (previewRequested) previewFactory.value = (await import('@/components/user/dashboard/studio/studioPreview')).createStudioPreview
  else refreshAll()
})
onBeforeUnmount(() => { statsVersion++; chartVersion++; requestVersion++ })
</script>

<style scoped>
.dashboard-studio{color:var(--studio-ink);display:flex;flex-direction:column;gap:24px;font-size:14px}
.studio-preview{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:8px 12px;border-radius:8px;background:var(--studio-haze);color:var(--studio-muted);font-size:12px}.studio-preview a{color:var(--studio-ink);white-space:nowrap}.studio-preview a:hover{text-decoration:underline;text-underline-offset:3px}
:global(.dark .dashboard-studio){--studio-card-surface:var(--studio-menu-surface);--studio-card-shadow:0 8px 28px #0000001f}
.eyebrow{display:block;font-size:12px;font-weight:500;letter-spacing:1.5px;color:var(--studio-muted)}
.studio-card{background:var(--studio-card-surface,var(--tf-console-surface));border:0;border-radius:18px;box-shadow:var(--studio-card-shadow,0 6px 28px #26385d05);min-width:0}
.summary-grid{display:grid;align-items:stretch;grid-template-columns:repeat(4,minmax(0,1fr));gap:18px}.summary-grid>.studio-card{padding:22px;position:relative;overflow:hidden;display:grid;grid-template-rows:20px 40px 20px 72px;row-gap:10px}.summary-grid:not(:has(.balance-card)){grid-template-columns:repeat(3,minmax(0,1fr))}
.metric-title{display:flex;align-items:center;justify-content:space-between;gap:8px}.metric-title h3,.response-card h3{font-size:14px;font-weight:500}.metric-title :deep(svg){color:var(--studio-muted);opacity:.7}
.metric-value{font-size:32px;letter-spacing:-1.1px;font-weight:600;font-variant-numeric:tabular-nums;line-height:40px;margin-top:0;overflow-wrap:anywhere}.metric-note{font-size:12px;color:var(--studio-muted);line-height:1.6;margin-top:7px}.summary-grid .metric-note{margin:0;line-height:20px}
.balance-card .metric-value{color:var(--studio-ink)}
.balance-note{display:flex;align-items:center;justify-content:space-between;gap:8px;min-width:0}.balance-recharge{display:inline-flex;align-items:center;gap:4px;font-size:12px;white-space:nowrap;color:var(--tf-console-active-ink);border-radius:6px;padding:2px 6px}.balance-recharge:hover{background:var(--tf-console-hover)}.balance-recharge:focus-visible{outline:2px solid #8297f5;outline-offset:2px}
.request-card{--studio-accent:#4d7bfa}.cost-card{--studio-accent:#ff9657}
.token-visual{display:grid;grid-template-rows:44px 20px;gap:8px;min-width:0}.token-plot{display:flex;align-items:center}.token-strip{width:100%;height:9px;background:var(--studio-haze);border-radius:5px;display:flex;gap:3px;overflow:hidden}.token-strip i{flex-basis:0;min-width:0}.token-caption{display:flex;align-items:center;gap:8px;white-space:nowrap;overflow-x:auto;scrollbar-width:none;font-size:12px}.token-caption::-webkit-scrollbar{display:none}.token-caption span+span::before{content:'/';margin-right:8px;color:var(--studio-muted)}

.analysis-grid{display:grid;align-items:stretch;grid-template-columns:minmax(0,1fr) minmax(310px,.55fr);gap:24px}.analysis-grid.without-stats{grid-template-columns:1fr}.model-card{padding:24px;position:relative;background:transparent;box-shadow:none;min-height:390px}.model-card :deep(.model-details){background:transparent;padding:0;box-shadow:none}.section-heading{display:flex;justify-content:space-between;align-items:center;gap:15px}.section-heading h3{font-size:17px;font-weight:600;margin-top:5px;letter-spacing:-.3px}.section-tag{font-size:12px;letter-spacing:1px;color:var(--studio-muted)}
.secondary-metrics{display:flex;flex-direction:column;justify-content:center;gap:16px}.total-card{padding:22px}.total-card .metric-value{margin-top:12px;font-size:30px}.micro-label{font-size:12px;color:var(--studio-muted)}
.total-breakdown{display:flex;flex-direction:column;gap:12px;margin-top:20px}.breakdown-row{display:grid;grid-template-columns:1fr auto;gap:6px;font-size:12px}.breakdown-row>span{display:flex;align-items:center;gap:7px;color:var(--studio-muted)}.breakdown-row>span i{width:6px;height:6px;border-radius:50%}.breakdown-row>b{font-weight:500;font-variant-numeric:tabular-nums}.breakdown-track{grid-column:1/-1;background:var(--studio-haze);height:5px;border-radius:3px;overflow:hidden}.breakdown-track i{display:block;height:100%;border-radius:3px}
.live-metrics{display:grid;grid-template-columns:1.15fr 1fr;gap:12px}.performance-card,.response-card{padding:18px;min-width:0;display:grid;grid-template-rows:43px 32px;gap:12px;align-content:start}.performance-card .metric-title{display:block}.performance-card .micro-label{display:block;margin-top:5px}.performance-values{display:flex;flex-wrap:wrap;gap:10px 18px;margin-top:0;align-items:center;flex-wrap:nowrap}.performance-values p{display:flex;align-items:baseline;gap:5px}.performance-values strong{font-size:24px;font-weight:600;letter-spacing:-.5px;line-height:32px}.performance-values span{font-size:12px;color:var(--studio-muted)}
.response-card{position:relative}.response-card strong{font-size:24px;font-weight:600;letter-spacing:-.5px;line-height:32px;margin:0;padding:0}.response-card :deep(svg){display:none}.response-card .metric-note{display:block;font-size:12px;margin-top:5px}
.trend-card{padding:24px 28px;position:relative}.trend-card .section-heading{margin-bottom:20px}.range-label{display:flex;align-items:center;justify-content:flex-end;gap:10px;color:var(--studio-muted);font-size:12px;font-variant-numeric:tabular-nums;white-space:nowrap}.granularity-tag{display:inline-flex;align-items:center;padding:3px 9px;border-radius:6px;background:var(--studio-haze);color:var(--tf-console-active-ink);font-size:12px;line-height:20px}
.chart-state{min-height:322px;display:flex;align-items:center;justify-content:center;flex-direction:column;gap:12px;color:var(--studio-muted);font-size:14px}.studio-error{padding:12px 16px;border-radius:12px;font-size:14px;color:var(--studio-ink);display:flex;justify-content:space-between;gap:15px}.studio-error button,.chart-state button{color:var(--tf-console-active-ink);text-decoration:underline;text-underline-offset:3px}.studio-loading{min-height:150px;display:flex;align-items:center;justify-content:center;gap:12px;color:var(--studio-muted);font-size:14px}
@media(max-width:1250px){.summary-grid{gap:12px}.summary-grid>.studio-card{padding:18px}.metric-value{font-size:28px}.analysis-grid{grid-template-columns:minmax(0,1fr) minmax(300px,.65fr);gap:16px}.model-card{padding:20px 0}.model-card :deep(.model-distribution){grid-template-columns:minmax(0,1fr) minmax(170px,1fr);gap:0}.performance-values{gap:8px}.live-metrics{grid-template-columns:1.1fr 1fr}.performance-card,.response-card{padding:15px}}
@media(max-width:1050px){.analysis-grid{grid-template-columns:1fr}.secondary-metrics{display:grid;grid-template-columns:1fr 1.2fr;gap:16px}.live-metrics{align-self:stretch}.model-card :deep(.model-distribution){grid-template-columns:minmax(0,1.4fr) minmax(220px,1fr)}.model-card{min-height:360px}}
@media(max-width:700px){.summary-grid,.summary-grid:not(:has(.balance-card)){grid-template-columns:repeat(2,minmax(0,1fr))}.secondary-metrics{display:flex}.model-card :deep(.model-distribution){grid-template-columns:1fr}.model-card{padding:16px 0}.trend-card{padding:20px 16px}.dashboard-studio{gap:18px}.metric-value{font-size:28px}.summary-grid>.studio-card{padding:16px;}.section-heading h3{font-size:16px}.trend-card .section-heading{flex-wrap:wrap}.range-label{margin-left:auto}.live-metrics{grid-template-columns:minmax(0,1.25fr) minmax(0,1fr)}.performance-values{gap:6px}.performance-values strong,.response-card strong{font-size:20px}.metric-title h3{font-size:13px}.total-breakdown{margin-top:16px}}
</style>
