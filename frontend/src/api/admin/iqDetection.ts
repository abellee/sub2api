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

const iqDetectionAPI = { getHealth, listAccountCredentials, saveAccountCredential }
export default iqDetectionAPI
