/** lotteryd API 类型与方法（与后端 DTO 一一对应）。 */
import type { LotteryClient } from './lotteryClient'
import { unwrap } from './lotteryClient'
import type { ConditionDef, PrizeType, VisibilitySettings } from './lottery'

export type TaskStatus = 'active' | 'ended'
export type TaskRepeatPolicy = 'unlimited' | 'join_once' | 'achieved_once'

export interface TaskView {
  id: number
  name: string
  cover?: string
  /** 任务说明，用户侧展示 */
  description?: string
  status: TaskStatus
  group_id: number
  group_name?: string
  model?: string
  start_date: string
  duration_days: number
  /** 用户侧视图由后端计算：最后一天（YYYY-MM-DD）与剩余天数 */
  end_date?: string
  days_left?: number
  settle_time: string
  /** 达成条件：每结算日 token 消耗量（原始 token 数，展示层换算 M 单位） */
  threshold_tokens: number
  reward_type: PrizeType
  reward_value: number
  repeat_policy: TaskRepeatPolicy
  whitelist?: string[]
  blacklist?: string[]
  conditions: ConditionDef[]
  created_at?: string
  updated_at?: string
}

/** 管理端列表项：任务 + 码池统计。 */
export interface AdminTaskView extends TaskView {
  codes_available: number
  codes_granted: number
}

export interface TaskInput {
  name: string
  cover: string
  description: string
  group_id: number
  group_name: string
  model: string
  start_date: string
  duration_days: number
  settle_time: string
  threshold_tokens: number
  reward_type: PrizeType
  reward_value: number
  repeat_policy: TaskRepeatPolicy
  whitelist: string[]
  blacklist: string[]
  conditions: ConditionDef[]
  status?: TaskStatus
}

/** 结算/发放记录。 */
export interface TaskReward {
  id: number
  task_id: number
  task_name?: string
  settle_date: string
  user_id: number
  email: string
  tokens: number
  units: number
  reward_type: PrizeType
  reward_value: number
  fulfillment: 'pending' | 'done' | 'failed'
  codes?: string[]
  note?: string
  error?: string
  created_at?: string
}

export interface TaskCode {
  id: number
  task_id: number
  code: string
  status: 'available' | 'granted'
  reward_id?: number
  created_at?: string
}

export interface IntegrationGroup {
  id: number
  name: string
  platform: string
  status: string
}

/** 用户侧：进行中的任务列表。 */
export async function listTasks(client: LotteryClient): Promise<TaskView[]> {
  const data = await unwrap<{ tasks: TaskView[] }>(client.http.get('/v1/tasks'))
  return data.tasks ?? []
}

/** 用户侧角标探测：有进行中任务 → 'active'。 */
export async function fetchTaskPhase(client: LotteryClient): Promise<'active' | 'none'> {
  const data = await unwrap<{ phase: 'active' | 'none' }>(client.http.get('/v1/me/tasks/phase'))
  return data.phase ?? 'none'
}

/** 用户侧：任务中心对该用户是否可见。 */
export async function fetchTaskVisibility(client: LotteryClient): Promise<boolean> {
  const data = await unwrap<{ visible: boolean }>(client.http.get('/v1/me/task-visibility'))
  return data.visible ?? false
}

/** 管理端：读取任务显隐设置。 */
export async function adminGetTaskSettings(
  client: LotteryClient
): Promise<{ visibility: VisibilitySettings }> {
  return unwrap(client.http.get('/v1/admin/task-settings'))
}

/** 管理端：保存任务显隐设置。 */
export async function adminSaveTaskSettings(
  client: LotteryClient,
  visibility: VisibilitySettings
): Promise<void> {
  await unwrap(client.http.put('/v1/admin/task-settings', { visibility }))
}

/** 用户侧：我的任务奖励/结算记录（含兑换码）。 */
export async function fetchMyTaskRewards(client: LotteryClient): Promise<TaskReward[]> {
  const data = await unwrap<{ rewards: TaskReward[] }>(client.http.get('/v1/me/task-rewards'))
  return data.rewards ?? []
}

/** 管理端：任务列表（含码池统计）。 */
export async function adminListTasks(client: LotteryClient): Promise<AdminTaskView[]> {
  const data = await unwrap<{ tasks: AdminTaskView[] }>(client.http.get('/v1/admin/tasks'))
  return data.tasks ?? []
}

/** 管理端：任务详情（含码池与发放记录）。 */
export async function adminGetTask(
  client: LotteryClient,
  id: number
): Promise<{ task: TaskView; codes: TaskCode[]; rewards: TaskReward[] }> {
  return unwrap(client.http.get(`/v1/admin/tasks/${id}`))
}

/** 管理端：创建任务。 */
export async function adminCreateTask(
  client: LotteryClient,
  input: TaskInput
): Promise<{ id: number }> {
  return unwrap<{ id: number }>(client.http.post('/v1/admin/tasks', input))
}

/** 管理端：更新任务。 */
export async function adminUpdateTask(
  client: LotteryClient,
  id: number,
  input: TaskInput
): Promise<{ id: number }> {
  return unwrap<{ id: number }>(client.http.put(`/v1/admin/tasks/${id}`, input))
}

/** 管理端：删除任务（含码池与结算记录）。 */
export async function adminDeleteTask(client: LotteryClient, id: number): Promise<void> {
  await unwrap(client.http.delete(`/v1/admin/tasks/${id}`))
}

/** 管理端：某任务的发放记录。 */
export async function adminListTaskRewards(client: LotteryClient, id: number): Promise<TaskReward[]> {
  const data = await unwrap<{ rewards: TaskReward[] }>(
    client.http.get(`/v1/admin/tasks/${id}/rewards`)
  )
  return data.rewards ?? []
}

/** 管理端：重试某任务失败发放（如补码后）。 */
export async function adminRetryTaskFulfillment(client: LotteryClient, id: number): Promise<void> {
  await unwrap(client.http.post(`/v1/admin/tasks/${id}/fulfill`))
}

/** 管理端：整体重置任务可用码池（granted 记录保留）。 */
export async function adminSetTaskCodes(
  client: LotteryClient,
  id: number,
  codes: string[]
): Promise<{ added: number; available: number; granted: number }> {
  return unwrap(client.http.put(`/v1/admin/tasks/${id}/codes`, { codes }))
}

/** 管理端：码池明细。 */
export async function adminListTaskCodes(
  client: LotteryClient,
  id: number
): Promise<{ codes: TaskCode[]; available: number; granted: number }> {
  return unwrap(client.http.get(`/v1/admin/tasks/${id}/codes`))
}

/** 管理端：分组下拉数据（代理 sub2api 集成接口）。 */
export async function adminTaskGroups(client: LotteryClient): Promise<IntegrationGroup[]> {
  const data = await unwrap<{ groups: IntegrationGroup[] }>(
    client.http.get('/v1/admin/task-options/groups')
  )
  return data.groups ?? []
}

/** 管理端：某分组的模型下拉数据。 */
export async function adminTaskGroupModels(
  client: LotteryClient,
  groupId: number
): Promise<string[]> {
  const data = await unwrap<{ models: string[] }>(
    client.http.get(`/v1/admin/task-options/groups/${groupId}/models`)
  )
  return data.models ?? []
}
