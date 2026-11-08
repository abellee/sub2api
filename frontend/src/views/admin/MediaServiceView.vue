<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-5" data-testid="media-service-page">
      <header class="flex flex-wrap items-center justify-between gap-3">
        <div><h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('mediaService.title') }}</h1><p class="mt-2 text-sm text-gray-500">{{ t('mediaService.description') }}</p></div>
        <button class="btn btn-secondary" :disabled="busy" @click="checkConnection">{{ t('mediaService.checkConnection') }}</button>
      </header>
      <p v-if="preview" class="rounded-lg bg-blue-50 p-3 text-sm text-blue-800 dark:bg-blue-950 dark:text-blue-200">{{ t('mediaService.preview') }}</p>
      <p v-if="error" role="alert" class="rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-950 dark:text-red-200">{{ error }}</p>
      <p v-if="notice" role="status" class="rounded-lg bg-green-50 p-3 text-sm text-green-800 dark:bg-green-950 dark:text-green-200">{{ notice }}</p>
      <nav class="flex flex-wrap gap-2 border-b border-gray-200 pb-3 dark:border-dark-700" :aria-label="t('mediaService.tabsLabel')">
        <button v-for="item in tabs" :key="item" class="btn" :class="tab === item ? 'btn-primary' : 'btn-secondary'" :aria-current="tab === item ? 'page' : undefined" :data-testid="`tab-${item}`" @click="selectTab(item)">{{ t(`mediaService.tabs.${item}`) }}</button>
      </nav>

      <section v-if="tab === 'main'" class="card space-y-5 p-5">
        <h2 class="text-lg font-medium">{{ t('mediaService.tabs.main') }}</h2>
        <p class="text-sm text-gray-500">{{ t('mediaService.mainHint') }}</p>
        <p v-if="mainView?.restart_required" class="rounded bg-amber-50 p-3 text-sm text-amber-900" data-testid="main-restart">{{ t('mediaService.mainRestart') }} {{ mainView.pending_fields.map(fieldLabel).join('、') }}</p>
        <form class="space-y-5" @submit.prevent="saveMain">
          <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <label v-for="f in mainFields" :key="f.key" class="flex flex-col gap-2 text-sm" :class="f.type === 'checkbox' ? 'justify-center' : ''">
              <span>{{ fieldLabel(f.key) }}<MediaSettingInfo :title="fieldLabel(f.key)" :content="fieldHelp(f.key)" /> <span v-if="main[f.key + '_configured']" class="text-green-600">{{ t('mediaService.configured') }}</span></span>
              <input v-if="f.type === 'checkbox'" v-model="main[f.key]" type="checkbox" class="h-5 w-5" :data-testid="`main-${f.key}`" />
              <input v-else-if="f.type === 'number'" v-model.number="main[f.key]" type="number" :min="f.min ?? 1" :max="f.max" class="input w-full" required :data-testid="`main-${f.key}`" />
              <input v-else v-model="main[f.key]" :type="f.type || 'text'" :autocomplete="f.type === 'password' ? 'new-password' : 'off'" :placeholder="f.type === 'password' ? t('mediaService.keepSecret') : ''" class="input w-full" :data-testid="`main-${f.key}`" />
            </label>
          </div>
          <button class="btn btn-primary" :disabled="busy || !mainView" data-testid="save-main">{{ t('mediaService.saveMain') }}</button>
        </form>
      </section>

      <section v-else-if="tab === 'runtime'" class="card space-y-5 p-5">
        <h2 class="text-lg font-medium">{{ t('mediaService.tabs.runtime') }}</h2>
        <p class="text-sm text-gray-500">{{ t('mediaService.runtimeHint') }}</p>
        <p v-if="runtimeView?.restart_required" class="rounded bg-amber-50 p-3 text-sm text-amber-900">{{ t('mediaService.agentRestart') }} {{ runtimeView.pending_fields.map(fieldLabel).join('、') }}</p>
        <form class="space-y-5" @submit.prevent="saveRuntime">
          <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <label v-for="f in runtimeFields" :key="f.key" class="flex flex-col gap-2 text-sm">
              <span>{{ fieldLabel(f.key) }}<MediaSettingInfo :title="fieldLabel(f.key)" :content="fieldHelp(f.key)" /> <span v-if="runtime[f.key + '_configured']" class="text-green-600">{{ t('mediaService.configured') }}</span></span>
              <input v-if="f.type === 'checkbox'" v-model="runtime[f.key]" type="checkbox" class="h-5 w-5" :disabled="f.key === 'persist_credentials'" />
              <input v-else-if="f.type === 'number'" v-model.number="runtime[f.key]" type="number" :min="f.min ?? 1" :max="f.max" required class="input w-full" />
              <select v-else-if="f.key === 'log_level'" v-model="runtime[f.key]" class="input"><option v-for="level in ['debug', 'info', 'warn', 'error']" :key="level">{{ level }}</option></select>
              <input v-else v-model="runtime[f.key]" :type="f.type || 'text'" :autocomplete="f.type === 'password' ? 'new-password' : 'off'" :placeholder="f.type === 'password' ? t('mediaService.keepSecret') : ''" class="input w-full" />
            </label>
          </div>
          <button class="btn btn-primary" :disabled="busy || !runtimeView">{{ t('mediaService.saveRuntime') }}</button>
        </form>
        <details v-if="runtimeView"><summary class="cursor-pointer text-sm">{{ t('mediaService.activeSettings') }}</summary><pre class="mt-3 overflow-auto rounded bg-gray-50 p-3 text-xs dark:bg-dark-900">{{ pretty(runtimeView.active) }}</pre></details>
      </section>

      <section v-else-if="tab === 'rules'" class="card space-y-5 p-5">
        <div class="flex flex-wrap items-center gap-3"><h2 class="mr-auto text-lg font-medium">{{ t('mediaService.tabs.rules') }}</h2><button class="btn btn-secondary" :disabled="busy" @click="newRule('image')">{{ t('mediaService.imageTemplate') }}</button><button class="btn btn-secondary" :disabled="busy" @click="newRule('video')">{{ t('mediaService.videoTemplate') }}</button></div>
        <p class="text-sm text-gray-500">{{ t('mediaService.rulesHint') }}</p>
        <div class="flex flex-wrap gap-2"><button v-for="s in schemas" :key="s.id" class="btn btn-secondary" :disabled="busy" @click="loadRule(s.id)">{{ s.id }} · v{{ s.version }}</button><span v-if="!schemas.length" class="text-sm text-gray-500">{{ t('mediaService.noRules') }}</span></div>
        <form class="space-y-4" @submit.prevent="saveRule">
          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            <label class="text-sm">{{ t('mediaService.ruleId') }}<MediaSettingInfo :title="t('mediaService.ruleId')" :content="t('mediaService.help.ruleId')" /><input v-model="rule.id" class="input mt-2 w-full" pattern="[A-Za-z0-9][A-Za-z0-9_.-]{0,127}" required /></label>
            <label class="text-sm">{{ t('mediaService.ruleVersion') }}<MediaSettingInfo :title="t('mediaService.ruleVersion')" :content="t('mediaService.help.ruleVersion')" /><input v-model.number="rule.version" type="number" min="1" required class="input mt-2 w-full" /></label>
            <label class="text-sm">{{ t('mediaService.upstreamMode') }}<MediaSettingInfo :title="t('mediaService.upstreamMode')" :content="t('mediaService.help.upstreamMode')" /><select v-model="rule.mode" class="input mt-2 w-full" @change="onModeChange"><option value="sync">{{ t('mediaService.syncUpstream') }}</option><option value="async">{{ t('mediaService.asyncUpstream') }}</option></select></label>
            <label class="text-sm">{{ t('mediaService.createPath') }}<MediaSettingInfo :title="t('mediaService.createPath')" :content="t('mediaService.help.createPath')" /><input v-model="rule.upstream.create.path" class="input mt-2 w-full" required /></label>
            <label class="text-sm">{{ t('mediaService.createTimeout') }}<MediaSettingInfo :title="t('mediaService.createTimeout')" :content="t('mediaService.help.createTimeout')" /><input v-model.number="rule.upstream.create.timeout_seconds" type="number" min="1" class="input mt-2 w-full" /></label>
            <label class="text-sm">{{ t('mediaService.resultPath') }}<MediaSettingInfo :title="t('mediaService.resultPath')" :content="t('mediaService.help.resultPath')" /><input v-model="rule.response.result.files" class="input mt-2 w-full" /></label>
            <template v-if="rule.mode === 'async' && rule.upstream.query">
              <label class="text-sm">{{ t('mediaService.queryPath') }}<MediaSettingInfo :title="t('mediaService.queryPath')" :content="t('mediaService.help.queryPath')" /><input v-model="rule.upstream.query.path" class="input mt-2 w-full" required /></label>
              <label class="text-sm">{{ t('mediaService.taskIdPath') }}<MediaSettingInfo :title="t('mediaService.taskIdPath')" :content="t('mediaService.help.taskIdPath')" /><input v-model="rule.response.task_id" class="input mt-2 w-full" required /></label>
              <label class="text-sm">{{ t('mediaService.statusPath') }}<MediaSettingInfo :title="t('mediaService.statusPath')" :content="t('mediaService.help.statusPath')" /><input v-model="rule.response.status" class="input mt-2 w-full" required /></label>
            </template>
            <label class="text-sm sm:col-span-2">{{ t('mediaService.ruleDescription') }}<MediaSettingInfo :title="t('mediaService.ruleDescription')" :content="t('mediaService.help.ruleDescription')" /><input v-model="rule.description" class="input mt-2 w-full" /></label>
          </div>
          <div class="flex flex-wrap gap-3"><button class="btn btn-primary" :disabled="busy" data-testid="save-rule">{{ t('mediaService.saveRule') }}</button><button type="button" class="btn btn-secondary" :disabled="busy || !loadedRuleId" @click="removeRule">{{ t('mediaService.deleteRule') }}</button></div>
        </form>
        <details @toggle="syncAdvanced"><summary class="cursor-pointer text-sm font-medium">{{ t('mediaService.advancedRule') }}<MediaSettingInfo :title="t('mediaService.advancedRule')" :content="t('mediaService.help.advancedRule')" /></summary><p class="my-3 text-sm text-gray-500">{{ t('mediaService.advancedHint') }}</p><label class="sr-only" for="rule-json">{{ t('mediaService.advancedRule') }}<MediaSettingInfo :title="t('mediaService.advancedRule')" :content="t('mediaService.help.advancedRule')" /></label><textarea id="rule-json" v-model="advancedRule" rows="20" spellcheck="false" class="input w-full font-mono text-xs" /><button class="btn btn-primary mt-3" :disabled="busy" @click="saveAdvancedRule">{{ t('mediaService.saveAdvanced') }}</button></details>
        <div class="border-t pt-4 dark:border-dark-700"><label class="text-sm" for="dry-run">{{ t('mediaService.dryRunHint') }}<MediaSettingInfo :title="t('mediaService.dryRunHint')" :content="t('mediaService.help.dryRunHint')" /></label><textarea id="dry-run" v-model="dryRun" rows="8" class="input mt-3 w-full font-mono text-xs" spellcheck="false" /><button class="btn btn-secondary mt-3" :disabled="busy" @click="runDryRun">{{ t('mediaService.dryRun') }}</button></div>
      </section>

      <section v-else-if="tab === 'bindings'" class="card space-y-5 p-5">
        <h2 class="text-lg font-medium">{{ t('mediaService.tabs.bindings') }}</h2><p class="text-sm text-gray-500">{{ t('mediaService.bindingsHint') }}</p>
        <form class="grid items-end gap-3 sm:grid-cols-2 lg:grid-cols-5" @submit.prevent="saveBinding">
          <label class="text-sm">{{ t('mediaService.groupId') }}<MediaSettingInfo :title="t('mediaService.groupId')" :content="t('mediaService.help.groupId')" /><input v-model.number="binding.group_id" type="number" min="1" required class="input mt-2 w-full" /></label>
          <label class="text-sm">{{ t('mediaService.ruleId') }}<MediaSettingInfo :title="t('mediaService.ruleId')" :content="t('mediaService.help.ruleId')" /><select v-model="binding.schema_id" required class="input mt-2 w-full"><option value="" disabled>{{ t('mediaService.selectRule') }}</option><option v-for="s in schemas" :key="s.id" :value="s.id">{{ s.id }}</option></select></label>
          <label class="text-sm">{{ t('mediaService.estimateKind') }}<MediaSettingInfo :title="t('mediaService.estimateKind')" :content="t('mediaService.help.estimateKind')" /><select v-model="binding.estimate_kind" class="input mt-2 w-full"><option value="image">{{ t('mediaService.image') }}</option><option value="video">{{ t('mediaService.video') }}</option><option value="token">Token</option></select></label>
          <label class="flex items-center gap-2 py-3 text-sm"><input v-model="binding.enabled" type="checkbox" />{{ t('mediaService.enabled') }}<MediaSettingInfo :title="t('mediaService.enabled')" :content="t('mediaService.help.bindingEnabled')" /></label>
          <button class="btn btn-primary" :disabled="busy" data-testid="save-binding">{{ t('mediaService.saveBinding') }}</button>
        </form>
        <div class="overflow-auto"><table class="w-full text-left text-sm"><thead><tr class="border-b dark:border-dark-700"><th class="p-3">{{ t('mediaService.groupId') }}<MediaSettingInfo :title="t('mediaService.groupId')" :content="t('mediaService.help.groupId')" /></th><th>{{ t('mediaService.ruleId') }}<MediaSettingInfo :title="t('mediaService.ruleId')" :content="t('mediaService.help.ruleId')" /></th><th>{{ t('mediaService.estimateKind') }}<MediaSettingInfo :title="t('mediaService.estimateKind')" :content="t('mediaService.help.estimateKind')" /></th><th>{{ t('mediaService.enabled') }}</th><th>{{ t('mediaService.actions') }}</th></tr></thead><tbody><tr v-for="b in bindings" :key="b.group_id" class="border-b dark:border-dark-700"><td class="p-3">{{ b.group_id }}</td><td>{{ b.schema_id }}</td><td>{{ b.estimate_kind }}</td><td>{{ b.enabled ? t('mediaService.yes') : t('mediaService.no') }}</td><td class="flex gap-2 py-2"><button class="btn btn-secondary" :disabled="busy" @click="binding = { ...b }">{{ t('mediaService.edit') }}</button><button class="btn btn-secondary" :disabled="busy" @click="deleteBinding(b.group_id)">{{ t('mediaService.delete') }}</button></td></tr></tbody></table><p v-if="!bindings.length" class="p-4 text-sm text-gray-500">{{ t('mediaService.noBindings') }}</p></div>
      </section>

      <section v-else-if="tab === 'callback'" class="card space-y-5 p-5">
        <h2 class="text-lg font-medium">{{ t('mediaService.tabs.callback') }}</h2><p class="text-sm text-gray-500">{{ t('mediaService.callbackHint') }}</p>
        <form class="space-y-4" @submit.prevent="saveCallback"><label class="block text-sm">{{ t('mediaService.callbackUrl') }}<MediaSettingInfo :title="t('mediaService.callbackUrl')" :content="t('mediaService.help.callbackUrl')" /><input v-model="callback.callback_url" type="url" class="input mt-2 w-full" placeholder="https://example.com/internal/media/result" :required="callback.enabled" /></label><div class="grid gap-4 sm:grid-cols-3"><label class="text-sm">{{ t('mediaService.maxAttempts') }}<MediaSettingInfo :title="t('mediaService.maxAttempts')" :content="t('mediaService.help.maxAttempts')" /><input v-model.number="callback.max_attempts" type="number" min="1" max="100" required class="input mt-2 w-full" /></label><label class="text-sm">{{ t('mediaService.backoff') }}<MediaSettingInfo :title="t('mediaService.backoff')" :content="t('mediaService.help.backoff')" /><input v-model.number="callback.backoff_base_seconds" type="number" min="1" max="60" required class="input mt-2 w-full" /></label><label class="flex items-center gap-2 text-sm"><input v-model="callback.enabled" type="checkbox" />{{ t('mediaService.enabled') }}<MediaSettingInfo :title="t('mediaService.enabled')" :content="t('mediaService.help.callbackEnabled')" /></label></div><div class="flex gap-3"><button class="btn btn-primary" :disabled="busy || !callbackLoaded">{{ t('mediaService.saveCallback') }}</button><button type="button" class="btn btn-secondary" :disabled="busy || !callbackLoaded" @click="testCallback">{{ t('mediaService.testCallback') }}</button></div></form>
      </section>

      <section v-else-if="tab === 'records'" class="card space-y-5 p-5">
        <h2 class="text-lg font-medium">{{ t('mediaService.tabs.records') }}</h2>
        <p class="text-sm text-gray-500">{{ t('mediaService.recordsHint') }}</p>
        <form class="flex flex-wrap items-end gap-3" @submit.prevent="loadRecords"><label class="text-sm">{{ t('mediaService.userId') }}<MediaSettingInfo :title="t('mediaService.userId')" :content="t('mediaService.help.userId')" /><input v-model.number="recordUserID" type="number" min="1" required class="input mt-2 block" /></label><button class="btn btn-primary" :disabled="busy">{{ t('mediaService.queryRecords') }}</button></form>
        <div class="overflow-auto"><table class="w-full text-left text-sm"><thead><tr class="border-b dark:border-dark-700"><th class="p-3">{{ t('mediaService.userId') }}</th><th>{{ t('mediaService.taskId') }}</th><th>{{ t('mediaService.fileInfo') }}</th><th>{{ t('mediaService.downloadLink') }}</th></tr></thead><tbody><tr v-for="record in records" :key="record.id" class="border-b dark:border-dark-700"><td class="p-3">{{ record.user_id }}</td><td class="font-mono text-xs">{{ record.task_id }}</td><td>{{ record.content_type }} · {{ record.size_bytes }} B</td><td><a :href="record.download_url" target="_blank" rel="noreferrer noopener" class="text-primary-500 underline">{{ t('mediaService.openDownload') }}</a></td></tr></tbody></table><p v-if="recordsLoaded && !records.length" class="p-4 text-sm text-gray-500">{{ t('mediaService.noRecords') }}</p></div>
      </section>
      <section v-else class="card space-y-5 p-5">
        <div class="flex items-center justify-between"><h2 class="text-lg font-medium">{{ t('mediaService.tabs.tasks') }}</h2><button class="btn btn-secondary" :disabled="busy" @click="reloadTasks">{{ t('mediaService.refresh') }}</button></div><p class="text-sm text-gray-500">{{ t('mediaService.tasksHint') }}</p>
        <div class="overflow-auto"><table class="w-full text-left text-sm"><thead><tr class="border-b dark:border-dark-700"><th class="p-3">{{ t('mediaService.taskId') }}</th><th>{{ t('mediaService.taskStatus') }}</th><th>{{ t('mediaService.callbackStatus') }}</th><th>{{ t('mediaService.actions') }}</th></tr></thead><tbody><tr v-for="task in tasks" :key="task.task_id" class="border-b dark:border-dark-700"><td class="p-3 font-mono text-xs">{{ task.task_id }}</td><td>{{ task.status }}</td><td>{{ task.callback_state }}</td><td><button class="btn btn-secondary my-2" :disabled="busy" @click="recoveryTask = task.task_id">{{ t('mediaService.recover') }}</button></td></tr></tbody></table><p v-if="!tasks.length" class="p-4 text-sm text-gray-500">{{ t('mediaService.noTasks') }}</p></div>
        <form v-if="recoveryTask" class="space-y-3 border-t pt-4 dark:border-dark-700" @submit.prevent="recoverTask"><p class="text-sm font-mono">{{ recoveryTask }}</p><label class="block text-sm">{{ t('mediaService.recoveryBody') }}<MediaSettingInfo :title="t('mediaService.recoveryBody')" :content="t('mediaService.help.recoveryBody')" /><textarea v-model="recoveryBody" rows="5" class="input mt-2 w-full font-mono text-xs" /></label><button class="btn btn-primary" :disabled="busy">{{ t('mediaService.recover') }}</button></form>
      </section>
      <details v-if="output" open class="card p-5"><summary class="cursor-pointer text-sm font-medium">{{ t('mediaService.result') }}</summary><pre class="mt-3 max-h-96 overflow-auto whitespace-pre-wrap break-all rounded bg-gray-50 p-3 text-xs dark:bg-dark-900">{{ output }}</pre></details>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import MediaSettingInfo from '@/components/admin/MediaSettingInfo.vue'
import { mediaServiceAPI as api, type Binding, type ConfigView, type Fields, type DownloadRecord } from '@/api/admin/mediaService'

const { t } = useI18n()
const preview = import.meta.env.DEV && import.meta.env.VITE_MEDIA_SERVICE_PREVIEW === 'true'
const tabs = ['main', 'runtime', 'rules', 'bindings', 'callback', 'tasks', 'records'] as const
type Tab = typeof tabs[number]
const tab = ref<Tab>('main'), busy = ref(false), error = ref(''), notice = ref(''), output = ref('')
const main = ref<Fields>({}), runtime = ref<Fields>({})
const mainView = ref<ConfigView>(), runtimeView = ref<ConfigView>()
type Field = { key: string; type?: string; min?: number; max?: number }
const numeric = (key: string, max: number): Field => ({ key, type: 'number', max })
const secret = (key: string): Field => ({ key, type: 'password' })
const mainFields: Field[] = [
  { key: 'enabled', type: 'checkbox' }, { key: 'mediad_base_url', type: 'url' }, secret('management_token'), secret('relay_secret'), secret('callback_secret'),
  { key: 'bindings_path' }, numeric('forward_timeout_seconds', 86400), numeric('management_timeout_seconds', 600)
]
const runtimeFields: Field[] = [
  { key: 'listen' }, { key: 'data_dir' }, { key: 'result_dir' }, { key: 'download_base_url', type: 'url' }, numeric('max_file_mib', 10240), numeric('max_sync_seconds', 86400), numeric('max_concurrent', 128),
  numeric('poll_interval_seconds', 3600), numeric('max_poll_seconds', 604800), numeric('download_timeout_seconds', 86400), numeric('download_attempts', 100), { key: 'file_retention_hours', type: 'number', min: 0 }, numeric('task_retention_hours', 87600), numeric('gc_minutes', 1440),
  { key: 'persist_credentials', type: 'checkbox' }, { key: 'log_level' }, secret('relay_secret'), secret('management_token')
]
const fieldLabel = (key: string) => t(`mediaService.fields.${key}`)
const fieldHelp = (key: string) => t(`mediaService.help.${key}`)
const pretty = (value: unknown) => JSON.stringify(value, null, 2)
const recordUserID = ref(1), records = ref<DownloadRecord[]>([]), recordsLoaded = ref(false)
async function loadRecords() { await action(async () => { recordsLoaded.value = false; records.value = (await api.records(recordUserID.value)).records; recordsLoaded.value = true }) }
async function action(fn: () => Promise<void>) {
  if (busy.value) return
  busy.value = true; error.value = ''; notice.value = ''
  try { await fn() } catch (e) { error.value = e instanceof Error ? e.message : (e as { message?: string })?.message || t('mediaService.failed') } finally { busy.value = false }
}
function clean(fields: Fields): Fields { return Object.fromEntries(Object.entries(fields).filter(([key]) => !key.endsWith('_configured'))) }
function applyMain(view: ConfigView) { mainView.value = view; main.value = { ...view.settings, management_token: '', relay_secret: '', callback_secret: '' } }
async function saveMain() { await action(async () => { applyMain(await api.saveConfig(clean(main.value))); notice.value = mainView.value?.restart_required ? t('mediaService.mainSavedRestart') : t('mediaService.saved') }) }
async function saveRuntime() { await action(async () => { runtimeView.value = await api.agent<ConfigView>('PUT', '/config', clean(runtime.value)); runtime.value = { ...runtimeView.value.settings, relay_secret: '', management_token: '' }; notice.value = runtimeView.value.restart_required ? t('mediaService.runtimeSavedRestart') : t('mediaService.saved') }) }
async function checkConnection() { await action(async () => { const result = await api.agent<ConfigView>('GET', '/config'); output.value = pretty({ management_connected: true, restart_required: result.restart_required, capabilities: await api.agent('GET', '/capabilities') }); notice.value = t('mediaService.connected') }) }

interface Rule {
  id: string; version: number; mode: string; description?: string
  upstream: { create: { method: string; path: string; timeout_seconds?: number }; query?: { method: string; path: string; poll_interval_seconds?: number; max_poll_seconds?: number } }
  request: Record<string, unknown>
  response: { task_id?: string; status?: string; result: Record<string, string>; usage: { quantity: number | string; unit: string } }
  task_states?: { success: string[]; failure: string[] }
}
function template(kind = 'image'): Rule {
  const video = kind === 'video'
  return { id: video ? 'video-default' : 'image-default', version: 1, mode: video ? 'async' : 'sync', description: '', upstream: { create: { method: 'POST', path: video ? '/v1/videos' : '/v1/images/generations', timeout_seconds: 600 }, ...(video ? { query: { method: 'GET', path: '/v1/videos/{task_id}', poll_interval_seconds: 5, max_poll_seconds: 1800 } } : {}) }, request: { credential: { Authorization: 'Bearer {credential}' }, body_mode: 'passthrough' }, response: { ...(video ? { task_id: 'id', status: 'status' } : {}), result: { files: video ? 'url' : 'data' }, usage: { quantity: video ? 'duration' : 1, unit: video ? 'second' : 'image' } }, ...(video ? { task_states: { success: ['completed', 'succeeded'], failure: ['failed', 'cancelled'] } } : {}) }
}
const rule = ref<Rule>(template()), loadedRuleId = ref(''), advancedRule = ref('')
const schemas = ref<{ id: string; version: number }[]>([])
const dryRun = ref(pretty({ schema_id: 'image-default', body: { model: 'your-image-model', prompt: 'A cat' }, upstream_response: { data: [{ url: 'https://example.com/image.png' }] } }))
function newRule(kind: string) { rule.value = template(kind); loadedRuleId.value = ''; advancedRule.value = pretty(rule.value) }
function onModeChange() {
  if (rule.value.mode === 'async') {
    rule.value.upstream.query ||= { method: 'GET', path: '/v1/tasks/{task_id}', poll_interval_seconds: 5, max_poll_seconds: 1800 }
    rule.value.response.task_id ||= 'id'; rule.value.response.status ||= 'status'
    rule.value.task_states ||= { success: ['completed', 'succeeded'], failure: ['failed', 'cancelled'] }
  }
}
function syncAdvanced(e: Event) { if ((e.target as HTMLDetailsElement).open) advancedRule.value = pretty(rule.value) }
async function reloadRules() { schemas.value = (await api.agent<{ schemas: { id: string; version: number }[] }>('GET', '/schemas')).schemas }
async function loadRule(id: string) { await action(async () => { rule.value = await api.agent<Rule>('GET', `/schemas/${encodeURIComponent(id)}`); rule.value.response.result ||= {}; loadedRuleId.value = id; advancedRule.value = pretty(rule.value) }) }
async function persistRule(value: Rule) { if (!/^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$/.test(value.id)) throw new Error(t('mediaService.invalidRuleId')); await api.agent('PUT', `/schemas/${encodeURIComponent(value.id)}`, value); rule.value = value; loadedRuleId.value = value.id; advancedRule.value = pretty(value); await reloadRules(); notice.value = t('mediaService.saved') }
async function saveRule() { await action(() => persistRule(rule.value)) }
async function saveAdvancedRule() { await action(async () => { await persistRule(JSON.parse(advancedRule.value)) }) }
async function removeRule() { if (!window.confirm(t('mediaService.confirmDelete'))) return; await action(async () => { await api.agent('DELETE', `/schemas/${encodeURIComponent(loadedRuleId.value)}`); newRule('image'); await reloadRules(); notice.value = t('mediaService.deleted') }) }
async function runDryRun() { await action(async () => { output.value = pretty(await api.agent('POST', '/dry-run', JSON.parse(dryRun.value))) }) }

const bindings = ref<Binding[]>([]), binding = ref<Binding>({ group_id: 1, schema_id: '', estimate_kind: 'image', enabled: true })
async function saveBinding() { await action(async () => { await api.saveBinding(binding.value); bindings.value = (await api.bindings()).bindings; notice.value = t('mediaService.saved') }) }
async function deleteBinding(id: number) { if (!window.confirm(t('mediaService.confirmDelete'))) return; await action(async () => { await api.deleteBinding(id); bindings.value = (await api.bindings()).bindings; notice.value = t('mediaService.deleted') }) }

const callbackLoaded = ref(false)
const callback = ref({ callback_url: '', enabled: false, max_attempts: 10, backoff_base_seconds: 2 })
async function saveCallback() { await action(async () => { await api.agent('PUT', '/callback', callback.value); notice.value = t('mediaService.saved') }) }
async function testCallback() { await action(async () => { const result = await api.agent('POST', '/callback/test', {}); output.value = pretty(result); if (!result.delivered) throw new Error(String(result.error || t('mediaService.callbackFailed'))); notice.value = t('mediaService.callbackPassed') }) }

const tasks = ref<{ task_id: string; status: string; callback_state: string }[]>([])
const recoveryTask = ref(''), recoveryBody = ref('{}')
async function fetchTasks() { tasks.value = (await api.agent<{ tasks: typeof tasks.value }>('GET', '/tasks/pending')).tasks }
async function reloadTasks() { await action(fetchTasks) }
async function recoverTask() { await action(async () => { output.value = pretty(await api.agent('POST', `/tasks/${encodeURIComponent(recoveryTask.value)}/recover`, JSON.parse(recoveryBody.value))); await fetchTasks(); notice.value = t('mediaService.recoveryStarted') }) }
async function selectTab(value: Tab) {
  if (busy.value) return
  tab.value = value; output.value = ''
  await action(async () => {
    if (value === 'runtime') { runtimeView.value = await api.agent<ConfigView>('GET', '/config'); runtime.value = { ...runtimeView.value.settings, relay_secret: '', management_token: '' } }
    if (value === 'rules') await reloadRules()
    if (value === 'bindings') { bindings.value = (await api.bindings()).bindings; await reloadRules() }
    if (value === 'callback') { callbackLoaded.value = false; const cb = await api.agent<typeof callback.value>('GET', '/callback'); callback.value = { ...cb, max_attempts: cb.max_attempts || 10, backoff_base_seconds: cb.backoff_base_seconds || 2 }; callbackLoaded.value = true }
    if (value === 'tasks') await fetchTasks()
  })
}
onMounted(() => action(async () => { applyMain(await api.config()) }))
</script>
