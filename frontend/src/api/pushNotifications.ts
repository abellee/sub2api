const PUSH_API_BASE = '/push-api/v1'

export interface PushSubscriptionPayload {
  clientId?: string
  endpoint: string
  expirationTime?: number | null
  keys: {
    p256dh: string
    auth: string
  }
  enabled?: boolean
  user?: { id: number; username?: string; email?: string }
}

export interface PushSubscriptionSaveResult {
  subscription: {
    id: string
    createdAt: string
    lastSeenAt: string
    enabled: boolean
    bound: boolean
  }
}

export interface PushSubscriptionStatus {
  found: boolean
  enabled: boolean
  bound: boolean
}

export interface PushOverview {
  activeSubscriptions: number
  messageCount: number
  lastSentAt: string | null
}

export interface PushSubscriptionSummary {
  id: string
  user: { id: number; username: string; email: string } | null
  enabled?: boolean
  createdAt: string
  lastSeenAt: string
}

export type PushChannel = 'browser' | 'page'

export interface PushSchedule {
  id: string
  title: string
  body: string
  url: string
  image?: string
  channels?: PushChannel[]
  scheduledAt: string
  status: 'pending' | 'processing' | 'failed'
  createdAt: string
  error?: string
  failedAt?: string
}

export interface PushMessage {
  id: string
  title: string
  body: string
  url: string
  image?: string
  channels?: PushChannel[]
  createdAt: string
  delivered: number
  failed: number
  removed: number
}

export interface PageNotice {
  id: string
  title: string
  body: string
  url: string
  image?: string
  createdAt: string
}

export interface BroadcastPushRequest {
  title: string
  body: string
  url: string
  image?: string
  channels?: PushChannel[]
}

async function request<T>(path: string, options: RequestInit = {}, admin = false, attachAuth = false): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Accept', 'application/json')
  if (options.body) headers.set('Content-Type', 'application/json')
  if (admin || attachAuth) {
    const token = localStorage.getItem('auth_token')
    if (token) headers.set('Authorization', `Bearer ${token}`)
  }

  const response = await fetch(`${PUSH_API_BASE}${path}`, { ...options, headers })
  const payload = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new Error(payload.message || `Push service request failed (${response.status})`)
  }
  return payload as T
}

export function getPushConfig(): Promise<{ publicKey: string }> {
  return request('/config')
}

export function savePushSubscription(subscription: PushSubscriptionPayload): Promise<PushSubscriptionSaveResult> {
  return request('/subscriptions', { method: 'POST', body: JSON.stringify(subscription) }, false, true)
}

export function getPushSubscriptionStatus(clientId: string): Promise<PushSubscriptionStatus> {
  return request(`/subscriptions/status?clientId=${encodeURIComponent(clientId)}`)
}

export function disablePushSubscription(clientId: string): Promise<{ removed: boolean; disabled: boolean }> {
  return request('/subscriptions', { method: 'DELETE', body: JSON.stringify({ clientId }) }, false, true)
}

export function getPushOverview(): Promise<PushOverview> {
  return request('/admin/overview', {}, true)
}

export async function listPushSubscriptions(): Promise<PushSubscriptionSummary[]> {
  const response = await request<{ subscriptions: PushSubscriptionSummary[] }>('/admin/subscriptions', {}, true)
  return response.subscriptions
}

export async function listPushSchedules(): Promise<PushSchedule[]> {
  const response = await request<{ schedules: PushSchedule[] }>('/admin/schedules', {}, true)
  return response.schedules
}

export function listPageNotifications(since = ''): Promise<{ notices: PageNotice[]; now: string }> {
  const query = since ? `?since=${encodeURIComponent(since)}` : ''
  return request(`/page-notifications${query}`, {}, false, true)
}

export function createPushSchedule(input: {
  title: string
  body: string
  url: string
  image?: string
  channels?: PushChannel[]
  scheduledAt: string
}): Promise<PushSchedule> {
  return request<{ schedule: PushSchedule }>('/admin/schedules', {
    method: 'POST',
    body: JSON.stringify(input),
  }, true).then((response) => response.schedule)
}

export function deletePushSchedule(id: string): Promise<{ removed: boolean }> {
  return request(`/admin/schedules/${encodeURIComponent(id)}`, { method: 'DELETE' }, true)
}

export async function listPushMessages(limit = 20): Promise<PushMessage[]> {
  const response = await request<{ messages: PushMessage[] }>(`/admin/messages?limit=${limit}`, {}, true)
  return response.messages
}

export function deletePushMessage(id: string): Promise<{ removed: boolean; overview: PushOverview }> {
  return request(`/admin/messages/${encodeURIComponent(id)}`, { method: 'DELETE' }, true)
}

export function clearPushMessages(): Promise<{ removed: number; overview: PushOverview }> {
  return request('/admin/messages', { method: 'DELETE' }, true)
}

export async function uploadPushImage(file: Blob): Promise<string> {
  const headers = new Headers()
  headers.set('Accept', 'application/json')
  headers.set('Content-Type', file.type || 'application/octet-stream')
  const token = localStorage.getItem('auth_token')
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const response = await fetch(`${PUSH_API_BASE}/admin/images`, { method: 'POST', headers, body: file })
  const payload = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new Error(payload.message || `Push service request failed (${response.status})`)
  }
  const imagePath = String(payload.url || '')
  if (!imagePath.startsWith('/')) throw new Error('image upload did not return a link')
  return new URL(imagePath, window.location.origin).href
}

export async function broadcastPush(input: BroadcastPushRequest): Promise<PushMessage> {
  const response = await request<{ message: PushMessage }>(
    '/admin/broadcast',
    { method: 'POST', body: JSON.stringify(input) },
    true,
  )
  return response.message
}
