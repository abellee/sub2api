<script setup lang="ts">
/**
 * LotteryAdminPanel —— 管理员侧抽奖配置面板（独立组件，合并单元）。
 * 页面：统计 + 当前活动概览 + 最近开奖；新增/编辑在模态框中完成。
 * 兑换码奖品支持管理员预存码池（开奖时按序发放）。
 * 样式使用 Sub2API 主站的 Tailwind 组件类，并入后零适配。
 */
import { computed, onMounted, ref, watch } from 'vue'
import {
  adminArchiveActivity,
  adminCreateActivity,
  adminDraw,
  adminGetActivity,
  adminListActivities,
  adminListParticipants,
  adminUpdateActivity,
  formatValue,
  type ActivityInput,
  type ActivityView,
  type ConditionDef,
  type ParticipantRecord,
  type PrizeDraft,
  type WinnerRecord
} from '../../api/lottery'
import type { LotteryClient } from '../../api/lotteryClient'
import PrizeIcon from './PrizeIcon.vue'
import { beijingInputToISO, formatBeijingDateTime, isoToBeijingInput } from '../../utils/beijingTime'
import {
  adminCloseActivity,
  adminCreateDailyConfig,
  adminDeleteDailyConfig,
  adminListDailyConfigs,
  adminSetDailyConfigEnabled,
  adminUpdateDailyConfig,
  totalWeightMultiplier,
  adminGetSettings,
  adminSaveSettings,
  adminSearchUsers,
  type AdminSettingsInfo,
  type DailyConfig,
  type SearchedUser,
  type VisibilitySettings
} from '../../api/lottery'
import LotterySelect from './LotterySelect.vue'
import FieldHint from './FieldHint.vue'
import LotteryConditionsEditor from './LotteryConditionsEditor.vue'
import LotteryPrizesEditor from './LotteryPrizesEditor.vue'

const props = defineProps<{ client: LotteryClient }>()

const activities = ref<ActivityView[]>([])
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const notice = ref('')
// 「所有场次」分页（每页 20）与行内中奖名单展开
const PAGE_SIZE = 20
const page = ref(1)
const expandedId = ref<number | null>(null)
const detailWinners = ref<Record<number, WinnerRecord[]>>({})
const detailLoading = ref<number | null>(null)

// 日常定时抽奖配置（横幅：启用时常显，当天场次未创建也可见）
const dailyConfigs = ref<DailyConfig[]>([])
const dailyWorkingId = ref<number | null>(null)
// 正在编辑的日常配置 ID（null = 新建）
const editingDailyId = ref<number | null>(null)
/** 正在编辑的活动所属日常配置 ID（0 = 手动创建），用于弹窗提示「仅影响这一场」。 */
const editingConfigId = ref<number>(0)
// 正在关闭的活动 id（防重复点击）
const closingId = ref<number | null>(null)
// 当前活动（进行中）的参与名单
const currentParticipants = ref<ParticipantRecord[]>([])
const showCurrentParticipants = ref(true)

// 排序：状态优先（进行中 > 未开始 > 已开奖/已归档），同状态内按开始时间倒序
function statusRank(a: ActivityView): number {
  // 已关闭/已归档固定沉底（其 phase 是时间制的，可能仍为 joining）
  if (a.status === 'closed' || a.status === 'archived') return 2
  return a.phase === 'joining' ? 0 : a.phase === 'upcoming' ? 1 : 2
}
const allActivities = computed(() =>
  [...activities.value].sort((a, b) => {
    const d = statusRank(a) - statusRank(b)
    return d !== 0 ? d : (b.starts_at || '').localeCompare(a.starts_at || '')
  })
)
const totalPages = computed(() => Math.max(1, Math.ceil(allActivities.value.length / PAGE_SIZE)))
const pagedActivities = computed(() => {
  const p = Math.min(page.value, totalPages.value)
  return allActivities.value.slice((p - 1) * PAGE_SIZE, p * PAGE_SIZE)
})
function gotoPage(n: number) {
  page.value = Math.min(Math.max(1, n), totalPages.value)
  expandedId.value = null
}


/** 展开某场次的中奖名单（懒加载，管理端完整邮箱）。 */
async function toggleWinners(a: ActivityView) {
  if (expandedId.value === a.id) {
    expandedId.value = null
    return
  }
  expandedId.value = a.id
  if (detailWinners.value[a.id]) return
  detailLoading.value = a.id
  try {
    const detail = await adminGetActivity(props.client, a.id)
    detailWinners.value = { ...detailWinners.value, [a.id]: detail.winners ?? [] }
  } catch {
    detailWinners.value = { ...detailWinners.value, [a.id]: [] }
  } finally {
    detailLoading.value = null
  }
}
const dialogOpen = ref(false)
const dialogError = ref('')
const settingsOpen = ref(false)
const settingsInfo = ref<AdminSettingsInfo | null>(null)
const visibilityForm = ref<VisibilitySettings>({ mode: 'all', allowed_emails: [] })
const settingsKeyInput = ref('')
const settingsSaving = ref(false)
const userSearchQuery = ref('')
const userSearchResults = ref<SearchedUser[]>([])
const userSearching = ref(false)
const confirmDialog = ref<{ title: string; message: string; confirmText: string; action: () => void } | null>(null)

// 弹窗打开时锁定背景滚动，避免「遮罩没盖满」的错觉
watch(dialogOpen, (open) => {
  document.body.style.overflow = open ? 'hidden' : ''
})

const conditionMatchOptions = [
  { value: 'all', label: '全部满足（AND）' },
  { value: 'any', label: '满足其一（OR）' }
]

type ConditionDraft = ConditionDef

const form = ref(emptyForm())

// 全部条件满足时的合计权重倍率（供奖池编辑器做概率预估）。
const formWeightMultiplier = computed(() =>
  totalWeightMultiplier(form.value.conditions, form.value.autoBonusPercent)
)
const editingId = ref<number | null>(null)

function emptyForm(): {
  name: string
  description: string
  startsAt: string
  drawsAt: string
  maxParticipants: number
  conditionMatch: 'all' | 'any'
  autoBonusPercent: number
  conditions: ConditionDraft[]
  prizes: PrizeDraft[]
  /** 每日定时模式：勾选后提交为 daily-config，由调度器每天自动创建活动。 */
  isDaily: boolean
  /** 每日定时模式字段：开启时刻 HH:MM 与持续时长（小时）。 */
  dailyStartTime: string
  dailyDurationHours: number
  /** 重复参与策略（仅每日定时模式）。 */
  repeatPolicy: 'unlimited' | 'join_once' | 'win_once'
} {
  return {
    name: '',
    description: '',
    startsAt: '',
    drawsAt: '',
    maxParticipants: 0,
    conditionMatch: 'all',
    autoBonusPercent: 25,
    conditions: [
      { dimension: 'token_usage', window_days: 7, mode: 'per_day', threshold: 100_000_000, bonus_mode: 'manual', bonus_percent: 50 }
    ],
    prizes: [{ name: '幸运奖', prize_type: 'balance', value: 5, weight: 0, stock: 1, codesText: '' }],
    isDaily: false,
    dailyStartTime: '08:00',
    dailyDurationHours: 12,
    repeatPolicy: 'unlimited'
  }
}

/** 拉取当前活动的参与名单。 */
async function loadCurrentParticipants() {
  if (!current.value) {
    currentParticipants.value = []
    return
  }
  try {
    currentParticipants.value = await adminListParticipants(props.client, current.value.id)
  } catch {
    currentParticipants.value = []
  }
}

/** 编辑指定日常配置：打开表单并填充（提交时走更新）。 */
function editDaily(c: DailyConfig) {
  editingId.value = null
  editingConfigId.value = 0
  form.value = emptyForm()
  form.value.name = c.name
  form.value.description = c.description
  form.value.maxParticipants = c.max_participants
  form.value.conditionMatch = c.condition_match
  form.value.autoBonusPercent = c.auto_bonus_percent
  form.value.conditions = c.conditions.map((x) => ({ ...x }))
  form.value.prizes = c.prizes.map((p) => ({ ...p, codesText: (p.codes ?? []).join(String.fromCharCode(10)) }))
  form.value.dailyStartTime = c.start_time
  form.value.dailyDurationHours = c.duration_hours
  form.value.repeatPolicy = c.repeat_policy || 'unlimited'
  form.value.isDaily = true
  editingDailyId.value = c.id
  dialogError.value = ''
  dialogOpen.value = true
}


/** 条件定义 → 人类可读摘要（日常定时抽奖横幅展示用）。 */
function conditionsSummary(conditions: ConditionDef[]): string {
  if (!conditions.length) return '无条件限制'
  return conditions
    .map((c) => {
      if (c.dimension === 'token_usage') {
        const m = c.threshold ? Math.round((c.threshold / 1e6) * 10) / 10 : 0
        return c.mode === 'total' ? `${c.window_days}天累计≥${m}M` : `每日≥${m}M`
      }
      if (c.dimension === 'activity_days') {
        return `${c.window_days}天活跃≥${c.min_active_days ?? 1}天`
      }
      const min = c.min_registered_days ?? 0
      const max = c.max_registered_days ?? 0
      if (min && max) return `注册第${min}~${max}天`
      if (max) return `注册≤第${max}天`
      if (min) return `注册≥第${min}天`
      return '注册时长不限'
    })
    .join(' 且 ')
}

function parseCodes(prize: PrizeDraft): string[] {
  return prize.codesText
    .split('\n')
    .map((s) => s.trim())
    .filter(Boolean)
}

function codesCount(prize: PrizeDraft): number {
  return parseCodes(prize).length
}

/** 当前活动：参与中/未开始的那一场。 */
const current = computed<ActivityView | null>(() => {
  // 仅 status=active 的场次可成为当前活动（归档/关闭的即使时间上进行中也不算）
  const active = activities.value.filter(
    (a) => a.status === 'active' && (a.phase === 'joining' || a.phase === 'upcoming')
  )
  return (
    active.sort((a, b) => a.draws_at.localeCompare(b.draws_at))[0] ?? null
  )
})

watch(
  () => current.value?.id ?? null,
  () => {
    currentParticipants.value = []
    void loadCurrentParticipants()
  },
  { immediate: true }
)


/** 最近一场已开奖活动。 */
const formTitle = computed(() => {
  if (editingId.value != null) return '编辑当前活动'
  if (editingDailyId.value != null) return '编辑日常定时抽奖'
  return '创建抽奖活动'
})
const canSubmit = computed(() =>
  form.value.prizes.length > 0 && form.value.name && (
    form.value.isDaily
      ? Boolean(form.value.dailyStartTime && form.value.dailyDurationHours > 0)
      : Boolean(form.value.startsAt && form.value.drawsAt)
  )
)

function statusBadge(a: ActivityView): { text: string; cls: string } {
  if (a.status === 'closed') {
    return { text: '已关闭', cls: 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-400' }
  }
  if (a.status === 'archived') {
    return { text: '已归档', cls: 'bg-gray-100 text-gray-400 dark:bg-dark-700 dark:text-dark-500' }
  }
  return { text: phaseText(a), cls: phaseBadgeClass(a) }
}

function phaseText(a: ActivityView): string {
  if (a.phase === 'joining') return '进行中'
  if (a.phase === 'upcoming') return '未开始'
  if (a.phase === 'fulfilled') return '已开奖'
  return '已开奖'
}

function phaseBadgeClass(a: ActivityView): string {
  if (a.phase === 'joining') return 'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300'
  if (a.phase === 'upcoming') return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300'
  return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
}

/** 打开任意活动的编辑弹窗（仅进行中/未开始可编辑）。 */
function openEdit(a: ActivityView) {
  fillForm(a)
  dialogOpen.value = true
}

/** 页头统计卡片。 */
const statCards = computed(() => [
  { label: '活动总数', value: activities.value.length, icon: '📋', hint: '' },
  {
    label: '进行中',
    value: activities.value.filter((a) => a.phase === 'joining').length,
    icon: '🎯',
    hint: current.value ? `下一场开奖 ${fmtDate(current.value.draws_at)}` : ''
  },
  {
    label: '累计参与人次',
    value: activities.value.reduce((s, a) => s + a.participant_count, 0),
    icon: '👥',
    hint: ''
  },
  {
    label: '已发放奖品',
    value: activities.value.reduce((s, a) => s + a.prizes.reduce((x, p) => x + p.granted_count, 0), 0),
    icon: '🎁',
    hint: ''
  }
])

function toLocalInput(iso: string): string {
  return isoToBeijingInput(iso)
}

function fillForm(a: ActivityView) {
  editingId.value = a.id
  editingDailyId.value = null
  editingConfigId.value = a.daily_config_id ?? 0
  form.value = {
    name: a.name,
    description: a.description,
    startsAt: toLocalInput(a.starts_at),
    drawsAt: toLocalInput(a.draws_at),
    maxParticipants: a.max_participants,
    conditionMatch: a.condition_match,
    autoBonusPercent: 25,
    repeatPolicy: 'unlimited',
    isDaily: false, // 编辑已有活动：始终普通模式
    dailyStartTime: '08:00',
    dailyDurationHours: 12,
    conditions: [],
    prizes: a.prizes.map((p) => ({
      name: p.name,
      prize_type: p.prize_type,
      value: p.value,
      weight: p.weight,
      stock: p.stock,
      codesText: ''
    }))
  }
  // 条件定义与兑换码码池需要从详情接口取（列表接口不含）。
  adminGetActivity(props.client, a.id).then((detail) => {
    form.value.conditions = detail.activity.conditions.map((c) => ({ ...c }))
    form.value.autoBonusPercent = detail.activity.auto_bonus_percent
    form.value.prizes = detail.activity.prizes.map((p) => ({
      name: p.name,
      prize_type: p.prize_type,
      value: p.value,
      weight: p.weight,
      stock: p.stock,
      codesText: (p.codes ?? []).join('\n')
    }))
  }).catch(() => undefined)
}

async function refresh() {
  loading.value = true
  error.value = ''
  try {
    activities.value = await adminListActivities(props.client)
    adminListDailyConfigs(props.client)
      .then((list) => { dailyConfigs.value = list })
      .catch(() => undefined)
    void loadCurrentParticipants()
    if (current.value) {
      fillForm(current.value)
    } else if (editingId.value != null) {
      editingId.value = null
      form.value = emptyForm()
    }
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

async function openSettings() {
  error.value = ''
  try {
    const s = await adminGetSettings(props.client)
    settingsInfo.value = s
    // 后端可能返回 allowed_emails: null（Go nil 切片），归一化为数组避免模板读 .length 抛错
    visibilityForm.value = {
      mode: s.visibility.mode ?? 'all',
      allowed_emails: s.visibility.allowed_emails ?? []
    }
    settingsKeyInput.value = ''
    settingsOpen.value = true
  } catch (e) {
    error.value = (e as Error).message
  }
}

let userSearchTimer: ReturnType<typeof setTimeout> | null = null

function onUserSearchInput() {
  if (userSearchTimer) clearTimeout(userSearchTimer)
  userSearchTimer = setTimeout(async () => {
    const q = userSearchQuery.value.trim()
    if (!q) {
      userSearchResults.value = []
      return
    }
    userSearching.value = true
    try {
      userSearchResults.value = await adminSearchUsers(props.client, q, 20)
    } catch {
      userSearchResults.value = []
    } finally {
      userSearching.value = false
    }
  }, 300)
}

function addUserToWhitelist(u: SearchedUser) {
  const email = u.email.trim()
  if (!email) return
  const list = visibilityForm.value.allowed_emails ?? []
  if (!list.some((e) => e.toLowerCase() === email.toLowerCase())) {
    visibilityForm.value.allowed_emails = [...list, email]
  }
  userSearchQuery.value = ''
  userSearchResults.value = []
}

function removeUserFromWhitelist(email: string) {
  visibilityForm.value.allowed_emails = (visibilityForm.value.allowed_emails ?? []).filter((e) => e !== email)
}

async function saveSettings() {
  // Key 留空 = 只保存显隐配置（不修改已保存的 Key）
  const key = settingsKeyInput.value.trim()
  settingsSaving.value = true
  error.value = ''
  try {
    await adminSaveSettings(props.client, key, visibilityForm.value)
    settingsOpen.value = false
    notice.value = '设置已保存，立即生效'
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    settingsSaving.value = false
  }
}

function openCreate() {
  editingId.value = null
  editingDailyId.value = null
  editingConfigId.value = 0
  form.value = emptyForm()
  dialogError.value = ''
  dialogOpen.value = true
}

function closeDialog() {
  dialogOpen.value = false
}


/** 条件与奖池（活动与每日定时配置共用）。 */
function buildSharedParts() {
  return {
    conditions: form.value.conditions,
    prizes: form.value.prizes.map((p) => ({
      name: p.name,
      prize_type: p.prize_type,
      value: p.value,
      weight: p.weight,
      stock: p.stock,
      codes: p.prize_type === 'redeem_code' ? parseCodes(p) : []
    }))
  }
}

function buildInput(): ActivityInput {
  return {
    name: form.value.name,
    description: form.value.description,
    // 普通模式必填（canSubmit 已校验）；每日定时模式不会走到这里
    starts_at: form.value.startsAt ? beijingInputToISO(form.value.startsAt) : new Date(0).toISOString(),
    draws_at: form.value.drawsAt ? beijingInputToISO(form.value.drawsAt) : new Date(0).toISOString(),
    max_participants: Number(form.value.maxParticipants) || 0,
    condition_match: form.value.conditionMatch,
    auto_bonus_percent: Number(form.value.autoBonusPercent) || 0,
    ...buildSharedParts()
  }
}

function validateForm(): string {
  for (const p of form.value.prizes) {
    if (p.prize_type === 'redeem_code' && codesCount(p) < p.stock) {
      return `奖品「${p.name}」为兑换码类型，需预填至少 ${p.stock} 个兑换码（当前 ${codesCount(p)} 个），每行一个`
    }
  }
  return ''
}

async function submit(regenerate = false) {
  const invalid = validateForm()
  if (invalid) {
    dialogError.value = invalid
    return
  }
  saving.value = true
  error.value = ''
  dialogError.value = ''
  notice.value = ''
  try {
    if (form.value.isDaily) {
      // 每日定时：编辑模式走更新，新建模式创建一条新的日常配置
      const cfg = {
        id: editingDailyId.value ?? 0,
        enabled: true,
        repeat_policy: form.value.repeatPolicy,
        start_time: form.value.dailyStartTime,
        duration_hours: Number(form.value.dailyDurationHours) || 0,
        name: form.value.name,
        description: form.value.description,
        max_participants: Number(form.value.maxParticipants) || 0,
        condition_match: form.value.conditionMatch,
        auto_bonus_percent: Number(form.value.autoBonusPercent) || 0,
        ...buildSharedParts()
      }
      if (editingDailyId.value != null) {
        await adminUpdateDailyConfig(props.client, editingDailyId.value, cfg, regenerate)
        notice.value = regenerate
          ? '配置已保存，并已按新配置重新生成当前场次'
          : '日常抽奖配置已保存（作为之后自动创建场次的依据）'
      } else {
        await adminCreateDailyConfig(props.client, cfg)
        notice.value = '日常抽奖已保存，调度器将每天自动创建活动'
      }
      editingDailyId.value = null
      dialogOpen.value = false
      form.value = emptyForm()
      await refresh()
      return
    }
    if (editingId.value == null) {
      await adminCreateActivity(props.client, buildInput())
      notice.value = '活动已创建'
    } else {
      await adminUpdateActivity(props.client, editingId.value, buildInput())
      notice.value = '活动已保存'
    }
    dialogOpen.value = false
    form.value = emptyForm()
    await refresh()
  } catch (e) {
    dialogError.value = (e as Error).message
  } finally {
    saving.value = false
  }
}

/** 立即开奖：按当前参与者开奖并发放奖品。 */
async function drawById(id: number, name: string) {
  error.value = ''
  try {
    const ws = await adminDraw(props.client, id)
    notice.value = `已开奖「${name}」，共 ${ws.length} 名中奖用户`
    detailWinners.value = { ...detailWinners.value, [id]: ws }
    await refresh()
  } catch (e) {
    error.value = (e as Error).message
  }
}

function drawNow() {
  if (!current.value) return
  askConfirm('立即开奖', `确定对「${current.value.name}」立即开奖并发放奖品？开奖后不可修改。`, '立即开奖', () => drawById(current.value!.id, current.value!.name))
}

/** 关闭活动：停止参与、不开奖不发奖（与立即开奖/归档区分）。 */
async function closeById(id: number, name: string) {
  if (closingId.value != null) return
  closingId.value = id
  error.value = ''
  try {
    await adminCloseActivity(props.client, id)
    notice.value = `已关闭「${name}」，不再接受参与，也不会开奖发奖`
    await refresh()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    closingId.value = null
  }
}

/** 启用/停用指定日常配置（保留其余字段）。启用时后端立即预约下一场次。 */
async function toggleDaily(c: DailyConfig) {
  if (dailyWorkingId.value != null) return
  dailyWorkingId.value = c.id
  error.value = ''
  try {
    const updated = await adminSetDailyConfigEnabled(props.client, c.id, !c.enabled)
    dailyConfigs.value = dailyConfigs.value.map((x) => (x.id === c.id ? updated : x))
    notice.value = updated.enabled ? `已启用日常抽奖「${c.name}」，下一场次已创建` : `已停用日常抽奖「${c.name}」`
    await refresh()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    dailyWorkingId.value = null
  }
}

/** 删除指定日常配置（已创建的场次不受影响）。 */
async function deleteDaily(c: DailyConfig) {
  if (dailyWorkingId.value != null) return
  dailyWorkingId.value = c.id
  error.value = ''
  try {
    await adminDeleteDailyConfig(props.client, c.id)
    dailyConfigs.value = dailyConfigs.value.filter((x) => x.id !== c.id)
    notice.value = `已删除日常抽奖「${c.name}」`
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    dailyWorkingId.value = null
  }
}

function confirmDeleteDaily(c: DailyConfig) {
  askConfirm('删除日常抽奖', `确定删除日常抽奖「${c.name}」？已创建的场次不受影响，但之后不会再自动创建该场次。`, '删除', () => deleteDaily(c))
}

function closeActivity(a: ActivityView) {
  askConfirm(
    '关闭活动',
    `确定关闭「${a.name}」？关闭后用户不可再参与，该活动不会开奖、已参与者不会获得奖品。此操作不可撤销。`,
    '确认关闭',
    () => closeById(a.id, a.name)
  )
}

function archiveCurrent() {
  if (!current.value) return
  const name = current.value.name
  askConfirm('归档活动', `确定归档「${name}」？归档后用户不可见，可创建新活动。`, '归档', doArchive)
}

async function doArchive() {
  if (!current.value) return
  error.value = ''
  try {
    await adminArchiveActivity(props.client, current.value.id)
    editingId.value = null
    form.value = emptyForm()
    notice.value = '已归档'
    await refresh()
  } catch (e) {
    error.value = (e as Error).message
  }
}

function askConfirm(title: string, message: string, confirmText: string, action: () => void) {
  confirmDialog.value = { title, message, confirmText, action }
}

function closeConfirm() {
  confirmDialog.value = null
}

function runConfirm() {
  const c = confirmDialog.value
  confirmDialog.value = null
  c?.action()
}

function fmtDate(v: string): string {
  return formatBeijingDateTime(v)
}

onMounted(refresh)
</script>

<template>
  <div class="mx-auto w-full max-w-4xl space-y-6">
    <!-- 页头 -->
    <div class="flex items-center gap-3">
      <span class="flex h-11 w-11 items-center justify-center rounded-2xl bg-gradient-primary text-xl shadow-md shadow-primary-500/25">
        🎰
      </span>
      <div class="min-w-0 flex-1">
        <h1 class="text-lg font-bold text-gray-900 dark:text-white">抽奖活动管理</h1>
        <p class="text-xs text-gray-500 dark:text-dark-400">同一时间仅一场抽奖；开奖后可创建下一场</p>
      </div>
      <button class="btn btn-secondary" @click="openSettings">⚙️ 设置</button>
      <button class="btn btn-primary" @click="openCreate()">新增抽奖</button>
    </div>

    <!-- 统计卡片 -->
    <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
      <div v-for="s in statCards" :key="s.label" class="card p-4">
        <div class="flex items-start justify-between">
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ s.label }}</span>
          <span class="text-base leading-none">{{ s.icon }}</span>
        </div>
        <p class="mt-2 text-2xl font-bold leading-none text-gray-900 dark:text-white">{{ s.value }}</p>
        <p v-if="s.hint" class="mt-1.5 truncate text-[11px] text-gray-400 dark:text-dark-500">{{ s.hint }}</p>
      </div>
    </div>

    <p v-if="notice" class="rounded-xl bg-primary-50 px-4 py-3 text-sm text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
      {{ notice }}
    </p>
    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400">
      {{ error }}
    </p>
    <p v-if="loading" class="py-8 text-center text-sm text-gray-400">加载中…</p>

    <!-- 当前活动概览 -->
    <section v-if="current" class="card">
      <div class="card-header flex items-start justify-between gap-3">
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ current.name }}</h2>
            <span
              :class="['inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-semibold',
                       current.phase === 'joining'
                         ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300'
                         : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300']"
            >
              <span
                class="h-1.5 w-1.5 rounded-full"
                :class="current.phase === 'joining' ? 'animate-pulse bg-primary-500' : 'bg-gray-400'"
              ></span>
              {{ current.phase === 'joining' ? '进行中' : '未开始' }}
            </span>
          </div>
          <p v-if="current.description" class="mt-1 line-clamp-2 text-xs text-gray-500 dark:text-dark-400">
            {{ current.description }}
          </p>
        </div>
      </div>
      <div class="card-body space-y-4">
        <dl class="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div>
            <dt class="text-xs text-gray-400 dark:text-dark-500">抽奖开始</dt>
            <dd class="mt-0.5 text-sm font-medium text-gray-900 dark:text-gray-100">{{ fmtDate(current.starts_at) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-gray-400 dark:text-dark-500">开奖时间</dt>
            <dd class="mt-0.5 text-sm font-medium text-gray-900 dark:text-gray-100">{{ fmtDate(current.draws_at) }}</dd>
          </div>
          <div>
            <dt class="text-xs text-gray-400 dark:text-dark-500">参与人数</dt>
            <dd class="mt-0.5 text-sm font-medium text-gray-900 dark:text-gray-100">
              {{ current.participant_count }}<template v-if="current.max_participants > 0"> / {{ current.max_participants }}</template>
            </dd>
          </div>
          <div>
            <dt class="text-xs text-gray-400 dark:text-dark-500">条件关系</dt>
            <dd class="mt-0.5 text-sm font-medium text-gray-900 dark:text-gray-100">
              {{ current.condition_match === 'all' ? 'AND' : 'OR' }}
            </dd>
          </div>
        </dl>

        <!-- 参与进度条 -->
        <div>
          <div class="h-2 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
            <div
              class="h-2 rounded-full bg-gradient-primary transition-all duration-500"
              :style="{ width: (current.max_participants > 0 ? Math.min(100, Math.round((current.participant_count / current.max_participants) * 100)) : Math.min(100, current.participant_count * 10)) + '%' }"
            ></div>
          </div>
          <div class="mt-1 flex justify-between text-[11px] text-gray-400 dark:text-dark-500">
            <span>已参与 {{ current.participant_count }} 人</span>
            <span>{{ current.max_participants > 0 ? '上限 ' + current.max_participants + ' 人' : '不限名额' }}</span>
          </div>
        </div>

        <!-- 已参与名单 -->
        <div>
          <button
            class="mb-1.5 flex items-center gap-1 text-xs font-semibold uppercase tracking-wide text-gray-400 transition-colors hover:text-gray-600 dark:hover:text-dark-300"
            @click="showCurrentParticipants = !showCurrentParticipants"
          >
            已参与的人（{{ currentParticipants.length }}）
            <span class="text-[10px]">{{ showCurrentParticipants ? '▲' : '▼' }}</span>
          </button>
          <div v-if="showCurrentParticipants" class="flex flex-wrap gap-1.5">
            <span
              v-for="p in currentParticipants"
              :key="p.id"
              class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-dark-300"
              :title="`中奖权重 ×${p.weight}`"
            >
              {{ p.email }}
              <b class="font-semibold text-primary-600 dark:text-primary-400">×{{ p.weight }}</b>
            </span>
            <span v-if="!currentParticipants.length" class="text-sm text-gray-400">还没有人参与</span>
          </div>
        </div>

        <!-- 奖池 chips -->
        <div class="flex flex-wrap gap-2">
          <span
            v-for="p in current.prizes"
            :key="p.id"
            class="inline-flex items-center gap-2 rounded-xl border border-gray-100 bg-white px-3 py-1.5 text-sm dark:border-dark-700 dark:bg-dark-800/60"
          >
            <span
              :class="['flex h-6 w-6 items-center justify-center rounded-lg',
                       p.prize_type === 'balance'
                         ? 'bg-amber-100 text-amber-600 dark:bg-amber-500/15 dark:text-amber-400'
                         : 'bg-primary-100 text-primary-600 dark:bg-primary-500/15 dark:text-primary-400']"
            >
              <PrizeIcon :type="p.prize_type" class="h-3.5 w-3.5" />
            </span>
            {{ p.name }}
            <b class="font-mono text-primary-600 dark:text-primary-400">{{ formatValue(p.value) }}</b>
            <span class="text-xs text-gray-400">{{ p.granted_count }}/{{ p.stock }}</span>
          </span>
        </div>

        <div class="flex items-center justify-end gap-3 border-t border-gray-100 pt-4 dark:border-dark-700">
          <button class="btn btn-secondary" @click="archiveCurrent">归档活动</button>
          <button v-if="current.phase === 'joining'" class="btn btn-danger" @click="drawNow">立即开奖</button>
          <button
            v-if="current.phase === 'joining' || current.phase === 'upcoming'"
            class="btn btn-danger"
            @click="closeActivity(current)"
          >关闭</button>
          <button class="btn btn-primary" @click="fillForm(current); dialogOpen = true">编辑活动</button>
        </div>
      </div>
    </section>

    <p v-if="!loading && !current" class="text-center text-sm text-gray-400">
      当前没有进行中的抽奖，点击右上角「新增抽奖」创建一场。
    </p>

    <!-- 日常定时抽奖：多配置列表（每条独立启停/编辑/删除；列表为空时整块隐藏） -->
    <section v-if="dailyConfigs.length" class="card border-dashed">
      <div class="card-header flex items-center justify-between">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">⏰ 日常定时抽奖（{{ dailyConfigs.length }}）</h2>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">到点自动创建场次并开奖；停用后保留配置</p>
      </div>
      <div class="card-body space-y-2">
        <div
          v-for="c in dailyConfigs"
          :key="c.id"
          class="flex flex-wrap items-center gap-3 rounded-xl border border-gray-100 px-4 py-3 dark:border-dark-700"
        >
          <span
            :class="['flex-shrink-0 rounded-full px-2.5 py-0.5 text-xs font-semibold',
                     c.enabled ? 'bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300'
                       : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-400']"
          >
            {{ c.enabled ? '已启用' : '已停用' }}
          </span>
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium text-gray-900 dark:text-gray-100">{{ c.name }}</p>
            <p class="mt-0.5 text-xs text-gray-400 dark:text-dark-500">
              每天 {{ c.start_time }}（北京时间）开启，{{ c.duration_hours }} 小时后开奖
              <template v-if="c.enabled"> · 到点自动创建场次</template>
            </p>
            <p class="mt-0.5 truncate text-xs" :class="c.conditions.length ? 'text-primary-600 dark:text-primary-400' : 'text-gray-400 dark:text-dark-500'">
              🎟️ 可参与条件：{{ conditionsSummary(c.conditions) }}
            </p>
          </div>
          <button
            class="btn btn-sm"
            :class="c.enabled ? 'btn-secondary' : 'btn-primary'"
            :disabled="dailyWorkingId === c.id"
            @click="toggleDaily(c)"
          >
            {{ dailyWorkingId === c.id ? '处理中…' : c.enabled ? '停用' : '启用' }}
          </button>
          <button class="btn btn-secondary btn-sm" @click="editDaily(c)">编辑</button>
          <button class="btn btn-danger btn-sm" :disabled="dailyWorkingId === c.id" @click="confirmDeleteDaily(c)">删除</button>
        </div>
      </div>
    </section>

    <!-- 所有场次（当前 + 历史，分页每页 20） -->
    <section class="card">
      <div class="card-header">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">所有场次（{{ allActivities.length }}）</h2>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
          用户端同一时间只展示一场：当前活动结束后，最早开始的下一场自动接档
        </p>
      </div>
      <div class="card-body space-y-2">
        <div
          v-for="a in pagedActivities"
          :key="a.id"
          class="rounded-xl border border-gray-100 px-4 py-3 dark:border-dark-700"
        >
          <div class="flex flex-wrap items-center gap-3">
            <span :class="['flex-shrink-0 rounded-full px-2.5 py-0.5 text-xs font-semibold', statusBadge(a).cls]">
              {{ statusBadge(a).text }}
            </span>
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium text-gray-900 dark:text-gray-100">{{ a.name }}</p>
              <p class="mt-0.5 text-xs text-gray-400 dark:text-dark-500">
                {{ fmtDate(a.starts_at) }} → {{ fmtDate(a.draws_at) }} · {{ a.participant_count }} 人参与
              </p>
            </div>
            <button class="btn btn-secondary btn-sm" @click="toggleWinners(a)">
              {{ expandedId === a.id ? '收起名单' : '中奖名单' }}
            </button>
            <button
              v-if="a.status === 'active' && (a.phase === 'joining' || a.phase === 'upcoming')"
              class="btn btn-secondary btn-sm"
              @click="openEdit(a)"
            >
              编辑
            </button>
            <button
              v-if="a.status === 'active' && (a.phase === 'joining' || a.phase === 'upcoming')"
              class="btn btn-danger btn-sm"
              :disabled="closingId === a.id"
              @click="closeActivity(a)"
            >
              {{ closingId === a.id ? '关闭中…' : '关闭' }}
            </button>
          </div>
          <!-- 行内中奖名单（懒加载，管理端完整邮箱 + 发放备注） -->
          <div v-if="expandedId === a.id" class="mt-3 border-t border-gray-100 pt-3 dark:border-dark-700">
            <p v-if="detailLoading === a.id" class="text-xs text-gray-400">加载中…</p>
            <template v-else>
              <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-400">
                中奖名单（{{ (detailWinners[a.id] ?? []).length }}）
              </p>
              <ul v-if="(detailWinners[a.id] ?? []).length" class="space-y-1.5">
                <li
                  v-for="w in detailWinners[a.id]"
                  :key="w.id"
                  class="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-gray-50 px-3 py-2 text-sm dark:bg-dark-900/60"
                >
                  <span class="flex min-w-0 items-center gap-2">
                    <span class="font-medium text-gray-700 dark:text-gray-300">{{ w.email }}</span>
                    <span
                      v-if="w.note"
                      class="truncate rounded-full bg-white px-2 py-0.5 text-xs text-gray-500 dark:bg-dark-800 dark:text-dark-300"
                      :title="w.note"
                    >{{ w.note }}</span>
                  </span>
                  <span class="flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                    {{ w.prize_name }} · {{ formatValue(w.value) }}
                    <em
                      :class="['rounded-full px-2 py-0.5 not-italic',
                               w.fulfillment === 'done' ? 'bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300'
                                 : w.fulfillment === 'pending' ? 'bg-amber-50 text-amber-600 dark:bg-amber-900/30 dark:text-amber-300'
                                 : 'bg-red-50 text-red-600 dark:bg-red-900/30 dark:text-red-300']"
                    >
                      {{ w.fulfillment === 'done' ? '已发放' : w.fulfillment === 'pending' ? '发放中' : '失败' }}
                    </em>
                  </span>
                </li>
              </ul>
              <p v-else class="text-xs text-gray-400">该场次暂无中奖用户</p>
            </template>
          </div>
        </div>
        <!-- 空状态 -->
        <p v-if="!pagedActivities.length" class="text-center text-sm text-gray-400">暂无场次</p>
        <!-- 分页 -->
        <div v-if="totalPages > 1" class="flex items-center justify-center gap-3 pt-2">
          <button class="btn btn-ghost btn-sm" :disabled="page <= 1" @click="gotoPage(page - 1)">上一页</button>
          <span class="text-xs text-gray-500 dark:text-dark-400">第 {{ page }} / {{ totalPages }} 页</span>
          <button class="btn btn-ghost btn-sm" :disabled="page >= totalPages" @click="gotoPage(page + 1)">下一页</button>
        </div>
      </div>
    </section>

    <!-- 新增/编辑模态框（Teleport 到 body：不受宿主布局的 transform/层叠上下文影响，遮罩恒为全屏） -->
    <Teleport to="body">
    <div
      v-if="dialogOpen"
      class="fixed inset-0 z-[1900] flex items-start justify-center overflow-y-auto bg-gray-900/60 px-4 py-8 backdrop-blur-sm"
    >
      <div class="w-full max-w-3xl rounded-2xl bg-white shadow-glass dark:bg-dark-800">
        <div class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-dark-700">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ formTitle }}</h2>
          <button class="btn btn-ghost btn-sm" @click="closeDialog">✕</button>
        </div>
        <div class="max-h-[70vh] space-y-5 overflow-y-auto px-6 py-5">
          <p v-if="dialogError" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400">
            {{ dialogError }}
          </p>
          <p
            v-if="editingId != null && editingConfigId > 0"
            class="rounded-xl bg-amber-50 px-4 py-3 text-xs text-amber-700 dark:bg-amber-500/10 dark:text-amber-300"
          >
            该场次由日常定时抽奖自动生成：这里的修改<b>仅影响这一场</b>。如需调整整个系列（含之后自动创建的场次），请编辑上方「⏰ 日常定时抽奖」里的配置。
          </p>
          <div class="grid gap-4 sm:grid-cols-2">
            <label class="sm:col-span-2">
              <span class="input-label">活动名称</span>
              <input v-model="form.name" class="input" placeholder="如：Grok Heavy 七日挑战" />
            </label>
            <label class="sm:col-span-2">
              <span class="input-label">活动说明（支持 Markdown）</span>
              <textarea v-model="form.description" class="input" rows="2" placeholder="展示给用户的说明文字，支持 **加粗**、列表、标题等"></textarea>
            </label>
            <label v-if="!form.isDaily">
              <span class="input-label">抽奖开始时间（北京时间）</span>
              <input v-model="form.startsAt" type="datetime-local" class="input" />
            </label>
            <label v-if="!form.isDaily">
              <span class="input-label">开奖时间（北京时间）</span>
              <input v-model="form.drawsAt" type="datetime-local" class="input" />
            </label>
            <label v-if="form.isDaily" class="sm:col-span-2">
              <span class="input-label">每日开启时刻（北京时间，到点自动创建当天活动）</span>
              <div class="flex flex-wrap items-center gap-3">
                <input v-model="form.dailyStartTime" type="time" class="input !w-32" />
                <span class="text-xs text-gray-500 dark:text-dark-400">持续</span>
                <input v-model.number="form.dailyDurationHours" type="number" min="0.5" step="0.5" class="input !w-20" />
                <span class="text-xs text-gray-500 dark:text-dark-400">小时后自动开奖</span>
              </div>
            </label>
            <label class="sm:col-span-2 flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
              <input v-model="form.isDaily" type="checkbox" class="h-4 w-4" :disabled="editingId != null" />
              每日定时抽奖
              <span class="text-xs font-normal text-gray-400 dark:text-dark-500">
                {{ editingId != null ? '（编辑已有活动时不支持切换）' : '（开启时刻自动创建当天活动，到点自动开奖；再次保存即覆盖日常配置）' }}
              </span>
            </label>
            <label>
              <span class="input-label">最多参与人数（0 = 不限）</span>
              <input v-model.number="form.maxParticipants" type="number" min="0" class="input" />
            </label>
            <label v-if="form.isDaily" class="sm:col-span-2">
              <FieldHint
                label="重复参与策略"
                hint="限制用户反复抽奖：不限 = 每期都能参与；限参与一次 = 参与过该系列任意一场后不能再参与；中奖后出局 = 在该系列中过奖的用户不能再参与。"
              />
              <LotterySelect
                v-model="form.repeatPolicy"
                :options="[
                  { value: 'unlimited', label: '不限（每期独立）' },
                  { value: 'join_once', label: '限参与一次（该系列）' },
                  { value: 'win_once', label: '中奖后不能再参与' }
                ]"
              />
            </label>
            <label>
              <FieldHint label="条件关系" hint="多条可参与条件之间的组合方式：AND 要求全部满足；OR 满足任意一条即可。" />
              <LotterySelect v-model="form.conditionMatch" :options="conditionMatchOptions" />
            </label>
            <label>
              <FieldHint label="自动加成默认百分比" hint="条件加成选「自动叠加」时使用的默认加成比例；满足该条件的用户中奖权重 ×(1+该百分比/100)。" />
              <input v-model.number="form.autoBonusPercent" type="number" min="0" max="1000" class="input" />
            </label>
          </div>

          <!-- 可参与条件 -->
          <div class="rounded-xl border border-gray-100 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-900/40">
            <div class="mb-3 flex items-center justify-between">
              <h3 class="flex items-center gap-1.5 text-sm font-semibold text-gray-900 dark:text-white">
                🎟️ 可参与条件
                <span class="rounded-md bg-white px-1.5 py-0.5 text-[11px] font-bold text-primary-600 ring-1 ring-primary-200 dark:bg-dark-800 dark:text-primary-300 dark:ring-primary-800">
                  {{ form.conditionMatch === 'all' ? 'AND' : 'OR' }}
                </span>
              </h3>
            </div>
          <LotteryConditionsEditor v-model="form.conditions" :auto-bonus-percent="form.autoBonusPercent" />
          </div>

          <!-- 奖池 -->
          <div class="rounded-xl border border-gray-100 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-900/40">
            <div class="mb-3 flex items-center justify-between">
              <h3 class="flex items-center gap-1.5 text-sm font-semibold text-gray-900 dark:text-white">🎁 奖池</h3>
            </div>
          <LotteryPrizesEditor
            v-model="form.prizes"
            :weight-multiplier="formWeightMultiplier"
            :max-participants="form.maxParticipants"
            :has-conditions="form.conditions.length > 0"
          />
        </div>
        </div>
        <div class="flex items-center justify-between gap-3 border-t border-gray-100 px-6 py-4 dark:border-dark-700">
          <p class="min-w-0 flex-1 text-xs text-gray-400 dark:text-dark-500">
            <template v-if="editingDailyId != null">保存仅作为之后自动创建场次的依据；需要立刻替换当前场次请用「保存并重新生成」</template>
            <template v-else-if="current && current.phase === 'joining' && editingId === current?.id">活动进行中，保存后立即对用户生效</template>
            <template v-else>保存后用户即可见（未到开始时间前显示「未开始」）</template>
          </p>
          <div class="flex flex-none gap-3">
            <button class="btn btn-secondary" @click="closeDialog">取消</button>
            <template v-if="editingDailyId != null">
              <button class="btn btn-primary" :disabled="!canSubmit || saving" @click="() => submit(false)">
                {{ saving ? '保存中…' : '保存' }}
              </button>
              <button
                class="btn btn-secondary"
                :disabled="!canSubmit || saving"
                @click="askConfirm('保存并重新生成', '将归档该配置当前未开奖的场次，并按新配置立即重新生成今天的场次（已有用户参与时会被拒绝）。确定继续？', '保存并重新生成', () => submit(true))"
              >
                {{ saving ? '处理中…' : '保存并重新生成' }}
              </button>
            </template>
            <button v-else class="btn btn-primary" :disabled="!canSubmit || saving" @click="() => submit(false)">
              {{ saving ? '保存中…' : '保存' }}
            </button>
          </div>
        </div>
      </div>
    </div>
    </Teleport>

    <!-- 二次确认弹窗（替代系统 confirm） -->
    <Teleport to="body">
    <div
      v-if="confirmDialog"
      class="fixed inset-0 z-[1950] flex items-center justify-center bg-gray-900/60 px-4 backdrop-blur-sm"
    >
      <div class="card w-full max-w-sm p-6 text-center">
        <h3 class="text-lg font-bold text-gray-900 dark:text-white">{{ confirmDialog.title }}</h3>
        <p class="mt-2 text-sm leading-relaxed text-gray-500 dark:text-dark-400">{{ confirmDialog.message }}</p>
        <div class="mt-5 flex justify-center gap-3">
          <button class="btn btn-secondary flex-1" @click="closeConfirm">取消</button>
          <button class="btn btn-danger flex-1" @click="runConfirm">{{ confirmDialog.confirmText }}</button>
        </div>
      </div>
    </div>
    </Teleport>
  </div>

  <!-- 设置弹窗：管理员 API Key -->
  <Teleport to="body">
  <div
    v-if="settingsOpen"
    class="fixed inset-0 z-[1950] flex items-start justify-center overflow-y-auto bg-gray-900/60 px-4 py-8 backdrop-blur-sm"
    @click.self="settingsOpen = false"
  >
    <div class="w-full max-w-2xl rounded-2xl bg-white shadow-glass dark:bg-dark-800">
      <div class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">设置</h2>
        <button class="btn btn-ghost btn-sm" @click="settingsOpen = false">✕</button>
      </div>
      <div class="space-y-4 px-6 py-5">
        <div class="rounded-xl bg-gray-50 px-4 py-3 text-sm dark:bg-dark-900/60">
          <span class="text-gray-500 dark:text-dark-400">上游地址</span>
          <p class="mt-0.5 font-mono text-xs text-gray-700 dark:text-gray-300">{{ settingsInfo?.sub2api_url }}</p>
          <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">lotteryd 启动参数配置，如需修改请调整启动命令</p>
        </div>
        <div class="rounded-xl border border-gray-100 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-900/40">
          <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">👁️ 用户侧显隐</h3>
          <label class="block">
            <span class="input-label">可见范围（管理员不受限）</span>
            <LotterySelect
              v-model="visibilityForm.mode"
              :options="[
                { value: 'all', label: '全员可见' },
                { value: 'partial', label: '部分人可见（按邮箱白名单）' }
              ]"
            />
          </label>
          <div v-if="visibilityForm.mode === 'partial'" class="mt-3">
            <span class="input-label">白名单用户（搜索后点击添加）</span>
            <div class="relative">
              <input
                v-model="userSearchQuery"
                type="text"
                class="input !py-2"
                placeholder="输入邮箱或用户名搜索用户…"
                @input="onUserSearchInput"
              />
              <span v-if="userSearching" class="absolute right-3 top-2 text-xs text-gray-400">搜索中…</span>
            </div>
            <div
              v-if="userSearchResults.length"
              class="mt-1 max-h-44 overflow-y-auto rounded-xl border border-gray-200 bg-white py-1 dark:border-dark-700 dark:bg-dark-800"
            >
              <button
                v-for="u in userSearchResults"
                :key="u.id"
                type="button"
                class="flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700"
                @click="addUserToWhitelist(u)"
              >
                <span class="min-w-0 truncate">{{ u.email }}</span>
                <span class="text-xs text-primary-600 dark:text-primary-400">添加</span>
              </button>
            </div>
            <div v-if="visibilityForm.allowed_emails.length" class="mt-2 flex flex-wrap gap-1.5">
              <span
                v-for="email in visibilityForm.allowed_emails"
                :key="email"
                class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-dark-300"
              >
                {{ email }}
                <button
                  type="button"
                  class="text-gray-400 transition-colors hover:text-red-500"
                  @click="removeUserFromWhitelist(email)"
                >✕</button>
              </span>
            </div>
            <p
              v-if="visibilityForm.mode === 'partial' && !visibilityForm.allowed_emails.length"
              class="mt-1 text-xs text-red-500"
            >
              部分人可见模式下白名单为空 = 除管理员外无人可见
            </p>
          </div>
        </div>
        <label>
          <span class="input-label flex items-center gap-1">
            管理员 API Key
            <span
              v-if="settingsInfo?.admin_api_key_set"
              class="rounded-md bg-primary-50 px-1.5 py-0.5 text-xs text-primary-600 dark:bg-primary-900/30 dark:text-primary-300"
            >
              已配置（{{ settingsInfo?.admin_api_key_masked }}）
            </span>
          </span>
          <input
            v-model="settingsKeyInput"
            class="input font-mono"
            :placeholder="settingsInfo?.admin_api_key_set ? '输入新 Key 覆盖，留空保持不变' : 'admin-xxxx'"
          />
          <span class="mt-1 block text-xs text-gray-400 dark:text-dark-500">
            用于拉取用量数据与发放奖品；在 Sub2API 管理后台「系统设置」生成。保存后立即生效，无需重启。
          </span>
        </label>
      </div>
      <div class="flex justify-end gap-3 border-t border-gray-100 px-6 py-4 dark:border-dark-700">
        <button class="btn btn-secondary" @click="settingsOpen = false">取消</button>
        <button class="btn btn-primary" :disabled="settingsSaving" @click="saveSettings">
          {{ settingsSaving ? '保存中…' : '保存' }}
        </button>
      </div>
    </div>
    </div>
    </Teleport>


</template>
