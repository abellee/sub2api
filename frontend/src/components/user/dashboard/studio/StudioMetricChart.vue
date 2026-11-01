<template>
  <div class="metric-chart" :class="kind">
    <template v-if="kind === 'balance'">
      <div class="balance-detail"><span class="balance-symbol"><img :src="pointsIcon" alt="" /></span><span>{{ t('common.points') }}</span><strong>—</strong></div>
    </template>
    <template v-else-if="kind === 'cost'">
      <div class="cost-bars" role="img" :aria-label="`${t('dashboard.actual')}: ${value}, ${t('dashboard.standard')}: ${reference}`"><span><i :style="{ width: `${maximum ? value / maximum * 100 : 0}%` }"></i></span><span><i :style="{ width: `${maximum ? reference / maximum * 100 : 0}%` }"></i></span></div>
      <div class="chart-key horizontal"><span><i></i>{{ t('dashboard.actual') }}</span><span class="secondary"><i></i>{{ t('dashboard.standard') }}</span></div>
    </template>
    <template v-else>
      <svg v-if="points.length" class="request-spark" viewBox="0 0 240 44" preserveAspectRatio="none" role="img" :aria-label="`${t('dashboard.studio.rangeRequests')} · ${caption}`"><g v-for="(point, index) in points" :key="index"><rect class="request-track" :x="barX(index)" y="0" :width="barWidth" height="44" :rx="barWidth / 2" /><rect class="request-bar" :x="barX(index)" :y="44 - barHeight(point)" :width="barWidth" :height="barHeight(point)" :rx="barWidth / 2"><title>{{ labels[index] }} · {{ t('dashboard.requests') }}: {{ point.toLocaleString() }}</title></rect></g></svg>
      <div v-else class="spark-empty">{{ t('dashboard.noDataAvailable') }}</div>
      <span class="chart-caption">{{ caption || t('dashboard.studio.rangeRequests') }}</span>
    </template>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import pointsIcon from '@/assets/icons/points-image-color.svg'
const props = withDefaults(defineProps<{ kind: 'balance' | 'requests' | 'cost'; value: number; reference?: number; points?: number[]; labels?: string[]; caption?: string }>(), { reference: 0, points: () => [], labels: () => [], caption: '' })
const { t } = useI18n()
const maximum = computed(() => Math.max(props.value, props.reference, 0))
const peak = computed(() => Math.max(0, ...props.points))
const barWidth = computed(() => Math.min(9, 240 / (props.points.length || 1) * 0.6))
const barX = (index: number) => (index + 0.5) / props.points.length * 240 - barWidth.value / 2
const barHeight = (point: number) => Math.max(0, point) / (peak.value || 1) * 44
</script>
<style scoped>
.balance-symbol img{width:20px;height:20px}
.metric-chart{color:var(--studio-accent,#4d7bfa);display:grid;grid-template-rows:44px 20px;gap:8px;min-width:0}.metric-chart.balance{display:flex;align-items:center}.balance-detail{display:flex;align-items:center;gap:10px;width:100%;min-width:0;font-size:12px;color:var(--studio-muted)}.balance-symbol{display:grid;place-items:center;flex:none;width:36px;height:36px;border-radius:12px;background:var(--studio-haze);color:#8297f5}.balance-detail strong{margin-left:auto;color:var(--studio-ink);font-size:17px;font-weight:500;font-variant-numeric:tabular-nums;overflow-wrap:anywhere}
.chart-key{display:flex;flex-direction:row;justify-content:space-between;gap:8px;font-size:12px;color:var(--studio-muted)}.chart-key span{white-space:nowrap;display:flex;align-items:center;gap:6px}.chart-key i{display:inline-block;width:6px;height:6px;border-radius:50%;background:currentColor;color:var(--studio-accent,#4d7bfa)}.chart-key .secondary i{background:#bac5e2}.horizontal{flex-direction:row;justify-content:space-between;margin-top:0}
.cost-bars{display:flex;flex-direction:column;gap:10px;justify-content:center}.cost-bars>span{height:6px;border-radius:8px;background:var(--studio-haze);overflow:hidden}.cost-bars i{display:block;height:100%;border-radius:inherit;background:linear-gradient(90deg,#ff874e,#ffb36b)}.cost-bars>span+span i{background:linear-gradient(90deg,#6992fc,#a9bcfa)}
.request-spark{height:44px;width:100%;overflow:visible}.request-track{fill:var(--studio-haze)}.request-bar{fill:currentColor}.request-spark g:nth-child(3n+2) .request-bar{fill:#ff9657}.request-spark g:nth-child(3n) .request-bar{fill:#35bfd0}.chart-caption{font-size:12px;color:var(--studio-muted);display:block;margin-top:0;line-height:20px}.spark-empty{font-size:12px;color:var(--studio-muted);height:44px;display:flex;align-items:center}
@media(max-width:700px){.balance-detail{flex-wrap:wrap;gap:6px}.balance-symbol{width:28px;height:28px;border-radius:9px}.balance-detail strong{font-size:15px}.chart-caption{font-size:12px;white-space:nowrap}}
</style>
