import { apiClient } from '../client'

export interface IQDetectAgentHealth {
  enabled: boolean
  reason?: string
  base_url: string
  status?: string
  version?: string
  uptime_seconds?: number
}

// The IQ detection admin page is still being designed; for now the menu only
// surfaces the sidecar health. More proxied endpoints (models, runs, scores)
// will be added with the page itself.
export async function getHealth(): Promise<IQDetectAgentHealth> {
  const { data } = await apiClient.get<IQDetectAgentHealth>('/admin/iq-detection/agent/health')
  return data
}

export interface SelectionAccount {
  id: number
  name?: string
  models?: string[]
}

export interface SelectionGroup {
  id: number
  name: string
  rate_multiplier: number
  // 该分组智商检测的时间间隔（分钟），0 = 仅手动触发
  interval_minutes?: number
  accounts: SelectionAccount[]
}

export interface DetectionSelection {
  groups: SelectionGroup[]
  updated_at?: string | null
}

export async function getSelection(): Promise<DetectionSelection> {
  const { data } = await apiClient.get<DetectionSelection>('/admin/iq-detection/selection')
  return { groups: data?.groups || [], updated_at: data?.updated_at ?? null }
}

export async function saveSelection(groups: SelectionGroup[]): Promise<DetectionSelection> {
  const { data } = await apiClient.put<DetectionSelection>('/admin/iq-detection/selection', { groups })
  return data
}

export interface DetectionModel {
  id: number
  name: string
  created_at: string
}

export interface DetectionModelSettings {
  items: DetectionModel[]
  total: number
  pelican_prompt: string
  candy_prompt: string
  candy_answer?: string
  reasoning_effort?: string
  temperature?: number | null
}

export interface SaveDetectionSettingsPayload {
  names: string[]
  pelican_prompt: string
  candy_prompt: string
  candy_answer?: string
  reasoning_effort?: string
  temperature?: number | null
}

export async function getDetectionModels(): Promise<DetectionModelSettings> {
  const { data } = await apiClient.get<DetectionModelSettings>('/admin/iq-detection/detection-models')
  return {
    items: data?.items || [],
    total: data?.total || 0,
    pelican_prompt: data?.pelican_prompt || '',
    candy_prompt: data?.candy_prompt || '',
    candy_answer: data?.candy_answer || '',
    reasoning_effort: data?.reasoning_effort || '',
    temperature: data?.temperature ?? null
  }
}

export async function saveDetectionModels(
  payload: SaveDetectionSettingsPayload
): Promise<DetectionModelSettings> {
  const { data } = await apiClient.put<DetectionModelSettings>(
    '/admin/iq-detection/detection-models',
    payload
  )
  return data
}

export interface TestRunPlanItem {
  model: string
  account_id?: number
  account_name?: string
  skipped: boolean
  reason?: string
}

export interface TestRunPlan {
  items: TestRunPlanItem[]
  triggered: number
}

export async function triggerTestRun(groupID?: number): Promise<TestRunPlan> {
  const { data } = await apiClient.post<TestRunPlan>('/admin/iq-detection/test-runs', {
    group_id: groupID || 0
  })
  return { items: data?.items || [], triggered: data?.triggered || 0 }
}

export interface TestResultItem {
  id: number
  group_id: number
  group_name: string
  account_id: number
  account_name: string
  model: string
  status: 'running' | 'completed' | 'failed'
  // 列表接口不返回 HTML 正文，只带长度；正文通过 getTestResultPelican 按需拉取
  pelican_html_len?: number
  pelican_error?: string
  pelican_latency_ms?: number
  candy_answer?: string
  candy_error?: string
  candy_latency_ms?: number
  candy_correct?: boolean | null
  pelican_started_at?: string
  pelican_finished_at?: string
  candy_started_at?: string
  candy_finished_at?: string
  pelican_review: 'pending' | 'approved' | 'rejected'
  candy_review: 'pending' | 'approved' | 'rejected'
  created_at: string
}

// groupID 传 0 表示只查未分组的历史结果；不传（undefined）返回全部
export async function listTestResults(limit = 20, offset = 0, groupID?: number): Promise<{
  items: TestResultItem[]
  total: number
}> {
  const { data } = await apiClient.get<{ items: TestResultItem[]; total: number }>(
    '/admin/iq-detection/test-results',
    { params: { limit, offset, ...(groupID === undefined ? {} : { group_id: groupID }) } }
  )
  return { items: data?.items || [], total: data?.total || 0 }
}

export async function getTestResultPelican(id: number): Promise<string> {
  const { data } = await apiClient.get<{ html: string }>(
    `/admin/iq-detection/test-results/${id}/pelican`
  )
  return data?.html || ''
}

export type TestResultProbe = 'pelican' | 'candy'
export type TestResultVerdict = 'approved' | 'rejected'

export async function updateTestResultReview(
  id: number,
  probe: TestResultProbe,
  verdict: TestResultVerdict
): Promise<void> {
  await apiClient.put(`/admin/iq-detection/test-results/${id}/review`, { probe, verdict })
}

const iqDetectionAPI = {
  getHealth,
  getSelection,
  saveSelection,
  getDetectionModels,
  saveDetectionModels,
  triggerTestRun,
  listTestResults,
  getTestResultPelican,
  updateTestResultReview
}
export default iqDetectionAPI
