import { beforeEach, expect, it, vi } from 'vitest'
import { fetchManagedItems, fetchShopProducts, pointsAssetURL, saveManagedItem, saveShopProduct, setShopProductStatus, uploadPointsAsset } from '../pointsShop'

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }))
vi.mock('../lotteryClient', async () => ({ ...await vi.importActual('../lotteryClient'), createDefaultLotteryClient: () => ({ baseURL: '/lotteryd', http: client }) }))
beforeEach(() => { vi.clearAllMocks(); for (const method of Object.values(client)) method.mockResolvedValue({ data: { code: 0, data: { items: [] } } }) })

it('uploads an actual multipart file with its artwork slot and keeps stored asset URLs on the API proxy', async () => {
  const file = new File(['<svg/>'], 'cover.svg', { type: 'image/svg+xml' })
  await uploadPointsAsset(file, 'cover')
  const [path, body] = client.post.mock.calls[0]
  expect(path).toBe('/v1/admin/points/assets')
  expect(body.get('file')).toBe(file)
  expect(body.get('slot')).toBe('cover')
  expect(pointsAssetURL('abc')).toBe('/lotteryd/v1/points/assets/abc')
})

it('separates admin catalog calls from the published shop and changes status without overwriting price', async () => {
  await fetchManagedItems(); expect(client.get).toHaveBeenLastCalledWith('/v1/admin/points/items')
  await fetchShopProducts(); expect(client.get).toHaveBeenLastCalledWith('/v1/points/shop')
  await fetchShopProducts(true); expect(client.get).toHaveBeenLastCalledWith('/v1/admin/points/products')
  const item = { id: 0, code: 'gift', name: 'Gift', description: '', cover_asset: 'a', active_asset: 'b', expired_asset: 'c', animation_asset: 'd' }
  await saveManagedItem(item); expect(client.post).toHaveBeenLastCalledWith('/v1/admin/points/items', item)
  await saveManagedItem({ ...item, id: 2 }); expect(client.put).toHaveBeenLastCalledWith('/v1/admin/points/items/2', { ...item, id: 2 })
  const product = { id: 0, kind: 'custom' as const, item_id: null, icon_asset: 'a', name: 'Gift', description: 'Gift card', price: '100', status: 'draft' as const, sort_order: 0 }
  await saveShopProduct(product); expect(client.post).toHaveBeenLastCalledWith('/v1/admin/points/products', product)
  await setShopProductStatus(2, 'published'); expect(client.put).toHaveBeenLastCalledWith('/v1/admin/points/products/2/status', { status: 'published' })
})
