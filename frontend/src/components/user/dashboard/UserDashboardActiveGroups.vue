<template>
  <section
    v-if="enabled && cards.length"
    class="dashboard-active-groups"
    data-test="dashboard-active-groups"
    :aria-label="t('channelMonitorV2.studio.groups.activeAria')"
  >
    <header class="mb-4 flex items-center justify-between gap-3">
      <div class="flex min-w-0 items-center gap-3">
        <span class="studio-brand-heading-mark">
          <Icon name="radar" size="md" class="studio-active-mark-icon" />
        </span>
        <h2 class="text-base font-extrabold tracking-tight text-gray-900 dark:text-white">
          {{ t('channelMonitorV2.studio.groups.active') }}
        </h2>
      </div>
      <router-link
        :to="{ name: 'ChannelStatus' }"
        class="shrink-0 text-sm font-medium text-primary-600 transition-colors hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
      >
        {{ t('dashboard.viewAllChannelStatus') }}
      </router-link>
    </header>

    <div class="dashboard-active-groups-grid grid grid-cols-1 gap-6 lg:grid-cols-2">
      <StudioKpiCard
        v-for="card in cards"
        :key="card.key"
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
        :coverage="coverage"
        :show-throughput="showThroughput"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import StudioKpiCard from '@/features/channel-monitor-v2-studio/StudioKpiCard.vue'
import { pickDashboardActiveGroups, sortStudioCardsByStatus } from '@/features/channel-monitor-v2-studio/studioFormat'
import { useStudioGroupCards } from '@/features/channel-monitor-v2-studio/useStudioGroupCards'
import type { MonitorRange } from '@/api/channelMonitorV2'
import { isChannelMonitorV2Mode } from '@/utils/featureFlags'

const { t } = useI18n()
const enabled = computed(() => isChannelMonitorV2Mode())
const range = ref<MonitorRange>('90m')
const { groupCards, coverage, showThroughput } = useStudioGroupCards({
  range,
  enabled,
  reportError: false,
})
const cards = computed(() => sortStudioCardsByStatus(pickDashboardActiveGroups(groupCards.value)))
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
  color: #14b8a6;
  background: color-mix(in srgb, #14b8a6 16%, white);
}
.studio-active-mark-icon {
  color: inherit;
}
.dark .studio-brand-heading-mark {
  background: color-mix(in srgb, #14b8a6 24%, rgb(15 23 42));
}
@media (min-width: 1024px) {
  .dashboard-active-groups-grid > :last-child:nth-child(odd) {
    grid-column: 1 / -1;
  }
}
</style>
