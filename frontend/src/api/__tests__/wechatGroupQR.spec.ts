import { beforeEach, describe, expect, it, vi } from 'vitest'

const { getMock, postMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  postMock: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get: getMock,
    post: postMock
  }
}))

import { getWechatGroupQR, uploadWechatGroupQR } from '@/api/wechatGroupQR'

describe('WeChat group QR API', () => {
  beforeEach(() => {
    getMock.mockReset()
    postMock.mockReset()
  })

  it('loads the public QR configuration', async () => {
    const info = {
      image_url: '/api/v1/community/wechat-group-qr/image?v=1',
      updated_at: '2026-08-25T04:00:00Z',
      custom: true
    }
    getMock.mockResolvedValue({ data: info })

    await expect(getWechatGroupQR()).resolves.toEqual(info)
    expect(getMock).toHaveBeenCalledWith('/community/wechat-group-qr')
  })

  it('uploads the selected image in the expected multipart field', async () => {
    const file = new File(['image'], 'wechat.png', { type: 'image/png' })
    const info = { image_url: '/api/v1/community/wechat-group-qr/image?v=2', custom: true }
    postMock.mockResolvedValue({ data: info })

    await expect(uploadWechatGroupQR(file, '2026-08-25T05:00:00.000Z')).resolves.toEqual(info)
    expect(postMock).toHaveBeenCalledOnce()
    const [url, form, config] = postMock.mock.calls[0]
    expect(url).toBe('/admin/wechat-group-qr')
    expect(form).toBeInstanceOf(FormData)
    expect((form as FormData).get('image')).toBe(file)
    expect((form as FormData).get('expires_at')).toBe('2026-08-25T05:00:00.000Z')
    expect(config).toMatchObject({
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 60000
    })
  })
})
