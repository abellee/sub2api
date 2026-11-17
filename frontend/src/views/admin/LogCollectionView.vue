<template>
  <AppLayout>
    <div class="space-y-6">
      <header>
        <h1 class="text-2xl font-semibold">{{ t('admin.logCollection.title') }}</h1>
        <h2 class="mt-4 text-lg font-medium">{{ t('admin.logCollection.timing') }}</h2>
        <p class="mt-2 text-sm text-gray-500">{{ t('admin.logCollection.description') }}</p>
      </header>
      <p v-if="error" role="alert" class="rounded-lg bg-red-50 p-3 text-red-700 dark:bg-red-950 dark:text-red-300">{{ error }}</p>
      <form class="card space-y-4 p-5" @submit.prevent="confirmStart">
        <div class="grid gap-4 sm:grid-cols-3">
          <label v-for="key in scopeKeys" :key="key" class="space-y-1">
            <span class="block text-sm">{{ t(`admin.logCollection.${key}`) }}</span>
            <input v-model.number="form[key]" type="number" min="1" max="9007199254740991" step="1" required class="input w-full" :disabled="busy" />
          </label>
        </div>
        <button type="submit" class="btn btn-primary" :disabled="busy">{{ t('admin.logCollection.start') }}</button>
        <p class="text-xs text-gray-500">{{ t('admin.logCollection.limits') }}</p>
      </form>
      <div v-if="health" class="text-sm text-gray-500" role="status">
        {{ t(health.ready ? 'admin.logCollection.ready' : 'admin.logCollection.unready') }} ·
        {{ t('admin.logCollection.health', { queue: health.queue_depth, dropped: health.dropped, failed: health.write_failed, refresh: health.refresh_failed }) }}
      </div>
      <section class="card space-y-4 p-5">
        <form class="flex flex-wrap gap-2" @submit.prevent="loadSessions()">
          <input v-model="collectionQuery" class="input min-w-0 flex-1" :aria-label="t('admin.logCollection.collectionId')" :placeholder="t('admin.logCollection.collectionId')" />
          <button class="btn btn-secondary" :disabled="loading">{{ t('common.search') }}</button>
          <button type="button" class="btn btn-secondary" :disabled="loading || recordsLoading" @click="refresh">{{ t('common.refresh') }}</button>
        </form>
        <p v-if="!sessions.length && !loading" class="text-gray-500">{{ t('admin.logCollection.empty') }}</p>
        <div v-for="session in sessions" :key="session.id" class="rounded-lg border p-4 dark:border-dark-600" :class="selected === session.id ? 'border-primary-500' : ''">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <button class="break-all text-left font-mono text-sm text-primary-600" @click="selectSession(session.id)">{{ session.id }}</button>
            <div class="flex items-center gap-3">
              <span :class="session.stopped_at ? 'text-gray-500' : 'text-green-600'">{{ t(session.stopped_at ? 'admin.logCollection.stopped' : 'admin.logCollection.active') }}</span>
              <button v-if="!session.stopped_at" class="btn btn-secondary" :disabled="busy" @click="stop(session.id)">{{ t('admin.logCollection.stop') }}</button>
            </div>
          </div>
          <p class="mt-2 text-sm">{{ t('admin.logCollection.scope', { user: session.user_id, group: session.group_id, account: session.account_id }) }}</p>
          <p class="mt-1 text-xs text-gray-500">
            {{ formatDate(session.started_at) }} → {{ session.stopped_at ? formatDate(session.stopped_at) : '…' }} ·
            {{ t('admin.logCollection.count', { count: session.record_count }) }}
            <span v-if="session.stop_reason === 'limit'"> · {{ t('admin.logCollection.limitReached') }}</span>
          </p>
        </div>
        <button v-if="nextSessions" class="btn btn-secondary" :disabled="loading" @click="loadSessions(true)">{{ t('admin.logCollection.more') }}</button>
      </section>
      <section v-if="selected" class="card space-y-4 p-5">
        <h2 class="break-all font-medium">{{ t('admin.logCollection.records') }} · {{ selected }}</h2>
        <p class="text-xs text-gray-500">{{ t('admin.logCollection.reading') }}</p>
        <form class="flex gap-2" @submit.prevent="loadRecords()">
          <input v-model="requestQuery" class="input min-w-0 flex-1" :aria-label="t('admin.logCollection.requestId')" :placeholder="t('admin.logCollection.requestId')" />
          <button class="btn btn-secondary" :disabled="recordsLoading">{{ t('common.search') }}</button>
        </form>
        <p v-if="!records.length && !recordsLoading" class="text-gray-500">{{ t('admin.logCollection.noRecords') }}</p>
        <details v-for="record in records" :key="record.id" class="rounded-lg border p-3 dark:border-dark-600">
          <summary class="cursor-pointer break-all text-sm">
            {{ formatDate(record.started_at) }} · {{ record.client_request_id ? `client:${record.client_request_id}` : record.request_id }} ·
            {{ record.duration_ms.toFixed(2) }} ms · {{ record.status }}
          </summary>
          <p class="my-3 break-all text-xs text-gray-500">{{ t('admin.logCollection.requestId') }}: {{ record.request_id }} · {{ t('admin.logCollection.bytes') }}: {{ record.request_bytes }}</p>
          <p v-if="record.truncated" class="text-amber-600">{{ t('admin.logCollection.truncated') }}</p>
          <div v-for="attempt in record.attempts" :key="attempt.number" class="mb-2 text-sm">
            {{ t('admin.logCollection.attempt') }} {{ attempt.number }} · {{ t('admin.logCollection.account_id') }} {{ attempt.account_id }} ·
            {{ attempt.transport.toUpperCase() }} · {{ t('admin.logCollection.bytes') }} {{ attempt.request_bytes < 0 ? '—' : attempt.request_bytes }} ·
            {{ attempt.status || '—' }}
            <span v-if="attempt.reused !== undefined"> · {{ t(attempt.reused ? 'admin.logCollection.reused' : 'admin.logCollection.newConnection') }}</span>
            <span v-if="attempt.failed" class="text-red-600"> · {{ t('admin.logCollection.failed') }}</span>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full text-left text-sm">
              <thead><tr class="border-b dark:border-dark-600"><th class="py-2">{{ t('admin.logCollection.stage') }}</th><th>{{ t('admin.logCollection.attempt') }}</th><th>{{ t('admin.logCollection.elapsed') }}</th><th>{{ t('admin.logCollection.duration') }}</th></tr></thead>
              <tbody><tr v-for="(event, index) in record.events" :key="index" class="border-b dark:border-dark-700">
                <td class="py-2">{{ stageName(event.name) }}</td><td>{{ event.attempt || '—' }}</td><td>{{ event.at_ms.toFixed(2) }} ms</td><td>{{ eventDuration(record, event) }}</td>
              </tr></tbody>
            </table>
          </div>
        </details>
        <button v-if="nextRecords" class="btn btn-secondary" :disabled="recordsLoading" @click="loadRecords(true)">{{ t('admin.logCollection.more') }}</button>
      </section>
    </div>
    <ConfirmDialog :show="!!pendingScope" :title="t('admin.logCollection.start')" :message="t('admin.logCollection.confirm', { user: pendingScope?.user_id, group: pendingScope?.group_id, account: pendingScope?.account_id })" @confirm="start" @cancel="!busy && (pendingScope = null)" />
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { latencyCollectionsAPI, type LatencyScope, type LatencySession, type LatencyRecord, type LatencyHealth, type LatencyEvent } from '@/api/admin/latencyCollections'
import { extractApiErrorMessage } from '@/utils/apiError'
import { latencyStageDuration } from '@/utils/latencyCollection'

const { t, te } = useI18n()
const scopeKeys = ['user_id', 'group_id', 'account_id'] as const
const form = ref<LatencyScope>({ user_id: 0, group_id: 0, account_id: 0 })
const pendingScope = ref<LatencyScope | null>(null)
const busy = ref(false), loading = ref(false), recordsLoading = ref(false)
const error = ref(''), collectionQuery = ref(''), requestQuery = ref(''), selected = ref('')
const sessions = ref<LatencySession[]>([]), records = ref<LatencyRecord[]>([])
const nextSessions = ref(0), nextRecords = ref(0)
const health = ref<LatencyHealth | null>(null)
let recordsVersion = 0
const formatDate = (value: string) => new Date(value).toLocaleString()
const stageName = (name: string) => te(`admin.logCollection.stages.${name}`) ? t(`admin.logCollection.stages.${name}`) : name
function eventDuration(record: LatencyRecord, event: LatencyEvent) {
  const duration = latencyStageDuration(record.events, event)
  return duration === undefined ? '—' : `${duration.toFixed(2)} ms`
}
function fail(err: unknown) { error.value = extractApiErrorMessage(err, t('admin.logCollection.error')) }
function confirmStart() {
  if (!scopeKeys.every(key => Number.isSafeInteger(form.value[key]) && form.value[key] > 0)) { error.value = t('admin.logCollection.invalid'); return }
  pendingScope.value = { ...form.value }
}
async function loadSessions(append = false) {
  if (loading.value) return
  loading.value = true; error.value = ''
  try {
    const result = await latencyCollectionsAPI.list(collectionQuery.value.trim(), append ? nextSessions.value : 0)
    sessions.value = append ? [...sessions.value, ...result.items] : result.items
    nextSessions.value = result.next_before; health.value = result.health
  } catch (err) { fail(err) } finally { loading.value = false }
}
async function selectSession(id: string) {
  selected.value = id; requestQuery.value = ''; records.value = []; nextRecords.value = 0
  await loadRecords()
}
async function loadRecords(append = false) {
  if (!selected.value) return
  const version = ++recordsVersion
  recordsLoading.value = true; error.value = ''
  try {
    const result = await latencyCollectionsAPI.records(selected.value, requestQuery.value.trim(), append ? nextRecords.value : 0)
    if (version !== recordsVersion) return
    records.value = append ? [...records.value, ...result.items] : result.items
    nextRecords.value = result.next_before
  } catch (err) { if (version === recordsVersion) fail(err) } finally { if (version === recordsVersion) recordsLoading.value = false }
}
async function start() {
  if (busy.value || !pendingScope.value) return
  busy.value = true; error.value = ''
  try {
    const session = await latencyCollectionsAPI.create(pendingScope.value)
    pendingScope.value = null; collectionQuery.value = ''
    await loadSessions(); await selectSession(session.id)
  } catch (err) { fail(err) } finally { busy.value = false }
}
async function stop(id: string) {
  if (busy.value) return
  busy.value = true; error.value = ''
  try { await latencyCollectionsAPI.stop(id); await loadSessions(); if (selected.value === id) await loadRecords() }
  catch (err) { fail(err) } finally { busy.value = false }
}
async function refresh() { await loadSessions(); if (selected.value) await loadRecords() }
onMounted(() => loadSessions())
</script>
