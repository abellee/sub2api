<template>
  <section class="radar-shell mt-8 overflow-hidden rounded-2xl border border-slate-200 bg-slate-950 text-slate-100 shadow-card dark:border-slate-700">
    <header class="radar-header border-b border-slate-800 px-5 py-5 sm:px-7">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <div class="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-cyan-300">
            <Icon name="radar" size="sm" class="h-4 w-4" />
            降智雷达
          </div>
          <h2 class="mt-2 text-xl font-semibold tracking-tight text-white sm:text-2xl">推荐参考与社区体感分</h2>
          <p class="mt-1.5 max-w-3xl text-sm leading-6 text-slate-400">
            根据综合智能、任务耗时、价格和近 24 小时社区投票，快速判断当前模型档位是否值得使用。
          </p>
        </div>
        <div class="flex items-center gap-3 text-xs text-slate-400">
          <span>最后更新 {{ updatedAt }} · {{ updateInterval }}</span>
          <button class="radar-icon-button" type="button" title="立即同步刷新三个模块" aria-label="立即同步刷新三个模块" @click="refresh">
            <Icon name="refresh" size="sm" :class="refreshing ? 'animate-spin' : ''" />
          </button>
        </div>
      </div>
    </header>

    <div class="space-y-6 p-4 sm:p-6">
      <section aria-labelledby="radar-recommend-title">
        <div class="mb-3 flex items-center justify-between gap-3">
          <div>
            <h3 id="radar-recommend-title" class="text-base font-semibold text-white">◎ 推荐参考</h3>
            <p class="mt-1 text-xs text-slate-400">根据综合智能结合任务场景整理的参考建议。</p>
          </div>
          <span class="hidden text-xs text-slate-500 sm:inline">只做参考，实际体验以你的任务为准</span>
        </div>
        <div class="grid gap-3 lg:grid-cols-4">
          <article v-for="item in apiRecommendations.length ? apiRecommendations : recommendations" :key="item.title" class="radar-panel p-4">
            <div class="flex items-start gap-3">
              <div class="flex min-w-0 items-center gap-1.5">
                <h4 class="truncate font-semibold text-white"><span class="mr-1.5 text-cyan-300">{{ item.icon }}</span>{{ item.title }}</h4>
                <div class="radar-info-wrap">
                  <button class="radar-info-button" type="button" :aria-label="`查看${item.title}推荐说明`">
                    <Icon name="infoCircle" size="xs" />
                  </button>
                  <div class="radar-tooltip" role="tooltip">
                    {{ item.description }}
                  </div>
                </div>
              </div>
            </div>
            <p class="mt-2 min-h-10 text-xs leading-5 text-slate-400">{{ item.description }}</p>
            <div class="mt-4 overflow-hidden rounded-lg border border-slate-700/80">
              <table class="w-full table-fixed text-xs">
                <colgroup><col class="w-[39%]" /><col class="w-[15%]" /><col class="w-[23%]" /><col class="w-[23%]" /></colgroup>
                <thead>
                  <tr class="border-b border-slate-700/80 bg-slate-900/80 text-[10px] uppercase tracking-wide text-slate-500">
                    <th class="px-3 py-2 text-left font-normal">模型 / 档位</th><th class="px-2 py-2 text-right font-normal">IQ</th><th class="px-2 py-2 text-right font-normal">耗时</th><th class="px-3 py-2 text-right font-normal">费用</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="model in item.models" :key="model.name" class="border-b border-slate-800/80 last:border-b-0">
                    <th class="truncate px-3 py-2.5 text-left font-semibold text-slate-200">{{ model.name }}</th>
                    <td class="px-2 py-2.5 text-right font-mono text-cyan-300">{{ model.iq }}</td>
                    <td class="px-2 py-2.5 text-right font-mono text-slate-400">{{ model.time }}</td>
                    <td class="px-3 py-2.5 text-right font-mono text-slate-400">{{ model.cost }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </article>
        </div>
      </section>

      <section aria-labelledby="radar-intelligence-title">
        <div class="radar-panel overflow-hidden">
          <div class="flex flex-col gap-3 border-b border-slate-800 p-4 sm:flex-row sm:items-center sm:justify-between sm:px-5">
            <div>
              <div>
                <h3 id="radar-intelligence-title" class="text-base font-semibold text-white">综合智能</h3>
                <p class="mt-1 text-xs text-slate-400">社区众测数据 · 每多一份贡献，结果就更准确。</p>
                <p class="mt-1 text-[11px] text-cyan-300/80">{{ updatedAt }} 更新 · {{ updateInterval }}</p>
              </div>
            </div>
            <div class="flex gap-1 overflow-x-auto rounded-lg bg-slate-900 p-1" role="tablist" aria-label="效能测试类型">
              <button v-for="tab in radarTabs" :key="tab" type="button" role="tab" :aria-selected="activeTab === tab" class="radar-tab" :class="activeTab === tab ? 'radar-tab-active' : ''" @click="activeTab = tab">{{ tab }}</button>
            </div>
          </div>
          <div class="grid gap-4 p-4 sm:p-5">
            <div class="radar-family-grid">
              <template v-for="family in familyRows" :key="family.name">
                <div v-if="family.modelId === 'deepseek-v4-flash'" class="radar-deepseek-divider" aria-hidden="true"></div>
                <div v-if="family.modelId === 'deepseek-v4-flash'" class="radar-price-row">
                  <div class="radar-segmented-control" role="group" aria-label="选择 DeepSeek 价格档位">
                    <button type="button" :aria-pressed="activeDeepSeekPriceBand === 'off_peak'" @click="setDeepSeekPriceBand('off_peak')">低谷价</button>
                    <button type="button" :aria-pressed="activeDeepSeekPriceBand === 'peak'" @click="setDeepSeekPriceBand('peak')">高峰价</button>
                  </div>
                </div>
                <div class="radar-family-row" :class="family.modelId.startsWith('deepseek-') ? 'radar-deepseek-row' : ''">
                  <button v-for="model in family.models" :key="model.name" type="button" class="radar-score-card text-left" :style="{ gridColumn: `${model.tierIndex + 1}` }" @click="selectedModel = model.name">
                    <div class="radar-score-iq">
                      <div class="radar-score-heading"><span class="font-semibold text-slate-200">{{ model.name }}</span><span class="radar-score-samples" :title="`近24小时两项能力合计 ${model.samples} 次有效作答`">{{ model.samples }}</span></div>
                      <strong class="text-3xl font-semibold tracking-tight text-white">{{ model.iq }}</strong>
                    </div>
                    <div class="radar-score-meta"><span>{{ model.cost }}</span><span>{{ model.time }}</span></div>
                  </button>
                </div>
              </template>
            </div>
            <figure ref="chartPanel" class="radar-chart-panel">
              <div class="flex items-center justify-between gap-3">
                <div>
                  <h4 class="text-sm font-semibold text-white">{{ chartMetric }} × IQ</h4>
                  <p class="mt-1 text-[11px] text-slate-500">越靠左上越高效 · {{ updatedAt }} 更新</p>
                </div>
                <div class="flex items-center gap-2">
                  <button class="radar-chart-action" type="button" title="全屏查看图表" aria-label="全屏查看图表" @click="toggleChartFullscreen">
                    <Icon name="arrowsUpDown" size="xs" />
                  </button>
                  <select v-model="chartMetric" class="radar-select" aria-label="切换指标">
                    <option value="综合成本">综合成本 × IQ</option><option value="时间成本">时间成本 × IQ</option><option value="费用成本">费用成本 × IQ</option>
                  </select>
                </div>
              </div>
              <div class="radar-chart-legend mt-4" aria-label="模型系列">
                <span v-for="series in chartSeriesData" :key="series.name"><i :style="{ backgroundColor: series.color }"></i>{{ series.name }}</span>
              </div>
              <div class="radar-chart-scroll mt-2">
                <svg class="radar-efficiency-svg" viewBox="0 0 1080 450" role="img" :aria-label="`${chartMetric}与 IQ 对比图`">
                  <line v-for="tick in xTicks" :key="`x-${tick.value}`" class="radar-chart-gridline" :x1="tick.x" y1="20" :x2="tick.x" y2="404" />
                  <text v-for="tick in xTicks" :key="`xt-${tick.value}`" class="radar-chart-tick" :x="tick.x" y="422" text-anchor="middle">{{ tick.value }}</text>
                  <line v-for="tick in yTicks" :key="`y-${tick}`" class="radar-chart-gridline" x1="54" :y1="chartY(tick)" x2="1062" :y2="chartY(tick)" />
                  <text v-for="tick in yTicks" :key="`yt-${tick}`" class="radar-chart-tick" x="46" :y="chartY(tick) + 3" text-anchor="end">{{ tick }}</text>
                  <line class="radar-chart-axis" x1="54" y1="20" x2="54" y2="404" />
                  <line class="radar-chart-axis" x1="54" y1="404" x2="1062" y2="404" />
                  <path v-if="chartBreak" class="radar-chart-break" :d="`M${chartBreak.markX - 6} 408l5 -8 M${chartBreak.markX} 408l5 -8`" />
                  <text class="radar-chart-axis-label" x="46" y="16" text-anchor="end">IQ</text>
                  <g v-for="series in chartSeriesData" :key="series.name" class="radar-chart-series" :aria-label="series.name">
                    <path v-if="series.points.length > 1" class="radar-chart-line" :style="{ stroke: series.color }" :d="chartPath(series.points)" />
                    <g v-for="point in series.points" :key="point.name" class="radar-chart-point-group" tabindex="0" role="button" :aria-label="`${point.name} IQ ${point.iq}`" @mouseenter="hoveredPoint = point" @mouseleave="hoveredPoint = null" @focus="hoveredPoint = point" @blur="hoveredPoint = null" @click="selectedModel = point.name">
                      <circle v-if="point.shape === 'circle'" class="radar-chart-point" :class="selectedModel === point.name ? 'chart-point-selected' : ''" :style="{ stroke: series.color }" :cx="point.x" :cy="point.y" r="5" />
                      <polygon v-else-if="point.shape === 'triangle'" class="radar-chart-point" :class="selectedModel === point.name ? 'chart-point-selected' : ''" :style="{ stroke: series.color }" :points="trianglePoints(point.x, point.y)" />
                      <rect v-else-if="point.shape === 'square'" class="radar-chart-point" :class="selectedModel === point.name ? 'chart-point-selected' : ''" :style="{ stroke: series.color }" :x="point.x - 5" :y="point.y - 5" width="10" height="10" rx="1" />
                      <polygon v-else-if="point.shape === 'hexagon'" class="radar-chart-point" :class="selectedModel === point.name ? 'chart-point-selected' : ''" :style="{ stroke: series.color }" :points="hexagonPoints(point.x, point.y)" />
                      <polygon v-else-if="point.shape === 'star'" class="radar-chart-point" :class="selectedModel === point.name ? 'chart-point-selected' : ''" :style="{ stroke: series.color }" :points="starPoints(point.x, point.y)" />
                      <polygon v-else class="radar-chart-point" :class="selectedModel === point.name ? 'chart-point-selected' : ''" :style="{ stroke: series.color }" :points="diamondPoints(point.x, point.y)" />
                      <text class="radar-chart-point-label" :x="point.x" :y="point.labelY" text-anchor="middle">{{ point.tier }}</text>
                    </g>
                  </g>
                  <text class="radar-chart-axis-label" x="558" y="446" text-anchor="middle">{{ chartAxisLabel }}</text>
                </svg>
              </div>
              <div v-if="hoveredPoint" class="radar-chart-tooltip" role="status">
                <strong>{{ hoveredPoint.name }}</strong><span>IQ {{ hoveredPoint.iq.toFixed(1) }}</span><span>{{ formatMetricValue(hoveredPoint) }}</span><span>{{ hoveredPoint.time }} · {{ hoveredPoint.cost }}</span>
              </div>
            </figure>
          </div>
          <div class="border-t border-slate-800 px-4 py-3 sm:px-5">
            <button type="button" class="flex w-full items-center justify-between text-left text-xs text-slate-400" @click="historyOpen = !historyOpen">
              <span><strong class="text-slate-200">IQ 历史数据</strong> · 每 4 小时观察点 · 左右滑动查看更早数据</span>
              <Icon :name="historyOpen ? 'chevronUp' : 'chevronDown'" size="sm" />
            </button>
            <div v-if="historyOpen" class="mt-3 overflow-x-auto pb-1">
              <div class="flex min-w-[620px] items-end gap-2">
                <div v-for="point in historyPoints" :key="point.label" class="flex min-w-12 flex-1 flex-col items-center gap-1 text-[10px] text-slate-500"><span class="font-mono text-cyan-300">{{ point.value }}</span><div class="flex h-24 w-full items-end rounded bg-slate-900"><span class="block w-full rounded-t bg-cyan-400/70" :style="{ height: `${point.value}%` }"></span></div><span>{{ point.label }}</span></div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section aria-labelledby="radar-history-title" class="radar-panel p-4 sm:p-5">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div><h3 id="radar-history-title" class="text-base font-semibold text-white">历史数据比较</h3><p class="mt-1 text-xs text-slate-400">选择一个或多个模型档位，对比滚动历史数据。</p></div>
          <select v-model="historyMetric" class="radar-select" aria-label="历史比较指标"><option>IQ</option><option>费用</option><option>耗时</option><option>Agent steps</option><option>cache 命中率</option><option>总 tokens</option></select>
        </div>
        <div class="mt-4 flex flex-wrap gap-2">
          <label v-for="model in comparisonModels" :key="model" class="radar-check" :class="selectedComparisons.includes(model) ? 'radar-check-active' : ''"><input v-model="selectedComparisons" type="checkbox" :value="model" /><span>{{ model }}</span></label>
        </div>
        <div class="mt-4 grid gap-2 sm:grid-cols-3">
          <div v-for="model in selectedComparisonRows" :key="model.name" class="rounded-lg border border-slate-800 bg-slate-900/60 p-3"><div class="flex items-center justify-between text-xs"><span class="font-semibold text-slate-200">{{ model.name }}</span><span class="font-mono text-cyan-300">{{ model.value }}</span></div><div class="mt-2 h-1.5 rounded-full bg-slate-800"><span class="block h-full rounded-full bg-cyan-400/80" :style="{ width: `${model.percent}%` }"></span></div></div>
          <p v-if="selectedComparisonRows.length === 0" class="text-xs text-slate-500">请选择要比较的模型档位</p>
        </div>
      </section>

      <section aria-labelledby="radar-community-title" class="radar-panel overflow-hidden">
        <div class="border-b border-slate-800 p-4 sm:p-5">
          <h3 id="radar-community-title" class="text-base font-semibold text-white">社区体感分</h3>
          <p class="mt-2 text-xs leading-5 text-slate-400">诚邀蹬友根据近 24 小时实际体验进行评分，请只给用过的模型打分，以免引入噪声。</p>
          <p class="mt-2 rounded-lg bg-slate-900/70 px-3 py-2 text-xs leading-5 text-slate-400">体感分参考：9–10 明显好用，7–8 正常可用，5–6 勉强可用，3–4 体验较差，1–2 几乎不可用。可综合准确性、返工次数、速度、稳定性和额度/服务影响。</p>
        </div>
        <div class="overflow-x-auto">
          <table class="radar-ratings-table min-w-[800px] w-full">
            <thead><tr><th>Model <span>入 / 缓 / 出</span></th><th v-for="mode in modes" :key="mode">{{ mode }}</th></tr></thead>
            <tbody>
              <tr v-for="row in (apiRatingRows.length ? apiRatingRows : ratingRows)" :key="row.model">
                <th><strong>{{ row.model }}</strong><span>{{ row.price }}</span></th>
                <td v-for="mode in modes" :key="mode">
                  <div v-if="row.scores[mode] !== null" class="rating-cell">
                    <div class="flex items-center justify-between gap-2"><strong>{{ row.scores[mode]?.toFixed(1) }}</strong><span>{{ row.votes[mode] }} 人评分</span></div>
                    <div class="rating-stars" role="radiogroup" :aria-label="`${row.model} ${mode}`">
                      <button v-for="score in ratingScale" :key="score" type="button" :aria-label="`${row.model} ${mode} ${score} 分`" :class="score <= (userRatings[ratingKey(row.model, mode)] || 0) ? 'rating-star-active' : ''" @click="rate(row.model, mode, score)">★</button>
                    </div>
                    <span class="rating-hint">{{ userRatings[ratingKey(row.model, mode)] ? `已评分 ${userRatings[ratingKey(row.model, mode)]} 分` : '未评分' }}</span>
                  </div>
                  <span v-else class="text-xs text-slate-600">不支持</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p class="border-t border-slate-800 px-4 py-3 text-xs leading-5 text-slate-500 sm:px-5">你的评分提交后会立即记录；全站平均分和投票人数使用滚动 24 小时窗口，约每几分钟刷新一次。你可以更新评分，近 24 小时内只按最新一次计算。</p>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'

type PriceBand = 'off_peak' | 'peak'
type RadarModel = { name: string; iq: number; cost: string; time: string; samples: number; short: string; model?: string; effort?: string; averageCost?: number; priceBands?: Partial<Record<PriceBand, number | null>>; averageMinutes?: number; combinedCostIndex?: number; agentSteps?: number | null; totalTokens?: number | null; cacheHitRate?: number | null }
type RatingRow = { model: string; price: string; scores: Record<string, number | null>; votes: Record<string, number> }

type RadarPoint = { model: string; effort: string; iq: number; total?: number; valid_tasks?: number; benchmark_tasks?: number; average_price_usd?: number | null; average_price_usd_by_band?: Partial<Record<PriceBand, number | null>> | null; price_samples?: number; average_minutes?: number | null; duration_samples?: number; combined_cost_index?: number | null; average_agent_steps?: number | null; average_total_tokens?: number | null; cache_hit_rate?: number | null; runs_24h?: number; runs_48h?: number; runs_total?: number }
type RadarPayload = { source_updated_at?: string; generated_at?: string; points?: RadarPoint[]; recommendations?: Array<{ key: string; title: string; rule?: string; items?: Array<{ model: string; effort: string; iq: number; average_cost_usd?: number; average_duration_minutes?: number; samples?: number; combined_cost_index?: number }> }>; models?: Array<{ id: string; label: string; group: string; average: number | null; count: number }> }
type RadarRecommendation = { title: string; icon: string; description: string; models: Array<{ name: string; iq: number; time: string; cost: string }> }

const radarApiBase = '/api/v1/model-plaza/downgrade-radar'
const apiModelSets = ref<Record<string, RadarModel[]>>({})
const apiRecommendations = ref<RadarRecommendation[]>([])
const apiRatingRows = ref<RatingRow[]>([])
const hoveredPoint = ref<RadarModel | null>(null)
const chartPanel = ref<HTMLElement | null>(null)
const fetchError = ref('')
let refreshTimer: number | undefined

const updatedAt = ref('加载中')
const updateInterval = '每 10 分钟自动更新'
const refreshing = ref(false)
const activeTab = ref('🧠 综合智能')
const selectedModel = ref('Sol ultra')
const chartMetric = ref('综合成本')
const activeDeepSeekPriceBand = ref<PriceBand>((() => {
  try { return localStorage.getItem('downgrade-radar-deepseek-price-band') === 'peak' ? 'peak' : 'off_peak' } catch { return 'off_peak' }
})())
const historyMetric = ref('IQ')
const historyOpen = ref(false)
const selectedComparisons = ref<string[]>(['Sol ultra', 'Sol max', 'Luna medium'])
const userRatings = ref<Record<string, number>>({})
const ratingScale = [2, 4, 6, 8, 10]
const modes = ['ultra', 'max', 'xhigh', 'high', 'medium', 'low', 'off']
const radarTabs = ['🧠 综合智能', '💻 软件工程能力', '🧩 视觉空间推理']
const modelInfo = {
  'gpt-5.6-sol': { label: 'Sol', color: '#eab308' },
  'gpt-5.6-terra': { label: 'Terra', color: '#60a5fa' },
  'gpt-5.6-luna': { label: 'Luna', color: '#c7d2e0' },
  'gpt-5.5': { label: '5.5', color: '#00e5ff' },
  'deepseek-v4-flash': { label: 'DSV4 Flash', color: '#2563eb' },
  'deepseek-v4-pro': { label: 'DSV4 Pro', color: '#a855f7' },
} as const
type SupportedModel = keyof typeof modelInfo
const chartSeries = Object.keys(modelInfo) as SupportedModel[]
const tierOrder = ['low', 'medium', 'high', 'xhigh', 'max', 'ultra']
const yTicks = computed(() => {
  const values = activeModels.value.map((model) => model.iq).filter(Number.isFinite)
  const min = activeTab.value === '🧩 视觉空间推理' ? 40 : 0
  const max = Math.min(150, Math.max(min + 20, Math.ceil((Math.max(...values, min + 20) + 5) / 10) * 10))
  return Array.from({ length: 7 }, (_, index) => Math.round(min + ((max - min) * index) / 6))
})
const chartDomain = computed(() => {
  const values = activeModels.value.map(chartMetricValue).filter((value) => Number.isFinite(value) && value > 0).sort((a, b) => a - b)
  return { min: values[0] || 0.01, max: values[values.length - 1] || 1 }
})
const chartAxisLabel = computed(() => chartMetric.value === '费用成本' ? '平均费用（美元）' : chartMetric.value === '时间成本' ? '平均耗时（分钟）' : '费用与耗时综合成本指数')
const chartBreak = computed(() => {
  const values = [...new Set(activeModels.value.map(chartMetricValue).filter((value) => Number.isFinite(value) && value > 0).sort((a, b) => a - b))]
  const min = values[0]
  const second = values[1]
  if (min == null || second == null || !Number.isFinite(min) || !Number.isFinite(second) || second / min < 4) return null
  const share = 0.14
  return { second, share, markX: 54 + 1008 * share / 2 }
})
const xTicks = computed(() => {
  const { min, max } = chartDomain.value
  const tickMin = chartBreak.value?.second ?? min
  const values = (chartBreak.value ? [min] : []).concat([0, .2, .4, .6, .8, 1].map((share) => tickMin === max ? tickMin : tickMin * Math.pow(max / tickMin, share)))
  return values.filter((value, index) => index === 0 || value / values[index - 1] > 1.08).map((value) => {
    const share = chartBreak.value && value >= chartBreak.value.second
      ? chartBreak.value.share + (Math.log(value / chartBreak.value.second) / Math.max(1e-9, Math.log(max / chartBreak.value.second))) * (1 - chartBreak.value.share)
      : value === min ? 0 : Math.log(value / min) / Math.max(1e-9, Math.log(max / min))
    const label = chartMetric.value === '费用成本' ? `$${value >= 10 ? Math.round(value) : value.toFixed(value < 1 ? 2 : 1)}` : value >= 10 ? String(Math.round(value)) : value.toFixed(value < 1 ? 2 : 1)
    return { value: label, x: 54 + share * 1008 }
  })
})

const recommendations = [
  { title: '日常开发', icon: '⌨', description: '日常开发追求智力、速度与成本的平衡', models: [{ name: 'Sol high', iq: 95, time: '20 分钟', cost: '$4.17' }, { name: '5.5 xhigh', iq: 93, time: '22 分钟', cost: '$5.29' }] },
  { title: '难题攻坚', icon: '◆', description: '困难任务解决只追求极致的智商', models: [{ name: 'Sol ultra', iq: 105, time: '48 分钟', cost: '$22.49' }, { name: 'Sol max', iq: 101, time: '33 分钟', cost: '$7.95' }] },
  { title: '后台自动化', icon: '↻', description: '自动化任务需要足够的智力下越便宜越好，速度慢点关系不大', models: [{ name: 'Luna high', iq: 80, time: '18 分钟', cost: '$0.19' }, { name: 'Luna xhigh', iq: 83, time: '26 分钟', cost: '$0.31' }] },
  { title: '跑龙虾类任务', icon: '◷', description: '龙虾类任务对智力要求不高，追求极致性价比', models: [{ name: 'Terra low', iq: 58, time: '8 分钟', cost: '$0.46' }, { name: 'Terra medium', iq: 65, time: '10 分钟', cost: '$0.58' }] }
]

const modelSets: Record<string, RadarModel[]> = {
  '🧠 综合智能': [
    { name: 'Sol ultra', iq: 105, cost: '$22.56', time: '49 分钟', samples: 31, short: 'S·U' }, { name: 'Sol max', iq: 101, cost: '$7.95', time: '33 分钟', samples: 16, short: 'S·M' }, { name: 'Sol xhigh', iq: 100, cost: '$5.70', time: '25 分钟', samples: 8, short: 'S·X' }, { name: 'Sol high', iq: 95, cost: '$4.17', time: '20 分钟', samples: 7, short: 'S·H' }, { name: 'Sol medium', iq: 90, cost: '$3.23', time: '17 分钟', samples: 14, short: 'S·Md' }, { name: 'Sol low', iq: 79, cost: '$1.83', time: '12 分钟', samples: 4, short: 'S·L' },
    { name: 'Terra ultra', iq: 97, cost: '$9.86', time: '41 分钟', samples: 17, short: 'T·U' }, { name: 'Terra max', iq: 94, cost: '$3.54', time: '32 分钟', samples: 2, short: 'T·M' }, { name: 'Terra xhigh', iq: 87, cost: '$1.74', time: '20 分钟', samples: 0, short: 'T·X' }, { name: 'Terra high', iq: 79, cost: '$1.03', time: '13 分钟', samples: 5, short: 'T·H' }, { name: 'Terra medium', iq: 65, cost: '$0.58', time: '10 分钟', samples: 2, short: 'T·Md' }, { name: 'Terra low', iq: 58, cost: '$0.46', time: '8 分钟', samples: 4, short: 'T·L' },
    { name: 'Luna max', iq: 88, cost: '$0.48', time: '37 分钟', samples: 7, short: 'L·M' }, { name: 'Luna xhigh', iq: 83, cost: '$0.31', time: '26 分钟', samples: 26, short: 'L·X' }, { name: 'Luna high', iq: 80, cost: '$0.19', time: '18 分钟', samples: 3, short: 'L·H' }, { name: 'Luna medium', iq: 47, cost: '$0.07', time: '9 分钟', samples: 64, short: 'L·Md' }, { name: 'Luna low', iq: 23, cost: '$0.03', time: '6 分钟', samples: 93, short: 'L·L' },
    { name: '5.5 xhigh', iq: 93, cost: '$5.29', time: '22 分钟', samples: 9, short: '5·X' }, { name: '5.5 high', iq: 89, cost: '$3.45', time: '17 分钟', samples: 6, short: '5·H' }
  ],
  '💻 软件工程能力': [
    { name: 'Sol ultra', iq: 103, cost: '$23.29', time: '54 分钟', samples: 5, short: 'S·U' }, { name: 'Sol max', iq: 103, cost: '$8.55', time: '34 分钟', samples: 6, short: 'S·M' }, { name: 'Sol xhigh', iq: 103, cost: '$6.05', time: '25 分钟', samples: 6, short: 'S·X' }, { name: 'Sol high', iq: 90, cost: '$4.37', time: '20 分钟', samples: 4, short: 'S·H' }, { name: 'Sol medium', iq: 87, cost: '$3.47', time: '17 分钟', samples: 1, short: 'S·Md' }, { name: 'Sol low', iq: 79, cost: '$1.99', time: '12 分钟', samples: 2, short: 'S·L' },
    { name: 'Terra ultra', iq: 94, cost: '$9.47', time: '43 分钟', samples: 2, short: 'T·U' }, { name: 'Terra max', iq: 95, cost: '$3.54', time: '31 分钟', samples: 1, short: 'T·M' }, { name: 'Terra xhigh', iq: 85, cost: '$1.74', time: '19 分钟', samples: 0, short: 'T·X' }, { name: 'Terra high', iq: 72, cost: '$1.07', time: '13 分钟', samples: 7, short: 'T·H' }, { name: 'Terra medium', iq: 58, cost: '$0.61', time: '10 分钟', samples: 1, short: 'T·Md' }, { name: 'Terra low', iq: 48, cost: '$0.49', time: '8 分钟', samples: 4, short: 'T·L' },
    { name: 'Luna max', iq: 92, cost: '$0.47', time: '33 分钟', samples: 5, short: 'L·M' }, { name: 'Luna xhigh', iq: 84, cost: '$0.32', time: '23 分钟', samples: 2, short: 'L·X' }, { name: 'Luna high', iq: 79, cost: '$0.20', time: '17 分钟', samples: 1, short: 'L·H' }, { name: 'Luna medium', iq: 33, cost: '$0.08', time: '10 分钟', samples: 9, short: 'L·Md' }, { name: 'Luna low', iq: 8, cost: '$0.03', time: '7 分钟', samples: 10, short: 'L·L' },
    { name: '5.5 xhigh', iq: 98, cost: '$5.55', time: '22 分钟', samples: 1, short: '5·X' }, { name: '5.5 high', iq: 90, cost: '$3.68', time: '17 分钟', samples: 6, short: '5·H' },
    { name: 'DSV4 Flash max', iq: 85, cost: '$0.22', time: '32 分钟', samples: 0, short: 'F·M' }, { name: 'DSV4 Flash high', iq: 65, cost: '$0.22', time: '31 分钟', samples: 0, short: 'F·H' },
    { name: 'DSV4 Pro max', iq: 88, cost: '$0.85', time: '42 分钟', samples: 0, short: 'P·M' }, { name: 'DSV4 Pro high', iq: 90, cost: '$0.59', time: '29 分钟', samples: 0, short: 'P·H' }
  ],
  '🧩 视觉空间推理': [
    { name: 'Sol ultra', iq: 107, cost: '$19.55', time: '28 分钟', samples: 26, short: 'S·U' }, { name: 'Sol max', iq: 99, cost: '$5.60', time: '30 分钟', samples: 10, short: 'S·M' }, { name: 'Sol xhigh', iq: 97, cost: '$4.37', time: '25 分钟', samples: 3, short: 'S·X' }, { name: 'Sol high', iq: 99, cost: '$3.36', time: '21 分钟', samples: 3, short: 'S·H' }, { name: 'Sol medium', iq: 93, cost: '$2.29', time: '17 分钟', samples: 13, short: 'S·Md' }, { name: 'Sol low', iq: 80, cost: '$1.24', time: '12 分钟', samples: 2, short: 'S·L' },
    { name: 'Terra ultra', iq: 100, cost: '$11.82', time: '34 分钟', samples: 15, short: 'T·U' }, { name: 'Terra max', iq: 93, cost: '$3.51', time: '39 分钟', samples: 1, short: 'T·M' }, { name: 'Terra xhigh', iq: 89, cost: '$1.74', time: '24 分钟', samples: 0, short: 'T·X' }, { name: 'Terra high', iq: 86, cost: '$0.87', time: '14 分钟', samples: 0, short: 'T·H' }, { name: 'Terra medium', iq: 72, cost: '$0.46', time: '9 分钟', samples: 1, short: 'T·Md' }, { name: 'Terra low', iq: 69, cost: '$0.34', time: '7 分钟', samples: 0, short: 'T·L' },
    { name: 'Luna max', iq: 85, cost: '$0.53', time: '52 分钟', samples: 2, short: 'L·M' }, { name: 'Luna xhigh', iq: 82, cost: '$0.31', time: '35 分钟', samples: 24, short: 'L·X' }, { name: 'Luna high', iq: 81, cost: '$0.15', time: '24 分钟', samples: 2, short: 'L·H' }, { name: 'Luna medium', iq: 69, cost: '$0.04', time: '8 分钟', samples: 55, short: 'L·Md' }, { name: 'Luna low', iq: 67, cost: '$0.01', time: '4 分钟', samples: 83, short: 'L·L' },
    { name: '5.5 xhigh', iq: 88, cost: '$4.29', time: '23 分钟', samples: 8, short: '5·X' }, { name: '5.5 high', iq: 89, cost: '$2.53', time: '18 分钟', samples: 0, short: '5·H' }
  ]
}

const activeModels = computed(() => {
  const source = apiModelSets.value[activeTab.value] || modelSets[activeTab.value] || modelSets['🧠 综合智能']
  if (activeTab.value !== '💻 软件工程能力') return source
  const priced = source.map((model) => {
    const modelId = model.model || modelIdForName(model.name)
    if (modelId !== 'deepseek-v4-flash' && modelId !== 'deepseek-v4-pro') return model
    const baseline = model.averageCost ?? Number.parseFloat(model.cost.replace(/[^0-9.]/g, ''))
    const bandValue = model.priceBands?.[activeDeepSeekPriceBand.value]
    const selected = bandValue != null && Number.isFinite(Number(bandValue)) ? Number(bandValue) : activeDeepSeekPriceBand.value === 'peak' ? baseline * 2 : baseline
    return { ...model, averageCost: selected, cost: formatMoney(selected) }
  })
  return normalizeModelCombinedCost(priced)
})
function modelIdForName(name: string): SupportedModel | undefined {
  return chartSeries.find((modelId) => name === modelInfo[modelId].label || name.startsWith(`${modelInfo[modelId].label} `))
}
const familyRows = computed(() => chartSeries.map((modelId) => ({
  name: modelInfo[modelId].label,
  modelId,
  color: modelInfo[modelId].color,
  models: activeModels.value
    .filter((model) => (model.model || modelIdForName(model.name)) === modelId)
    .sort((a, b) => tierOrder.indexOf(b.name.split(' ').pop() || '') - tierOrder.indexOf(a.name.split(' ').pop() || ''))
    .map((model) => ({ ...model, tierIndex: 5 - tierOrder.indexOf(model.name.split(' ').pop() || '') }))
})).filter((family) => family.models.length > 0))
const chartSeriesData = computed(() => {
  return familyRows.value.map((series, seriesIndex) => ({
    name: series.name,
    color: series.color,
    points: [...series.models]
      .sort((a, b) => tierOrder.indexOf(a.effort || a.name.split(' ').pop() || '') - tierOrder.indexOf(b.effort || b.name.split(' ').pop() || ''))
      .map((model, pointIndex) => {
        const tier = model.effort || model.name.split(' ').pop() || ''
        const y = chartY(model.iq)
        return {
          ...model,
          tier,
          shape: ({ low: 'circle', medium: 'triangle', high: 'square', xhigh: 'diamond', max: 'hexagon', ultra: 'star' } as Record<string, string>)[tier] || 'circle',
          x: chartX(model),
          y,
          labelY: y + ((seriesIndex + pointIndex) % 2 ? 17 : -9),
        }
      })
  }))
})
const comparisonModels = computed(() => activeModels.value.map((model) => model.name))
const selectedComparisonRows = computed(() => activeModels.value.filter((model) => selectedComparisons.value.includes(model.name)).map((model) => ({ name: model.name, value: historyMetric.value === '费用' ? model.cost : historyMetric.value === '耗时' ? model.time : model.iq, percent: historyMetric.value === 'IQ' ? model.iq : Math.max(18, 100 - activeModels.value.indexOf(model) * 11) })))
const historyPoints = [72, 81, 76, 88, 84, 91, 86, 94].map((value, index) => ({ value, label: ['7/20', '7/24', '7/28', '8/1', '8/5', '8/9', '8/13', '8/17'][index] }))
const ratingRows: RatingRow[] = [
  { model: 'GPT-5.6 Sol', price: '$5 · $0.50 · $30', scores: { ultra: 8.6, max: 7.1, xhigh: 7.8, high: 7.2, medium: 6.7, low: 8.0, off: null }, votes: { ultra: 19, max: 19, xhigh: 37, high: 20, medium: 9, low: 2, off: 0 } },
  { model: 'GPT-5.6 Terra', price: '$2 · $0.20 · $12', scores: { ultra: 7.3, max: 6.9, xhigh: 5.7, high: 2.0, medium: 6.0, low: 4.0, off: null }, votes: { ultra: 9, max: 7, xhigh: 7, high: 1, medium: 1, low: 2, off: 0 } },
  { model: 'GPT-5.6 Luna', price: '$0.20 · $0.02 · $1.20', scores: { ultra: null, max: 8.7, xhigh: 8.8, high: 8.0, medium: 9.3, low: 10.0, off: null }, votes: { ultra: 0, max: 50, xhigh: 5, high: 6, medium: 3, low: 2, off: 0 } },
  { model: 'GPT-5.5', price: '$5 · $0.50 · $15', scores: { ultra: null, max: null, xhigh: 7.7, high: 9.5, medium: 6.0, low: 4.7, off: null }, votes: { ultra: 0, max: 0, xhigh: 7, high: 4, medium: 2, low: 3, off: 0 } },
  { model: 'DSV4 Flash', price: '非峰 $0.22/$0.007/$0.66 · 峰×2', scores: { ultra: null, max: 5.6, xhigh: null, high: 6.0, medium: null, low: null, off: 6.0 }, votes: { ultra: 0, max: 14, xhigh: 0, high: 5, medium: 0, low: 0, off: 4 } },
  { model: 'DSV4 Pro', price: '非峰 $0.66/$0.022/$1.98 · 峰×2', scores: { ultra: null, max: 5.5, xhigh: null, high: 6.4, medium: null, low: null, off: 5.6 }, votes: { ultra: 0, max: 12, xhigh: 0, high: 5, medium: 0, low: 0, off: 5 } }
]

const tabForBenchmark = (benchmark: 'software' | 'visual' | 'comprehensive') => benchmark === 'software' ? '💻 软件工程能力' : benchmark === 'visual' ? '🧩 视觉空间推理' : '🧠 综合智能'
const isSupportedModel = (model: string): model is SupportedModel => model in modelInfo
const familyForModel = (model: string) => isSupportedModel(model) ? modelInfo[model].label : model
const formatMoney = (value: number | null | undefined) => value == null || !Number.isFinite(value) ? '—' : `$${value < 1 ? value.toFixed(2) : value.toFixed(2)}`
const formatMinutes = (value: number | null | undefined) => value == null || !Number.isFinite(value) ? '—' : `${value.toFixed(1)} 分钟`
const modelFromPoint = (point: RadarPoint): RadarModel => ({
  name: `${familyForModel(point.model)} ${point.effort}`,
  model: point.model,
  effort: point.effort,
  iq: Number(point.iq) || 0,
  cost: formatMoney(point.average_price_usd),
  time: formatMinutes(point.average_minutes),
  samples: Number(point.runs_24h ?? point.valid_tasks ?? point.runs_total ?? 0),
  short: `${familyForModel(point.model).slice(0, 2)}·${point.effort.slice(0, 2)}`,
  averageCost: point.average_price_usd ?? undefined,
  priceBands: point.average_price_usd_by_band ?? undefined,
  averageMinutes: point.average_minutes ?? undefined,
  combinedCostIndex: point.combined_cost_index ?? undefined,
  agentSteps: point.average_agent_steps,
  totalTokens: point.average_total_tokens,
  cacheHitRate: point.cache_hit_rate,
})
const normalizePoints = (points: RadarPoint[]) => points.filter((point) => isSupportedModel(point.model) && Number.isFinite(Number(point.iq))).map(modelFromPoint)
const combinedCostWeight = Math.log(2.5) / Math.log(1.35)
const normalizeCombinedCost = (points: RadarPoint[]) => {
  const rows = points.map((point) => ({
    point,
    raw: Number(point.average_price_usd) > 0 && Number(point.average_minutes) > 0
      ? Number(point.average_price_usd) * Math.pow(Number(point.average_minutes) / 10, combinedCostWeight) * 100
      : null,
  }))
  const max = Math.max(0, ...rows.map((row) => row.raw || 0))
  return rows.map(({ point, raw }) => ({ ...point, combined_cost_index: raw != null && max > 0 ? raw / max * 100 : null }))
}
const normalizeModelCombinedCost = (models: RadarModel[]) => {
  const rows = models.map((model) => ({
    model,
    raw: Number(model.averageCost) > 0 && Number(model.averageMinutes ?? Number.parseFloat(model.time)) > 0
      ? Number(model.averageCost) * Math.pow(Number(model.averageMinutes ?? Number.parseFloat(model.time)) / 10, combinedCostWeight) * 100
      : null,
  }))
  const max = Math.max(0, ...rows.map((row) => row.raw || 0))
  return rows.map(({ model, raw }) => ({ ...model, combinedCostIndex: raw != null && max > 0 ? raw / max * 100 : undefined }))
}
const combinedPoints = (software: RadarPoint[], visual: RadarPoint[]) => {
  const visualByKey = new Map(visual.map((point) => [`${point.model}|${point.effort}`, point]))
  const combined = software.flatMap((left) => {
    const right = visualByKey.get(`${left.model}|${left.effort}`)
    if (!right || !Number.isFinite(Number(left.iq)) || !Number.isFinite(Number(right.iq))) return []
    const weighted = (a: number | null | undefined, b: number | null | undefined, aw: number | undefined, bw: number | undefined) => {
      if (a == null) return b ?? null
      if (b == null) return a
      const wa = Math.max(1, aw || 1); const wb = Math.max(1, bw || 1)
      return (a * wa + b * wb) / (wa + wb)
    }
    const leftTasks = left.valid_tasks ?? left.total
    const rightTasks = right.valid_tasks ?? right.total
    return [{ ...left, iq: Math.sqrt(Number(left.iq) * Number(right.iq)), valid_tasks: (leftTasks || 0) + (rightTasks || 0), runs_24h: (left.runs_24h || 0) + (right.runs_24h || 0), average_price_usd: weighted(left.average_price_usd, right.average_price_usd, left.price_samples ?? leftTasks, right.price_samples ?? rightTasks), average_minutes: weighted(left.average_minutes, right.average_minutes, left.duration_samples ?? leftTasks, right.duration_samples ?? rightTasks) }]
  })
  return normalizeCombinedCost(combined)
}

function formatUpdatedAt(value: string | undefined) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

function chartY(iq: number) {
  const ticks = yTicks.value
  const min = ticks[0] || 0
  const max = ticks[ticks.length - 1] || 120
  return 404 - ((Math.max(min, Math.min(max, iq)) - min) / Math.max(1, max - min)) * 384
}
function chartMetricValue(model: RadarModel) {
  const cost = model.averageCost ?? (Number.parseFloat(model.cost.replace(/[^0-9.]/g, '')) || 0.01)
  const time = model.averageMinutes ?? (Number.parseFloat(model.time.replace(/[^0-9.]/g, '')) || 1)
  if (chartMetric.value === '时间成本') return time
  if (chartMetric.value === '费用成本') return cost
  return Math.max(0.0002, model.combinedCostIndex ?? (cost * time) / 1000)
}
function formatMetricValue(model: RadarModel) {
  const value = chartMetricValue(model)
  return chartMetric.value === '费用成本' ? `$${value.toFixed(2)}` : chartMetric.value === '时间成本' ? `${value.toFixed(1)} 分钟` : value.toFixed(2)
}
function chartX(model: RadarModel) {
  const value = chartMetricValue(model)
  const { min, max } = chartDomain.value
  const broken = chartBreak.value
  const normalized = max === min ? 0.5 : broken && value >= broken.second
    ? broken.share + (Math.log(value / broken.second) / Math.max(1e-9, Math.log(max / broken.second))) * (1 - broken.share)
    : broken ? 0 : Math.max(0, Math.min(1, Math.log(value / min) / Math.log(max / min)))
  return 54 + normalized * 1008
}
function chartPath(points: Array<{ x: number; y: number }>) { return points.map((point, index) => `${index === 0 ? 'M' : 'L'}${point.x.toFixed(1)} ${point.y.toFixed(1)}`).join(' ') }
function trianglePoints(x: number, y: number) { return `${x},${y - 7} ${x - 7},${y + 6} ${x + 7},${y + 6}` }
function diamondPoints(x: number, y: number) { return `${x},${y - 7} ${x + 7},${y} ${x},${y + 7} ${x - 7},${y}` }
function hexagonPoints(x: number, y: number) { return `${x - 5.5},${y - 3.2} ${x},${y - 6.2} ${x + 5.5},${y - 3.2} ${x + 5.5},${y + 3.2} ${x},${y + 6.2} ${x - 5.5},${y + 3.2}` }
function starPoints(x: number, y: number) { return Array.from({ length: 10 }, (_, index) => { const radius = index % 2 ? 2.8 : 6; const angle = -Math.PI / 2 + index * Math.PI / 5; return `${(x + Math.cos(angle) * radius).toFixed(1)},${(y + Math.sin(angle) * radius).toFixed(1)}` }).join(' ') }
function setDeepSeekPriceBand(value: PriceBand) {
  activeDeepSeekPriceBand.value = value
  try { localStorage.setItem('downgrade-radar-deepseek-price-band', value) } catch { /* Storage may be disabled. */ }
}

async function fetchRadar<T extends RadarPayload>(resource: string, query = '') {
  const response = await fetch(`${radarApiBase}/${resource}${query}`, { cache: query ? 'no-store' : 'default', credentials: 'same-origin' })
  if (!response.ok) throw new Error(`radar ${resource}: HTTP ${response.status}`)
  return await response.json() as T
}

function buildRecommendations(payload: RadarPayload) {
  const icons: Record<string, string> = { daily_development: '⌨', hard_problems: '◆', background_automation: '↻', lobster_tasks: '◷' }
  return (payload.recommendations || []).map((item) => ({
    title: item.title,
    icon: icons[item.key] || '◎',
    description: item.rule || '根据当前公开测试数据生成的推荐参考。',
    models: (item.items || []).map((model) => ({
      name: `${familyForModel(model.model)} ${model.effort}`,
      iq: Number(model.iq) || 0,
      time: formatMinutes(model.average_duration_minutes),
      cost: formatMoney(model.average_cost_usd),
    })),
  }))
}

function buildRatings(payload: RadarPayload) {
  const grouped = new Map<string, RatingRow>()
  for (const item of payload.models || []) {
    const group = item.group.replace(/^GPT-5\.6 /, '').replace(/^GPT-5\.5$/, '5.5')
    const row = grouped.get(group) || { model: group, price: '—', scores: {}, votes: {} }
    const effort = item.id.split('-').pop() || 'off'
    row.scores[effort] = item.average
    row.votes[effort] = item.count
    grouped.set(group, row)
  }
  return [...grouped.values()]
}

async function refresh() {
  refreshing.value = true
  fetchError.value = ''
  try {
    const [software, visual, recommendations, ratings] = await Promise.all([
      fetchRadar<RadarPayload>('intelligence-efficiency-metrics', '?refresh=1'),
      fetchRadar<RadarPayload>('visual-spatial-reasoning', '?refresh=1'),
      fetchRadar<RadarPayload>('radar-insights', '?refresh=1'),
      fetchRadar<RadarPayload>('model-ratings?view=public'),
    ])
    const softwarePoints = software.points || []
    const visualPoints = visual.points || []
    apiModelSets.value = {
      [tabForBenchmark('software')]: normalizePoints(softwarePoints),
      [tabForBenchmark('visual')]: normalizePoints(visualPoints),
      [tabForBenchmark('comprehensive')]: normalizePoints(combinedPoints(softwarePoints, visualPoints)),
    }
    apiRecommendations.value = buildRecommendations(recommendations)
    apiRatingRows.value = buildRatings(ratings)
    updatedAt.value = formatUpdatedAt(recommendations.source_updated_at || software.source_updated_at || visual.source_updated_at)
  } catch (error) {
    fetchError.value = error instanceof Error ? error.message : '降智雷达数据暂时不可用'
  } finally {
    refreshing.value = false
  }
}

async function toggleChartFullscreen() {
  await nextTick()
  if (!chartPanel.value) return
  if (document.fullscreenElement) await document.exitFullscreen()
  else await chartPanel.value.requestFullscreen?.()
}

onMounted(() => {
  void refresh()
  refreshTimer = window.setInterval(() => void refresh(), 10 * 60 * 1000)
})
onBeforeUnmount(() => { if (refreshTimer) window.clearInterval(refreshTimer) })

function ratingKey(model: string, mode: string) { return `${model}-${mode}` }
async function rate(model: string, mode: string, score: number) {
  userRatings.value[ratingKey(model, mode)] = score
  const slug = model.toLowerCase().replace(/^gpt-5\.6 /, 'gpt-5.6-').replace(/^gpt-5\.5$/, 'gpt-5.5')
  try {
    await fetch(`${radarApiBase}/model-ratings`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, credentials: 'same-origin', body: JSON.stringify({ model: `${slug}-${mode}`, score }) })
  } catch {
    // Keep the local selection visible when anonymous rating submission is unavailable.
  }
}
</script>

<style scoped>
.radar-shell { color-scheme: dark; }
.radar-panel { border: 1px solid rgb(51 65 85 / .75); border-radius: .75rem; background: rgb(15 23 42 / .7); }
.radar-icon-button, .radar-info-button { display: inline-grid; place-items: center; border: 1px solid rgb(71 85 105 / .8); color: rgb(148 163 184); transition: .16s ease; }
.radar-icon-button { height: 2rem; width: 2rem; border-radius: .5rem; }
.radar-info-button { height: 1.5rem; width: 1.5rem; border-radius: 9999px; }
.radar-info-wrap { position: relative; flex: none; }
.radar-tooltip { position: absolute; top: calc(100% + .5rem); left: 0; z-index: 10; width: 15rem; border: 1px solid rgb(34 211 238 / .35); border-radius: .45rem; background: rgb(8 47 73 / .96); padding: .55rem .65rem; color: rgb(207 250 254); font-size: .7rem; line-height: 1.35rem; box-shadow: 0 8px 24px rgb(2 6 23 / .4); opacity: 0; visibility: hidden; pointer-events: none; transition: opacity .16s ease, visibility .16s ease; }
.radar-info-wrap:hover .radar-tooltip, .radar-info-wrap:focus-within .radar-tooltip { opacity: 1; visibility: visible; }
.radar-icon-button:hover, .radar-info-button:hover { color: rgb(103 232 249); border-color: rgb(34 211 238 / .7); background: rgb(8 47 73 / .45); }
.radar-tab { white-space: nowrap; border-radius: .375rem; padding: .45rem .65rem; font-size: .7rem; color: rgb(148 163 184); }
.radar-tab-active { background: rgb(34 211 238 / .12); color: rgb(165 243 252); }
.radar-family-grid { display: grid; gap: .5rem; overflow-x: auto; padding-bottom: .2rem; }
.radar-family-row { display: grid; grid-template-columns: repeat(6, minmax(10rem, 1fr)); min-width: 60rem; gap: .5rem; }
.radar-deepseek-divider { min-width: 60rem; height: 1px; margin: .35rem 0 .15rem; background: rgb(71 85 105 / .72); }
.radar-price-row { display: flex; min-width: 60rem; justify-content: flex-start; padding: .15rem 0; }
.radar-segmented-control { display: inline-flex; border: 1px solid rgb(71 85 105 / .8); border-radius: .4rem; overflow: hidden; background: rgb(15 23 42 / .78); }
.radar-segmented-control button { min-width: 4.5rem; padding: .38rem .7rem; color: rgb(148 163 184); font-size: .7rem; transition: background .16s ease, color .16s ease; }
.radar-segmented-control button + button { border-left: 1px solid rgb(71 85 105 / .8); }
.radar-segmented-control button[aria-pressed="true"] { background: rgb(37 99 235 / .28); color: rgb(191 219 254); }
.radar-segmented-control button:hover { color: rgb(219 234 254); }
.radar-deepseek-row .radar-score-card { border-color: rgb(71 85 105 / .9); }
.radar-score-card { display: flex; min-height: 4.875rem; align-items: stretch; justify-content: space-between; gap: .75rem; border: 1px solid rgb(51 65 85 / .75); border-radius: .65rem; padding: .65rem .75rem; background: rgb(15 23 42 / .55); transition: .16s ease; }
.radar-score-card:hover { border-color: rgb(34 211 238 / .7); background: rgb(8 47 73 / .4); }
.radar-score-iq { display: flex; min-width: 0; flex: 1; flex-direction: column; justify-content: space-between; }
.radar-score-heading { display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: .4rem; font-size: .72rem; }
.radar-score-heading > span:first-child { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.radar-score-samples { flex: none; border-radius: 9999px; background: rgb(34 211 238 / .1); padding: .1rem .35rem; color: rgb(103 232 249); font-size: .6rem; line-height: 1rem; }
.radar-score-meta { display: flex; flex: none; flex-direction: column; justify-content: space-between; text-align: right; color: rgb(148 163 184); font-size: .68rem; line-height: 1.25rem; }
.radar-chart-panel { position: relative; border: 1px solid rgb(51 65 85 / .75); border-radius: .65rem; padding: .9rem; background: rgb(2 6 23 / .45); }
.radar-select { border: 1px solid rgb(71 85 105 / .8); border-radius: .4rem; background: rgb(15 23 42); color: rgb(203 213 225); padding: .4rem .55rem; font-size: .7rem; }
.radar-chart-action { display: inline-grid; place-items: center; height: 1.8rem; width: 1.8rem; border: 1px solid rgb(71 85 105 / .8); border-radius: .4rem; color: rgb(148 163 184); }.radar-chart-action:hover { color: rgb(103 232 249); border-color: rgb(34 211 238 / .7); background: rgb(8 47 73 / .45); }
.radar-chart-legend { display: flex; flex-wrap: wrap; gap: .75rem 1rem; color: rgb(100 116 139); font-size: .65rem; }
.radar-chart-legend span { display: inline-flex; align-items: center; gap: .35rem; }
.radar-chart-legend i { display: inline-block; width: .55rem; height: .18rem; border-radius: 9999px; background: rgb(148 163 184); }
.radar-chart-scroll { overflow-x: auto; padding-bottom: .2rem; }
.radar-efficiency-svg { display: block; width: 100%; min-width: 720px; height: auto; }
.radar-chart-gridline { stroke: rgb(51 65 85 / .65); stroke-dasharray: 3 5; stroke-width: 1; }
.radar-chart-axis { stroke: rgb(100 116 139 / .8); stroke-width: 1.2; }
.radar-chart-break { fill: none; stroke: rgb(148 163 184); stroke-width: 1.5; stroke-linecap: round; }
.radar-chart-tick, .radar-chart-axis-label { fill: rgb(100 116 139); font-size: 11px; }
.radar-chart-line { fill: none; stroke-width: 2.5; stroke-linecap: round; stroke-linejoin: round; opacity: .85; }
.radar-chart-point { fill: rgb(2 6 23); stroke-width: 2.5; cursor: pointer; transition: fill .16s ease, stroke-width .16s ease; }
.radar-chart-point-group:hover .radar-chart-point, .radar-chart-point-group:focus .radar-chart-point { stroke-width: 3; }
.radar-chart-point-group:focus { outline: none; }
.radar-chart-tooltip { position: absolute; z-index: 4; right: .9rem; top: 4.5rem; display: grid; gap: .2rem; min-width: 10rem; border: 1px solid rgb(34 211 238 / .45); border-radius: .45rem; background: rgb(8 47 73 / .96); padding: .55rem .65rem; color: rgb(207 250 254); font-size: .68rem; line-height: 1.25rem; box-shadow: 0 8px 24px rgb(2 6 23 / .4); pointer-events: none; }.radar-chart-tooltip strong { color: white; }
.radar-chart-point-label { fill: rgb(148 163 184); font-size: 10px; pointer-events: none; }
.radar-chart-point.chart-point-selected { fill: rgb(30 41 59); stroke-width: 3.5; }
.radar-check { display: inline-flex; align-items: center; gap: .4rem; border: 1px solid rgb(51 65 85 / .8); border-radius: .45rem; padding: .4rem .55rem; font-size: .7rem; color: rgb(148 163 184); cursor: pointer; }.radar-check input { accent-color: rgb(34 211 238); }.radar-check-active { border-color: rgb(34 211 238 / .65); color: rgb(165 243 252); background: rgb(8 47 73 / .35); }
.radar-ratings-table th, .radar-ratings-table td { border-bottom: 1px solid rgb(51 65 85 / .65); padding: .75rem .7rem; text-align: left; vertical-align: top; }.radar-ratings-table thead th { background: rgb(15 23 42 / .85); color: rgb(100 116 139); font-size: .68rem; font-weight: 600; }.radar-ratings-table thead th span, .radar-ratings-table tbody th span { display: block; margin-top: .25rem; color: rgb(100 116 139); font-size: .62rem; font-weight: 400; }.radar-ratings-table tbody th { min-width: 145px; color: rgb(226 232 240); font-size: .75rem; }.rating-cell { min-width: 84px; }.rating-cell strong { color: rgb(165 243 252); font-size: .9rem; }.rating-cell > div > span { color: rgb(100 116 139); font-size: .62rem; }.rating-stars { display: flex; gap: .05rem; margin-top: .2rem; }.rating-stars button { padding: 0; color: rgb(71 85 105); font-size: .8rem; line-height: 1; }.rating-stars button:hover, .rating-stars .rating-star-active { color: rgb(250 204 21); }.rating-hint { display: block; margin-top: .25rem; color: rgb(100 116 139); font-size: .6rem; }
</style>
