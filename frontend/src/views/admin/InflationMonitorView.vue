<template>
  <AppLayout>
    <div class="space-y-6 pb-8">
      <header class="page-header"><h1 class="page-title">{{ t('inflationMonitor.title') }}</h1><p class="page-description">{{ t('inflationMonitor.description') }}</p></header>
      <p class="text-xs text-gray-500">{{ t('inflationMonitor.limit') }}</p>
      <div v-if="loading" role="status">{{ t('common.loading') }}</div>
      <div v-else-if="loadError" role="alert"><button class="btn btn-secondary" @click="load">{{ t('inflationMonitor.retry') }}</button></div>
      <template v-else>
        <div class="flex flex-wrap items-center gap-3">
          <select v-model="addGroupId" class="input w-auto" :aria-label="t('inflationMonitor.selectGroup')"><option :value="0">{{ t('inflationMonitor.selectGroup') }}</option><option v-for="group in availableGroups" :key="group.id" :value="group.id">{{ group.name }}</option></select>
          <button class="btn btn-secondary" :disabled="!addGroupId || saving" @click="addGroup">{{ t('inflationMonitor.addGroup') }}</button>
          <button class="btn btn-primary" :disabled="saving || accountLoading.size > 0" @click="save">{{ t('inflationMonitor.save') }}</button>
        </div>
        <section v-for="plan in plans" :key="plan.group_id" class="card space-y-4 p-5">
          <header class="flex items-center justify-between gap-4"><h2 class="font-semibold">{{ groups.find(g => g.id === plan.group_id)?.name || `#${plan.group_id}` }}</h2><button class="btn btn-secondary btn-sm" :disabled="saving" @click="plans = plans.filter(p => p !== plan)">{{ t('inflationMonitor.remove') }}</button></header>
          <fieldset :disabled="saving" class="space-y-4">
            <label class="flex items-center gap-2"><input v-model="plan.enabled" type="checkbox" />{{ t('inflationMonitor.enabled') }}</label>
            <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              <label class="block">{{ t('inflationMonitor.interval') }}<input v-model.number="plan.interval_minutes" class="input mt-1" type="number" min="1" max="1440" step="1" /></label>
              <label class="block">{{ t('inflationMonitor.model') }}<input v-model="plan.model" class="input mt-1" maxlength="200" required /></label>
              <label class="block">{{ t('inflationMonitor.effort') }}<select v-model="plan.reasoning_effort" class="input mt-1"><option value="">{{ t('inflationMonitor.defaultEffort') }}</option><option v-for="effort in ['none', 'minimal', 'low', 'medium', 'high', 'xhigh']" :key="effort" :value="effort">{{ effort }}</option></select></label>
              <label class="block">{{ t('inflationMonitor.encoding') }}<select v-model="plan.encoding" class="input mt-1"><option>o200k_base</option><option>cl100k_base</option></select></label>
              <label class="block">{{ t('inflationMonitor.aggregation') }}<select v-model="plan.aggregation" class="input mt-1"><option value="mean">{{ t('inflationMonitor.mean') }}</option><option value="median">{{ t('inflationMonitor.median') }}</option></select></label>
              <label class="block">{{ t('inflationMonitor.yellow') }}<input v-model.number="plan.yellow_threshold" class="input mt-1" type="number" min="0" step="any" /></label>
              <label class="block">{{ t('inflationMonitor.red') }}<input v-model.number="plan.red_threshold" class="input mt-1" type="number" min="0" step="any" /></label>
            </div>
            <p class="text-xs text-gray-500">{{ t('inflationMonitor.thresholdHint') }} {{ t('inflationMonitor.effortHint') }}</p>
            <label class="block">{{ t('inflationMonitor.prompt') }}<textarea v-model="plan.prompt" class="input mt-1" rows="3" required /></label>
            <fieldset><legend class="mb-2 font-medium">{{ t('inflationMonitor.accounts') }} ({{ plan.account_ids.length }}/20)</legend>
              <span v-if="accountLoading.has(plan.group_id)">{{ t('common.loading') }}</span>
              <button v-else-if="accountErrors.has(plan.group_id)" class="btn btn-secondary" @click="loadAccounts(plan.group_id)">{{ t('inflationMonitor.retry') }}</button>
              <div v-else class="grid max-h-64 gap-2 overflow-y-auto sm:grid-cols-2 lg:grid-cols-3"><label v-for="account in accounts[plan.group_id]" :key="account.id" class="flex items-center gap-2 text-sm"><input v-model="plan.account_ids" type="checkbox" :value="account.id" :disabled="!plan.account_ids.includes(account.id) && plan.account_ids.length >= 20" />{{ account.name }} <span class="text-xs text-gray-400">#{{ account.id }}</span></label></div>
            </fieldset>
          </fieldset>
        </section>
      </template>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { groupsAPI } from '@/api/admin/groups'
import { accountsAPI } from '@/api/admin/accounts'
import { inflationMonitorAPI, type InflationPlan } from '@/api/inflationMonitor'
import type { AdminGroup } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
const { t } = useI18n()
const app = useAppStore()
const loading = ref(true), loadError = ref(false), saving = ref(false)
const plans = ref<InflationPlan[]>([]), groups = ref<AdminGroup[]>([]), addGroupId = ref(0)
const accounts = ref<Record<number, { id: number; name: string }[]>>({})
const accountLoading = ref(new Set<number>()), accountErrors = ref(new Set<number>())
const availableGroups = computed(() => groups.value.filter(group => !plans.value.some(p => p.group_id === group.id)))
async function loadAccounts(groupId: number) {
  accountLoading.value.add(groupId); accountErrors.value.delete(groupId)
  try {
    let page = 1
    const items: { id: number; name: string }[] = []
    while (true) {
      const result = await accountsAPI.list(page, 100, { group: String(groupId), lite: 'true' })
      items.push(...result.items)
      if (page >= result.pages || !result.items.length) break
      page++
    }
    accounts.value[groupId] = items
  } catch { accountErrors.value.add(groupId); app.showError(t('inflationMonitor.loadFailed')) }
  finally { accountLoading.value.delete(groupId) }
}
async function load() {
  loading.value = true; loadError.value = false
  try {
    const [config, catalog] = await Promise.all([inflationMonitorAPI.plans(), groupsAPI.getAllIncludingInactive()])
    plans.value = config; groups.value = catalog
    await Promise.all(config.map(p => loadAccounts(p.group_id)))
  } catch { loadError.value = true; app.showError(t('inflationMonitor.loadFailed')) }
  finally { loading.value = false }
}
function addGroup() {
  if (!addGroupId.value) return
  plans.value.push({ group_id: addGroupId.value, enabled: false, interval_minutes: 60, account_ids: [], model: '', prompt: t('inflationMonitor.defaultPrompt'), encoding: 'o200k_base', reasoning_effort: '', aggregation: 'mean', yellow_threshold: 10, red_threshold: 30 })
  void loadAccounts(addGroupId.value); addGroupId.value = 0
}
async function save() {
  if (plans.value.some(p => !p.model.trim() || !p.prompt.trim() || !Number.isInteger(p.interval_minutes) || p.interval_minutes < 1 || p.interval_minutes > 1440 || !Number.isFinite(p.yellow_threshold) || !Number.isFinite(p.red_threshold) || p.yellow_threshold < 0 || p.red_threshold <= p.yellow_threshold || (p.enabled && !p.account_ids.length))) {
    app.showError(t('inflationMonitor.invalid')); return
  }
  saving.value = true
  try { await inflationMonitorAPI.save(plans.value); app.showSuccess(t('inflationMonitor.saved')) }
  catch (error) { app.showError(extractApiErrorMessage(error, t('inflationMonitor.saveFailed'))) }
  finally { saving.value = false }
}
onMounted(load)
</script>
