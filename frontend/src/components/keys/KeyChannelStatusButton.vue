<template>
  <button
    type="button"
    class="key-channel-status"
    :title="`${card.successLabel}\n${card.ttftLabel}\n${card.cacheLabel}`"
    :aria-label="`${t('channelMonitorV2.title')} · ${card.label}`"
    aria-haspopup="dialog"
    @click="$emit('open')"
  >
    <strong :class="metricTextClass(card.successState)">{{ card.successRate }}</strong>
    <strong :class="metricTextClass(card.ttftState)">{{ card.ttft }}</strong>
    <strong :class="metricTextClass(card.cacheState)">{{ card.cacheRate }}</strong>
  </button>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { metricTextClass } from '@/features/channel-monitor-v2-studio/studioFormat'
import type { StudioGroupCard } from '@/features/channel-monitor-v2-studio/useStudioGroupCards'

defineProps<{ card: StudioGroupCard }>()
defineEmits<{ open: [] }>()
const { t } = useI18n()
</script>

<style scoped>
.key-channel-status { display: flex; flex: 0 0 auto; min-height: 46px; flex-direction: column; align-items: center; align-self: stretch; justify-content: center; gap: 1px; padding: 0; border-radius: 8px; font-size: 10px; line-height: 12px; color: var(--studio-muted); font-variant-numeric: tabular-nums; white-space: nowrap; }
.key-channel-status strong { flex-shrink: 0; font-weight: 600; }
.key-channel-status:hover { background: var(--studio-haze); }
.key-channel-status:focus-visible { outline: 2px solid var(--tf-console-active-ink); outline-offset: 2px; }
</style>
