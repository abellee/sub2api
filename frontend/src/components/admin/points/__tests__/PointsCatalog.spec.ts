import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import PointsItemsPanel from '../PointsItemsPanel.vue'
import PointsProductsPanel from '../PointsProductsPanel.vue'

const api = vi.hoisted(() => ({ fetchManagedItems: vi.fn(), fetchShopProducts: vi.fn(), saveManagedItem: vi.fn(), saveShopProduct: vi.fn(), setShopProductStatus: vi.fn(), showError: vi.fn(), showSuccess: vi.fn() }))
vi.mock('@/api/pointsShop', () => api)
vi.mock('@/stores', () => ({ useAppStore: () => api }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: ref('en') }) }))
vi.mock('@/components/user/PointsAssetPreview.vue', () => ({ default: { template: '<div />' } }))
vi.mock('../PointsAssetUpload.vue', () => ({ default: { props: ['assetSlot', 'modelValue'], emits: ['update:modelValue', 'busy'], template: '<button type="button" :data-slot="assetSlot" @click="$emit(\'update:modelValue\', assetSlot + \'-asset\')">upload</button>' } }))
const global = { stubs: {
  BaseDialog: { props: ['show'], template: '<div v-if="show" role="dialog"><slot /></div>' },
  Select: { props: ['modelValue', 'options', 'id'], emits: ['update:modelValue'], template: '<div :id="id"><button type="button" v-for="o in options" :key="o.value" :data-value="o.value" @click="$emit(\'update:modelValue\',o.value)">{{ o.label }}</button></div>' }
} }
const item = { id: 2, code: 'gift', name: 'Gift', description: 'A gift', cover_asset: 'a', active_asset: 'b', expired_asset: 'c', animation_asset: 'd' }
const product = { id: 5, kind: 'custom' as const, item_id: null, icon_asset: 'icon', name: 'Custom gift', description: 'A gift', price: '100', status: 'draft' as const, sort_order: 0 }
enableAutoUnmount(afterEach)
beforeEach(() => { vi.clearAllMocks(); api.fetchManagedItems.mockResolvedValue([item]); api.fetchShopProducts.mockResolvedValue([product]); api.saveManagedItem.mockResolvedValue(item); api.saveShopProduct.mockResolvedValue(product); api.setShopProductStatus.mockResolvedValue({ saved: true }) })

it('requires all four artwork assets before a new item can be saved', async () => {
  const wrapper = mount(PointsItemsPanel, { global })
  await flushPromises()
  await wrapper.get('section > div button').trigger('click')
  await wrapper.get('#item-code').setValue('new-gift'); await wrapper.get('#item-name').setValue('New gift')
  expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
  for (const slot of ['cover', 'active', 'expired']) await wrapper.get(`[data-slot="${slot}"]`).trigger('click')
  await wrapper.get('form').trigger('submit'); expect(api.saveManagedItem).not.toHaveBeenCalled()
  await wrapper.get('[data-slot="animation"]').trigger('click')
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(api.saveManagedItem).toHaveBeenCalledWith(expect.objectContaining({ code: 'new-gift', cover_asset: 'cover-asset', active_asset: 'active-asset', expired_asset: 'expired-asset', animation_asset: 'animation-asset' }))
})

it('changes publication status using a dedicated call', async () => {
  const wrapper = mount(PointsProductsPanel, { global })
  await flushPromises()
  await wrapper.get('article button:last-child').trigger('click'); await flushPromises()
  expect(api.setShopProductStatus).toHaveBeenCalledWith(5, 'published')
  expect(api.saveShopProduct).not.toHaveBeenCalled()
})

it('requires custom icon, name, description and a positive integer price and keeps them when editing', async () => {
  const wrapper = mount(PointsProductsPanel, { global })
  await flushPromises()
  await wrapper.get('article button').trigger('click'); await flushPromises()
  expect((wrapper.get('#product-name').element as HTMLInputElement).value).toBe('Custom gift')
  expect((wrapper.get('#product-price').element as HTMLInputElement).value).toBe('100')
  await wrapper.get('#product-price').setValue('1.5')
  await wrapper.get('form').trigger('submit'); expect(api.saveShopProduct).not.toHaveBeenCalled()
  await wrapper.get('#product-price').setValue('200')
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(api.saveShopProduct).toHaveBeenCalledWith({ ...product, price: '200' })
})

it('uses the selected item without trusting client-supplied item display fields', async () => {
  const wrapper = mount(PointsProductsPanel, { global })
  await flushPromises()
  await wrapper.get('section > div button').trigger('click'); await flushPromises()
  await wrapper.get('#product-item [data-value="2"]').trigger('click')
  await wrapper.get('#product-price').setValue('50')
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(api.saveShopProduct).toHaveBeenCalledWith(expect.objectContaining({ kind: 'item', item_id: 2, price: '50', name: '', description: '', icon_asset: '' }))
})
