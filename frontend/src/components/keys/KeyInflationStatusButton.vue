<template>
  <button type="button" class="key-inflation-status" :class="inflationTone(summary.rate, summary.yellow_threshold, summary.red_threshold)" :title="tooltip" :aria-label="tooltip" aria-haspopup="dialog" @click="$emit('open')">
    <span class="key-inflation-value">{{ summary.rate === null ? t('inflationMonitor.notMeasured') : `${summary.rate.toFixed(1)}%` }}</span>
    <Icon name="infoCircle" size="xs" aria-hidden="true" />
  </button>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { inflationTone, type InflationSummary } from '@/api/inflationMonitor'
const props = defineProps<{ summary: InflationSummary }>()
defineEmits<{ open: [] }>()
const { t, locale } = useI18n()
const tooltip = computed(() => t('inflationMonitor.tooltip', { mode: t(`inflationMonitor.${props.summary.aggregation}`), time: props.summary.measured_at ? new Date(props.summary.measured_at).toLocaleString(locale.value) : t('inflationMonitor.notMeasured') }))
</script>
<style scoped>
.key-inflation-status { display: inline-flex; align-items: center; flex: 0 0 auto; gap: 4px; padding: 0; border: 0; background: transparent; border-radius: 3px; font-size: 12px; font-weight: 600; font-variant-numeric: tabular-nums; white-space: nowrap; }
.key-inflation-status:hover .key-inflation-value { text-decoration: underline; text-underline-offset: 3px; }.key-inflation-status:focus-visible { outline: 2px solid var(--tf-console-active-ink); outline-offset: 2px; }
.healthy { color: #16a34a; }.warning { color: #d97706; }.critical { color: #e5484d; }.unknown { color: var(--studio-muted); }
</style>
