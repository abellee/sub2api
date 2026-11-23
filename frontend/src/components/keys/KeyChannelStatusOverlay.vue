<template>
  <KeyCardOverlay :label="card.label" @close="$emit('close')">
    <StudioGroupStatusCard :card="card" :face-coverage="faceCoverage" :show-throughput="showThroughput" :health-thresholds="healthThresholds" />
  </KeyCardOverlay>
</template>
<script setup lang="ts">
import KeyCardOverlay from './KeyCardOverlay.vue'
import StudioGroupStatusCard from '@/features/channel-monitor-v2-studio/StudioGroupStatusCard.vue'
import type { StudioGroupCard } from '@/features/channel-monitor-v2-studio/useStudioGroupCards'
import type { MonitorCoverage } from '@/api/channelMonitorV2'
import type { StudioThresholds } from '@/features/channel-monitor-v2-studio/studioFormat'
defineProps<{ card: StudioGroupCard; faceCoverage: MonitorCoverage | null; showThroughput: boolean; healthThresholds: StudioThresholds | null }>()
defineEmits<{ close: [] }>()
</script>
