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

<style scoped src="./studioFilters.css"></style>
