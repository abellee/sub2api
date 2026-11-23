<template>
  <BaseDialog :show="show" :title="t('admin.accounts.inflation.title')" width="wide" @close="close">
    <div class="space-y-4">
      <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('admin.accounts.inflation.hint', { count: accountIds.length }) }}</p>
      <div class="grid gap-3 sm:grid-cols-2">
        <label class="space-y-1 text-sm">
          <span>{{ t('admin.accounts.inflation.model') }}</span>
          <input v-model="model" class="input w-full" :disabled="running" :placeholder="t('admin.accounts.inflation.modelHint')" />
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
        <span>{{ t('admin.accounts.inflation.prompt') }}</span>
        <textarea v-model="prompt" class="input w-full" rows="3" :disabled="running" maxlength="8000" />
      </label>
      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.inflation.formula') }}</p>
      <div ref="terminal" role="log" aria-live="polite" :aria-label="t('admin.accounts.inflation.title')" class="max-h-[460px] min-h-[200px] space-y-4 overflow-auto rounded-xl border border-gray-700 bg-gray-950 p-4 font-mono text-xs text-gray-300">
        <p v-if="!results.length" class="text-gray-500">$ {{ t('admin.accounts.inflation.ready') }}</p>
        <section v-for="result in results" :key="result.id" class="space-y-1 border-b border-gray-800 pb-3">
          <p class="text-cyan-300">$ [{{ result.id }}] {{ result.name }} · {{ result.model || model || 'auto' }}</p>
          <p v-if="result.text" class="whitespace-pre-wrap break-words text-green-300">{{ result.text }}</p>
          <p v-if="result.measurement" class="text-yellow-200">
            tokenizer={{ result.measurement.token_count }} · {{ result.measurement.output_field || 'output_tokens' }}={{ result.measurement.output_tokens ?? '—' }} · reasoning_tokens={{ result.measurement.reasoning_tokens ?? '—' }} · {{ t('admin.accounts.inflation.rate') }}={{ formatRate(result.measurement) }}
          </p>
          <p :class="result.status === 'error' ? 'text-red-400' : 'text-gray-400'">{{ result.message }}</p>
        </section>
        <p v-if="summary" class="text-cyan-300">{{ summary }}</p>
      </div>
    </div>
    <template #footer>
      <button class="btn btn-secondary" @click="close">{{ t('common.close') }}</button>
      <button v-if="running" class="btn btn-danger" @click="stop">{{ t('admin.accounts.inflation.stop') }}</button>
      <button v-else class="btn btn-primary" :disabled="!prompt.trim() || !accountIds.length" @click="run">{{ t('admin.accounts.inflation.start') }}</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { inflationRate, probeAccountInflation, supportsAccountInflation, type InflationMeasurement } from '@/api/admin/accountInflation'

const props = defineProps<{ show: boolean; accountIds: number[]; accounts: { id: number; name: string }[] }>()
const emit = defineEmits<{ (e: 'close'): void }>()
const { t } = useI18n()
const model = ref('')
const prompt = ref(t('admin.accounts.inflation.defaultPrompt'))
const encoding = ref('o200k_base')
const running = ref(false)
const terminal = ref<HTMLElement | null>(null)
const summary = ref('')
interface ProbeResult {
  id: number
  name: string
  model: string
  text: string
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
const close = () => { stop(); emit('close') }
onBeforeUnmount(stop)

async function run() {
  if (running.value) return
  const ids = [...new Set(props.accountIds)]
  const options = { model: model.value.trim(), prompt: prompt.value, encoding: encoding.value }
  stopped = false
  running.value = true
  results.value = []
  summary.value = ''
  try {
    if (!await supportsAccountInflation()) {
      summary.value = t('admin.accounts.inflation.backendRequired')
      return
    }
    for (const id of ids) {
      if (stopped) break
      results.value.push({ id, name: props.accounts.find(a => a.id === id)?.name || `#${id}`, model: '', text: '', status: 'running', message: t('admin.accounts.inflation.testing') })
      const result = results.value[results.value.length - 1]!
      controller = new AbortController()
      let timedOut = false
      const timeout = setTimeout(() => { timedOut = true; controller?.abort() }, 120000)
      let complete = false
      try {
        await probeAccountInflation(id, options, event => {
          if (event.type === 'test_start') result.model = event.model || ''
          if (event.type === 'content' && event.text) result.text = (result.text + event.text).slice(0, 1 << 20)
          if (event.type === 'error') throw new Error(event.error || t('common.unknownError'))
          if (event.type === 'inflation_result') result.measurement = event.data
          if (event.type === 'test_complete') complete = event.success === true
          scroll()
        }, controller.signal)
        if (!complete) throw new Error(t('admin.accounts.inflation.incomplete'))
        if (!result.measurement) throw new Error(t('admin.accounts.inflation.backendRequired'))
        if (result.measurement.error) throw new Error(result.measurement.error)
        result.status = 'done'
        result.message = inflationRate(result.measurement) === null ? t('admin.accounts.inflation.missingUsage') : t('admin.accounts.inflation.done')
      } catch (error) {
        result.status = 'error'
        result.message = stopped ? t('admin.accounts.inflation.stopped') : timedOut ? t('admin.accounts.inflation.timeout') : error instanceof Error ? error.message : t('common.unknownError')
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
      total: ids.length,
      measured: results.value.filter(r => r.status === 'done' && r.measurement && inflationRate(r.measurement) !== null).length
    })
  } finally {
    running.value = false
    scroll()
  }
}
</script>
