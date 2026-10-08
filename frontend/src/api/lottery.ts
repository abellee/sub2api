/** lotteryd API 类型与方法（与后端 DTO 一一对应）。 */
import type { LotteryClient } from './lotteryClient'
import { unwrap } from './lotteryClient'

export type ConditionDimension = 'token_usage' | 'activity_days' | 'registered_days'
export type ConditionMatch = 'all' | 'any'
export type BonusMode = 'none' | 'manual' | 'auto'
export type PrizeType = 'redeem_code' | 'balance'
export type ActivityPhase = 'upcoming' | 'joining' | 'drawn' | 'fulfilled'

export interface ConditionDef {
  dimension: ConditionDimension
  window_days: number
  /** token_usage 维度：per_day = 每天达标，total = 窗口累计达标 */
  mode?: 'per_day' | 'total'
  threshold?: number
  min_active_days?: number
  daily_min_tokens?: number
  /** registered_days 维度：注册天数在 [min, max] 内（0 = 该侧不设限）。min=0 且 max=N → 新用户 */
  min_registered_days?: number
  max_registered_days?: number
  bonus_mode: BonusMode
  /** manual 加成：50 = +50% */
  bonus_percent?: number
}

export interface ConditionEvalResult {
  condition: ConditionDef
  satisfied: boolean
  detail: string
  bonus_percent: number
}

export interface PrizeView {
  id: number
  name: string
  prize_type: PrizeType
  value: number
  weight: number
  stock: number
  granted_count: number
  /** 仅管理端详情返回；用户侧永不携带。 */
  codes?: string[]
}

export interface WinnerRecord {
  id: number
  activity_id: number
  activity_name?: string
  user_id: number
  email: string
  prize_id: number
  prize_name: string
  prize_type: PrizeType
  value: number
  fulfillment: 'pending' | 'done' | 'failed'
  redeem_code?: string
  note?: string
  error?: string
  created_at: string
}

export interface ParticipantRecord {
  id: number
  activity_id: number
  user_id: number
  email: string
  weight: number
  joined_at: string
}

export interface ActivityView {
  id: number
  name: string
  description: string
  starts_at: string
  draws_at: string
  max_participants: number
  /** 关闭显示时服务端不下发该字段。 */
  participant_count?: number
  show_participant_count?: boolean
  condition_match: ConditionMatch
  phase: ActivityPhase
  conditions?: ConditionEvalResult[]
  eligible: boolean
  eligible_reason?: string
  weight?: number
  joined: boolean
  won?: WinnerRecord
  prizes: PrizeView[]
  drawn_at?: string
  winners?: WinnerRecord[]
  status: string
  created_at: string
  /** 来源日常定时抽奖配置 ID（0/缺省 = 手动创建） */
  daily_config_id?: number
  /** 管理端列表才有：当前生效的共用参与组。 */
  repeat_group?: string
  repeat_policy?: RepeatPolicy | string
}

/** 管理端活动详情（原始条件定义，供编辑）。 */
export interface AdminActivityDetail {
  activity: {
    id: number
    name: string
    description: string
    starts_at: string
    draws_at: string
    max_participants: number
    show_participant_count?: boolean
    condition_match: ConditionMatch
    auto_bonus_percent: number
    status: string
    drawn_at: string
    conditions: ConditionDef[]
    prizes: PrizeView[]
    daily_config_id?: number
    repeat_policy?: RepeatPolicy | string
    repeat_group?: string
  }
  winners: WinnerRecord[]
}

export interface ActivityInput {
  name: string
  description: string
  starts_at: string
  draws_at: string
  max_participants: number
  show_participant_count: boolean
  condition_match: ConditionMatch
  auto_bonus_percent: number
  /** 手动活动的重复策略和共用参与组。定时场次由日常配置决定，编辑单场时服务端会忽略。 */
  repeat_policy?: RepeatPolicy
  repeat_group?: string
  conditions: ConditionDef[]
  prizes: Array<{
    name: string
    prize_type: PrizeType
    value: number
    weight: number
    stock: number
    /** 兑换码奖的预存码池（每行一个）；为空时开奖回退为主服务实时生成。 */
    codes?: string[]
  }>
}

const num = (v: unknown): number => (typeof v === 'number' ? v : 0)

/** 用户侧：活动列表（含我的资格/参与/中奖状态）。 */
export async function listActivities(client: LotteryClient): Promise<ActivityView[]> {
  const data = await unwrap<{ activities: ActivityView[] }>(client.http.get('/v1/activities'))
  return data.activities ?? []
}

/** 全局资格查询：符合条件、未参与的参与中活动（供引导弹窗）。 */
export async function fetchEligibility(client: LotteryClient): Promise<ActivityView[]> {
  const data = await unwrap<{ activities: ActivityView[] }>(client.http.get('/v1/eligibility'))
  return data.activities ?? []
}

/** 我的中奖记录。 */
export async function fetchMyWinnings(client: LotteryClient): Promise<WinnerRecord[]> {
  const data = await unwrap<{ winners: WinnerRecord[] }>(client.http.get('/v1/my/winnings'))
  return data.winners ?? []
}

/** 用户侧：某活动的开奖名单（后端已脱敏）。 */
export async function fetchActivityWinners(
  client: LotteryClient,
  id: number
): Promise<WinnerRecord[]> {
  const data = await unwrap<{ winners: WinnerRecord[] }>(
    client.http.get(`/v1/activities/${id}/winners`)
  )
  return data.winners ?? []
}

/** 参与抽奖。 */
export async function participate(client: LotteryClient, activityId: number): Promise<ActivityView> {
  return unwrap<ActivityView>(
    client.http.post(`/v1/activities/${activityId}/participate`)
  )
}

/** 管理端：活动列表。 */
export async function adminListActivities(client: LotteryClient): Promise<ActivityView[]> {
  const data = await unwrap<{ activities: ActivityView[] }>(client.http.get('/v1/admin/activities'))
  return data.activities ?? []
}

/** 管理端：活动详情（含条件定义与脱敏中奖名单）。 */
export async function adminGetActivity(
  client: LotteryClient,
  id: number
): Promise<AdminActivityDetail> {
  return unwrap<AdminActivityDetail>(client.http.get(`/v1/admin/activities/${id}`))
}

/** 管理端：创建活动。 */
export async function adminCreateActivity(
  client: LotteryClient,
  input: ActivityInput
): Promise<{ id: number }> {
  return unwrap<{ id: number }>(client.http.post('/v1/admin/activities', input))
}

/** 管理端：更新活动。 */
export async function adminUpdateActivity(
  client: LotteryClient,
  id: number,
  input: ActivityInput
): Promise<{ id: number }> {
  return unwrap<{ id: number }>(client.http.put(`/v1/admin/activities/${id}`, input))
}

/** 管理端：归档活动。 */
export async function adminArchiveActivity(client: LotteryClient, id: number): Promise<void> {
  await unwrap(client.http.delete(`/v1/admin/activities/${id}`))
}

/** 管理端：参与者名单。 */
export async function adminListParticipants(
  client: LotteryClient,
  id: number
): Promise<ParticipantRecord[]> {
  const data = await unwrap<{ participants: ParticipantRecord[] }>(
    client.http.get(`/v1/admin/activities/${id}/participants`)
  )
  return data.participants ?? []
}

/** 管理端：中奖名单（脱敏）。 */
export async function adminListWinners(client: LotteryClient, id: number): Promise<WinnerRecord[]> {
  const data = await unwrap<{ winners: WinnerRecord[] }>(
    client.http.get(`/v1/admin/activities/${id}/winners`)
  )
  return data.winners ?? []
}

/** 管理端：立即开奖。 */
export async function adminDraw(client: LotteryClient, id: number): Promise<WinnerRecord[]> {
  const data = await unwrap<{ winners: WinnerRecord[] }>(
    client.http.post(`/v1/admin/activities/${id}/draw`)
  )
  return data.winners ?? []
}

/** 管理端：重试失败发放。 */
export async function adminRetryFulfillment(client: LotteryClient, id: number): Promise<void> {
  await unwrap(client.http.post(`/v1/admin/activities/${id}/fulfill`))
}

export const formatValue = (v: unknown): string => {
  const n = num(v)
  return Number.isInteger(n) ? String(n) : String(Number(n.toFixed(6)))
}

/** 管理端设置（密钥脱敏）。 */
export interface AdminSettingsInfo {
  sub2api_url: string
  admin_api_key_set: boolean
  admin_api_key_masked: string
}

/** 管理端：读取设置（含显隐配置）。 */
export async function adminGetSettings(client: LotteryClient): Promise<
  AdminSettingsInfo & { visibility: VisibilitySettings }
> {
  return unwrap<AdminSettingsInfo & { visibility: VisibilitySettings }>(
    client.http.get('/v1/admin/settings')
  )
}

/** 管理端：保存管理员 API Key（立即生效，无需重启）。 */
export async function adminSaveSettings(
  client: LotteryClient,
  adminApiKey: string,
  visibility?: VisibilitySettings
): Promise<void> {
  await unwrap(
    client.http.put('/v1/admin/settings', {
      // Key 留空表示不修改，仅更新显隐等配置
      ...(adminApiKey ? { admin_api_key: adminApiKey } : {}),
      ...(visibility ? { visibility } : {})
    })
  )
}

/** 用户侧显隐配置。 */
export interface VisibilitySettings {
  mode: 'all' | 'partial'
  allowed_emails: string[]
}

/** 当前用户对抽奖功能的可见性。 */
export type LotteryPhase = 'joining' | 'upcoming' | 'drawn' | ''

/** 用户侧显隐结果 + 当前抽奖状态（phase 为空表示当前没有任何活动数据）。 */
export async function fetchMyVisibility(
  client: LotteryClient
): Promise<{ visible: boolean; phase: LotteryPhase }> {
  return unwrap<{ visible: boolean; phase: LotteryPhase }>(client.http.get('/v1/me/visibility'))
}

/** 用户搜索结果（白名单选择用）。 */
export interface SearchedUser {
  id: number
  email: string
  username: string
}

/** 管理端：按关键字搜索用户。 */
export async function adminSearchUsers(
  client: LotteryClient,
  q: string,
  limit = 20
): Promise<SearchedUser[]> {
  const data = await unwrap<{ users: SearchedUser[] }>(
    client.http.get('/v1/admin/users/search', { params: { q, limit } })
  )
  return data.users ?? []
}

/** 日常定时抽奖配置：调度器每天到达 start_time 后自动创建一场活动。 */
export type RepeatPolicy = 'unlimited' | 'join_once' | 'win_once'

export interface DailyConfig {
  /** 配置 ID（新建时传 0，服务端分配） */
  id: number
  enabled: boolean
  /** 重复参与策略：unlimited 每期独立 / join_once 限参与一次 / win_once 中奖后出局 */
  repeat_policy: RepeatPolicy
  /** 共用参与组。相同非空名称的配置共用次数；空则只限制本配置。 */
  repeat_group?: string
  /** 每日开启时刻，"HH:MM"（本地时区） */
  start_time: string
  /** 开奖时刻 = 开启 + 时长（小时） */
  duration_hours: number
  name: string
  description: string
  max_participants: number
  /** 缺省为不显示。为 true 时才向用户下发参与人数。 */
  show_participant_count?: boolean
  condition_match: ConditionMatch
  auto_bonus_percent: number
  conditions: ConditionDef[]
  prizes: Array<{
    name: string
    prize_type: PrizeType
    value: number
    weight: number
    stock: number
    codes?: string[]
  }>
}

export function emptyDailyConfig(): DailyConfig {
  return {
    id: 0,
    enabled: false,
    repeat_policy: 'unlimited',
    repeat_group: '',
    start_time: '08:00',
    duration_hours: 12,
    name: '每日抽奖',
    description: '',
    max_participants: 0,
    condition_match: 'all',
    auto_bonus_percent: 0,
    conditions: [],
    prizes: [{ name: '日常奖', prize_type: 'balance', value: 1, weight: 0, stock: 5 }]
  }
}

/** 管理端：列出全部日常定时抽奖配置。 */
export async function adminListDailyConfigs(client: LotteryClient): Promise<DailyConfig[]> {
  const data = await unwrap<{ configs: DailyConfig[] }>(client.http.get('/v1/admin/daily-configs'))
  return data.configs ?? []
}

/** 管理端：新建日常定时抽奖配置（id 传 0，服务端分配）。 */
export async function adminCreateDailyConfig(client: LotteryClient, config: DailyConfig): Promise<DailyConfig> {
  const data = await unwrap<{ config: DailyConfig }>(client.http.post('/v1/admin/daily-configs', { config }))
  return data.config
}

/** 管理端：更新日常定时抽奖配置（全量）；regenerate=true 时保存并重建当前场次。 */
export async function adminUpdateDailyConfig(
  client: LotteryClient,
  id: number,
  config: DailyConfig,
  regenerate = false
): Promise<{ config: DailyConfig; regenerated?: boolean }> {
  return unwrap(client.http.put(`/v1/admin/daily-configs/${id}`, { config, regenerate }))
}

/** 管理端：启用/停用单个配置（保留其余字段）。 */
export async function adminSetDailyConfigEnabled(
  client: LotteryClient,
  id: number,
  enabled: boolean
): Promise<DailyConfig> {
  const data = await unwrap<{ config: DailyConfig }>(
    client.http.put(`/v1/admin/daily-configs/${id}/enabled`, { enabled })
  )
  return data.config
}

/** 管理端：删除单个日常定时抽奖配置（已创建的场次不受影响）。 */
export async function adminDeleteDailyConfig(client: LotteryClient, id: number): Promise<void> {
  await unwrap(client.http.delete(`/v1/admin/daily-configs/${id}`))
}

/** 奖池编辑器的奖品草稿（codesText 为码池文本，提交前转 codes 数组）。 */
export interface PrizeDraft {
  name: string
  prize_type: PrizeType
  value: number
  weight: number
  stock: number
  codesText: string
}

/** 条件当前实际加成百分比：none=0，manual=手填值，auto=表单默认值。 */
export function conditionEffectiveBonus(c: ConditionDef, autoBonusPercent: number): number {
  if (c.bonus_mode === 'manual') return c.bonus_percent ?? 0
  if (c.bonus_mode === 'auto') return autoBonusPercent ?? 0
  return 0
}

/** 全部条件满足时的合计权重倍率（各条件 (1+加成/100) 连乘，与后端开奖一致）。 */
export function totalWeightMultiplier(conditions: ConditionDef[], autoBonusPercent: number): number {
  return conditions.reduce((acc, c) => acc * (1 + conditionEffectiveBonus(c, autoBonusPercent) / 100), 1)
}

/** 管理端：手动关闭活动（停止参与、不开奖、不发奖）。 */
export async function adminCloseActivity(client: LotteryClient, id: number): Promise<void> {
  await unwrap(client.http.post(`/v1/admin/activities/${id}/close`))
}


