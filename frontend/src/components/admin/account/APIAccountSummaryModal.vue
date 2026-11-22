<template>
  <BaseDialog :show="show" :title="t('admin.accounts.apiSummary.title')" width="extra-wide" @close="emit('close')">
    <div class="min-h-[32rem] space-y-5">
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.accounts.apiSummary.matchHint') }}</p>
      <form class="flex flex-wrap items-end gap-3" @submit.prevent="loadSummary">
        <label class="min-w-64 flex-1 space-y-1 text-sm">
          <span>{{ t('admin.accounts.apiSummary.address') }}</span>
          <input v-model="baseURL" type="url" required maxlength="2048" class="input w-full" placeholder="https://api.example.com/v1" />
        </label>
        <div class="space-y-1 text-sm">
          <div>{{ t('admin.accounts.apiSummary.dateRange') }}</div>
          <DateRangePicker v-model:start-date="startDate" v-model:end-date="endDate" />
        </div>
        <label class="w-36 space-y-1 text-sm">
          <span>{{ t('admin.accounts.apiSummary.granularity') }}</span>
          <Select v-model="granularity" :options="granularityOptions" :searchable="false" />
        </label>
        <button type="submit" class="btn btn-primary" :disabled="loading">
          {{ loading ? t('common.loading') : t('admin.accounts.apiSummary.query') }}
        </button>
      </form>
      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.apiSummary.costHint') }}</p>
      <p v-if="error" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
      <div v-if="loading" class="flex justify-center py-8"><LoadingSpinner /></div>
      <template v-else-if="result">
        <p class="break-all text-sm text-gray-500 dark:text-gray-400">
          {{ result.base_url }} · {{ result.start_date }} – {{ result.end_date }} · {{ result.timezone }}
        </p>
        <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <div v-for="card in cards" :key="card.label" class="card p-4">
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ card.label }}</div>
            <div class="mt-2 break-all text-xl font-semibold">{{ card.value }}</div>
          </div>
        </div>
        <p v-if="!result.accounts.length" class="py-4 text-center text-sm text-gray-500">
          {{ t('admin.accounts.apiSummary.noAccounts') }}
        </p>
        <template v-else>
          <h4 class="font-semibold">{{ t('admin.accounts.apiSummary.byTime') }}</h4>
          <DataTable :columns="periodColumns" :data="result.periods" row-key="period" />
          <h4 class="font-semibold">{{ t('admin.accounts.apiSummary.byAccount') }}</h4>
          <DataTable :columns="accountColumns" :data="result.accounts" row-key="account_id" />
        </template>
      </template>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { APIAccountSummary, APIAccountSummaryParams } from '@/api/admin/accounts'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Select from '@/components/common/Select.vue'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (event: 'close'): void }>()
const { t, locale } = useI18n()
const baseURL = ref('')
const dateString = (date: Date) => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
const today = new Date()
const endDate = ref(dateString(today))
const startDate = ref(dateString(new Date(today.getFullYear(), today.getMonth(), today.getDate() - 29)))
const granularity = ref<APIAccountSummaryParams['granularity']>('day')
const loading = ref(false)
const error = ref('')
const result = ref<APIAccountSummary | null>(null)
let controller: AbortController | null = null

const money = (amount: number) => new Intl.NumberFormat(locale.value, {
  style: 'currency', currency: 'USD', minimumFractionDigits: 2, maximumFractionDigits: 6
}).format(amount)
const count = (value: number) => new Intl.NumberFormat(locale.value).format(value)
const granularityOptions = computed(() => ['hour', 'day', 'week', 'month'].map(value => ({
  value, label: t(`admin.accounts.apiSummary.${value}`)
})))
const metricColumns = computed(() => [
  ...['user_cost', 'account_cost', 'margin', 'standard_cost'].map(key => ({
    key, label: t(`admin.accounts.apiSummary.${key}`), formatter: money
  })),
  ...['requests', 'tokens'].map(key => ({
    key, label: t(`admin.accounts.apiSummary.${key}`), formatter: count
  }))
])
const periodColumns = computed(() => [{ key: 'period', label: t('admin.accounts.apiSummary.period') }, ...metricColumns.value])
const accountColumns = computed(() => [
  { key: 'account_id', label: 'ID' }, { key: 'name', label: t('admin.accounts.apiSummary.account') }, ...metricColumns.value
])
const cards = computed(() => result.value ? [
  { label: t('admin.accounts.apiSummary.accountCount'), value: count(result.value.accounts.length) },
  ...metricColumns.value.map(column => ({
    label: column.label,
    value: column.formatter(result.value!.summary[column.key as keyof APIAccountSummary['summary']])
  }))
] : [])

function clearResult() {
  controller?.abort()
  controller = null
  loading.value = false
  result.value = null
  error.value = ''
}

watch([baseURL, startDate, endDate, granularity], clearResult)
watch(() => props.show, show => { if (!show) clearResult() })
onUnmounted(clearResult)

async function loadSummary() {
  clearResult()
  const request = new AbortController()
  controller = request
  loading.value = true
  try {
    const data = await adminAPI.accounts.getAPIAccountSummary({
      base_url: baseURL.value.trim(), start_date: startDate.value, end_date: endDate.value,
      granularity: granularity.value, timezone: Intl.DateTimeFormat().resolvedOptions().timeZone
    }, request.signal)
    if (!request.signal.aborted) result.value = data
  } catch (err) {
    if (!request.signal.aborted) {
      const message = extractApiErrorMessage(err, t('admin.accounts.apiSummary.loadFailed'))
      const status = (err as { status?: number } | null)?.status
      error.value = status === 404 || (status === 400 && message === 'Invalid account ID')
        ? t('admin.accounts.apiSummary.backendUnavailable')
        : message
    }
  } finally {
    if (controller === request) {
      controller = null
      loading.value = false
    }
  }
}
</script>
