import {
  getPushConfig,
  removePushSubscription,
  savePushSubscription,
  type PushSubscriptionPayload,
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

function serializeSubscription(subscription: PushSubscription): PushSubscriptionPayload {
  const value = subscription.toJSON()
  if (!value.endpoint || !value.keys?.p256dh || !value.keys?.auth) {
    throw new Error('Browser returned an incomplete push subscription')
  }
  return {
    endpoint: value.endpoint,
    expirationTime: value.expirationTime,
    keys: { p256dh: value.keys.p256dh, auth: value.keys.auth },
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
  const worker = await registration()
  const subscription = await worker.pushManager.getSubscription()
  if (!subscription) return 'disabled'
  if (sync) await savePushSubscription(serializeSubscription(subscription))
  return 'enabled'
}

export async function enablePushNotifications(): Promise<PushPermissionState> {
  if (!isPushSupported()) return 'unsupported'
  const permission = await Notification.requestPermission()
  if (permission !== 'granted') return permission === 'denied' ? 'denied' : 'disabled'

  const worker = await registration()
  let subscription = await worker.pushManager.getSubscription()
  if (!subscription) {
    const config = await getPushConfig()
    subscription = await worker.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: decodeVapidPublicKey(config.publicKey),
    })
  }

  try {
    await savePushSubscription(serializeSubscription(subscription))
  } catch (error) {
    await subscription.unsubscribe().catch(() => false)
    throw error
  }
  return 'enabled'
}

export async function disablePushNotifications(): Promise<PushPermissionState> {
  if (!isPushSupported()) return 'unsupported'
  const worker = await registration()
  const subscription = await worker.pushManager.getSubscription()
  if (!subscription) return 'disabled'
  await removePushSubscription(subscription.endpoint)
  await subscription.unsubscribe()
  return 'disabled'
}
