const PUSH_API_BASE = '/push-api/v1'

export interface PushSubscriptionPayload {
  endpoint: string
  expirationTime?: number | null
  keys: {
    p256dh: string
    auth: string
  }
}

export interface PushOverview {
  activeSubscriptions: number
  messageCount: number
  lastSentAt: string | null
}

export interface PushMessage {
  id: string
  title: string
  body: string
  url: string
  image?: string
  createdAt: string
  delivered: number
  failed: number
  removed: number
}

export interface BroadcastPushRequest {
  title: string
  body: string
  url: string
  image?: string
}

async function request<T>(path: string, options: RequestInit = {}, admin = false): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Accept', 'application/json')
  if (options.body) headers.set('Content-Type', 'application/json')
  if (admin) {
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

export function savePushSubscription(subscription: PushSubscriptionPayload): Promise<void> {
  return request('/subscriptions', { method: 'POST', body: JSON.stringify(subscription) })
}

export function removePushSubscription(endpoint: string): Promise<{ removed: boolean }> {
  return request('/subscriptions', { method: 'DELETE', body: JSON.stringify({ endpoint }) })
}

export function getPushOverview(): Promise<PushOverview> {
  return request('/admin/overview', {}, true)
}

export async function listPushMessages(limit = 20): Promise<PushMessage[]> {
  const response = await request<{ messages: PushMessage[] }>(`/admin/messages?limit=${limit}`, {}, true)
  return response.messages
}

export function deletePushMessage(id: string): Promise<{ removed: boolean; overview: PushOverview }> {
  return request(`/admin/messages/${encodeURIComponent(id)}`, { method: 'DELETE' }, true)
}

export async function broadcastPush(input: BroadcastPushRequest): Promise<PushMessage> {
  const response = await request<{ message: PushMessage }>(
    '/admin/broadcast',
    { method: 'POST', body: JSON.stringify(input) },
    true,
  )
  return response.message
}
