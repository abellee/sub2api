<template>
  <AppLayout>
    <div class="space-y-6 pb-8">
      <header class="page-header"><h1 class="page-title">{{ t('inflationMonitor.title') }}</h1><p class="page-description">{{ t('inflationMonitor.description') }}</p></header>
      <div v-if="loading" role="status">{{ t('common.loading') }}</div>
      <div v-else-if="loadError" role="alert"><button class="btn btn-secondary" @click="load">{{ t('inflationMonitor.retry') }}</button></div>
      <template v-else>
        <div class="flex flex-wrap items-center gap-3">
          <select v-model="addGroupId" class="input w-auto" :aria-label="t('inflationMonitor.selectGroup')"><option :value="0">{{ t('inflationMonitor.selectGroup') }}</option><option v-for="group in availableGroups" :key="group.id" :value="group.id">{{ group.name }}</option></select>
          <button class="btn btn-primary" :disabled="!addGroupId || saving" @click="addGroup">{{ t('inflationMonitor.addGroup') }}</button>
          <button class="btn btn-secondary" :disabled="statusLoading" @click="refreshStatus()">{{ t('common.refresh') }}</button>
        </div>
        <p v-if="statusError" role="alert" class="text-sm text-amber-600">{{ t('inflationMonitor.statusFailed') }}</p>
        <div class="grid gap-5 sm:grid-cols-2 xl:grid-cols-3">
          <article v-for="plan in plans" :key="plan.group_id" class="card flex min-w-0 flex-col gap-4 p-5" :data-group-id="plan.group_id">
            <header class="flex items-start justify-between gap-3">
              <h2 class="min-w-0 break-words font-semibold">{{ groupName(plan.group_id) }}</h2>
              <span class="badge shrink-0" :class="isRunning(plan.group_id) ? 'badge-primary' : plan.enabled ? 'badge-success' : 'badge-gray'">{{ isRunning(plan.group_id) ? t('inflationMonitor.running') : plan.enabled ? t('inflationMonitor.scheduled') : t('inflationMonitor.paused') }}</span>
            </header>
            <div class="flex items-end justify-between gap-3">
              <div><p class="text-xs text-gray-500">{{ t('inflationMonitor.latest') }} · {{ t(`inflationMonitor.${plan.aggregation}`) }}</p><p class="mt-1 text-2xl font-semibold tabular-nums" :class="rateClass(plan)">{{ formatRate(statusByGroup.get(plan.group_id)?.rate) }}</p></div>
              <span class="text-right text-xs text-gray-500">{{ formatTime(statusByGroup.get(plan.group_id)?.measured_at) }}</span>
            </div>
            <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-sm">
              <dt class="text-gray-500">{{ t('inflationMonitor.interval') }}</dt><dd class="text-right tabular-nums">{{ plan.interval_minutes }}</dd>
              <dt class="text-gray-500">{{ t('inflationMonitor.accounts') }}</dt><dd class="text-right tabular-nums">{{ plan.account_ids.length }}</dd>
              <dt class="text-gray-500">{{ t('inflationMonitor.model') }}</dt><dd class="break-words text-right">{{ plan.model }}</dd>
              <dt class="text-gray-500">{{ t('inflationMonitor.effort') }}</dt><dd class="text-right">{{ plan.reasoning_effort || t('inflationMonitor.defaultEffort') }}</dd>
            </dl>
            <p class="text-xs"><span class="text-amber-600 dark:text-amber-400">{{ t('inflationMonitor.yellow') }} {{ plan.yellow_threshold }}</span><span class="ml-3 text-red-600 dark:text-red-400">{{ t('inflationMonitor.red') }} {{ plan.red_threshold }}</span></p>
            <footer class="mt-auto flex flex-wrap items-center gap-2 border-t border-gray-100 pt-4 dark:border-dark-700">
              <button class="btn btn-secondary btn-sm" :disabled="saving || isRunning(plan.group_id)" data-test="edit-plan" @click="editPlan(plan)">{{ t('common.edit') }}</button>
              <button class="btn btn-primary btn-sm" :disabled="isRunning(plan.group_id) || saving || !plan.account_ids.length" data-test="run-now" @click="runNow(plan)">{{ isRunning(plan.group_id) ? t('inflationMonitor.running') : t('inflationMonitor.runNow') }}</button>
              <button class="btn btn-secondary btn-sm ml-auto" :disabled="saving || isRunning(plan.group_id)" data-test="remove-plan" @click="removePlan(plan)">{{ t('inflationMonitor.remove') }}</button>
            </footer>
          </article>
        </div>
        <p v-if="!plans.length" class="card p-8 text-center text-sm text-gray-500">{{ t('inflationMonitor.emptyPlans') }}</p>
      </template>
    </div>
    <BaseDialog :show="draft !== null" :title="draft ? groupName(draft.group_id) : t('inflationMonitor.title')" width="wide" :close-on-escape="!saving" :show-close-button="!saving" @close="closeEditor">
      <form v-if="draft" id="inflation-plan-form" class="space-y-4" @submit.prevent="save">
        <p class="text-xs text-gray-500">{{ t('inflationMonitor.limit') }}</p>
          <fieldset :disabled="saving" class="space-y-4">
            <label class="flex items-center gap-2"><input v-model="draft.enabled" type="checkbox" />{{ t('inflationMonitor.enabled') }}</label>
            <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              <label class="block">{{ t('inflationMonitor.interval') }}<input v-model.number="draft.interval_minutes" class="input mt-1" type="number" min="1" max="1440" step="1" /></label>
              <label class="block">{{ t('inflationMonitor.model') }}<input v-model="draft.model" class="input mt-1" maxlength="200" required /></label>
              <label class="block">{{ t('inflationMonitor.effort') }}<select v-model="draft.reasoning_effort" class="input mt-1"><option value="">{{ t('inflationMonitor.defaultEffort') }}</option><option v-for="effort in ['none', 'minimal', 'low', 'medium', 'high', 'xhigh']" :key="effort" :value="effort">{{ effort }}</option></select></label>
              <label class="block">{{ t('inflationMonitor.encoding') }}<select v-model="draft.encoding" class="input mt-1"><option>o200k_base</option><option>cl100k_base</option></select></label>
              <label class="block">{{ t('inflationMonitor.aggregation') }}<select v-model="draft.aggregation" class="input mt-1"><option value="mean">{{ t('inflationMonitor.mean') }}</option><option value="median">{{ t('inflationMonitor.median') }}</option></select></label>
              <label class="block">{{ t('inflationMonitor.yellow') }}<input v-model.number="draft.yellow_threshold" class="input mt-1" type="number" min="0" step="any" /></label>
              <label class="block">{{ t('inflationMonitor.red') }}<input v-model.number="draft.red_threshold" class="input mt-1" type="number" min="0" step="any" /></label>
            </div>
            <p class="text-xs text-gray-500">{{ t('inflationMonitor.thresholdHint') }} {{ t('inflationMonitor.effortHint') }}</p>
            <label class="block">{{ t('inflationMonitor.prompt') }}<textarea v-model="draft.prompt" class="input mt-1" rows="3" required /></label>
            <fieldset><legend class="mb-2 font-medium">{{ t('inflationMonitor.accounts') }} ({{ draft.account_ids.length }}/20)</legend>
              <span v-if="accountLoading.has(draft.group_id)">{{ t('common.loading') }}</span>
              <button type="button" v-else-if="accountErrors.has(draft.group_id)" class="btn btn-secondary" @click="loadAccounts(draft.group_id)">{{ t('inflationMonitor.retry') }}</button>
              <div v-else class="grid max-h-64 gap-2 overflow-y-auto sm:grid-cols-2 lg:grid-cols-3"><label v-for="account in accounts[draft.group_id]" :key="account.id" class="flex items-center gap-2 text-sm"><input v-model="draft.account_ids" type="checkbox" :value="account.id" :disabled="!draft.account_ids.includes(account.id) && draft.account_ids.length >= 20" />{{ account.name }} <span class="text-xs text-gray-400">#{{ account.id }}</span></label></div>
            </fieldset>
          </fieldset>
      </form>
      <template #footer>
        <button class="btn btn-secondary" :disabled="saving" @click="closeEditor">{{ t('common.cancel') }}</button>
        <button type="submit" form="inflation-plan-form" class="btn btn-primary" :disabled="saving || (draft !== null && (accountLoading.has(draft.group_id) || accountErrors.has(draft.group_id)))">{{ t('inflationMonitor.save') }}</button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { groupsAPI } from '@/api/admin/groups'
import { accountsAPI } from '@/api/admin/accounts'
import { inflationMonitorAPI, inflationTone, type InflationPlan, type InflationStatus } from '@/api/inflationMonitor'
import type { AdminGroup } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
const { t, locale } = useI18n()
const app = useAppStore()
const loading = ref(true), loadError = ref(false), saving = ref(false), statusLoading = ref(false), statusError = ref(false)
const plans = ref<InflationPlan[]>([]), groups = ref<AdminGroup[]>([]), addGroupId = ref(0)
const draft = ref<InflationPlan | null>(null)
const statuses = ref<InflationStatus[]>([])
const pendingRuns = ref(new Set<number>())
const statusByGroup = computed(() => new Map(statuses.value.map(s => [s.group_id, s])))
const accounts = ref<Record<number, { id: number; name: string }[]>>({})
const accountLoading = ref(new Set<number>()), accountErrors = ref(new Set<number>())
const availableGroups = computed(() => groups.value.filter(group => !plans.value.some(p => p.group_id === group.id)))
const groupName = (id: number) => groups.value.find(g => g.id === id)?.name || `#${id}`
const isRunning = (id: number) => pendingRuns.value.has(id) || statusByGroup.value.get(id)?.running === true
const formatRate = (rate?: number | null) => rate == null ? t('inflationMonitor.notMeasured') : `${rate.toFixed(1)}%`
const formatTime = (value?: string | null) => value ? new Date(value).toLocaleString(locale.value) : '—'
function rateClass(plan: InflationPlan) {
  const tone = inflationTone(statusByGroup.value.get(plan.group_id)?.rate ?? null, plan.yellow_threshold, plan.red_threshold)
  return { healthy: 'text-green-600 dark:text-green-400', warning: 'text-amber-600 dark:text-amber-400', critical: 'text-red-600 dark:text-red-400', unknown: 'text-gray-400' }[tone]
}
let pollTimer: ReturnType<typeof setTimeout> | undefined
let disposed = false
const statusController = new AbortController()
async function refreshStatus() {
  if (disposed || statusLoading.value) return
  clearTimeout(pollTimer)
  statusLoading.value = true
  try { statuses.value = await inflationMonitorAPI.statuses(statusController.signal); statusError.value = false }
  catch { if (!disposed) statusError.value = true }
  finally {
    statusLoading.value = false
    if (!disposed) pollTimer = setTimeout(refreshStatus, statuses.value.some(s => s.running) || pendingRuns.value.size ? 3000 : 30000)
  }
}
async function loadAccounts(groupId: number) {
  accountLoading.value.add(groupId); accountErrors.value.delete(groupId)
  try {
    let page = 1
    const items: { id: number; name: string }[] = []
    while (true) {
      const result = await accountsAPI.list(page, 100, { group: String(groupId), lite: 'true' })
      if (disposed) return
      items.push(...result.items)
      if (page >= result.pages || !result.items.length) break
      page++
    }
    accounts.value[groupId] = items
  } catch { if (!disposed) { accountErrors.value.add(groupId); app.showError(t('inflationMonitor.loadFailed')) } }
  finally { accountLoading.value.delete(groupId) }
}
async function load() {
  loading.value = true; loadError.value = false
  try {
    const [config, catalog] = await Promise.all([inflationMonitorAPI.plans(), groupsAPI.getAllIncludingInactive()])
    if (disposed) return
    plans.value = config; groups.value = catalog
    await refreshStatus()
  } catch { if (!disposed) { loadError.value = true; app.showError(t('inflationMonitor.loadFailed')) } }
  finally { loading.value = false }
}
function addGroup() {
  if (!addGroupId.value) return
  draft.value = { group_id: addGroupId.value, enabled: false, interval_minutes: 60, account_ids: [], model: '', prompt: t('inflationMonitor.defaultPrompt'), encoding: 'o200k_base', reasoning_effort: '', aggregation: 'mean', yellow_threshold: 10, red_threshold: 30 }
  void loadAccounts(addGroupId.value); addGroupId.value = 0
}
function editPlan(plan: InflationPlan) {
  draft.value = { ...plan, account_ids: [...plan.account_ids] }
  void loadAccounts(plan.group_id)
}
function closeEditor() { if (!saving.value) draft.value = null }
async function save() {
  const plan = draft.value
  if (!plan || saving.value) return
  if (!plan.model.trim() || !plan.prompt.trim() || !Number.isInteger(plan.interval_minutes) || plan.interval_minutes < 1 || plan.interval_minutes > 1440 || !Number.isFinite(plan.yellow_threshold) || !Number.isFinite(plan.red_threshold) || plan.yellow_threshold < 0 || plan.red_threshold <= plan.yellow_threshold || (plan.enabled && !plan.account_ids.length)) {
    app.showError(t('inflationMonitor.invalid')); return
  }
  const next = plans.value.some(p => p.group_id === plan.group_id) ? plans.value.map(p => p.group_id === plan.group_id ? plan : p) : [...plans.value, plan]
  saving.value = true
  try { await inflationMonitorAPI.save(next); plans.value = next; draft.value = null; app.showSuccess(t('inflationMonitor.saved')); await refreshStatus() }
  catch (error) { app.showError(extractApiErrorMessage(error, t('inflationMonitor.saveFailed'))) }
  finally { saving.value = false }
}
async function removePlan(plan: InflationPlan) {
  if (saving.value) return
  saving.value = true
  const next = plans.value.filter(p => p.group_id !== plan.group_id)
  try { await inflationMonitorAPI.save(next); plans.value = next; await refreshStatus() }
  catch (error) { app.showError(extractApiErrorMessage(error, t('inflationMonitor.saveFailed'))) }
  finally { saving.value = false }
}
async function runNow(plan: InflationPlan) {
  if (isRunning(plan.group_id)) return
  pendingRuns.value.add(plan.group_id)
  try { await inflationMonitorAPI.runNow(plan.group_id); app.showSuccess(t('inflationMonitor.runStarted')) }
  catch (error) { app.showError(extractApiErrorMessage(error, t('inflationMonitor.runFailed'))) }
  finally { pendingRuns.value.delete(plan.group_id); await refreshStatus() }
}
onMounted(load)
onBeforeUnmount(() => { disposed = true; clearTimeout(pollTimer); statusController.abort() })
</script>
