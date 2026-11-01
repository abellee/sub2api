<template>
  <div class="studio-token-trend">
    <div v-if="loading" class="flex h-64 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="trendData.length > 0 && chartData">
      <div class="trend-legend" :aria-label="t('dashboard.tokenUsageTrend')"><button v-for="(dataset, index) in chartData.datasets" :key="dataset.label" type="button" :aria-pressed="!hiddenSeries.includes(index)" :class="{ muted: hiddenSeries.includes(index) }" @click="toggleSeries(index)"><i :style="{ borderColor: dataset.borderColor }" aria-hidden="true"></i><span>{{ dataset.label }}</span></button></div>
      <div class="h-64">
      <Line :data="chartData" :options="lineOptions" />
      </div>
    </div>
    <div
      v-else
      class="flex h-64 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
    >
      {{ t('admin.dashboard.noDataAvailable') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useTheme } from '@/composables/useTheme'
import { useI18n } from 'vue-i18n'
import {
  type ScriptableContext,
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { TrendDataPoint } from '@/types'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

const { t } = useI18n()

const props = defineProps<{
  trendData: TrendDataPoint[]
  loading?: boolean
}>()

const hiddenSeries = ref<number[]>([])
function toggleSeries(index: number) {
  hiddenSeries.value = hiddenSeries.value.includes(index) ? hiddenSeries.value.filter(value => value !== index) : [...hiddenSeries.value, index]
}
const { isDark: isDarkMode } = useTheme()

const chartColors = computed(() => ({
  text: isDarkMode.value ? '#99a3b9' : '#73809b',
  grid: isDarkMode.value ? '#ffffff0a' : '#253b650a',
  input: '#4d7bfa',
  output: '#ff9657',
  cacheCreation: '#35bfd0',
  cacheRead: '#8297f5',
  cacheHitRate: '#f5b94f'
}))

function areaGradient(color: string) {
  return ({ chart }: ScriptableContext<'line'>) => {
    const { ctx, chartArea } = chart
    if (!chartArea) return `${color}10`
    const gradient = ctx.createLinearGradient(0, chartArea.top, 0, chartArea.bottom)
    gradient.addColorStop(0, `${color}${isDarkMode.value ? '30' : '26'}`)
    gradient.addColorStop(1, `${color}00`)
    return gradient
  }
}

const chartData = computed(() => {
  if (!props.trendData?.length) return null

  return {
    labels: props.trendData.map((d) => d.date),
    datasets: [
      {
        label: t('dashboard.input'),
        data: props.trendData.map((d) => d.input_tokens),
        borderColor: chartColors.value.input,
        backgroundColor: areaGradient(chartColors.value.input),
        fill: true,
        tension: 0.4,
        borderWidth: 2.5,
        pointRadius: props.trendData.length <= 14 ? 3 : 0,
        pointBackgroundColor: chartColors.value.input,
        pointHoverRadius: 5
      },
      {
        label: t('dashboard.output'),
        data: props.trendData.map((d) => d.output_tokens),
        borderColor: chartColors.value.output,
        backgroundColor: `${chartColors.value.output}20`,
        fill: false,
        tension: 0.4,
        borderWidth: 2,
        pointRadius: 0,
        pointHoverRadius: 5
      },
      {
        label: t('dashboard.studio.cacheCreation'),
        data: props.trendData.map((d) => d.cache_creation_tokens),
        borderColor: chartColors.value.cacheCreation,
        backgroundColor: `${chartColors.value.cacheCreation}20`,
        fill: false,
        tension: 0.4,
        borderWidth: 2,
        pointRadius: 0,
        pointHoverRadius: 5
      },
      {
        label: t('dashboard.studio.cacheRead'),
        data: props.trendData.map((d) => d.cache_read_tokens),
        borderColor: chartColors.value.cacheRead,
        backgroundColor: areaGradient(chartColors.value.cacheRead),
        fill: true,
        tension: 0.4,
        borderWidth: 2.5,
        pointRadius: props.trendData.length <= 14 ? 3 : 0,
        pointBackgroundColor: chartColors.value.cacheRead,
        pointHoverRadius: 5
      },
      {
        label: t('dashboard.studio.cacheHitRate'),
        data: props.trendData.map((d) => {
          const totalPromptTokens = d.input_tokens + d.cache_read_tokens + d.cache_creation_tokens
          return totalPromptTokens > 0 ? (d.cache_read_tokens / totalPromptTokens) * 100 : 0
        }),
        borderColor: chartColors.value.cacheHitRate,
        backgroundColor: `${chartColors.value.cacheHitRate}20`,
        borderDash: [5, 5],
        fill: false,
        tension: 0.3,
        borderWidth: 2,
        pointRadius: 0,
        pointHoverRadius: 5,
        yAxisID: 'yPercent'
      }
    ].map((dataset, index) => ({ ...dataset, hidden: hiddenSeries.value.includes(index) }))
  }
})

const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  animation: false as const,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: { display: false },
    tooltip: {
      backgroundColor: isDarkMode.value ? '#282d46' : '#ffffff',
      titleColor: isDarkMode.value ? '#e9edfa' : '#20273e',
      bodyColor: chartColors.value.text,
      footerColor: chartColors.value.text,
      padding: 14,
      cornerRadius: 12,
      boxPadding: 5,
      usePointStyle: true,
      callbacks: {
        label: (context: any) => {
          if (context.dataset.yAxisID === 'yPercent') {
            return `${context.dataset.label}: ${context.raw.toFixed(1)}%`
          }
          return `${context.dataset.label}: ${formatTokens(context.raw)}`
        },
        footer: (tooltipItems: any) => {
          const dataIndex = tooltipItems[0]?.dataIndex
          if (dataIndex !== undefined && props.trendData[dataIndex]) {
            const data = props.trendData[dataIndex]
            return `${t('dashboard.actual')}: $${formatCost(data.actual_cost)} | ${t('dashboard.standard')}: $${formatCost(data.cost)}`
          }
          return ''
        }
      }
    }
  },
  scales: {
    x: {
      border: { display: false },
      grid: {
        display: false
      },
      ticks: {
        color: chartColors.value.text,
        maxTicksLimit: 8,
        maxRotation: 0,
        padding: 10,
        font: {
          size: 12
        }
      }
    },
    y: {
      beginAtZero: true,
      border: { display: false },
      grid: {
        color: chartColors.value.grid,
        drawTicks: false
      },
      ticks: {
        color: chartColors.value.text,
        maxTicksLimit: 5,
        padding: 10,
        font: {
          size: 12
        },
        callback: (value: string | number) => formatTokens(Number(value))
      }
    },
    yPercent: {
      border: { display: false },
      position: 'right' as const,
      min: 0,
      max: 100,
      grid: {
        drawOnChartArea: false
      },
      ticks: {
        maxTicksLimit: 5,
        padding: 10,
        color: chartColors.value.cacheHitRate,
        font: {
          size: 12
        },
        callback: (value: string | number) => `${value}%`
      }
    }
  }
}))

const formatTokens = (value: number): string => {
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const formatCost = (value: number): string => {
  if (value >= 1000) {
    return (value / 1000).toFixed(2) + 'K'
  } else if (value >= 1) {
    return value.toFixed(2)
  } else if (value >= 0.01) {
    return value.toFixed(3)
  }
  return value.toFixed(4)
}
</script>

<style scoped>
.trend-legend{display:flex;justify-content:center;flex-wrap:wrap;gap:8px 18px;margin-bottom:16px}.trend-legend button{display:inline-flex;align-items:center;gap:7px;min-height:24px;padding:2px 4px;border:0;border-radius:5px;background:transparent;color:var(--studio-muted);font-size:12px;line-height:20px}.trend-legend button:hover{background:var(--studio-haze)}.trend-legend i{display:block;flex-shrink:0;width:12px;height:12px;border:2px solid;border-radius:50%;box-sizing:border-box}.trend-legend span{display:block;line-height:20px}.trend-legend .muted{opacity:.45}.trend-legend .muted span{text-decoration:line-through}.trend-legend button:focus-visible{outline:2px solid var(--tf-console-active-ink);outline-offset:2px}
</style>
