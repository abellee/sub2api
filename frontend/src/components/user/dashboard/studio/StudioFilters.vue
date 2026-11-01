<template>
  <div class="studio-filters">
    <div class="filter-field"><span>{{ t('dashboard.timeRange') }}</span><DateRangePicker :start-date="startDate" :end-date="endDate" @update:start-date="$emit('update:startDate', $event)" @update:end-date="$emit('update:endDate', $event)" @change="$emit('rangeChange')" /></div>
    <div class="filter-field granularity"><span>{{ t('dashboard.granularity') }}</span><Select :model-value="granularity" :aria-label="t('dashboard.granularity')" :options="[{ value: 'day', label: t('dashboard.day') }, { value: 'hour', label: t('dashboard.hour') }]" @update:model-value="changeGranularity"><template #option="{ option, selected }"><span class="studio-granularity-option">{{ option.label }}</span><Icon v-if="selected" name="check" size="sm" /></template></Select></div>
    <button class="refresh-button" type="button" :disabled="loading" :aria-label="t('common.refresh')" :title="t('common.refresh')" @click="$emit('refresh')"><Icon name="refresh" size="sm" /></button>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

defineProps<{ startDate: string; endDate: string; granularity: 'day' | 'hour'; loading: boolean }>()
const emit = defineEmits<{
  'update:startDate': [value: string]
  'update:endDate': [value: string]
  'update:granularity': [value: 'day' | 'hour']
  rangeChange: []
  granularityChange: []
  refresh: []
}>()
const { t } = useI18n()
function changeGranularity(value: string | number | boolean | null) {
  if (value !== 'day' && value !== 'hour') return
  emit('update:granularity', value)
  emit('granularityChange')
}
</script>

<style scoped>
.studio-filters{display:flex;align-items:center;gap:20px}.filter-field{display:flex;align-items:center;gap:5px}.filter-field>span{font-size:12px;color:var(--studio-muted);white-space:nowrap}.granularity :deep(.relative){width:106px}
.studio-filters :deep(.date-picker-trigger),.studio-filters :deep(.select-trigger){background:transparent;border:0;border-radius:8px;box-shadow:none;--tw-ring-shadow:0 0 #0000;font-size:13px;min-height:36px;padding:8px 10px;color:var(--studio-ink)}
.studio-filters :deep(.date-picker-trigger:hover),.studio-filters :deep(.select-trigger:hover){background:var(--tf-console-hover);color:var(--tf-console-active-ink);box-shadow:none;border-color:transparent}
.studio-filters :deep(.date-picker-trigger:focus-visible),.studio-filters :deep(.select-trigger:focus-visible),.refresh-button:focus-visible{outline:2px solid var(--tf-console-active-ink);outline-offset:3px;border-radius:5px}
.studio-filters :deep(.date-picker-trigger-open),.studio-filters :deep(.select-trigger-open){background:var(--tf-console-hover);color:var(--tf-console-active-ink)}
.studio-filters :deep(.date-picker-trigger:active),.studio-filters :deep(.select-trigger:active){background:var(--studio-haze);transform:translateY(1px)}
.studio-filters :deep(.date-picker-label){font-size:12px}
.refresh-button{display:grid;place-items:center;width:30px;height:32px;border:0;background:transparent;color:var(--studio-muted);border-radius:8px}.refresh-button:hover{color:var(--tf-console-active-ink);background:var(--tf-console-hover)}.refresh-button:disabled{opacity:.4}
/* The option marker scopes the teleported list to this filter only. */
.studio-filters :deep(.date-picker-dropdown),:global(.select-dropdown-portal:has(.studio-granularity-option)){--studio-filter-surface:#fff;--studio-filter-ink:#20273e;--studio-filter-muted:#69748d;--studio-filter-line:#bdc8df;--studio-filter-shadow:0 16px 40px -8px #17234b33,0 4px 12px #17234b14,inset 0 1px 0 #ffffffb3;background:var(--studio-filter-surface);color:var(--studio-filter-ink);border:1px solid var(--studio-filter-line);border-radius:16px;box-shadow:var(--studio-filter-shadow);padding:6px}
:global(.dark .studio-filters .date-picker-dropdown),:global(.dark .select-dropdown-portal:has(.studio-granularity-option)){--studio-filter-surface:#1b1e32;--studio-filter-ink:#e9edfa;--studio-filter-muted:#a0abc4;--studio-filter-line:#505773;--studio-filter-shadow:0 20px 48px -8px #0009,0 4px 14px #0006,inset 0 1px 0 #ffffff0d;color-scheme:dark}
.studio-filters :deep(.date-picker-presets){padding:2px;gap:4px}
.studio-filters :deep(.date-picker-preset),:global(.select-dropdown-portal:has(.studio-granularity-option) .select-option){font-size:13px;font-weight:400;color:var(--studio-filter-muted);border-radius:10px;padding:9px 12px}
.studio-filters :deep(.date-picker-preset:hover),:global(.select-dropdown-portal:has(.studio-granularity-option) .select-option:hover),:global(.select-dropdown-portal:has(.studio-granularity-option) .select-option-focused){color:var(--studio-filter-ink);background:var(--tf-console-hover)}
.studio-filters :deep(.date-picker-preset-active),:global(.select-dropdown-portal:has(.studio-granularity-option) .select-option-selected){color:var(--tf-console-active-ink);background:var(--tf-console-hover);font-weight:500}
.studio-filters :deep(.date-picker-divider){border-color:color-mix(in srgb,var(--studio-filter-line) 35%,transparent);margin:6px}
.studio-filters :deep(.date-picker-custom){padding:8px 6px 12px;gap:6px}
.studio-filters :deep(.date-picker-field){min-width:0}
.studio-filters :deep(.date-picker-label){color:var(--studio-filter-muted)}
.studio-filters :deep(.date-picker-input){min-width:0;border:1px solid color-mix(in srgb,var(--studio-filter-line) 50%,transparent);border-radius:9px;background:var(--tf-console-hover);color:var(--studio-filter-ink);font-size:12px;padding:8px 6px}
.studio-filters :deep(.date-picker-input:focus){border-color:#8297f5;box-shadow:0 0 0 2px #8297f526}
.studio-filters :deep(.date-picker-actions){padding:0 6px 6px}
.studio-filters :deep(.date-picker-apply){background:#4d7bfa;border-radius:9px;font-size:12px;padding:8px 16px}
.studio-filters :deep(.date-picker-apply:hover){background:#3f6de9}
.studio-filters :deep(.date-picker-preset:focus-visible),.studio-filters :deep(.date-picker-apply:focus-visible){outline:2px solid #8297f5;outline-offset:2px}
:global(.select-dropdown-portal:has(.studio-granularity-option) .select-options){padding:0}
.studio-granularity-option{flex:1}
@media(max-width:1100px){.studio-filters{gap:12px}.filter-field>span{display:none}}
@media(max-width:640px){.studio-filters{width:100%;justify-content:space-between;gap:8px}.granularity :deep(.relative){width:106px}.studio-filters :deep(.date-picker-dropdown){left:0;right:auto;width:min(320px,calc(100vw - 32px))}}
</style>
