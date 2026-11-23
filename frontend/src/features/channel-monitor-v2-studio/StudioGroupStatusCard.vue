<template>
  <div
    class="studio-model-row overflow-hidden rounded-3xl shadow-sm ring-1 ring-gray-900/5 dark:ring-dark-700"
    :class="{ 'studio-model-row--critical': card.state === 'critical' }"
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
      :row-index="rowIndex"
    />
  </div>
</template>

<script setup lang="ts">
import type { MonitorCoverage } from '@/api/channelMonitorV2'
import StudioKpiCard from './StudioKpiCard.vue'
import StudioStatusFaces from './StudioStatusFaces.vue'
import type { StudioGroupCard } from './useStudioGroupCards'
import type { StudioThresholds } from './studioFormat'

defineProps<{
  card: StudioGroupCard
  faceCoverage: MonitorCoverage | null
  showThroughput: boolean
  healthThresholds: StudioThresholds | null
  rowIndex?: number
}>()
</script>

<style scoped>
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
.studio-model-row--critical {
  filter: brightness(0.72);
}
.dark .studio-model-row {
  --studio-faces-bg: rgb(30 41 59);
}
</style>
