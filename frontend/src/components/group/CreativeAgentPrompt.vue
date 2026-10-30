<template>
  <section id="agent" class="creative-agent" aria-labelledby="agent-title">
    <div class="agent-eyebrow">LET YOUR AGENT HANDLE IT</div>
    <h2 id="agent-title">文档太长不想看</h2>
    <p class="agent-description">复制提示词，交给 Agent 完成对接。接口、参数、任务查询与示例都已包含。</p>
    <div class="agent-prompt">
      <p>{{ instructions }}</p>
      <div class="agent-key-row">
        <label for="creative-agent-key">API Key：</label>
        <Select v-if="keys.length > 1" id="creative-agent-key" v-model="selectedKeyId" :options="keyOptions" :placeholder="`${group.name} · 请选择 Key`" aria-label="选择当前分组的 API Key" searchable>
          <template #search="{ modelValue, update, placeholder }">
            <Input :model-value="modelValue" :placeholder="placeholder" aria-label="搜索 Key 名称" @update:model-value="update" />
          </template>
        </Select>
        <span v-else class="agent-key-token">{{ selectedKey ? group.name : '<API_KEY>' }}</span>
      </div>
      <p v-if="keysLoading" class="agent-key-note" role="status">正在读取当前分组的 Key…</p>
      <p v-else-if="keysError" class="agent-key-note" role="alert">{{ keysError }} <button type="button" @click="loadKeys">重试</button></p>
      <p v-else class="agent-key-note">{{ selectedKey ? '页面显示分组名称，复制时自动填入选中的真实 Key。' : keys.length > 1 ? '请选择要使用的 Key，选项显示分组名称与 Key 名称。' : '当前分组没有可用 Key，复制后请替换 <API_KEY>。' }}</p>
      <details :key="group.id" class="agent-details">
        <summary>查看提示词中的完整接入说明 <span>{{ documentSize }} 字 · 已包含全部章节</span></summary>
        <pre>{{ reference }}</pre>
      </details>
    </div>
    <div class="agent-actions">
      <button type="button" class="agent-copy" :disabled="keysLoading || !!keysError || (keys.length > 1 && !selectedKey)" @click="copyPrompt">复制完整提示词 ↗</button>
      <span>直接发给 Agent，无需再附文档。</span>
    </div>
    <p v-if="copyStatus" class="agent-status" role="status" aria-live="polite">{{ copyStatus }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import Select from '@/components/common/Select.vue'
import Input from '@/components/common/Input.vue'
import { list } from '@/api/keys'
import type { ApiKey } from '@/types'
import type { DocumentationGroup } from '@/utils/creativeDocumentation'
import { referencePages } from '../../../public/creative-docs/reference-content.js'
import { agentInstructions, agentPrompt, documentMarkdown } from '../../../public/creative-docs/documentation.js'

const props = defineProps<{ group: DocumentationGroup }>()
const page = computed(() => referencePages.find(page => page.id === props.group.documentId)!)
const instructions = computed(() => agentInstructions(page.value, props.group.name))
const reference = computed(() => documentMarkdown(page.value))
const documentSize = computed(() => reference.value.length.toLocaleString())
const keys = ref<ApiKey[]>([])
const selectedKeyId = ref<string | number | boolean | null>(null)
const selectedKey = computed(() => keys.value.find(key => key.id === selectedKeyId.value))
const keyOptions = computed(() => keys.value.map(key => ({ value: key.id, label: `${props.group.name} · ${key.name}` })))
const keysLoading = ref(false)
const keysError = ref('')
const copyStatus = ref('')
let request: AbortController | null = null

async function loadKeys() {
  request?.abort()
  const current = new AbortController()
  request = current
  keys.value = []
  selectedKeyId.value = null
  keysError.value = ''
  copyStatus.value = ''
  keysLoading.value = false
  const groupId = Number(props.group.id)
  if (!Number.isSafeInteger(groupId) || groupId <= 0) return
  keysLoading.value = true
  try {
    const available: ApiKey[] = []
    for (let page = 1; ; page++) {
      const response = await list(page, 100, { group_id: groupId, status: 'active' }, { signal: current.signal })
      if (current.signal.aborted) return
      available.push(...response.items.filter(key => key.group_id === groupId && key.status === 'active' && key.key && (!key.expires_at || Date.parse(key.expires_at) > Date.now()) && (!key.quota || key.quota_used < key.quota)))
      if (page >= response.pages || !response.items.length) break
    }
    keys.value = available
    if (available.length === 1) selectedKeyId.value = available[0].id
  } catch {
    if (!current.signal.aborted) keysError.value = 'Key 读取失败，请重试。'
  } finally {
    if (!current.signal.aborted) keysLoading.value = false
  }
}

async function copyPrompt() {
  if (keysLoading.value || keysError.value || (keys.value.length > 1 && !selectedKey.value)) return
  const groupId = props.group.id
  const keyId = selectedKeyId.value
  try {
    await navigator.clipboard.writeText(agentPrompt(page.value, props.group.name, selectedKey.value?.key))
    if (props.group.id === groupId && selectedKeyId.value === keyId) copyStatus.value = selectedKey.value ? '已复制完整提示词与选中的 Key。' : '已复制完整提示词，请将 <API_KEY> 替换为当前分组的 Key。'
  } catch {
    if (props.group.id === groupId && selectedKeyId.value === keyId) copyStatus.value = '复制失败，请允许浏览器访问剪贴板后重试。'
  }
}

watch(() => props.group.id, loadKeys, { immediate: true })
watch(selectedKeyId, () => { copyStatus.value = '' })
onBeforeUnmount(() => { request?.abort(); keys.value = [] })
</script>

<style scoped>
.creative-agent { position: relative; padding: 30px; border: 1px solid var(--tf-console-line); border-radius: 14px; background: linear-gradient(120deg,rgb(var(--tf-console-primary-500) / .10),rgb(var(--tf-console-primary-500) / .025)); }
.creative-agent::before { content: ''; position: absolute; inset: 18px auto 18px 0; width: 3px; background: var(--tf-console-action-gradient); }
.agent-eyebrow { font-size: 12px; letter-spacing: 1.5px; opacity: .6; margin-bottom: 14px; }
h2 { margin: 0 0 12px; font-size: 28px; font-weight: 400; }
.agent-description,.agent-key-note { font-size: 12px; opacity: .65; line-height: 1.9; }
.agent-description { margin-bottom: 20px; }
.agent-prompt { padding: 20px; border: 1px solid var(--tf-console-line); border-radius: 9px; background: var(--tf-console-surface); font-size: 12px; line-height: 1.95; }
.agent-prompt > p:first-child { white-space: pre-wrap; margin: 0 0 18px; }
.agent-key-row { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; }
.agent-key-row > :last-child { min-width: min(100%,280px); max-width: 100%; }
.agent-key-token { display: inline-flex; border: 1px solid var(--tf-console-line); background: var(--tf-console-table-head); color: var(--tf-console-active-ink); padding: 5px 12px; border-radius: 6px; overflow-wrap: anywhere; }
.agent-key-note { margin: 8px 0 0; }
.agent-key-note button { text-decoration: underline; }
.agent-details { margin-top: 18px; border-top: 1px solid var(--tf-console-line); padding-top: 16px; }
summary { cursor: pointer; color: var(--tf-console-active-ink); }
summary span { color: inherit; opacity: .6; margin-left: 14px; }
pre { font: inherit; white-space: pre-wrap; overflow-wrap: anywhere; margin: 20px 0 0; }
.agent-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 18px; margin-top: 20px; font-size: 12px; }
.agent-actions > span { opacity: .6; }
.agent-copy { padding: 12px 18px; border-radius: 7px; color: white; background: var(--tf-console-action-gradient); }
.agent-copy:disabled { opacity: .5; cursor: not-allowed; }
.agent-status { margin: 14px 0 0; color: var(--tf-console-active-ink); font-size: 12px; }
@media(max-width:700px) { .creative-agent { padding: 22px 18px; } .agent-prompt { padding: 14px; } h2 { font-size: 24px; } summary span { display: block; margin: 6px 0 0; } }
</style>
