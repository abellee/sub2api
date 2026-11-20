<template>
  <div class="studio-chart-skeleton" :class="`skeleton-${kind}`" role="status" :aria-label="t('common.loading')" aria-busy="true">
    <div v-if="kind === 'model'" class="model-placeholder" aria-hidden="true">
      <div class="orbit-placeholder"><Skeleton variant="circle" width="68%" height="180px" /></div>
      <div class="legend-placeholder"><div v-for="row in 5" :key="row" class="legend-row"><Skeleton variant="circle" :width="8" :height="8" /><Skeleton :width="`${72 - row * 6}%`" :height="12" /><Skeleton width="22%" :height="12" /></div><div class="legend-totals"><Skeleton v-for="column in 3" :key="column" :height="42" /></div></div>
    </div>
    <div v-else-if="kind === 'trend'" class="trend-placeholder" aria-hidden="true">
      <div class="legend-chips"><Skeleton v-for="chip in 4" :key="chip" :width="76" :height="14" /></div>
      <div class="plot-placeholder"><div v-for="line in 5" :key="line" class="grid-line"><Skeleton :width="28" :height="10" /><span /></div></div>
      <div class="axis-placeholder"><Skeleton v-for="tick in 7" :key="tick" :width="32" :height="10" /></div>
    </div>
    <div v-else class="request-placeholder" aria-hidden="true"><Skeleton :height="44" /><Skeleton width="65%" :height="12" /></div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Skeleton from '@/components/common/Skeleton.vue'
defineProps<{ kind: 'model' | 'trend' | 'requests' }>()
const { t } = useI18n()
</script>

<style scoped>
.studio-chart-skeleton{min-width:0}
.studio-chart-skeleton :deep(.animate-pulse){background:linear-gradient(90deg,transparent 30%,color-mix(in srgb,var(--studio-muted) 8%,transparent) 50%,transparent 70%) 200% 0/200% 100% no-repeat var(--studio-haze);animation:studio-skeleton-shimmer 3.6s ease-in-out infinite}
.model-placeholder{display:grid;grid-template-columns:minmax(0,1.35fr) minmax(185px,1fr);gap:12px;min-height:374px;align-items:start}
.orbit-placeholder{height:374px;display:flex;align-items:center;justify-content:center}
.legend-placeholder{padding-top:36px}.legend-row{height:49px;display:flex;align-items:center;gap:10px}.legend-row>:last-child{margin-left:auto}
.legend-totals{display:grid;grid-template-columns:repeat(3,1fr);gap:10px;margin-top:12px;padding-top:12px;border-top:1px solid var(--studio-haze)}
.trend-placeholder{height:290px;display:flex;flex-direction:column;gap:16px}
.legend-chips{display:flex;gap:18px;flex-wrap:wrap;min-height:18px}.plot-placeholder{display:flex;flex:1;flex-direction:column;justify-content:space-between}
.grid-line{display:flex;align-items:center;gap:14px}.grid-line>span{flex:1;border-top:1px solid var(--studio-haze)}
.axis-placeholder{display:flex;justify-content:space-between;padding-left:42px}
.request-placeholder{display:grid;grid-template-rows:44px 20px;gap:8px;align-items:center}
@media(max-width:700px){.model-placeholder{grid-template-columns:1fr}.orbit-placeholder{height:290px}.legend-placeholder{padding-top:0}.legend-row{height:36px}.axis-placeholder>:nth-child(even){display:none}.legend-chips{gap:12px}}
@keyframes studio-skeleton-shimmer{from{background-position:200% 0}to{background-position:-200% 0}}
@media(prefers-reduced-motion:reduce){.studio-chart-skeleton :deep(.animate-pulse){animation:none}}
</style>
