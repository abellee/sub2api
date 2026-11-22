import apiClient from '../client'

export interface ContentSafetyBillingConfig {
  enabled: boolean
  amount: number
  match_description: string
  client_message: string
}

export interface ContentSafetyDebit {
  id: string
  user_id: number
  api_key_id: number
  account_id: number
  upstream_request_id: string
  amount: number
  notes: string
  client_message: string
  status: 'pending' | 'charged'
  created_at: string
  charged_at: string
  last_error: string
}

export interface ContentSafetyDebitPage {
  items: ContentSafetyDebit[]
  total: number
  page: number
  page_size: number
}

const endpoint = '/admin/content-safety-billing'

export async function getContentSafetyBilling(): Promise<ContentSafetyBillingConfig> {
  return (await apiClient.get<ContentSafetyBillingConfig>(endpoint)).data
}

export async function updateContentSafetyBilling(config: ContentSafetyBillingConfig): Promise<ContentSafetyBillingConfig> {
  return (await apiClient.put<ContentSafetyBillingConfig>(endpoint, config)).data
}

export async function getContentSafetyDebits(page = 1): Promise<ContentSafetyDebitPage> {
  return (await apiClient.get<ContentSafetyDebitPage>(`${endpoint}/records`, { params: { page, page_size: 20 } })).data
}
