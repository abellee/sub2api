<template>
  <BaseDialog :show="show" :title="t('admin.accounts.inflation.title')" width="full" @close="close">
    <div class="grid min-w-0 gap-5 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
      <section class="min-w-0 space-y-4 lg:max-h-[72vh] lg:overflow-y-auto lg:pr-2" :aria-label="t('admin.accounts.inflation.settings')">
        <h4 class="font-medium">{{ t('admin.accounts.inflation.settings') }}</h4>
        <div class="flex gap-2">
          <button class="btn btn-sm" :class="sourceMode === 'accounts' ? 'btn-primary' : 'btn-secondary'" :aria-pressed="sourceMode === 'accounts'" :disabled="running" @click="sourceMode = 'accounts'">{{ t('admin.accounts.inflation.savedAccounts') }}</button>
          <button class="btn btn-sm" :class="sourceMode === 'manual' ? 'btn-primary' : 'btn-secondary'" :aria-pressed="sourceMode === 'manual'" :disabled="running" @click="sourceMode = 'manual'">{{ t('admin.accounts.inflation.manualAPI') }}</button>
        </div>
        <template v-if="sourceMode === 'accounts'">
          <div class="grid items-end gap-3 sm:grid-cols-[1fr_1fr_auto]">
            <div class="space-y-1 text-sm">
              <div>{{ t('admin.accounts.platform') }}</div>
              <Select v-model="platform" :options="platformOptions" :disabled="running" :aria-label="t('admin.accounts.platform')" />
            </div>
            <div class="space-y-1 text-sm">
              <div>{{ t('admin.accounts.inflation.group') }}</div>
              <Select v-model="group" :options="groupOptions" :disabled="running" :aria-label="t('admin.accounts.inflation.group')" />
            </div>
            <button class="btn btn-secondary" :disabled="running || loadingAccounts" @click="loadAccounts">
              {{ loadingAccounts ? t('common.loading') : t('admin.accounts.inflation.loadAccounts') }}
            </button>
          </div>
          <p v-if="loadError" role="alert" class="text-sm text-red-500">{{ loadError }}</p>
          <p v-if="!loaded && !loadingAccounts" class="text-sm text-gray-500">{{ t('admin.accounts.inflation.selectHint') }}</p>
          <div v-if="loaded" class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
            <div class="flex flex-wrap items-center gap-3 bg-gray-50 px-3 py-2 text-sm dark:bg-dark-800">
              <label class="flex items-center gap-2">
                <input type="checkbox" :checked="allVisibleSelected" :disabled="running || !accounts.length" @change="toggleVisible(($event.target as HTMLInputElement).checked)" />
                {{ t('common.selectAll') }}
              </label>
              <span>{{ t('admin.accounts.inflation.selectionCount', { selected: selectedIds.length, total: accounts.length }) }}</span>
            </div>
            <div class="max-h-48 overflow-auto">
              <p v-if="!accounts.length" class="p-3 text-sm text-gray-500">{{ t('admin.accounts.inflation.noAccounts') }}</p>
              <label v-for="account in accounts" :key="account.id" class="flex cursor-pointer items-center gap-3 border-t border-gray-100 px-3 py-2 text-sm hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-800">
                <input type="checkbox" :checked="isSelected(account.id)" :disabled="running" @change="toggle(account.id)" />
                <span class="min-w-0 flex-1 break-all">{{ account.name }}</span>
                <span class="shrink-0 text-xs text-gray-500">#{{ account.id }} · {{ account.platform }} · {{ account.type }}</span>
              </label>
            </div>
          </div>
        </template>
        <div v-else class="space-y-3">
          <p class="text-xs text-gray-500">{{ t('admin.accounts.inflation.manualHint') }}</p>
          <div v-for="(target, index) in directTargets" :key="target.id" class="space-y-2 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
            <div class="flex items-center justify-between gap-2">
              <span class="text-sm font-medium">API {{ index + 1 }}</span>
              <button class="btn btn-secondary btn-sm" :disabled="running" :aria-label="t('admin.accounts.inflation.removeTarget', { index: index + 1 })" @click="directTargets.splice(index, 1)">{{ t('common.remove') }}</button>
            </div>
            <label class="block space-y-1 text-sm">
              <span>{{ t('admin.accounts.inflation.protocol') }}</span>
              <select v-model="target.protocol" class="input w-full" :disabled="running">
                <option value="anthropic">Anthropic Messages</option>
                <option value="chat_completions">OpenAI Chat Completions</option>
                <option value="responses">OpenAI Responses</option>
              </select>
            </label>
            <label class="block space-y-1 text-sm">
              <span>{{ t('admin.accounts.inflation.apiAddress') }}</span>
              <input v-model="target.base_url" type="url" class="input w-full" :disabled="running" maxlength="2048" autocomplete="off" placeholder="https://api.example.com/v1" />
            </label>
            <label class="block space-y-1 text-sm">
              <span>API Key</span>
              <input v-model="target.api_key" type="password" class="input w-full" :disabled="running" maxlength="4096" autocomplete="new-password" spellcheck="false" placeholder="sk-..." />
            </label>
          </div>
          <button class="btn btn-secondary btn-sm" :disabled="running" @click="addDirectTarget">{{ t('admin.accounts.inflation.addTarget') }}</button>
        </div>
        <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('admin.accounts.inflation.hint', { count: targetCount }) }}</p>
        <div class="grid gap-3 sm:grid-cols-2">
          <label class="space-y-1 text-sm">
            <span>{{ t('admin.accounts.inflation.model') }}</span>
            <input v-model="model" class="input w-full" :disabled="running" maxlength="200" :placeholder="sourceMode === 'manual' ? t('admin.accounts.inflation.manualModelHint') : t('admin.accounts.inflation.modelHint')" />
          </label>
          <label class="space-y-1 text-sm">
            <span>Tokenizer</span>
            <select v-model="encoding" class="input w-full" :disabled="running">
              <option value="o200k_base">o200k_base (GPT-4o / GPT-5 / GPT-6)</option>
              <option value="cl100k_base">cl100k_base (GPT-3.5 / GPT-4)</option>
            </select>
          </label>
        </div>
        <label class="block space-y-1 text-sm">
          <span>{{ t('admin.accounts.inflation.timeoutSeconds') }}</span>
          <input v-model.number="timeoutSeconds" type="number" class="input w-full" min="1" max="3600" step="1" :disabled="running" />
          <span class="block text-xs text-gray-500">{{ t('admin.accounts.inflation.timeoutHint') }}</span>
        </label>
        <label class="block space-y-1 text-sm">
          <span>{{ t('admin.accounts.inflation.prompt') }}</span>
          <textarea v-model="prompt" class="input w-full" rows="3" :disabled="running" maxlength="8000" />
        </label>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.inflation.formula') }}</p>
      </section>
      <section class="flex min-w-0 flex-col gap-3" :aria-label="t('admin.accounts.inflation.output')">
        <h4 class="font-medium">{{ t('admin.accounts.inflation.output') }}</h4>
        <div ref="terminal" role="log" aria-live="polite" :aria-label="t('admin.accounts.inflation.title')" class="min-h-[320px] max-h-[60vh] lg:h-[calc(72vh-2rem)] lg:max-h-none space-y-4 overflow-auto rounded-xl border border-gray-700 bg-gray-950 p-4 font-mono text-xs text-gray-300">
          <p v-if="!results.length" class="text-gray-500">$ {{ t('admin.accounts.inflation.ready') }}</p>
          <section v-for="result in results" :key="result.id" class="space-y-1 border-b border-gray-800 pb-3">
            <p class="break-words text-cyan-300">$ [{{ result.id }}] {{ result.name }} · {{ result.model || model || 'auto' }}</p>
            <details v-if="result.reasoning" open class="rounded border border-purple-900/60 bg-purple-950/20 p-2 text-purple-300">
              <summary class="cursor-pointer">{{ t('admin.accounts.inflation.reasoning') }}</summary>
              <p class="mt-1 whitespace-pre-wrap break-words">{{ result.reasoning }}</p>
            </details>
            <p v-if="result.redacted || result.measurement?.reasoning_redacted" class="text-purple-300">{{ t('admin.accounts.inflation.reasoningRedacted') }}</p>
            <p v-else-if="result.status !== 'running' && !result.reasoning" class="text-gray-500">{{ t('admin.accounts.inflation.noReasoning') }}</p>
            <p v-if="result.text" class="whitespace-pre-wrap break-words text-green-300">{{ result.text }}</p>
            <p v-if="result.measurement" class="text-yellow-200">
              tokenizer={{ result.measurement.token_count }} · {{ result.measurement.output_field || 'output_tokens' }}={{ result.measurement.output_tokens ?? '—' }} · reasoning_tokens={{ result.measurement.reasoning_tokens ?? '—' }} · {{ t('admin.accounts.inflation.rate') }}={{ formatRate(result.measurement) }}
            </p>
            <p v-if="result.measurement?.reasoning_text_tokens !== undefined" class="text-purple-300">
              {{ t('admin.accounts.inflation.reasoningCount') }}={{ result.measurement.reasoning_text_tokens }}
            </p>
            <p :class="result.status === 'error' ? 'text-red-400' : 'text-gray-400'">{{ result.message }}</p>
          </section>
          <p v-if="summary" class="text-cyan-300">{{ summary }}</p>
        </div>
      </section>
    </div>
    <template #footer>
      <button class="btn btn-secondary" @click="close">{{ t('common.close') }}</button>
      <button v-if="running" class="btn btn-danger" @click="stop">{{ t('admin.accounts.inflation.stop') }}</button>
      <button v-else class="btn btn-primary" :disabled="!canRun" @click="run">{{ t('admin.accounts.inflation.start') }}</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import { list as listAccounts } from '@/api/admin/accounts'
import type { AccountListItem, AdminGroup } from '@/types'
import { CONCRETE_PLATFORM_OPTIONS } from '@/constants/platforms'
import { useTableSelection } from '@/composables/useTableSelection'
import { fetchAllAccountIds } from '@/utils/accountSelection'
import { extractApiErrorMessage } from '@/utils/apiError'
import { inflationRate, probeAccountInflation, probeDirectInflation, supportsAccountInflation, type DirectInflationTarget, type InflationEvent, type InflationMeasurement } from '@/api/admin/accountInflation'

const props = defineProps<{ show: boolean; groups: Pick<AdminGroup, 'id' | 'name' | 'platform'>[] }>()
const emit = defineEmits<{ (e: 'close'): void }>()
const { t } = useI18n()
const sourceMode = ref<'accounts' | 'manual'>('accounts')
let directTargetId = 0
const directTargets = ref<(DirectInflationTarget & { id: number })[]>([])
function addDirectTarget() { directTargets.value.push({ id: ++directTargetId, base_url: '', api_key: '', protocol: 'anthropic' }) }
addDirectTarget()
const timeoutSeconds = ref(120)
const platform = ref('')
const group = ref('')
const platformOptions = computed(() => [{ value: '', label: t('admin.accounts.allPlatforms') }, ...CONCRETE_PLATFORM_OPTIONS])
const groupOptions = computed(() => [
  { value: '', label: t('admin.accounts.allGroups') },
  { value: 'ungrouped', label: t('admin.accounts.ungroupedGroup') },
  ...props.groups.map(g => ({ value: String(g.id), label: g.name }))
])
const accounts = ref<AccountListItem[]>([])
const { selectedIds, allVisibleSelected, isSelected, toggle, toggleVisible, clear } = useTableSelection({ rows: accounts, getId: account => account.id })
const loadingAccounts = ref(false)
const loaded = ref(false)
const loadError = ref('')
let loadController: AbortController | undefined

function clearAccounts() {
  loadController?.abort()
  loadController = undefined
  loadingAccounts.value = false
  loaded.value = false
  loadError.value = ''
  accounts.value = []
  clear()
}
watch([platform, group], clearAccounts)

async function loadAccounts() {
  if (running.value || loadingAccounts.value) return
  clearAccounts()
  const request = new AbortController()
  loadController = request
  loadingAccounts.value = true
  const rows = new Map<number, AccountListItem>()
  try {
    await fetchAllAccountIds(async (page, pageSize, filters) => {
      const result = await listAccounts(page, pageSize, filters, { signal: request.signal })
      result.items.forEach(account => rows.set(account.id, account))
      return result
    }, { platform: platform.value, group: group.value, sort_by: 'name', sort_order: 'asc' })
    if (request.signal.aborted) return
    accounts.value = [...rows.values()]
    loaded.value = true
  } catch (error) {
    if (!request.signal.aborted) loadError.value = extractApiErrorMessage(error, t('admin.accounts.inflation.loadFailed'))
  } finally {
    if (loadController === request) {
      loadingAccounts.value = false
      loadController = undefined
    }
  }
}
const targetCount = computed(() => sourceMode.value === 'manual' ? directTargets.value.length : selectedIds.value.length)
const canRun = computed(() => {
  if (running.value || !prompt.value.trim() || !Number.isInteger(timeoutSeconds.value) || timeoutSeconds.value < 1 || timeoutSeconds.value > 3600) return false
  if (sourceMode.value === 'accounts') return !loadingAccounts.value && selectedIds.value.length > 0
  return !!model.value.trim() && directTargets.value.length > 0 && directTargets.value.every(target => {
    try {
      const url = new URL(target.base_url.trim())
      return ['http:', 'https:'].includes(url.protocol) && !url.username && !url.password && !url.search && !url.hash && !!target.api_key.trim() && !/[\r\n]/.test(target.api_key)
    } catch { return false }
  })
})
const model = ref('')
const prompt = ref(t('admin.accounts.inflation.defaultPrompt'))
const encoding = ref('o200k_base')
const running = ref(false)
const terminal = ref<HTMLElement | null>(null)
const summary = ref('')
interface ProbeResult {
  id: number | string
  name: string
  model: string
  text: string
  reasoning: string
  redacted: boolean
  status: 'running' | 'done' | 'error'
  message: string
  measurement?: InflationMeasurement
}
const results = ref<ProbeResult[]>([])
let controller: AbortController | undefined
let stopped = false

const formatRate = (value: InflationMeasurement) => {
  const rate = inflationRate(value)
  return rate === null ? t('admin.accounts.inflation.unavailable') : `${rate.toFixed(2)}%`
}
const scroll = () => { void nextTick(() => { if (terminal.value) terminal.value.scrollTop = terminal.value.scrollHeight }) }
const stop = () => { stopped = true; controller?.abort() }
const clearKeys = () => { directTargets.value.forEach(target => { target.api_key = '' }) }
const close = () => { stop(); clearAccounts(); clearKeys(); emit('close') }
onBeforeUnmount(() => { stop(); clearAccounts(); clearKeys() })

async function run() {
  if (!canRun.value) return
  const manual = sourceMode.value === 'manual'
  const targets: { id: number | string; name: string; direct?: DirectInflationTarget }[] = manual
    ? directTargets.value.map((target, index) => ({ id: `API ${index + 1}`, name: target.base_url.trim(), direct: { base_url: target.base_url.trim(), api_key: target.api_key.trim(), protocol: target.protocol } }))
    : selectedIds.value.map(id => ({ id, name: accounts.value.find(a => a.id === id)?.name || `#${id}` }))
  const options = { model: model.value.trim(), prompt: prompt.value, encoding: encoding.value, timeoutSeconds: timeoutSeconds.value }
  stopped = false
  running.value = true
  results.value = []
  summary.value = ''
  try {
    if (!await supportsAccountInflation(manual)) {
      summary.value = t('admin.accounts.inflation.backendRequired')
      return
    }
    for (const target of targets) {
      if (stopped) break
      results.value.push({ id: target.id, name: target.name, model: '', text: '', reasoning: '', redacted: false, status: 'running', message: t('admin.accounts.inflation.testing') })
      const result = results.value[results.value.length - 1]!
      controller = new AbortController()
      let timedOut = false
      const timeout = setTimeout(() => { timedOut = true; controller?.abort() }, options.timeoutSeconds * 1000)
      let complete = false
      try {
        const onEvent = (event: InflationEvent) => {
          if (event.type === 'test_start') result.model = event.model || ''
          if (event.type === 'content' && event.text) result.text = (result.text + event.text).slice(0, 1 << 20)
          if (event.type === 'reasoning' && event.text) result.reasoning = (result.reasoning + event.text).slice(0, 1 << 20)
          if (event.type === 'reasoning_redacted') result.redacted = true
          if (event.type === 'error') throw new Error(event.error || t('common.unknownError'))
          if (event.type === 'inflation_result') result.measurement = event.data
          if (event.type === 'test_complete') complete = event.success === true
          scroll()
        }
        if (target.direct) await probeDirectInflation(target.direct, options, onEvent, controller.signal)
        else await probeAccountInflation(Number(target.id), options, onEvent, controller.signal)
        if (!complete) throw new Error(t('admin.accounts.inflation.incomplete'))
        if (!result.measurement) throw new Error(t('admin.accounts.inflation.backendRequired'))
        if (result.measurement.error) throw new Error(result.measurement.error)
        result.status = 'done'
        result.message = inflationRate(result.measurement) === null ? t('admin.accounts.inflation.missingUsage') : t('admin.accounts.inflation.done')
      } catch (error) {
        result.status = 'error'
        result.message = stopped ? t('admin.accounts.inflation.stopped') : timedOut ? t('admin.accounts.inflation.timeout', { seconds: options.timeoutSeconds }) : error instanceof Error ? error.message : t('common.unknownError')
      } finally {
        clearTimeout(timeout)
        controller = undefined
      }
      scroll()
      // Older backends ignore measure_tokens; stop before paying for more unusable probes.
      if (complete && !result.measurement) break
    }
    summary.value = t('admin.accounts.inflation.summary', {
      tested: results.value.length,
      total: targets.length,
      measured: results.value.filter(r => r.status === 'done' && r.measurement && inflationRate(r.measurement) !== null).length
    })
  } finally {
    running.value = false
    scroll()
  }
}
</script>
