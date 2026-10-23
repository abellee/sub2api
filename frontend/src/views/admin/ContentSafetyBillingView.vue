<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6 pb-8">
      <header>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.contentSafetyBilling.title') }}</h1>
        <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.contentSafetyBilling.description') }}</p>
      </header>

      <p v-if="configError" role="alert" class="text-sm text-red-600 dark:text-red-400">
        {{ configError }}
        <button class="btn btn-secondary ml-3" :disabled="loadingConfig" @click="loadConfig">{{ t('admin.contentSafetyBilling.retry') }}</button>
      </p>
      <p v-if="loadingConfig" role="status">{{ t('common.loading') }}</p>
      <form v-else-if="draft" class="card space-y-5 p-6" @submit.prevent="save">
        <label class="flex items-center gap-3 text-sm font-medium text-gray-900 dark:text-white">
          <input v-model="draft.enabled" type="checkbox" :disabled="saving" data-test="enabled" />
          {{ t('admin.contentSafetyBilling.enabled') }}
        </label>
        <fieldset :disabled="saving" class="space-y-5">
          <div class="max-w-sm">
            <label for="safety-amount" class="input-label">{{ t('admin.contentSafetyBilling.amount') }}</label>
            <input id="safety-amount" v-model.number="draft.amount" class="input" type="number" :min="draft.enabled ? 0.00000001 : 0" max="1000000" step="0.00000001" required data-test="amount" />
            <p class="input-hint">{{ t('admin.contentSafetyBilling.amountHint') }}</p>
          </div>
          <div>
            <label for="safety-match" class="input-label">{{ t('admin.contentSafetyBilling.match') }}</label>
            <input id="safety-match" v-model="draft.match_description" class="input" type="text" required maxlength="4096" data-test="match" />
            <p class="input-hint">{{ t('admin.contentSafetyBilling.matchHint') }}</p>
          </div>
          <div>
            <label for="safety-message" class="input-label">{{ t('admin.contentSafetyBilling.clientMessage') }}</label>
            <input id="safety-message" v-model="draft.client_message" class="input" type="text" required maxlength="4096" data-test="client-message" />
            <p class="input-hint">{{ t('admin.contentSafetyBilling.clientMessageHint') }}</p>
          </div>
        </fieldset>
        <p v-if="saveError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ saveError }}</p>
        <p v-if="saved" role="status" class="text-sm text-green-600 dark:text-green-400">{{ t('common.saved') }}</p>
        <button type="submit" class="btn btn-primary" :disabled="saving" data-test="save">{{ saving ? t('common.submitting') : t('common.save') }}</button>
      </form>

      <section class="card p-6" :aria-label="t('admin.contentSafetyBilling.records')" :aria-busy="loadingRecords">
        <div class="mb-4 flex items-center justify-between gap-4">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.contentSafetyBilling.records') }}</h2>
          <button class="btn btn-secondary" :disabled="loadingRecords" @click="loadRecords(records.page)">{{ t('admin.contentSafetyBilling.refresh') }}</button>
        </div>
        <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.contentSafetyBilling.recordsHint') }}</p>
        <p v-if="recordsError" role="alert" class="mb-4 text-sm text-red-600 dark:text-red-400">{{ recordsError }}</p>
        <div class="overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead class="border-b border-gray-200 text-gray-500 dark:border-dark-500 dark:text-gray-400">
              <tr>
                <th class="p-3">{{ t('admin.contentSafetyBilling.time') }}</th>
                <th class="p-3">{{ t('admin.contentSafetyBilling.identity') }}</th>
                <th class="p-3">{{ t('admin.contentSafetyBilling.debit') }}</th>
                <th class="p-3">{{ t('admin.contentSafetyBilling.note') }}</th>
                <th class="p-3">{{ t('admin.contentSafetyBilling.status') }}</th>
              </tr>
            </thead>
            <tbody class="text-gray-800 dark:text-gray-200">
              <tr v-for="row in records.items" :key="row.id" class="border-b border-gray-100 align-top dark:border-dark-600">
                <td class="whitespace-nowrap p-3">
                  {{ formatDate(row.created_at) }}
                  <span class="mt-1 block max-w-48 break-all whitespace-normal text-xs text-gray-500">{{ row.id }}</span>
                </td>
                <td class="p-3">
                  <span class="whitespace-nowrap">User #{{ row.user_id }} · Key #{{ row.api_key_id }}</span>
                  <span class="mt-1 block text-xs text-gray-500">Account #{{ row.account_id }}</span>
                  <span v-if="row.upstream_request_id" class="mt-1 block max-w-48 break-all text-xs text-gray-500">{{ row.upstream_request_id }}</span>
                </td>
                <td class="whitespace-nowrap p-3 font-mono">${{ row.amount.toFixed(8) }}</td>
                <td class="min-w-60 max-w-md break-words p-3">{{ row.notes }}</td>
                <td class="min-w-32 p-3">
                  <span :class="row.status === 'charged' ? 'text-green-600 dark:text-green-400' : 'text-amber-600 dark:text-amber-400'">{{ t(`admin.contentSafetyBilling.${row.status}`) }}</span>
                  <span v-if="row.charged_at" class="mt-1 block text-xs text-gray-500">{{ formatDate(row.charged_at) }}</span>
                  <span v-if="row.last_error" class="mt-1 block text-xs text-gray-500">{{ row.last_error }}</span>
                </td>
              </tr>
              <tr v-if="!records.items.length"><td colspan="5" class="p-8 text-center text-gray-500">{{ loadingRecords ? t('common.loading') : t('admin.contentSafetyBilling.empty') }}</td></tr>
            </tbody>
          </table>
        </div>
        <div class="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm text-gray-500">
          <span>{{ t('admin.contentSafetyBilling.pagination', { page: records.page, total: records.total }) }}</span>
          <div class="flex gap-2">
            <button class="btn btn-secondary" :disabled="loadingRecords || records.page <= 1" @click="loadRecords(records.page - 1)">{{ t('admin.contentSafetyBilling.previous') }}</button>
            <button class="btn btn-secondary" :disabled="loadingRecords || records.page * records.page_size >= records.total" @click="loadRecords(records.page + 1)">{{ t('admin.contentSafetyBilling.next') }}</button>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import {
  getContentSafetyBilling, updateContentSafetyBilling, getContentSafetyDebits,
  type ContentSafetyBillingConfig, type ContentSafetyDebitPage,
} from '@/api/admin/contentSafetyBilling'

const { t } = useI18n()
const draft = ref<ContentSafetyBillingConfig | null>(null)
const records = ref<ContentSafetyDebitPage>({ items: [], total: 0, page: 1, page_size: 20 })
const loadingConfig = ref(false)
const loadingRecords = ref(false)
const saving = ref(false)
const saved = ref(false)
const configError = ref('')
const saveError = ref('')
const recordsError = ref('')
const formatDate = (value: string) => new Date(value).toLocaleString()

async function loadConfig() {
  loadingConfig.value = true
  configError.value = ''
  try { draft.value = await getContentSafetyBilling() }
  catch { configError.value = t('admin.contentSafetyBilling.loadError') }
  finally { loadingConfig.value = false }
}

async function save() {
  if (!draft.value || saving.value) return
  saving.value = true
  saved.value = false
  saveError.value = ''
  try {
    draft.value = await updateContentSafetyBilling({ ...draft.value })
    saved.value = true
  } catch { saveError.value = t('admin.contentSafetyBilling.saveError') }
  finally { saving.value = false }
}

async function loadRecords(page: number) {
  if (loadingRecords.value) return
  loadingRecords.value = true
  recordsError.value = ''
  try { records.value = await getContentSafetyDebits(page) }
  catch { recordsError.value = t('admin.contentSafetyBilling.recordsError') }
  finally { loadingRecords.value = false }
}

onMounted(() => { void loadConfig(); void loadRecords(1) })
</script>
