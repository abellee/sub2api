<script setup lang="ts">
/** 任务管理面板：任务 CRUD、码池维护、发放记录与重试。 */
import { computed, onMounted, ref, watch } from 'vue'
import type { LotteryClient } from '@/api/lotteryClient'
import { adminSearchUsers, formatValue, type SearchedUser } from '@/api/lottery'
import type { ConditionDef } from '@/api/lottery'
import {
  adminCreateTask,
  adminDeleteTask,
  adminGetTaskSettings,
  adminListTaskCodes,
  adminListTaskRewards,
  adminListTasks,
  adminRetryTaskFulfillment,
  adminSaveTaskSettings,
  adminSetTaskCodes,
  adminTaskGroupModels,
  adminTaskGroups,
  adminUpdateTask,
  type AdminTaskView,
  type IntegrationGroup,
  type TaskInput,
  type TaskReward
} from '@/api/task'
import type { VisibilitySettings } from '@/api/lottery'
import LotteryConditionsEditor from '@/components/lottery/LotteryConditionsEditor.vue'
import LotterySelect from '@/components/lottery/LotterySelect.vue'
import FieldHint from '@/components/lottery/FieldHint.vue'
import ImageUpload from '@/components/common/ImageUpload.vue'

const props = defineProps<{ client: LotteryClient }>()

type RepeatPolicy = 'unlimited' | 'join_once' | 'achieved_once'

interface TaskForm {
  name: string
  cover: string
  description: string
  /** '0' = 全部分组；其余为分组 ID 字符串（LotterySelect 仅支持字符串值） */
  groupId: string
  model: string
  startDate: string
  durationDays: number
  settleTime: string
  thresholdTokensM: number
  rewardType: 'balance' | 'redeem_code'
  rewardValue: number
  repeatPolicy: RepeatPolicy
  whitelist: string[]
  blacklist: string[]
  conditions: ConditionDef[]
}

const todayStr = () => {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function emptyForm(): TaskForm {
  return {
    name: '',
    cover: '',
    description: '',
    groupId: '0',
    model: '',
    startDate: todayStr(),
    durationDays: 7,
    settleTime: '00:10',
    thresholdTokensM: 100,
    rewardType: 'balance',
    rewardValue: 1,
    repeatPolicy: 'unlimited',
    whitelist: [],
    blacklist: [],
    conditions: []
  }
}

const tasks = ref<AdminTaskView[]>([])
const groups = ref<IntegrationGroup[]>([])
const groupModels = ref<string[]>([])
const groupsLoading = ref(false)
const groupsError = ref('')
const loading = ref(false)
const error = ref('')
const notice = ref('')

const dialogOpen = ref(false)
const dialogError = ref('')
const saving = ref(false)
const editingId = ref<number | null>(null)
const form = ref<TaskForm>(emptyForm())
const savingCodes = ref(false)

const codesOpen = ref(false)
const codesTask = ref<AdminTaskView | null>(null)
const codesText = ref('')

const rewardsOpen = ref(false)
const rewardsTask = ref<AdminTaskView | null>(null)
const rewards = ref<TaskReward[]>([])
const rewardsLoading = ref(false)

const confirmDialog = ref<{ title: string; message: string; confirmText: string; action: () => void } | null>(null)

// 显隐设置弹窗
const settingsOpen = ref(false)
const settingsLoading = ref(false)
const settingsSaving = ref(false)
const visibilityForm = ref<VisibilitySettings>({ mode: 'partial', allowed_emails: [] })

// 弹窗打开时锁定背景滚动
watch(dialogOpen, (open) => { document.body.style.overflow = open ? 'hidden' : '' })
watch(codesOpen, (open) => { document.body.style.overflow = open ? 'hidden' : '' })
watch(rewardsOpen, (open) => { document.body.style.overflow = open ? 'hidden' : '' })
watch(settingsOpen, (open) => { document.body.style.overflow = open ? 'hidden' : '' })

// 白名单/黑名单各自的搜索状态（互不同步）
interface ListSearchState {
  query: string
  results: SearchedUser[]
  searching: boolean
  open: boolean
}
const listSearch = ref<Record<'whitelist' | 'blacklist', ListSearchState>>({
  whitelist: { query: '', results: [], searching: false, open: false },
  blacklist: { query: '', results: [], searching: false, open: false }
})
let searchTimer: ReturnType<typeof setTimeout> | null = null

const statCards = computed(() => [
  { label: '任务总数', value: String(tasks.value.length), icon: '📋' },
  { label: '进行中', value: String(tasks.value.filter((t) => t.status === 'active').length), icon: '▶️' },
  { label: '已结束', value: String(tasks.value.filter((t) => t.status === 'ended').length), icon: '🏁' },
  { label: '兑换码池余量', value: String(tasks.value.reduce((sum, t) => sum + (t.reward_type === 'redeem_code' ? t.codes_available : 0), 0)), icon: '🎫' }
])

const formTitle = computed(() => (editingId.value != null ? '编辑任务' : '新建任务'))
const canSubmit = computed(() => form.value.name.trim() !== '' && form.value.thresholdTokensM > 0 && form.value.rewardValue > 0 && form.value.durationDays > 0)

function fmtDate(s: string): string {
  return s ? s.slice(0, 10).replace(/-/g, '/') : '—'
}

/** 任务周期展示：起 ~ 止（剩余 N 天） */
function periodText(t: AdminTaskView): string {
  const end = endDate(t)
  const daysLeft = Math.max(0, Math.round((new Date(end).getTime() - new Date(todayStr()).getTime()) / 86400000))
  return `${fmtDate(t.start_date)} ~ ${fmtDate(end)}（剩 ${daysLeft} 天）`
}

function endDate(t: { start_date: string; duration_days: number }): string {
  const d = new Date(t.start_date + 'T00:00:00')
  d.setDate(d.getDate() + t.duration_days - 1)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

const repeatPolicyLabel: Record<RepeatPolicy, string> = {
  unlimited: '不限',
  join_once: '限参与一次',
  achieved_once: '达成后不能再参与'
}

const rewardTypeLabel: Record<string, string> = {
  balance: '余额',
  redeem_code: '兑换码'
}

async function refresh() {
  loading.value = true
  error.value = ''
  try {
    tasks.value = await adminListTasks(props.client)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

async function loadGroups() {
  groupsLoading.value = true
  groupsError.value = ''
  try {
    groups.value = await adminTaskGroups(props.client)
  } catch (e) {
    groups.value = []
    groupsError.value = '分组列表加载失败：' + (e instanceof Error ? e.message : String(e))
  } finally {
    groupsLoading.value = false
  }
}

async function onGroupChange() {
  form.value.model = ''
  groupModels.value = []
  const gid = Number(form.value.groupId)
  if (!gid) return
  try {
    groupModels.value = await adminTaskGroupModels(props.client, gid)
  } catch {
    groupModels.value = []
  }
}

function openCreate() {
  editingId.value = null
  form.value = emptyForm()
  groupModels.value = []
  listSearch.value.whitelist = { query: '', results: [], searching: false, open: false }
  listSearch.value.blacklist = { query: '', results: [], searching: false, open: false }
  dialogError.value = ''
  dialogOpen.value = true
  void loadGroups()
}

function editTask(t: AdminTaskView) {
  editingId.value = t.id
  form.value = {
    name: t.name,
    cover: t.cover ?? '',
    description: t.description ?? '',
    groupId: t.group_id > 0 ? String(t.group_id) : '0',
    model: t.model ?? '',
    startDate: t.start_date,
    durationDays: t.duration_days,
    settleTime: t.settle_time,
    thresholdTokensM: t.threshold_tokens / 1e6,
    rewardType: t.reward_type as 'balance' | 'redeem_code',
    rewardValue: t.reward_value,
    repeatPolicy: (t.repeat_policy || 'unlimited') as RepeatPolicy,
    whitelist: [...(t.whitelist ?? [])],
    blacklist: [...(t.blacklist ?? [])],
    conditions: (t.conditions ?? []).map((c) => ({ ...c, bonus_mode: 'none' as const }))
  }
  listSearch.value.whitelist = { query: '', results: [], searching: false, open: false }
  listSearch.value.blacklist = { query: '', results: [], searching: false, open: false }
  dialogError.value = ''
  dialogOpen.value = true
  void loadGroups().then(() => {
    if (Number(form.value.groupId) > 0) void onGroupChange()
  })
}

function validateForm(): string {
  if (!form.value.name.trim()) return '请填写任务名称'
  if (form.value.durationDays < 1) return '持续周期至少 1 天'
  if (!/^\d{2}:\d{2}$/.test(form.value.settleTime)) return '结算时间格式应为 HH:MM'
  if (form.value.thresholdTokensM <= 0) return '达成条件（每结算日消耗量）须大于 0'
  if (form.value.rewardValue <= 0) return '任务奖励值须大于 0'
  return ''
}

function buildInput(): TaskInput {
  const gid = Number(form.value.groupId)
  return {
    name: form.value.name.trim(),
    cover: form.value.cover,
    description: form.value.description.trim(),
    group_id: gid,
    group_name: groups.value.find((g) => g.id === gid)?.name ?? '',
    model: gid === 0 ? '' : form.value.model,
    start_date: form.value.startDate,
    duration_days: form.value.durationDays,
    settle_time: form.value.settleTime,
    threshold_tokens: form.value.thresholdTokensM * 1e6,
    reward_type: form.value.rewardType,
    reward_value: form.value.rewardValue,
    repeat_policy: form.value.repeatPolicy,
    whitelist: form.value.whitelist,
    blacklist: form.value.blacklist,
    conditions: form.value.conditions,
    status: editingId.value != null ? 'active' : undefined
  }
}

async function submit() {
  const invalid = validateForm()
  if (invalid) {
    dialogError.value = invalid
    return
  }
  saving.value = true
  dialogError.value = ''
  try {
    if (editingId.value != null) {
      await adminUpdateTask(props.client, editingId.value, buildInput())
      notice.value = '任务已更新'
    } else {
      await adminCreateTask(props.client, buildInput())
      notice.value = '任务已创建，符合条件的用户将自动参与'
    }
    dialogOpen.value = false
    await refresh()
  } catch (e) {
    dialogError.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

function askConfirm(title: string, message: string, confirmText: string, action: () => void) {
  confirmDialog.value = { title, message, confirmText, action }
}

function runConfirm() {
  const action = confirmDialog.value?.action
  confirmDialog.value = null
  action?.()
}

function deleteTask(t: AdminTaskView) {
  askConfirm('删除任务', `确定删除「${t.name}」？其码池与全部结算记录将一并删除，不可恢复。`, '删除', async () => {
    try {
      await adminDeleteTask(props.client, t.id)
      notice.value = '任务已删除'
      await refresh()
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    }
  })
}

async function openCodes(t: AdminTaskView) {
  codesTask.value = t
  try {
    const detail = await adminListTaskCodes(props.client, t.id)
    codesText.value = detail.codes.filter((c) => c.status === 'available').map((c) => c.code).join('\n')
  } catch {
    codesText.value = ''
  }
  codesOpen.value = true
}

async function saveCodes() {
  if (!codesTask.value) return
  savingCodes.value = true
  try {
    const codes = codesText.value.split('\n').map((s) => s.trim()).filter(Boolean)
    const r = await adminSetTaskCodes(props.client, codesTask.value.id, codes)
    notice.value = `码池已更新：可用 ${r.available} 张，已发放 ${r.granted} 张`
    codesOpen.value = false
    await refresh()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    savingCodes.value = false
  }
}

async function openRewards(t: AdminTaskView) {
  rewardsTask.value = t
  rewardsLoading.value = true
  rewardsOpen.value = true
  try {
    rewards.value = await adminListTaskRewards(props.client, t.id)
  } catch (e) {
    rewards.value = []
  } finally {
    rewardsLoading.value = false
  }
}

async function retryFulfill() {
  if (!rewardsTask.value) return
  try {
    await adminRetryTaskFulfillment(props.client, rewardsTask.value.id)
    notice.value = '已重试发放'
    await openRewards(rewardsTask.value)
    await refresh()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

function fulfillmentLabel(f: string): string {
  return f === 'done' ? '已发放' : f === 'failed' ? '失败' : '待发放'
}

function fulfillmentClass(f: string): string {
  return f === 'done'
    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
    : f === 'failed'
      ? 'bg-red-100 text-red-600 dark:bg-red-500/15 dark:text-red-300'
      : 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
}

async function openSettings() {
  settingsOpen.value = true
  settingsLoading.value = true
  try {
    const s = await adminGetTaskSettings(props.client)
    visibilityForm.value = {
      mode: s.visibility?.mode ?? 'partial',
      allowed_emails: Array.isArray(s.visibility?.allowed_emails) ? s.visibility.allowed_emails : []
    }
  } catch {
    visibilityForm.value = { mode: 'partial', allowed_emails: [] }
  } finally {
    settingsLoading.value = false
  }
}

async function saveSettings() {
  settingsSaving.value = true
  try {
    await adminSaveTaskSettings(props.client, { ...visibilityForm.value })
    notice.value = '任务显隐设置已保存'
    settingsOpen.value = false
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    settingsSaving.value = false
  }
}

// 设置弹窗里的白名单搜索（独立于表单的白/黑名单搜索）
const settingsSearch = ref<{ query: string; results: SearchedUser[]; searching: boolean }>({
  query: '',
  results: [],
  searching: false
})

function onSettingsSearchInput() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(async () => {
    const q = settingsSearch.value.query.trim()
    if (!q) {
      settingsSearch.value.results = []
      return
    }
    settingsSearch.value.searching = true
    try {
      settingsSearch.value.results = await adminSearchUsers(props.client, q, 8)
    } catch {
      settingsSearch.value.results = []
    } finally {
      settingsSearch.value.searching = false
    }
  }, 300)
}

function addSettingsUser(email: string) {
  if (email && !visibilityForm.value.allowed_emails.includes(email)) {
    visibilityForm.value.allowed_emails.push(email)
  }
  settingsSearch.value.query = ''
  settingsSearch.value.results = []
}

function removeSettingsUser(email: string) {
  visibilityForm.value.allowed_emails = visibilityForm.value.allowed_emails.filter((e) => e !== email)
}

function onListSearchInput(target: 'whitelist' | 'blacklist') {
  const state = listSearch.value[target]
  state.open = true
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(async () => {
    const q = state.query.trim()
    if (!q) {
      state.results = []
      return
    }
    state.searching = true
    try {
      state.results = await adminSearchUsers(props.client, q, 8)
    } catch {
      state.results = []
    } finally {
      state.searching = false
    }
  }, 300)
}

function addUser(target: 'whitelist' | 'blacklist', email: string) {
  const list = target === 'whitelist' ? form.value.whitelist : form.value.blacklist
  if (email && !list.includes(email)) list.push(email)
  const state = listSearch.value[target]
  state.query = ''
  state.results = []
  state.open = false
}

function removeUser(target: 'whitelist' | 'blacklist', email: string) {
  if (target === 'whitelist') {
    form.value.whitelist = form.value.whitelist.filter((e) => e !== email)
  } else {
    form.value.blacklist = form.value.blacklist.filter((e) => e !== email)
  }
}

onMounted(refresh)
</script>

<template>
  <div class="mx-auto w-full max-w-4xl space-y-5">
    <!-- 页头 -->
    <div class="flex items-center gap-3">
      <span class="flex h-11 w-11 items-center justify-center rounded-2xl bg-gradient-primary text-xl shadow-md shadow-primary-500/25">
        📋
      </span>
      <div class="min-w-0 flex-1">
        <h1 class="text-lg font-bold text-gray-900 dark:text-white">任务管理</h1>
        <p class="text-xs text-gray-500 dark:text-dark-400">持续型消耗任务：按结算日自动达成并发放奖励</p>
      </div>
      <button class="btn btn-secondary" @click="openSettings">⚙️ 设置</button>
      <button class="btn btn-primary" @click="openCreate">新建任务</button>
    </div>

    <!-- 统计卡片 -->
    <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
      <div v-for="s in statCards" :key="s.label" class="card p-4">
        <div class="flex items-start justify-between">
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ s.label }}</span>
          <span class="text-base leading-none">{{ s.icon }}</span>
        </div>
        <p class="mt-2 text-2xl font-bold leading-none text-gray-900 dark:text-white">{{ s.value }}</p>
      </div>
    </div>

    <p v-if="notice" class="rounded-xl bg-primary-50 px-4 py-3 text-sm text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
      {{ notice }}
    </p>
    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400">
      {{ error }}
    </p>
    <p v-if="loading" class="py-8 text-center text-sm text-gray-400">加载中…</p>

    <!-- 任务列表 -->
    <section v-for="t in tasks" :key="t.id" class="card">
      <div class="card-header flex items-start justify-between gap-3">
        <div class="flex min-w-0 items-center gap-3">
          <img
            v-if="t.cover"
            :src="t.cover"
            alt=""
            class="h-12 w-12 flex-none rounded-xl object-cover ring-1 ring-gray-200 dark:ring-dark-700"
          />
          <span v-else class="flex h-12 w-12 flex-none items-center justify-center rounded-xl bg-primary-100 text-xl dark:bg-primary-500/15">📋</span>
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <h2 class="truncate text-base font-semibold text-gray-900 dark:text-white">{{ t.name }}</h2>
              <span
                :class="['inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold',
                         t.status === 'active'
                           ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300'
                           : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-400']"
              >
                {{ t.status === 'active' ? '进行中' : '已结束' }}
              </span>
            </div>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
              {{ t.group_name || '全部分组' }}<template v-if="t.model"> · {{ t.model }}</template>
            </p>
            <p v-if="t.description" class="mt-1 line-clamp-2 text-xs text-gray-500 dark:text-dark-400">{{ t.description }}</p>
          </div>
        </div>
        <div class="flex flex-none gap-2">
          <button class="btn btn-secondary btn-sm" @click="openRewards(t)">发放记录</button>
          <button v-if="t.reward_type === 'redeem_code'" class="btn btn-secondary btn-sm" @click="openCodes(t)">码池</button>
          <button class="btn btn-secondary btn-sm" @click="editTask(t)">编辑</button>
          <button class="btn btn-danger btn-sm" @click="deleteTask(t)">删除</button>
        </div>
      </div>
      <div class="card-body">
        <dl class="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div>
            <dt class="text-xs text-gray-400 dark:text-dark-500">任务周期</dt>
            <dd class="mt-0.5 text-sm font-medium text-gray-900 dark:text-gray-100">{{ periodText(t) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-gray-400 dark:text-dark-500">结算时间</dt>
            <dd class="mt-0.5 text-sm font-medium text-gray-900 dark:text-gray-100">每日 {{ t.settle_time }}（结算前一天）</dd>
          </div>
          <div>
            <dt class="text-xs text-gray-400 dark:text-dark-500">达成条件</dt>
            <dd class="mt-0.5 text-sm font-medium text-gray-900 dark:text-gray-100">每结算日 {{ t.threshold_tokens / 1e6 }}M</dd>
          </div>
          <div>
            <dt class="text-xs text-gray-400 dark:text-dark-500">任务奖励</dt>
            <dd class="mt-0.5 text-sm font-medium text-gray-900 dark:text-gray-100">
              每 {{ t.threshold_tokens / 1e6 }}M → {{ rewardTypeLabel[t.reward_type] }} {{ formatValue(t.reward_value) }}
              <template v-if="t.reward_type === 'redeem_code'">（池余 {{ t.codes_available }}）</template>
            </dd>
          </div>
        </dl>
        <p class="text-xs text-gray-400 dark:text-dark-500">重复参与策略：{{ repeatPolicyLabel[t.repeat_policy as RepeatPolicy] ?? t.repeat_policy }}</p>
      </div>
    </section>

    <!-- 空状态 -->
    <section v-if="!loading && !tasks.length" class="card">
      <div class="flex flex-col items-center gap-2 px-6 py-12 text-center">
        <span class="flex h-14 w-14 items-center justify-center rounded-full bg-primary-100 text-2xl dark:bg-primary-500/15">📋</span>
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">还没有任务</h3>
        <p class="text-sm text-gray-500 dark:text-dark-400">点击右上角「新建任务」创建第一个消耗任务</p>
      </div>
    </section>

    <!-- 新建/编辑模态框 -->
    <Teleport to="body">
      <div
        v-if="dialogOpen"
        class="fixed inset-0 z-[1900] flex items-start justify-center overflow-y-auto bg-gray-900/60 px-4 py-8 backdrop-blur-sm"
      >
        <div class="w-full max-w-3xl rounded-2xl bg-white shadow-glass dark:bg-dark-800">
          <div class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ formTitle }}</h2>
            <button class="btn btn-ghost btn-sm" @click="dialogOpen = false">✕</button>
          </div>
          <div class="max-h-[70vh] space-y-5 overflow-y-auto px-6 py-5">
            <p v-if="dialogError" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400">
              {{ dialogError }}
            </p>
            <div class="grid gap-4 sm:grid-cols-2">
              <label>
                <span class="input-label">任务名称</span>
                <input v-model="form.name" class="input" placeholder="如：Grok-Heavy 七日计划" />
              </label>
              <label>
                <span class="input-label">封面</span>
                <ImageUpload v-model="form.cover" upload-label="上传封面" remove-label="移除封面" hint="建议方形图，≤300KB" />
              </label>
              <label class="sm:col-span-2">
                <span class="input-label">任务说明（展示给用户，支持换行）</span>
                <textarea v-model="form.description" class="input" rows="3" placeholder="描述任务玩法、奖励说明、注意事项等"></textarea>
              </label>
              <label>
                <FieldHint label="分组" hint="任务消耗量统计的分组范围；「全部分组」= 不限分组与模型。" />
                <LotterySelect
                  v-model="form.groupId"
                  :options="[{ value: '0', label: '全部分组' }, ...groups.map((g) => ({ value: String(g.id), label: g.name }))]"
                  :placeholder="groupsLoading ? '分组加载中…' : '请选择'"
                  @update:model-value="onGroupChange"
                />
                <p v-if="groupsError" class="mt-1 text-xs text-red-500">{{ groupsError }}</p>
              </label>
              <label>
                <FieldHint label="模型" hint="选定分组后可指定该分组内的模型；「全部分组」时不限模型。" />
                <LotterySelect
                  v-if="Number(form.groupId) > 0"
                  v-model="form.model"
                  :options="[{ value: '', label: '不限模型' }, ...groupModels.map((m) => ({ value: m, label: m }))]"
                />
                <input v-else class="input" value="不限模型" disabled />
              </label>
              <label>
                <span class="input-label">起始日期（第 1 天）</span>
                <input v-model="form.startDate" type="date" class="input" />
              </label>
              <label>
                <span class="input-label">持续周期（天，到期自动「已结束」）</span>
                <input v-model.number="form.durationDays" type="number" min="1" class="input" />
              </label>
              <label>
                <FieldHint label="结算时间" hint="最终结算的时间点：次日凌晨该时刻结算前一天的消耗量，如 00:10。" />
                <input v-model="form.settleTime" type="time" class="input" />
              </label>
              <label>
                <FieldHint label="达成条件（每结算日消耗量，M tokens）" hint="每个结算日每消耗满该数量即达成一个单位，奖励依次叠加。" />
                <input v-model.number="form.thresholdTokensM" type="number" min="0" class="input" placeholder="如 100" />
              </label>
              <label>
                <span class="input-label">任务奖励类型</span>
                <LotterySelect
                  v-model="form.rewardType"
                  :options="[
                    { value: 'balance', label: '余额（自动入账）' },
                    { value: 'redeem_code', label: '兑换码（码池随机发放）' }
                  ]"
                />
              </label>
              <label>
                <FieldHint label="每单位奖励值" hint="每达成一个单位（每满 X M 消耗）发放的奖励，如 1 = 1 元。" />
                <input v-model.number="form.rewardValue" type="number" min="0" step="0.01" class="input" />
              </label>
              <label class="sm:col-span-2">
                <FieldHint
                  label="重复参与策略"
                  hint="不限 = 每个结算日达标即发；限参与一次 = 任务周期内最多获得 1 次奖励；达成后不能再参与 = 获得奖励后不再参与后续结算。"
                />
                <LotterySelect
                  v-model="form.repeatPolicy"
                  :options="[
                    { value: 'unlimited', label: '不限（每日达标即发）' },
                    { value: 'join_once', label: '限参与一次（任务内最多 1 次奖励）' },
                    { value: 'achieved_once', label: '达成后不能再参与' }
                  ]"
                />
              </label>
            </div>

            <!-- 可参与条件 -->
            <div class="rounded-xl border border-gray-100 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-900/40">
              <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">🎟️ 可参与条件（全部满足）</h3>
              <LotteryConditionsEditor v-model="form.conditions" :auto-bonus-percent="0" :show-bonus="false" />

              <!-- 白名单 -->
              <div class="mt-4">
                <FieldHint label="白名单（可选）" hint="留空 = 全部用户可见可参与；填写后仅名单内用户能看到并参与该任务。" />
                <input
                  v-model="listSearch.whitelist.query"
                  class="input"
                  placeholder="搜索用户邮箱/用户名加入白名单"
                  @focus="listSearch.whitelist.open = true"
                  @input="onListSearchInput('whitelist')"
                />
                <div v-if="listSearch.whitelist.open && listSearch.whitelist.query.trim() && listSearch.whitelist.results.length" class="mt-1.5 space-y-1 rounded-xl border border-gray-100 bg-white p-2 dark:border-dark-700 dark:bg-dark-800">
                  <p v-if="listSearch.whitelist.searching" class="px-1 text-xs text-gray-400">搜索中…</p>
                  <button
                    v-for="u in listSearch.whitelist.results"
                    :key="u.id"
                    class="block w-full rounded-lg px-2 py-1 text-left text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-dark-700"
                    @click="addUser('whitelist', u.email)"
                  >
                    {{ u.email }}<span v-if="u.username" class="text-gray-400">（{{ u.username }}）</span>
                  </button>
                </div>
                <div v-if="form.whitelist.length" class="mt-2 flex flex-wrap gap-1.5">
                  <span
                    v-for="email in form.whitelist"
                    :key="email"
                    class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-dark-300"
                  >
                    {{ email }}
                    <button class="text-gray-400 hover:text-red-500" @click="removeUser('whitelist', email)">✕</button>
                  </span>
                </div>
              </div>

              <!-- 黑名单 -->
              <div class="mt-4">
                <FieldHint label="黑名单（可选）" hint="名单内用户无法看到也无法参与该任务。" />
                <input
                  v-model="listSearch.blacklist.query"
                  class="input"
                  placeholder="搜索用户邮箱/用户名加入黑名单"
                  @focus="listSearch.blacklist.open = true"
                  @input="onListSearchInput('blacklist')"
                />
                <div v-if="listSearch.blacklist.open && listSearch.blacklist.query.trim() && listSearch.blacklist.results.length" class="mt-1.5 space-y-1 rounded-xl border border-gray-100 bg-white p-2 dark:border-dark-700 dark:bg-dark-800">
                  <p v-if="listSearch.blacklist.searching" class="px-1 text-xs text-gray-400">搜索中…</p>
                  <button
                    v-for="u in listSearch.blacklist.results"
                    :key="u.id"
                    class="block w-full rounded-lg px-2 py-1 text-left text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-dark-700"
                    @click="addUser('blacklist', u.email)"
                  >
                    {{ u.email }}<span v-if="u.username" class="text-gray-400">（{{ u.username }}）</span>
                  </button>
                </div>
                <div v-if="form.blacklist.length" class="mt-2 flex flex-wrap gap-1.5">
                  <span
                    v-for="email in form.blacklist"
                    :key="email"
                    class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-dark-300"
                  >
                    {{ email }}
                    <button class="text-gray-400 hover:text-red-500" @click="removeUser('blacklist', email)">✕</button>
                  </span>
                </div>
              </div>
            </div>

            <!-- 码池提示 -->
            <p v-if="form.rewardType === 'redeem_code'" class="rounded-xl bg-amber-50 px-4 py-3 text-xs text-amber-700 dark:bg-amber-500/10 dark:text-amber-300">
              兑换码奖励从管理员录入的码池中随机抽取发放，不自动生成。保存任务后请点击列表中的「码池」录入兑换码（每行一个）。
            </p>
          </div>
          <div class="flex items-center justify-between gap-3 border-t border-gray-100 px-6 py-4 dark:border-dark-700">
            <p class="text-xs text-gray-400 dark:text-dark-500">符合参与条件的用户自动参与，无需报名</p>
            <div class="flex gap-3">
              <button class="btn btn-secondary" @click="dialogOpen = false">取消</button>
              <button class="btn btn-primary" :disabled="!canSubmit || saving" @click="submit">
                {{ saving ? '保存中…' : '保存' }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- 码池弹窗 -->
    <Teleport to="body">
      <div v-if="codesOpen" class="fixed inset-0 z-[1950] flex items-start justify-center overflow-y-auto bg-gray-900/60 px-4 py-8 backdrop-blur-sm">
        <div class="w-full max-w-lg rounded-2xl bg-white shadow-glass dark:bg-dark-800">
          <div class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">兑换码码池{{ codesTask ? ` · ${codesTask.name}` : '' }}</h2>
            <button class="btn btn-ghost btn-sm" @click="codesOpen = false">✕</button>
          </div>
          <div class="space-y-3 px-6 py-5">
            <p class="text-xs text-gray-500 dark:text-dark-400">
              每行一个兑换码；保存后整体替换「可用」码（已发放记录保留）。结算时按达成单位数随机抽取。
            </p>
            <textarea v-model="codesText" class="input font-mono" rows="8" placeholder="TASK-CODE-0001&#10;TASK-CODE-0002"></textarea>
          </div>
          <div class="flex justify-end gap-3 border-t border-gray-100 px-6 py-4 dark:border-dark-700">
            <button class="btn btn-secondary" @click="codesOpen = false">取消</button>
            <button class="btn btn-primary" :disabled="savingCodes" @click="saveCodes">{{ savingCodes ? '保存中…' : '保存码池' }}</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- 发放记录弹窗 -->
    <Teleport to="body">
      <div v-if="rewardsOpen" class="fixed inset-0 z-[1950] flex items-start justify-center overflow-y-auto bg-gray-900/60 px-4 py-8 backdrop-blur-sm">
        <div class="w-full max-w-2xl rounded-2xl bg-white shadow-glass dark:bg-dark-800">
          <div class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">发放记录{{ rewardsTask ? ` · ${rewardsTask.name}` : '' }}</h2>
            <button class="btn btn-ghost btn-sm" @click="rewardsOpen = false">✕</button>
          </div>
          <div class="max-h-[60vh] space-y-3 overflow-y-auto px-6 py-5">
            <p v-if="rewardsLoading" class="py-6 text-center text-sm text-gray-400">加载中…</p>
            <template v-else-if="rewards.length">
              <div class="flex justify-end">
                <button class="btn btn-secondary btn-sm" @click="retryFulfill">重试失败发放</button>
              </div>
              <div
                v-for="r in rewards"
                :key="r.id"
                class="rounded-xl border border-gray-100 px-4 py-3 dark:border-dark-700"
              >
                <div class="flex items-center justify-between gap-2">
                  <p class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ r.email }}</p>
                  <span :class="['rounded-full px-2.5 py-0.5 text-xs font-semibold', fulfillmentClass(r.fulfillment)]">
                    {{ fulfillmentLabel(r.fulfillment) }}
                  </span>
                </div>
                <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                  结算日 {{ r.settle_date }} · 消耗 {{ (r.tokens / 1e6).toFixed(1) }}M · 达成 {{ r.units }} 单位 ·
                  {{ rewardTypeLabel[r.reward_type] }} {{ formatValue(r.reward_value * r.units) }}
                </p>
                <p v-if="r.codes?.length" class="mt-1 break-all font-mono text-xs text-primary-600 dark:text-primary-400">{{ r.codes.join(' / ') }}</p>
                <p v-if="r.error" class="mt-1 text-xs text-red-500">{{ r.error }}</p>
              </div>
            </template>
            <p v-else class="py-6 text-center text-sm text-gray-400">还没有结算记录</p>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- 显隐设置弹窗 -->
    <Teleport to="body">
      <div v-if="settingsOpen" class="fixed inset-0 z-[1950] flex items-start justify-center overflow-y-auto bg-gray-900/60 px-4 py-8 backdrop-blur-sm">
        <div class="w-full max-w-lg rounded-2xl bg-white shadow-glass dark:bg-dark-800">
          <div class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">任务显隐设置</h2>
            <button class="btn btn-ghost btn-sm" @click="settingsOpen = false">✕</button>
          </div>
          <div class="space-y-4 px-6 py-5">
            <p v-if="settingsLoading" class="py-4 text-center text-sm text-gray-400">加载中…</p>
            <template v-else>
              <label>
                <span class="input-label">可见范围</span>
                <LotterySelect
                  v-model="visibilityForm.mode"
                  :options="[
                    { value: 'all', label: '全员可见' },
                    { value: 'partial', label: '部分人可见（白名单）' }
                  ]"
                />
                <p class="mt-1.5 text-xs text-gray-400 dark:text-dark-500">默认部分人可见；白名单为空时仅管理员可见任务中心。侧边栏菜单、任务列表与奖励结算均受此控制。</p>
              </label>
              <div v-if="visibilityForm.mode === 'partial'">
                <span class="input-label">可见用户白名单</span>
                <input
                  v-model="settingsSearch.query"
                  class="input"
                  placeholder="搜索用户邮箱/用户名加入白名单"
                  @input="onSettingsSearchInput"
                />
                <div v-if="settingsSearch.query.trim() && settingsSearch.results.length" class="mt-1.5 space-y-1 rounded-xl border border-gray-100 bg-white p-2 dark:border-dark-700 dark:bg-dark-800">
                  <p v-if="settingsSearch.searching" class="px-1 text-xs text-gray-400">搜索中…</p>
                  <button
                    v-for="u in settingsSearch.results"
                    :key="u.id"
                    class="block w-full rounded-lg px-2 py-1 text-left text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-dark-700"
                    @click="addSettingsUser(u.email)"
                  >
                    {{ u.email }}<span v-if="u.username" class="text-gray-400">（{{ u.username }}）</span>
                  </button>
                </div>
                <div v-if="visibilityForm.allowed_emails.length" class="mt-2 flex flex-wrap gap-1.5">
                  <span
                    v-for="email in visibilityForm.allowed_emails"
                    :key="email"
                    class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-dark-300"
                  >
                    {{ email }}
                    <button class="text-gray-400 hover:text-red-500" @click="removeSettingsUser(email)">✕</button>
                  </span>
                </div>
              </div>
            </template>
          </div>
          <div class="flex justify-end gap-3 border-t border-gray-100 px-6 py-4 dark:border-dark-700">
            <button class="btn btn-secondary" @click="settingsOpen = false">取消</button>
            <button class="btn btn-primary" :disabled="settingsLoading || settingsSaving" @click="saveSettings">
              {{ settingsSaving ? '保存中…' : '保存' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- 二次确认弹窗 -->
    <Teleport to="body">
      <div v-if="confirmDialog" class="fixed inset-0 z-[2000] flex items-center justify-center bg-gray-900/60 px-4 backdrop-blur-sm">
        <div class="card w-full max-w-sm p-6">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ confirmDialog.title }}</h3>
          <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ confirmDialog.message }}</p>
          <div class="mt-5 flex justify-end gap-3">
            <button class="btn btn-secondary" @click="confirmDialog = null">取消</button>
            <button class="btn btn-danger" @click="runConfirm">{{ confirmDialog.confirmText }}</button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
