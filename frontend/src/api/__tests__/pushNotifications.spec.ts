import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  broadcastPush,
  clearPushMessages,
  deletePushMessage,
  getPushOverview,
  savePushSubscription,
} from '@/api/pushNotifications'

describe('push notification API', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.restoreAllMocks()
  })

  it('stores browser subscriptions without requiring a login token', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(
      JSON.stringify({ subscription: { id: 'subscription-id' } }),
      { status: 201, headers: { 'Content-Type': 'application/json' } },
    ))

    await savePushSubscription({
      endpoint: 'https://push.example.test/1',
      keys: { p256dh: 'public-key', auth: 'auth-key' },
    })

    expect(fetchMock).toHaveBeenCalledWith('/push-api/v1/subscriptions', expect.objectContaining({ method: 'POST' }))
    const headers = fetchMock.mock.calls[0][1]?.headers as Headers
    expect(headers.has('Authorization')).toBe(false)
  })

  it('forwards the existing sub2api token to administrator requests', async () => {
    localStorage.setItem('auth_token', 'admin-token')
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(
      JSON.stringify({ activeSubscriptions: 3, messageCount: 4, lastSentAt: null }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    ))

    await expect(getPushOverview()).resolves.toMatchObject({ activeSubscriptions: 3 })
    const headers = fetchMock.mock.calls[0][1]?.headers as Headers
    expect(headers.get('Authorization')).toBe('Bearer admin-token')
  })

  it('returns the recorded broadcast result', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      message: {
        id: 'message-id',
        title: '倍率调整',
        body: '倍率现已调整为 0.08x',
        url: '/model-plaza',
        image: 'https://cdn.example.com/rate-update.jpg',
        createdAt: '2026-08-25T00:00:00Z',
        delivered: 8,
        failed: 1,
        removed: 0,
      },
    }), { status: 201, headers: { 'Content-Type': 'application/json' } }))

    const result = await broadcastPush({
      title: '倍率调整',
      body: '倍率现已调整为 0.08x',
      url: '/model-plaza',
      image: 'https://cdn.example.com/rate-update.jpg',
    })
    expect(result.delivered).toBe(8)
    expect(result.failed).toBe(1)
    expect(result.image).toBe('https://cdn.example.com/rate-update.jpg')
  })

  it('deletes a recorded notification with administrator authorization', async () => {
    localStorage.setItem('auth_token', 'admin-token')
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      removed: true,
      overview: { activeSubscriptions: 2, messageCount: 3, lastSentAt: null },
    }), { status: 200, headers: { 'Content-Type': 'application/json' } }))

    await expect(deletePushMessage('message/id')).resolves.toMatchObject({ removed: true })
    expect(fetchMock).toHaveBeenCalledWith(
      '/push-api/v1/admin/messages/message%2Fid',
      expect.objectContaining({ method: 'DELETE' }),
    )
    const headers = fetchMock.mock.calls[0][1]?.headers as Headers
    expect(headers.get('Authorization')).toBe('Bearer admin-token')
  })

  it('clears all recorded notifications with administrator authorization', async () => {
    localStorage.setItem('auth_token', 'admin-token')
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      removed: 3,
      overview: { activeSubscriptions: 2, messageCount: 0, lastSentAt: null },
    }), { status: 200, headers: { 'Content-Type': 'application/json' } }))

    await expect(clearPushMessages()).resolves.toMatchObject({ removed: 3 })
    expect(fetchMock).toHaveBeenCalledWith(
      '/push-api/v1/admin/messages',
      expect.objectContaining({ method: 'DELETE' }),
    )
  })
})
