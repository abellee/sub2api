<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-200 px-4 py-3 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between sm:px-6">
          <div
            class="inline-flex w-full rounded-md bg-gray-100 p-1 dark:bg-dark-800 sm:w-auto"
            role="tablist"
            :aria-label="t('admin.ranking.metricLabel')"
          >
            <button
              v-for="option in metricOptions"
              :key="option.value"
              type="button"
              role="tab"
              class="min-w-0 flex-1 rounded px-4 py-2 text-sm font-medium transition-colors sm:flex-none"
              :class="metric === option.value
                ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-gray-100'
                : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'"
              :aria-selected="metric === option.value"
              @click="metric = option.value"
            >
              {{ option.label }}
            </button>
          </div>
          <div class="flex flex-wrap items-center gap-3">
            <span class="text-xs text-gray-400 dark:text-gray-500">
              {{ t('admin.ranking.rangeHint') }}
            </span>
            <DateRangePicker
              v-model:start-date="startDate"
              v-model:end-date="endDate"
              @preset-change="onDateRangeChange"
              @change="onDateRangeChange"
            />
          </div>
        </div>

        <UserTokenRanking
          :start-date="startDate"
          :end-date="endDate"
          :filters="{}"
          :metric="metric"
          :result-limit="0"
          :show-limit="false"
          metric-only
          @select-user="openUserUsage"
        />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import UserTokenRanking from '@/components/admin/usage/UserTokenRanking.vue'

type RankingMetric = 'tokens' | 'cost'

const { t } = useI18n()
const router = useRouter()

const formatLocalDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const getLast24HoursRangeDates = () => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return { start: formatLocalDate(start), end: formatLocalDate(end) }
}

const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)
const metric = ref<RankingMetric>('tokens')

const metricOptions = computed(() => [
  { value: 'tokens' as const, label: t('admin.ranking.metrics.tokens') },
  { value: 'cost' as const, label: t('admin.ranking.metrics.cost') },
])

const onDateRangeChange = (range: { startDate: string; endDate: string }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
}

const openUserUsage = (userId: number) => {
  router.push({
    path: '/admin/usage',
    query: {
      user_id: String(userId),
      start_date: startDate.value,
      end_date: endDate.value,
    },
  })
}
</script>
