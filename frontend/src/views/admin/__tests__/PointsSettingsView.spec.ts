import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import PointsSettingsView from '../PointsSettingsView.vue'

const api = vi.hoisted(() => ({ fetchPointsSettings: vi.fn(), savePointsSettings: vi.fn(), showSuccess: vi.fn(), showError: vi.fn(), refreshDailyCheckIn: vi.fn() }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('@/components/admin/points/PointsProductsPanel.vue', () => ({ default: { template: '<div data-test="products" />' } }))
vi.mock('@/api/points', () => ({ fetchPointsSettings: api.fetchPointsSettings, savePointsSettings: api.savePointsSettings }))
vi.mock('@/stores', () => ({ useAppStore: () => api }))
vi.mock('@/composables/useDailyCheckIn', () => ({ refreshDailyCheckIn: api.refreshDailyCheckIn }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: unknown) => key + (params ? JSON.stringify(params) : ''), locale: ref('zh') }) }))
enableAutoUnmount(afterEach)
beforeEach(() => { vi.clearAllMocks(); api.fetchPointsSettings.mockResolvedValue({ daily_checkin_points: 15 }); api.savePointsSettings.mockResolvedValue({ daily_checkin_points: 20 }) })
const render = () => mount(PointsSettingsView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })

it('loads the saved base reward, previews double days and saves one integer setting', async () => {
  const wrapper = render()
  await flushPromises()
  expect((wrapper.get('input').element as HTMLInputElement).value).toBe('15')
  expect(wrapper.text()).toContain('"points":"30"')
  await wrapper.get('input').setValue('20')
  expect(wrapper.text()).toContain('"points":"40"')
  expect(wrapper.text()).toContain('"points":"180"')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(api.savePointsSettings).toHaveBeenCalledWith({ daily_checkin_points: 20 })
  expect(api.showSuccess).toHaveBeenCalledWith('common.saved')
  expect(api.refreshDailyCheckIn).toHaveBeenCalledWith(true)
})

it('does not overwrite unknown production settings when the load fails', async () => {
  api.fetchPointsSettings.mockRejectedValueOnce(new Error('not deployed'))
  const wrapper = render()
  await flushPromises()
  expect(wrapper.find('form').exists()).toBe(false)
  expect(api.savePointsSettings).not.toHaveBeenCalled()
  await wrapper.get('[role="alert"] button').trigger('click')
  await flushPromises()
  expect(wrapper.find('form').exists()).toBe(true)
})

it('blocks fractional and out-of-range settings', async () => {
  const wrapper = render()
  await flushPromises()
  for (const value of ['0', '1.5', '1000001']) {
    await wrapper.get('input').setValue(value)
    await wrapper.get('form').trigger('submit')
  }
  expect(api.savePointsSettings).not.toHaveBeenCalled()
})

it('keeps only check-in and products in points settings, including when check-in is unavailable', async () => {
  api.fetchPointsSettings.mockRejectedValueOnce(new Error('not deployed'))
  const wrapper = render()
  await flushPromises()
  const tabs = wrapper.findAll('nav button')
  expect(tabs).toHaveLength(2)
  expect(wrapper.text()).not.toContain('common.pointsCatalog.items')
  await tabs[1].trigger('click')
  expect(wrapper.find('[data-test="products"]').exists()).toBe(true)
})
