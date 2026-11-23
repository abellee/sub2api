<template>
  <div class="inflation-history">
    <p v-if="loading" role="status">{{ t('common.loading') }}</p>
    <button v-else-if="failed" class="btn btn-secondary" @click="load">{{ t('inflationMonitor.retry') }}</button>
    <template v-else-if="history">
      <p class="inflation-heading">{{ t('inflationMonitor.chart') }} · {{ t(`inflationMonitor.${history.aggregation}`) }}</p>
      <svg v-if="candles.some(Boolean)" viewBox="0 0 320 150" class="inflation-chart" role="group" :aria-label="t('inflationMonitor.chart')">
        <g v-for="tick in [0, 1, 2]" :key="tick">
          <line x1="35" x2="316" :y1="20 + tick * 50" :y2="20 + tick * 50" stroke="currentColor" opacity=".15" />
          <text x="31" :y="23 + tick * 50" text-anchor="end">{{ format(bounds.max - tick * (bounds.max - bounds.min) / 2) }}</text>
        </g>
        <template v-for="(candle, index) in candles" :key="index">
          <g v-if="candle" role="button" tabindex="0" :aria-label="candleLabel(index)" :class="tone(candle.close)" @click="selected = index" @keydown.enter.prevent="selected = index" @keydown.space.prevent="selected = index">
            <title>{{ candleLabel(index) }}</title>
            <rect :x="x(index) - 5" y="8" width="10" height="122" fill="transparent" />
            <line :x1="x(index)" :x2="x(index)" :y1="y(candle.high)" :y2="y(candle.low)" stroke="currentColor" />
            <rect :x="x(index) - 3" :y="Math.min(y(candle.open), y(candle.close))" width="6" :height="Math.max(2, Math.abs(y(candle.open) - y(candle.close)))" fill="currentColor" :opacity="selected === index ? 1 : .65" />
            <circle v-if="selected === index" :cx="x(index)" cy="134" r="2" fill="currentColor" />
          </g>
        </template>
        <text x="35" y="147">{{ time(history.results[0]?.measured_at) }}</text>
        <text x="316" y="147" text-anchor="end">{{ time(history.results.at(-1)?.measured_at) }}</text>
      </svg>
      <p v-else>{{ t('inflationMonitor.noData') }}</p>
      <template v-if="result">
        <div class="inflation-selection"><span>{{ time(result.measured_at) }}</span><strong :class="tone(rate)">{{ format(rate) }}</strong></div>
        <dl class="inflation-settings">
          <dt>{{ t('inflationMonitor.effort') }}</dt><dd>{{ result.reasoning_effort || t('inflationMonitor.defaultEffort') }}</dd>
          <dt>{{ t('inflationMonitor.encoding') }}</dt><dd>{{ result.encoding }}</dd>
        </dl>
        <div v-for="(sample, index) in result.samples" :key="index" class="inflation-sample">
          <p>{{ t('inflationMonitor.sample') }} {{ index + 1 }} · {{ sample.model }} <strong :class="tone(sample.rate)">{{ format(sample.rate) }}</strong></p>
          <p v-if="sample.error">{{ t('inflationMonitor.failed') }}</p>
          <dl v-else class="inflation-settings">
            <dt>{{ t('inflationMonitor.input') }}</dt><dd>{{ sample.input_tokens ?? '—' }}</dd>
            <dt>{{ t('inflationMonitor.output') }}</dt><dd>{{ sample.output_tokens ?? '—' }}</dd>
            <dt>{{ t('inflationMonitor.textTokens') }}</dt><dd>{{ sample.text_tokens }}</dd>
            <dt>{{ t('inflationMonitor.reasoningTokens') }}</dt><dd>{{ sample.reasoning_tokens ?? '—' }}</dd>
          </dl>
        </div>
        <details><summary>{{ t('inflationMonitor.prompt') }}</summary><p class="inflation-prompt">{{ result.prompt }}</p></details>
      </template>
      <footer><p>{{ t('inflationMonitor.formula') }}</p><p>{{ t('inflationMonitor.aggregateHelp') }}</p><p>{{ t('inflationMonitor.candleHelp') }}</p></footer>
    </template>
  </div>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { inflationMonitorAPI, inflationAggregate, inflationCandles, inflationTone, type InflationHistory } from '@/api/inflationMonitor'
const props = defineProps<{ keyId: number }>()
const { t, locale } = useI18n()
const loading = ref(true), failed = ref(false), history = ref<InflationHistory | null>(null), selected = ref(0)
let controller: AbortController | null = null
async function load() {
  controller?.abort()
  const request = new AbortController(); controller = request
  loading.value = true; failed.value = false; history.value = null
  try {
    const data = await inflationMonitorAPI.history(props.keyId, request.signal)
    if (request.signal.aborted) return
    history.value = data; selected.value = data.results.length - 1
  } catch { if (!request.signal.aborted) failed.value = true }
  finally { if (!request.signal.aborted) loading.value = false }
}
const candles = computed(() => history.value ? inflationCandles(history.value) : [])
const result = computed(() => history.value?.results[selected.value])
const rate = computed(() => result.value && history.value ? inflationAggregate(result.value.samples, history.value.aggregation) : null)
const bounds = computed(() => {
  const valid = candles.value.filter(c => c !== null)
  const min = Math.min(0, ...valid.map(c => c.low)), max = Math.max(1, ...valid.map(c => c.high))
  const padding = (max - min) * .08
  return { min: min - padding, max: max + padding }
})
const x = (index: number) => 40 + index * 270 / 24
const y = (value: number) => 120 - (value - bounds.value.min) / (bounds.value.max - bounds.value.min) * 100
const tone = (value: number | null) => inflationTone(value, history.value?.yellow_threshold ?? 10, history.value?.red_threshold ?? 30)
const format = (value: number | null) => value === null ? '—' : `${value.toFixed(1)}%`
function time(value?: string) { return value ? new Date(value).toLocaleString(locale.value, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) : '' }
function candleLabel(index: number) {
  const c = candles.value[index]
  return c ? `${time(history.value?.results[index]?.measured_at)} · ${t('inflationMonitor.open')} ${format(c.open)} · ${t('inflationMonitor.close')} ${format(c.close)} · ${t('inflationMonitor.high')} ${format(c.high)} · ${t('inflationMonitor.low')} ${format(c.low)}` : ''
}
watch(() => props.keyId, load, { immediate: true })
onBeforeUnmount(() => controller?.abort())
</script>
<style scoped>
.inflation-history { padding: 16px; font-size: 11px; color: var(--studio-ink); }
.inflation-heading { font-weight: 600; margin-bottom: 8px; }
.inflation-chart { display: block; width: 100%; color: var(--studio-muted); }
.inflation-chart text { fill: currentColor; font-size: 8px; }
.inflation-chart [role=button] { cursor: pointer; outline: none; }
.inflation-chart [role=button]:focus-visible { outline: 1px solid currentColor; }
.healthy { color: #16a34a; }.warning { color: #d97706; }.critical { color: #e5484d; }.unknown { color: var(--studio-muted); }
.inflation-selection { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 6px; margin: 8px 0; }
.inflation-settings { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 4px 8px; margin-top: 6px; overflow-wrap: anywhere; }
.inflation-settings dt { color: var(--studio-muted); }.inflation-settings dd { text-align: right; font-variant-numeric: tabular-nums; }
.inflation-sample { border-top: 1px solid var(--studio-haze); padding-top: 8px; margin-top: 8px; overflow-wrap: anywhere; }
.inflation-sample strong { float: right; }.inflation-history details { margin-top: 12px; }.inflation-history summary { cursor: pointer; }
.inflation-prompt { white-space: pre-wrap; overflow-wrap: anywhere; margin-top: 8px; }
.inflation-history footer { margin-top: 14px; color: var(--studio-muted); line-height: 1.7; }.inflation-history footer p + p { margin-top: 8px; }
</style>
