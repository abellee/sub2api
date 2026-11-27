import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import PointsAssetUpload from '../PointsAssetUpload.vue'

const api = vi.hoisted(() => ({ uploadPointsAsset: vi.fn() }))
vi.mock('@/api/pointsShop', () => api)
vi.mock('@/components/user/PointsAssetPreview.vue', () => ({ default: { template: '<div />' } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
beforeEach(() => { vi.clearAllMocks(); api.uploadPointsAsset.mockResolvedValue({ id: 'stored', kind: 'svg' }) })
async function choose(wrapper: ReturnType<typeof mount>, file: File) {
  const input = wrapper.get('input').element as HTMLInputElement
  Object.defineProperty(input, 'files', { value: [file], configurable: true })
  await wrapper.get('input').trigger('change'); await flushPromises()
}
it('rejects non-SVG artwork before upload and permits Lottie only for animation', async () => {
  const cover = mount(PointsAssetUpload, { props: { modelValue: '', assetSlot: 'cover', label: 'Cover' } })
  await choose(cover, new File(['{}'], 'animation.json'))
  expect(api.uploadPointsAsset).not.toHaveBeenCalled()
  expect(cover.get('[role="alert"]').text()).toBe('common.pointsCatalog.invalidFile')
  const animation = mount(PointsAssetUpload, { props: { modelValue: '', assetSlot: 'animation', label: 'Animation' } })
  const file = new File(['{}'], 'animation.lottie')
  await choose(animation, file)
  expect(api.uploadPointsAsset).toHaveBeenCalledWith(file, 'animation')
  expect(animation.emitted('update:modelValue')?.[0]).toEqual(['stored'])
  expect(animation.emitted('busy')).toEqual([[true], [false]])
})
it('retains the existing asset when replacing it fails', async () => {
  api.uploadPointsAsset.mockRejectedValueOnce(new Error('invalid SVG'))
  const wrapper = mount(PointsAssetUpload, { props: { modelValue: 'existing', assetSlot: 'cover', label: 'Cover' } })
  await choose(wrapper, new File(['<svg/>'], 'cover.svg'))
  expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  expect(wrapper.get('[role="alert"]').text()).toBe('invalid SVG')
  expect(wrapper.emitted('busy')).toEqual([[true], [false]])
})
