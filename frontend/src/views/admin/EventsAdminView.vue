<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useAppStore } from '@/stores'
import {
  blankEventRule, disableEventRule, fetchEventCatalog, fetchEventLogs, fetchEventRules,
  fetchEventSettings, saveEventRule, saveEventSettings,
  type EventDefinition, type EventLog, type EventRule, type EventSettings, type EventCondition
} from '@/api/events'
import { fetchManagedItems, type ManagedItem } from '@/api/pointsShop'

const { t, te } = useI18n()
const app = useAppStore()
const catalog = ref<EventDefinition[]>([])
const rules = ref<EventRule[]>([])
const items = ref<ManagedItem[]>([])
const settings = ref<EventSettings | null>(null)
const loading = ref(false), loadFailed = ref(false), saving = ref(false), settingsBusy = ref(false)
const query = ref(''), group = ref(''), selectedEvent = ref('')
const tab = ref<'rules' | 'executions' | 'events'>('rules')
const dialog = ref(false), detail = ref<EventLog | null>(null), disableTarget = ref<EventRule | null>(null)
const form = reactive<EventRule>(blankEventRule())
const probability = ref(100)
const startsAt = ref(''), endsAt = ref('')
const logStatus = ref(''), logPage = ref(1), logTotal = ref(0), logLoading = ref(false), logFailed = ref(false)
const logs = ref<EventLog[]>([])
let logRevision = 0

const groups = computed(() => [...new Set(catalog.value.map(event => event.group))])
const visibleEvents = computed(() => catalog.value.filter(event =>
  (!group.value || event.group === group.value) && (!selectedEvent.value || event.code === selectedEvent.value) &&
  (!query.value.trim() || `${event.name} ${event.code}`.toLowerCase().includes(query.value.trim().toLowerCase()))))
const activeEvents = computed(() => catalog.value.filter(event => event.status === 'active'))
const selectedDefinition = computed(() => catalog.value.find(event => event.code === form.event_type))
const availableItems = computed(() => form.action.kind === 'item_effect' ? items.value.filter(item => item.effect && item.effect.kind !== 'none') : items.value)
const maxLogPage = computed(() => Math.max(1, Math.ceil(logTotal.value / 20)))
const operators: EventCondition['operator'][] = ['eq', 'ne', 'in', 'contains', 'gt', 'gte', 'lt', 'lte']
const statuses = computed(() => tab.value === 'events' ? ['pending', 'processed', 'failed'] : ['applied', 'not_matched', 'missed', 'limited', 'skipped', 'failed'])
const safeInteger = (value: number, min = 0, max = 1_000_000_000_000) => Number.isSafeInteger(value) && value >= min && value <= max
const valid = computed(() => {
  if (!form.name.trim() || !selectedDefinition.value || selectedDefinition.value.status !== 'active') return false
  if (!Number.isFinite(probability.value) || probability.value < 0 || probability.value > 100) return false
  if (![form.user_limit, form.daily_limit, form.total_limit].every(value => safeInteger(value)) || !safeInteger(form.cooldown_seconds, 0, 315360000) || !safeInteger(form.every_n, 1)) return false
  if (form.action.kind === 'grant_points' && !safeInteger(form.action.points, 1)) return false
  if (form.action.kind !== 'grant_points' && (!availableItems.value.some(item => item.id === form.action.item_id) || !safeInteger(form.action.quantity, 1, 1_000_000))) return false
  if (form.conditions.some(condition => !selectedDefinition.value?.fields.includes(condition.field) || !condition.value.trim())) return false
  if (startsAt.value && endsAt.value && new Date(startsAt.value) >= new Date(endsAt.value)) return false
  return true
})

function message(error: unknown) { return error instanceof Error ? error.message : t('common.events.loadError') }
function fieldLabel(field: string) { return te(`common.events.fields.${field}`) ? t(`common.events.fields.${field}`) : field }
function eventLabel(code: string) { return catalog.value.find(event => event.code === code)?.name || code }
function statusLabel(value: string) { return te(`common.events.statuses.${value}`) ? t(`common.events.statuses.${value}`) : value }
function localDate(value: string) {
  if (!value) return ''
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return ''
  return new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 16)
}
function displayDate(value: string) { const date = new Date(value); return Number.isFinite(date.getTime()) ? date.toLocaleString() : value }
function rulesFor(code: string) { return rules.value.filter(rule => rule.event_type === code) }
function rewardLabel(rule: EventRule) {
  if (rule.action.kind === 'grant_points') return t('common.events.pointsSummary', { amount: rule.action.points })
  const name = items.value.find(item => item.id === rule.action.item_id)?.name || `#${rule.action.item_id}`
  return t(rule.action.kind === 'grant_item' ? 'common.events.itemSummary' : 'common.events.effectSummary', { name, quantity: rule.action.quantity })
}
async function load() {
  loading.value = true; loadFailed.value = false
  try {
    const result = await Promise.all([fetchEventCatalog(), fetchEventRules(), fetchManagedItems(), fetchEventSettings()])
    ;[catalog.value, rules.value, items.value, settings.value] = result
  } catch (error) { loadFailed.value = true; app.showError(message(error)) }
  finally { loading.value = false }
}
function edit(rule?: EventRule, eventType = '') {
  Object.assign(form, rule ? JSON.parse(JSON.stringify(rule)) : blankEventRule(eventType || activeEvents.value[0]?.code))
  form.conditions ??= []
  probability.value = form.probability / 10_000
  startsAt.value = localDate(form.starts_at); endsAt.value = localDate(form.ends_at)
  dialog.value = true
}
function changeEvent() { form.conditions = [] }
function changeAction() { form.action.item_id = 0; form.action.consume_item = false }
function addCondition() { if (form.conditions.length < 12) form.conditions.push({ field: selectedDefinition.value?.fields[0] || 'user_id', operator: 'eq', value: '' }) }
async function save() {
  if (!valid.value || saving.value) return
  saving.value = true
  try {
    const saved = await saveEventRule({ ...form, name: form.name.trim(), probability: Math.round(probability.value * 10_000),
      starts_at: startsAt.value ? new Date(startsAt.value).toISOString() : '', ends_at: endsAt.value ? new Date(endsAt.value).toISOString() : '' })
    const index = rules.value.findIndex(rule => rule.id === saved.id)
    if (index < 0) rules.value.push(saved); else rules.value[index] = saved
    dialog.value = false; app.showSuccess(t('common.saved'))
  } catch (error) { app.showError(message(error)) }
  finally { saving.value = false }
}
async function disable() {
  if (!disableTarget.value || saving.value) return
  saving.value = true
  try {
    await disableEventRule(disableTarget.value.id)
    const found = rules.value.find(rule => rule.id === disableTarget.value?.id)
    if (found) found.enabled = false
    disableTarget.value = null; app.showSuccess(t('common.saved'))
  } catch (error) { app.showError(message(error)) }
  finally { saving.value = false }
}
async function toggleRewards() {
  if (!settings.value || settingsBusy.value) return
  settingsBusy.value = true
  try { settings.value = await saveEventSettings(!settings.value.enabled) }
  catch (error) { app.showError(message(error)) }
  finally { settingsBusy.value = false }
}
async function loadLogs(reset = false) {
  if (reset) logPage.value = 1
  const revision = ++logRevision
  logLoading.value = true; logFailed.value = false
  try {
    const result = await fetchEventLogs(tab.value === 'executions', selectedEvent.value, logStatus.value, logPage.value)
    if (revision === logRevision) { logs.value = result.items; logTotal.value = result.total }
  } catch (error) { if (revision === logRevision) { logFailed.value = true; app.showError(message(error)) } }
  finally { if (revision === logRevision) logLoading.value = false }
}
function changeTab(value: 'rules' | 'executions' | 'events') {
  tab.value = value; logStatus.value = ''
  if (value !== 'rules') void loadLogs(true)
}
function changeFilter() { if (tab.value !== 'rules') void loadLogs(true) }
function turnPage(delta: number) { logPage.value += delta; void loadLogs() }
onMounted(load)
</script>

<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-7xl space-y-5">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div><h1 class="text-xl font-semibold">{{ t('common.events.title') }}</h1><p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">{{ t('common.events.description') }}</p></div>
        <button class="btn btn-primary" type="button" :disabled="loading || loadFailed || !activeEvents.length" @click="edit()">{{ t('common.events.addTrigger') }}</button>
      </div>
      <div v-if="loading" role="status" class="card p-6">{{ t('common.loading') }}</div>
      <div v-else-if="loadFailed" class="card space-y-3 p-6" role="alert"><p>{{ t('common.events.loadError') }}</p><button class="btn btn-secondary" @click="load">{{ t('common.tryAgain') }}</button></div>
      <template v-else>
        <section v-if="settings" class="card flex flex-wrap items-center justify-between gap-4 p-4">
          <div><p class="font-medium">{{ t('common.events.rewardSwitch') }} · {{ settings.enabled ? t('common.enabled') : t('common.disabled') }}</p><p class="mt-1 text-xs text-gray-500">{{ t('common.events.stats', { pending: settings.pending, processed: settings.processed, failed: settings.failed }) }}</p><p v-if="settings.settlement_disabled" class="mt-2 text-sm text-amber-600">{{ t('common.events.settlementDisabled') }}</p></div>
          <button class="btn btn-secondary" type="button" :disabled="settingsBusy" @click="toggleRewards">{{ settings.enabled ? t('common.events.pauseRewards') : t('common.events.enableRewards') }}</button>
        </section>
        <div class="flex flex-wrap items-center gap-2 border-b border-gray-200 pb-3 dark:border-dark-700" role="tablist" :aria-label="t('common.events.title')">
          <button v-for="entry in (['rules', 'executions', 'events'] as const)" :key="entry" class="btn btn-sm" :class="tab === entry ? 'btn-primary' : 'btn-secondary'" role="tab" :aria-selected="tab === entry" @click="changeTab(entry)">{{ t('common.events.tabs.' + entry) }}</button>
        </div>
        <div class="flex flex-wrap gap-3">
          <label class="min-w-52 flex-1"><span class="sr-only">{{ t('common.events.event') }}</span><select v-model="selectedEvent" class="input" @change="changeFilter"><option value="">{{ t('common.events.allEvents') }}</option><option v-for="event in catalog" :key="event.code" :value="event.code">{{ event.name }}</option></select></label>
          <template v-if="tab === 'rules'">
            <label class="min-w-40"><span class="sr-only">{{ t('common.events.group') }}</span><select v-model="group" class="input"><option value="">{{ t('common.events.allGroups') }}</option><option v-for="name in groups" :key="name">{{ name }}</option></select></label>
            <input v-model="query" class="input min-w-48 flex-1" :placeholder="t('common.events.search')" :aria-label="t('common.events.search')" />
          </template>
          <label v-else class="min-w-40"><span class="sr-only">{{ t('common.status') }}</span><select v-model="logStatus" class="input" @change="loadLogs(true)"><option value="">{{ t('common.all') }}</option><option v-for="status in statuses" :key="status" :value="status">{{ statusLabel(status) }}</option></select></label>
          <button v-if="tab !== 'rules'" class="btn btn-secondary" :disabled="logLoading" @click="loadLogs()">{{ t('common.events.refresh') }}</button>
        </div>
        <template v-if="tab === 'rules'">
          <p class="text-xs text-gray-500">{{ t('common.events.independentHint') }}</p>
          <p v-if="!visibleEvents.length" class="card p-6 text-sm text-gray-500">{{ t('common.noData') }}</p>
          <section v-for="event in visibleEvents" :key="event.code" class="card overflow-hidden">
            <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 p-4 dark:border-dark-700">
              <div><h2 class="font-semibold">{{ event.name }} <span v-if="event.status === 'reserved'" class="ml-2 text-xs font-normal text-gray-500">{{ t('common.events.reserved') }}</span></h2><p class="mt-1 text-xs text-gray-500">{{ event.group }} · {{ event.code }}</p><p v-if="event.description" class="mt-1 text-xs text-gray-500">{{ event.description }}</p></div>
              <button class="btn btn-secondary btn-sm" :disabled="event.status !== 'active'" @click="edit(undefined, event.code)">{{ t('common.events.addTrigger') }}</button>
            </div>
            <p v-if="!rulesFor(event.code).length" class="p-4 text-sm text-gray-400">{{ event.status === 'reserved' ? t('common.events.reservedHint') : t('common.events.noTriggers') }}</p>
            <article v-for="rule in rulesFor(event.code)" :key="rule.id" class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 p-4 last:border-0 dark:border-dark-700">
              <div class="min-w-0 flex-1"><div class="flex flex-wrap items-center gap-2"><h3 class="break-words font-medium">{{ rule.name }}</h3><span class="rounded-full px-2 py-0.5 text-xs" :class="rule.enabled ? 'bg-green-50 text-green-700 dark:bg-green-950 dark:text-green-300' : 'bg-gray-100 text-gray-500 dark:bg-dark-700'">{{ rule.enabled ? t('common.enabled') : t('common.disabled') }}</span></div><p class="mt-1 text-sm text-gray-500">{{ rewardLabel(rule) }} · {{ t('common.events.probabilitySummary', { probability: rule.probability / 10_000 }) }}</p><p class="mt-1 text-xs text-gray-500">{{ t('common.events.limitSummary', { user: rule.user_limit || '∞', daily: rule.daily_limit || '∞', total: rule.total_limit || '∞', count: rule.total_triggered }) }}</p></div>
              <div class="flex gap-2"><button class="btn btn-secondary btn-sm" @click="edit(rule)">{{ t('common.edit') }}</button><button v-if="rule.enabled" class="btn btn-secondary btn-sm" @click="disableTarget = rule">{{ t('common.events.disable') }}</button></div>
            </article>
          </section>
        </template>
        <section v-else class="card overflow-hidden">
          <p v-if="logLoading" class="p-6" role="status">{{ t('common.loading') }}</p>
          <p v-else-if="logFailed" class="p-6" role="alert">{{ t('common.events.loadError') }}</p>
          <p v-else-if="!logs.length" class="p-6 text-sm text-gray-500">{{ t('common.noData') }}</p>
          <div v-else class="overflow-x-auto"><table class="w-full text-left text-sm"><thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800"><tr><th class="p-3">{{ t('common.events.time') }}</th><th class="p-3">{{ t('common.events.event') }}</th><th class="p-3">{{ t('common.events.user') }}</th><th v-if="tab === 'executions'" class="p-3">{{ t('common.events.trigger') }}</th><th class="p-3">{{ t('common.status') }}</th><th class="p-3">{{ t('common.actions') }}</th></tr></thead><tbody><tr v-for="log in logs" :key="log.id" class="border-t border-gray-100 dark:border-dark-700"><td class="whitespace-nowrap p-3">{{ displayDate(log.created_at) }}</td><td class="p-3">{{ eventLabel(log.event_type) }}</td><td class="p-3">{{ log.user_id || '—' }}</td><td v-if="tab === 'executions'" class="p-3">{{ log.rule_name }}</td><td class="p-3">{{ statusLabel(log.status) }}</td><td class="p-3"><button class="btn btn-secondary btn-sm" @click="detail = log">{{ t('common.events.details') }}</button></td></tr></tbody></table></div>
          <div class="flex items-center justify-between gap-3 border-t border-gray-100 p-4 dark:border-dark-700"><p class="text-xs text-gray-500">{{ t('common.events.pagination', { total: logTotal, page: logPage, pages: maxLogPage }) }}</p><div class="flex gap-2"><button class="btn btn-secondary btn-sm" :disabled="logLoading || logPage <= 1" @click="turnPage(-1)">{{ t('common.events.previous') }}</button><button class="btn btn-secondary btn-sm" :disabled="logLoading || logPage >= maxLogPage" @click="turnPage(1)">{{ t('common.next') }}</button></div></div>
        </section>
      </template>
      <BaseDialog :show="dialog" :title="form.id ? t('common.events.editTrigger') : t('common.events.addTrigger')" width="medium-wide" :close-on-escape="!saving" @close="!saving && (dialog = false)">
        <form class="event-rule-form space-y-5" @submit.prevent="save">
          <div class="grid gap-4 sm:grid-cols-2"><label><span class="input-label">{{ t('common.events.triggerName') }}</span><input v-model="form.name" class="input" maxlength="100" required :disabled="saving" /></label><label><span class="input-label">{{ t('common.events.event') }}</span><select v-model="form.event_type" class="input" required :disabled="saving" @change="changeEvent"><option v-for="event in activeEvents" :key="event.code" :value="event.code">{{ event.name }}</option></select></label></div>
          <label class="flex items-center gap-2 text-sm"><input v-model="form.enabled" type="checkbox" :disabled="saving" />{{ t('common.events.enableTrigger') }}</label>
          <div class="grid gap-4 sm:grid-cols-2"><label><span class="input-label">{{ t('common.events.probability') }}</span><input v-model.number="probability" class="input" type="number" min="0" max="100" step="0.0001" required :disabled="saving" /></label><label><span class="input-label">{{ t('common.events.recipient') }}</span><select v-model="form.recipient" class="input" :disabled="saving"><option v-for="recipient in ['user', 'inviter', 'invitee', 'actor']" :key="recipient" :value="recipient">{{ t('common.events.recipients.' + recipient) }}</option></select></label></div>
          <p class="text-xs text-gray-500">{{ t('common.events.recipientHint') }}</p>
          <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700"><label><span class="input-label">{{ t('common.events.reward') }}</span><select v-model="form.action.kind" class="input" :disabled="saving" @change="changeAction"><option value="grant_points">{{ t('common.events.actions.grant_points') }}</option><option value="grant_item">{{ t('common.events.actions.grant_item') }}</option><option value="item_effect">{{ t('common.events.actions.item_effect') }}</option></select></label><label v-if="form.action.kind === 'grant_points'" class="mt-4 block"><span class="input-label">{{ t('common.events.points') }}</span><input v-model.number="form.action.points" class="input" type="number" min="1" max="1000000000000" step="1" required :disabled="saving" /></label><template v-else><div class="mt-4 grid gap-4 sm:grid-cols-2"><label><span class="input-label">{{ t('common.events.item') }}</span><select v-model.number="form.action.item_id" class="input" required :disabled="saving"><option :value="0" disabled>{{ t('common.pointsCatalog.selectItem') }}</option><option v-for="item in availableItems" :key="item.id" :value="item.id">{{ item.name }}</option></select></label><label><span class="input-label">{{ t('common.events.quantity') }}</span><input v-model.number="form.action.quantity" class="input" type="number" min="1" max="1000000" step="1" required :disabled="saving" /></label></div><template v-if="form.action.kind === 'item_effect'"><label class="mt-3 flex items-center gap-2 text-sm"><input v-model="form.action.consume_item" type="checkbox" :disabled="saving" />{{ t('common.events.consumeItem') }}</label><p class="mt-2 text-xs text-gray-500">{{ t('common.events.itemEffectHint') }}</p></template></template></div>
          <div><div class="mb-2 flex items-center justify-between gap-2"><h3 class="font-medium">{{ t('common.events.conditions') }}</h3><button class="btn btn-secondary btn-sm" type="button" :disabled="saving || form.conditions.length >= 12" @click="addCondition">{{ t('common.events.addCondition') }}</button></div><p class="mb-3 text-xs text-gray-500">{{ t('common.events.conditionsHint') }}</p><div v-for="(condition, index) in form.conditions" :key="index" class="mb-2 grid grid-cols-[1fr_1fr_auto] gap-2 sm:grid-cols-[1fr_1fr_1fr_auto]"><select v-model="condition.field" class="input" :aria-label="t('common.events.field')" :disabled="saving"><option v-for="field in selectedDefinition?.fields || []" :key="field" :value="field">{{ fieldLabel(field) }}</option></select><select v-model="condition.operator" class="input" :aria-label="t('common.events.operator')" :disabled="saving"><option v-for="operator in operators" :key="operator" :value="operator">{{ t('common.events.operators.' + operator) }}</option></select><input v-model="condition.value" class="input col-span-2 sm:col-span-1" maxlength="1024" required :aria-label="t('common.events.value')" :disabled="saving" /><button class="btn btn-secondary col-start-3 row-start-1 sm:col-start-auto sm:row-start-auto" type="button" :aria-label="t('common.events.removeCondition')" :disabled="saving" @click="form.conditions.splice(index, 1)">×</button></div></div>
          <div><h3 class="mb-2 font-medium">{{ t('common.events.limits') }}</h3><p class="mb-3 text-xs text-gray-500">{{ t('common.events.limitsHint') }}</p><div class="grid gap-4 sm:grid-cols-2"><label v-for="field in (['user_limit', 'daily_limit', 'total_limit', 'cooldown_seconds', 'every_n'] as const)" :key="field"><span class="input-label">{{ t('common.events.limitsLabels.' + field) }}</span><input v-model.number="form[field]" class="input" type="number" :min="field === 'every_n' ? 1 : 0" :max="field === 'cooldown_seconds' ? 315360000 : 1000000000000" step="1" required :disabled="saving" /></label></div></div>
          <div class="grid gap-4 sm:grid-cols-2"><label><span class="input-label">{{ t('common.events.startsAt') }}</span><input v-model="startsAt" class="input" type="datetime-local" :disabled="saving" /></label><label><span class="input-label">{{ t('common.events.endsAt') }}</span><input v-model="endsAt" class="input" type="datetime-local" :disabled="saving" /></label></div>
          <div class="flex justify-end gap-3"><button class="btn btn-secondary" type="button" :disabled="saving" @click="dialog = false">{{ t('common.cancel') }}</button><button class="btn btn-primary" type="submit" :disabled="!valid || saving">{{ saving ? t('common.saving') : t('common.save') }}</button></div>
        </form>
      </BaseDialog>
      <BaseDialog :show="!!disableTarget" :title="t('common.events.disable')" width="narrow" :close-on-escape="!saving" @close="!saving && (disableTarget = null)"><p>{{ t('common.events.disableConfirm', { name: disableTarget?.name }) }}</p><div class="mt-5 flex justify-end gap-3"><button class="btn btn-secondary" :disabled="saving" @click="disableTarget = null">{{ t('common.cancel') }}</button><button class="btn btn-primary" :disabled="saving" @click="disable">{{ t('common.confirm') }}</button></div></BaseDialog>
      <BaseDialog :show="!!detail" :title="t('common.events.details')" width="medium-wide" @close="detail = null"><dl v-if="detail" class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-5 gap-y-3 text-sm"><dt class="text-gray-500">{{ t('common.events.event') }}</dt><dd>{{ eventLabel(detail.event_type) }}</dd><dt class="text-gray-500">{{ t('common.events.user') }}</dt><dd>{{ detail.user_id || '—' }}</dd><dt class="text-gray-500">{{ t('common.status') }}</dt><dd>{{ statusLabel(detail.status) }}</dd><dt v-if="detail.rule_name" class="text-gray-500">{{ t('common.events.trigger') }}</dt><dd v-if="detail.rule_name">{{ detail.rule_name }}</dd><dt v-if="detail.reason || detail.last_error" class="text-gray-500">{{ t('common.events.reason') }}</dt><dd v-if="detail.reason || detail.last_error" class="break-words">{{ detail.reason || detail.last_error }}</dd><dt v-if="detail.amount" class="text-gray-500">{{ t('common.events.reward') }}</dt><dd v-if="detail.amount">{{ detail.amount }}<span v-if="detail.item_id"> · #{{ detail.item_id }}</span></dd><template v-for="(value, field) in detail.payload?.data || {}" :key="field"><dt class="break-words text-gray-500">{{ fieldLabel(String(field)) }}</dt><dd class="whitespace-pre-wrap break-all">{{ value }}</dd></template></dl></BaseDialog>
    </div>
  </AppLayout>
</template>

<style scoped>
:global(.modal-overlay:has(.event-rule-form)) {
  padding-block: 24px;
}
:global(.modal-overlay:has(.event-rule-form) .modal-content) {
  max-height: calc(100dvh - 48px);
}
</style>
