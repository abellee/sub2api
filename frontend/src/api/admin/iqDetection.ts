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

export interface SavedAccountCredential {
  account_id: number
  api_url: string
  has_api_key: boolean
  updated_at: string
}

export interface SaveAccountCredentialPayload {
  account_id: number
  api_url: string
  api_key?: string
}

export async function listAccountCredentials(): Promise<{ items: SavedAccountCredential[]; total: number }> {
  const { data } = await apiClient.get<{ items: SavedAccountCredential[]; total: number }>(
    '/admin/iq-detection/account-credentials'
  )
  return { items: data?.items || [], total: data?.total || 0 }
}

export async function saveAccountCredential(
  payload: SaveAccountCredentialPayload
): Promise<SavedAccountCredential> {
  const { data } = await apiClient.put<SavedAccountCredential>(
    '/admin/iq-detection/account-credentials',
    payload
  )
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
  reasoning_effort?: string
  temperature?: number | null
}

export interface SaveDetectionSettingsPayload {
  names: string[]
  pelican_prompt: string
  candy_prompt: string
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

export async function triggerTestRun(): Promise<TestRunPlan> {
  const { data } = await apiClient.post<TestRunPlan>('/admin/iq-detection/test-runs', {})
  return { items: data?.items || [], triggered: data?.triggered || 0 }
}

const iqDetectionAPI = {
  getHealth,
  listAccountCredentials,
  saveAccountCredential,
  getSelection,
  saveSelection,
  getDetectionModels,
  saveDetectionModels,
  triggerTestRun
}
export default iqDetectionAPI
