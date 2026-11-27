import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import PointsSettingsView from '../PointsSettingsView.vue'
import Select from '@/components/common/Select.vue'

const api = vi.hoisted(() => ({ fetchPointsSettings: vi.fn(), savePointsSettings: vi.fn(), showSuccess: vi.fn(), showError: vi.fn(), refreshDailyCheckIn: vi.fn() }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('@/components/admin/points/PointsProductsPanel.vue', () => ({ default: { template: '<div data-test="products" />' } }))
vi.mock('@/api/points', () => ({ fetchPointsSettings: api.fetchPointsSettings, savePointsSettings: api.savePointsSettings }))
vi.mock('@/stores', () => ({ useAppStore: () => api }))
vi.mock('@/composables/useDailyCheckIn', () => ({ refreshDailyCheckIn: api.refreshDailyCheckIn }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: unknown) => key + (params ? JSON.stringify(params) : ''), locale: ref('zh') }) }))
enableAutoUnmount(afterEach)
const rewards = [15, 12, 30, 18, 50, 7, 9]
beforeEach(() => { vi.clearAllMocks(); api.fetchPointsSettings.mockResolvedValue({ daily_checkin_rewards: rewards }); api.savePointsSettings.mockResolvedValue({ daily_checkin_rewards: rewards }) })
const render = () => mount(PointsSettingsView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })

it('loads and saves all seven independent rewards and previews their sum', async () => {
  const wrapper = render()
  await flushPromises()
  const inputs = wrapper.findAll('input')
  expect(inputs.map(input => (input.element as HTMLInputElement).value)).toEqual(rewards.map(String))
  expect(wrapper.text()).toContain('"points":"141"')
  await inputs[5].setValue('20')
  expect((inputs[6].element as HTMLInputElement).value).toBe('9')
  expect(wrapper.text()).toContain('"points":"154"')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(api.savePointsSettings).toHaveBeenCalledWith({ daily_checkin_rewards: [15, 12, 30, 18, 50, 20, 9], daily_checkin_multipliers: [1, 1, 1, 1, 1, 1, 1] })
  expect(api.showSuccess).toHaveBeenCalledWith('common.saved')
  expect(api.refreshDailyCheckIn).toHaveBeenCalledWith(true)
})

it('defaults to one and independently saves multipliers with final daily and cycle rewards', async () => {
  const wrapper = render()
  await flushPromises()
  const selects = wrapper.findAllComponents(Select)
  expect(selects).toHaveLength(7)
  expect(selects.map(select => select.props('modelValue'))).toEqual([1, 1, 1, 1, 1, 1, 1])
  expect(selects[0].props('options').map(option => option.value)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10])
  await selects[5].vm.$emit('update:modelValue', 2)
  await selects[6].vm.$emit('update:modelValue', 10)
  expect(wrapper.text()).toContain('common.pointsAdmin.dailyTotal{"points":"14"}')
  expect(wrapper.text()).toContain('common.pointsAdmin.dailyTotal{"points":"90"}')
  expect(wrapper.text()).toContain('common.pointsAdmin.cycleTotal{"points":"229"}')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(api.savePointsSettings).toHaveBeenCalledWith({ daily_checkin_rewards: rewards, daily_checkin_multipliers: [1, 1, 1, 1, 1, 2, 10] })
})

it('loads saved multipliers and rejects invalid values', async () => {
  api.fetchPointsSettings.mockResolvedValueOnce({ daily_checkin_rewards: rewards, daily_checkin_multipliers: [1, 2, 3, 4, 5, 6, 10] })
  const wrapper = render()
  await flushPromises()
  const selects = wrapper.findAllComponents(Select)
  expect(selects.map(select => select.props('modelValue'))).toEqual([1, 2, 3, 4, 5, 6, 10])
  for (const invalid of [0, 11, 1.5]) {
    await selects[6].vm.$emit('update:modelValue', invalid)
    await wrapper.get('form').trigger('submit')
  }
  expect(api.savePointsSettings).not.toHaveBeenCalled()
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
  for (const value of ['', '0', '-1', '1.5', '2000001']) {
    await wrapper.get('#daily-checkin-points-7').setValue(value)
    await wrapper.get('form').trigger('submit')
  }
  expect(api.savePointsSettings).not.toHaveBeenCalled()
})

it('does not allow saving an incomplete schedule or an old server response', async () => {
  for (const settings of [{ daily_checkin_rewards: [10, 20] }, { daily_checkin_points: 10 }]) {
    api.fetchPointsSettings.mockResolvedValueOnce(settings)
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('form').exists()).toBe(false)
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    wrapper.unmount()
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
