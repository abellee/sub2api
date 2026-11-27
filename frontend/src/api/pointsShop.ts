import { createDefaultLotteryClient, unwrap } from './lotteryClient'

export interface ManagedItem {
  id: number
  code: string
  name: string
  description: string
  cover_asset: string
  active_asset: string
  expired_asset: string
  animation_asset: string
  effect?: { kind: 'none' | 'grant_points' | 'grant_item'; points?: number; item_id?: number; quantity?: number }
}
export interface ShopProduct {
  id: number
  kind: 'item' | 'custom'
  item_id: number | null
  icon_asset: string
  name: string
  description: string
  price: string
  status: 'draft' | 'published'
  sort_order: number
}
export type AssetSlot = 'cover' | 'active' | 'expired' | 'animation' | 'icon'
const client = createDefaultLotteryClient()
export function pointsAssetURL(id: string) {
  return `${client.baseURL.replace(/\/$/, '')}/v1/points/assets/${encodeURIComponent(id)}`
}
export async function uploadPointsAsset(file: File, slot: AssetSlot): Promise<{ id: string; kind: string }> {
  const body = new FormData()
  body.append('file', file)
  body.append('slot', slot)
  return unwrap(client.http.post('/v1/admin/points/assets', body))
}
export async function fetchPointsAnimation(id: string): Promise<unknown> {
  return (await client.http.get(`/v1/points/assets/${encodeURIComponent(id)}`)).data
}
export async function fetchManagedItems(): Promise<ManagedItem[]> {
  return (await unwrap<{ items: ManagedItem[] }>(client.http.get('/v1/admin/points/items'))).items
}
export function saveManagedItem(item: ManagedItem): Promise<ManagedItem> {
  return unwrap(item.id ? client.http.put(`/v1/admin/points/items/${item.id}`, item) : client.http.post('/v1/admin/points/items', item))
}
export async function fetchShopProducts(admin = false): Promise<ShopProduct[]> {
  return (await unwrap<{ items: ShopProduct[] }>(client.http.get(admin ? '/v1/admin/points/products' : '/v1/points/shop'))).items
}
export function saveShopProduct(product: ShopProduct): Promise<ShopProduct> {
  return unwrap(product.id ? client.http.put(`/v1/admin/points/products/${product.id}`, product) : client.http.post('/v1/admin/points/products', product))
}
export function setShopProductStatus(id: number, status: ShopProduct['status']): Promise<{ saved: boolean }> {
  return unwrap(client.http.put(`/v1/admin/points/products/${id}/status`, { status }))
}
