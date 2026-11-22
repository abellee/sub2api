import {
  disablePushSubscription,
  getPushConfig,
  getPushSubscriptionStatus,
  savePushSubscription,
  type PushSubscriptionPayload,
  type PushSubscriptionSaveResult,
} from '@/api/pushNotifications'

export type PushPermissionState = 'unsupported' | 'denied' | 'disabled' | 'enabled'

export function isPushSupported(): boolean {
  return 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window
}

export function decodeVapidPublicKey(value: string): Uint8Array {
  const padding = '='.repeat((4 - (value.length % 4)) % 4)
  const base64 = (value + padding).replace(/-/g, '+').replace(/_/g, '/')
  const raw = atob(base64)
  return Uint8Array.from(raw, (character) => character.charCodeAt(0))
}

const CLIENT_ID_KEY = 'push_client_id'
const CLIENT_BOUND_KEY = 'push_client_bound'
const CLIENT_ID_PATTERN = /^[A-Za-z0-9_-]{8,128}$/

export function readPushClientId(): string | null {
  try {
    const existing = localStorage.getItem(CLIENT_ID_KEY)
    return existing && CLIENT_ID_PATTERN.test(existing) ? existing : null
  } catch {
    return null
  }
}

export function getOrCreatePushClientId(): string {
  const existing = readPushClientId()
  if (existing) return existing
  const created = crypto.randomUUID()
  localStorage.setItem(CLIENT_ID_KEY, created)
  return created
}

function rememberPushBinding(result: PushSubscriptionSaveResult) {
  if (result.subscription.bound) localStorage.setItem(CLIENT_BOUND_KEY, '1')
}

function browserProfile(): PushSubscriptionPayload['user'] {
  if (localStorage.getItem(CLIENT_BOUND_KEY) === '1') return undefined
  try {
    const savedUser = localStorage.getItem('auth_user')
    if (!savedUser) return undefined
    const parsed = JSON.parse(savedUser)
    const id = Number(parsed?.id)
    if (!Number.isSafeInteger(id) || id <= 0) return undefined
    return { id, username: String(parsed?.username || ''), email: String(parsed?.email || '') }
  } catch {
    return undefined
  }
}

function serializeSubscription(subscription: PushSubscription, clientId: string, enabled?: boolean): PushSubscriptionPayload {
  const value = subscription.toJSON()
  if (!value.endpoint || !value.keys?.p256dh || !value.keys?.auth) {
    throw new Error('Browser returned an incomplete push subscription')
  }
  return {
    clientId,
    endpoint: value.endpoint,
    expirationTime: value.expirationTime,
    keys: { p256dh: value.keys.p256dh, auth: value.keys.auth },
    ...(enabled === undefined ? {} : { enabled }),
    user: browserProfile(),
  }
}

async function registration(): Promise<ServiceWorkerRegistration> {
  return navigator.serviceWorker.register('/push-worker.js', { scope: '/' })
}

export async function getCurrentPushSubscription(): Promise<PushSubscription | null> {
  if (!isPushSupported()) return null
  const worker = await registration()
  return worker.pushManager.getSubscription()
}

export async function getPushPermissionState(sync = false): Promise<PushPermissionState> {
  if (!isPushSupported()) return 'unsupported'
  if (Notification.permission === 'denied') return 'denied'
  const subscription = await getCurrentPushSubscription()
  const clientId = readPushClientId()
  if (!subscription) return 'disabled'
  if (sync) {
    const saved = await savePushSubscription(serializeSubscription(subscription, clientId || getOrCreatePushClientId()))
    rememberPushBinding(saved)
    return saved.subscription.enabled ? 'enabled' : 'disabled'
  }
  if (!clientId) return 'enabled'
  try {
    const status = await getPushSubscriptionStatus(clientId)
    if (status.bound) localStorage.setItem(CLIENT_BOUND_KEY, '1')
    if (!status.found) return 'enabled'
    return status.enabled ? 'enabled' : 'disabled'
  } catch {
    return 'enabled'
  }
}

export async function enablePushNotifications(): Promise<PushPermissionState> {
  if (!isPushSupported()) return 'unsupported'
  const permission = await Notification.requestPermission()
  if (permission !== 'granted') return permission === 'denied' ? 'denied' : 'disabled'

  const worker = await registration()
  let subscription = await worker.pushManager.getSubscription()
  const createdSubscription = !subscription
  if (!subscription) {
    const config = await getPushConfig()
    subscription = await worker.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: decodeVapidPublicKey(config.publicKey),
    })
  }

  try {
    const saved = await savePushSubscription(serializeSubscription(subscription, getOrCreatePushClientId(), true))
    rememberPushBinding(saved)
  } catch (error) {
    if (createdSubscription) await subscription.unsubscribe().catch(() => false)
    throw error
  }
  return 'enabled'
}

export async function disablePushNotifications(): Promise<PushPermissionState> {
  if (!isPushSupported()) return 'unsupported'
  const clientId = readPushClientId()
  const subscription = await getCurrentPushSubscription()
  if (subscription) {
    const saved = await savePushSubscription(serializeSubscription(subscription, clientId || getOrCreatePushClientId(), false))
    rememberPushBinding(saved)
    return 'disabled'
  }
  if (clientId) await disablePushSubscription(clientId)
  return 'disabled'
}
