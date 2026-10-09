import { describe, expect, it } from 'vitest'
import { decodeVapidPublicKey } from '@/services/pushNotifications'

describe('push notification helpers', () => {
  it('decodes URL-safe VAPID public keys', () => {
    expect(Array.from(decodeVapidPublicKey('AQID-v8'))).toEqual([1, 2, 3, 250, 255])
  })
})
