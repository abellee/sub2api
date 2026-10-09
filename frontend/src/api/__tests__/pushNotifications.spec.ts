import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  broadcastPush,
  listPageNotifications,
  clearPushMessages,
  deletePushMessage,
  getPushOverview,
  savePushSubscription,
  uploadPushImage,
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

  it('sends the selected channels and reads page notices', async () => {
    localStorage.setItem('auth_token', 'user-token')
    const fetchMock = vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response(JSON.stringify({
        message: {
          id: 'message-id',
          title: '页面',
          body: '右下角',
          url: '/',
          channels: ['page'],
          createdAt: '2026-10-10T00:00:00Z',
          delivered: 0,
          failed: 0,
          removed: 0,
        },
      }), { status: 201, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        notices: [],
        now: '2026-10-10T00:00:01Z',
      }), { status: 200, headers: { 'Content-Type': 'application/json' } }))

    await broadcastPush({
      title: '页面',
      body: '右下角',
      url: '/',
      channels: ['page'],
    })
    const sent = JSON.parse(String(fetchMock.mock.calls[0][1]?.body))
    expect(sent.channels).toEqual(['page'])

    await expect(listPageNotifications('2026-10-10T00:00:00Z')).resolves.toMatchObject({ notices: [] })
    expect(fetchMock).toHaveBeenLastCalledWith(
      '/push-api/v1/page-notifications?since=2026-10-10T00%3A00%3A00Z',
      expect.objectContaining({}),
    )
    const headers = fetchMock.mock.calls[1][1]?.headers as Headers
    expect(headers.get('Authorization')).toBe('Bearer user-token')
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

  it('uploads the image file and turns the stored path into an absolute link', async () => {
    localStorage.setItem('auth_token', 'admin-token')
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(
      JSON.stringify({ url: '/push-api/v1/images/abc.png' }),
      { status: 201, headers: { 'Content-Type': 'application/json' } },
    ))
    const file = new File([Uint8Array.from([1, 2, 3])], 'notice.png', { type: 'image/png' })

    await expect(uploadPushImage(file)).resolves.toBe(`${window.location.origin}/push-api/v1/images/abc.png`)
    expect(fetchMock).toHaveBeenCalledWith('/push-api/v1/admin/images', expect.objectContaining({
      method: 'POST',
      body: file,
    }))
    const headers = fetchMock.mock.calls[0][1]?.headers as Headers
    expect(headers.get('Authorization')).toBe('Bearer admin-token')
    expect(headers.get('Content-Type')).toBe('image/png')
    expect(headers.get('Content-Type')).not.toBe('application/json')
  })
})
