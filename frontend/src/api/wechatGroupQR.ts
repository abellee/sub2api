import { apiClient } from './client'

export interface WechatGroupQRInfo {
  image_url: string
  updated_at?: string
  expires_at?: string
  custom: boolean
  expired?: boolean
}

export const WECHAT_GROUP_QR_UPDATED_EVENT = 'wechat-group-qr-updated'

export async function getWechatGroupQR(): Promise<WechatGroupQRInfo> {
  const { data } = await apiClient.get<WechatGroupQRInfo>('/community/wechat-group-qr')
  return data
}

export async function uploadWechatGroupQR(file: File, expiresAt?: string): Promise<WechatGroupQRInfo> {
  const form = new FormData()
  form.append('image', file)
  if (expiresAt) form.append('expires_at', expiresAt)
  const { data } = await apiClient.post<WechatGroupQRInfo>('/admin/wechat-group-qr', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 60000
  })
  return data
}
