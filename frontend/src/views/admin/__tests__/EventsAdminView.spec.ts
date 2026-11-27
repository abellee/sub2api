import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import EventsAdminView from '../EventsAdminView.vue'
import { blankEventRule, type EventRule } from '@/api/events'

const api = vi.hoisted(() => ({
  fetchEventCatalog: vi.fn(), fetchEventRules: vi.fn(), fetchEventSettings: vi.fn(), fetchEventLogs: vi.fn(),
  saveEventRule: vi.fn(), disableEventRule: vi.fn(), saveEventSettings: vi.fn(), fetchManagedItems: vi.fn(),
  showError: vi.fn(), showSuccess: vi.fn()
}))
vi.mock('@/api/events', async () => ({ ...await vi.importActual('@/api/events'), ...api }))
vi.mock('@/api/pointsShop', () => ({ fetchManagedItems: api.fetchManagedItems }))
vi.mock('@/stores', () => ({ useAppStore: () => api }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, te: () => false }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))

enableAutoUnmount(afterEach)
const first = (): EventRule => ({ ...blankEventRule('billing.completed'), id: 1, name: 'First trigger', enabled: true, probability: 250000, total_triggered: 3 })
const second = (): EventRule => ({ ...blankEventRule('billing.completed'), id: 2, name: 'Second trigger', probability: 500000 })
const render = () => mount(EventsAdminView, { global: { stubs: {
  BaseDialog: { props: ['show'], template: '<div v-if="show" role="dialog"><slot /></div>' }
} } })
beforeEach(() => {
  vi.clearAllMocks()
  api.fetchEventCatalog.mockResolvedValue([
    { code: 'billing.completed', name: 'Billing complete', group: 'Billing', status: 'active', fields: ['user_id', 'actual_cost'], description: '' },
    { code: 'shield.triggered', name: 'Shield trigger', group: 'Future', status: 'reserved', fields: [], description: '' }
  ])
  api.fetchEventRules.mockResolvedValue([first(), second()])
  api.fetchManagedItems.mockResolvedValue([{ id: 5, name: 'Gift', effect: { kind: 'grant_points', points: 10 } }])
  api.fetchEventSettings.mockResolvedValue({ enabled: true, settlement_disabled: false, pending: 0, processed: 2, failed: 0 })
  api.fetchEventLogs.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
  api.saveEventRule.mockImplementation(async (rule: EventRule) => ({ ...rule, id: rule.id || 3, total_triggered: rule.total_triggered || 0 }))
})

it('creates another independent trigger disabled by default and converts percentage precisely', async () => {
  const wrapper = render(); await flushPromises()
  expect(wrapper.findAll('article')).toHaveLength(2)
  await wrapper.get('main > div > div button').trigger('click')
  expect((wrapper.get('form input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false)
  expect(wrapper.get('form select').findAll('option').map(option => option.attributes('value'))).toEqual(['billing.completed'])
  await wrapper.get('form input[maxlength="100"]').setValue('Third trigger')
  await wrapper.get('form input[step="0.0001"]').setValue('12.3456')
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(api.saveEventRule).toHaveBeenCalledWith(expect.objectContaining({ id: 0, event_type: 'billing.completed', name: 'Third trigger', enabled: false, probability: 123456 }))
  expect(wrapper.findAll('article')).toHaveLength(3)
  expect(wrapper.text()).toContain('First trigger'); expect(wrapper.text()).toContain('Second trigger')
})

it('edits only the selected trigger and keeps the other triggers', async () => {
  const wrapper = render(); await flushPromises()
  await wrapper.findAll('article')[1].get('button').trigger('click')
  expect((wrapper.get('form input[maxlength="100"]').element as HTMLInputElement).value).toBe('Second trigger')
  await wrapper.get('form input[maxlength="100"]').setValue('Second edited')
  await wrapper.get('form input[step="0.0001"]').setValue('100')
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(api.saveEventRule).toHaveBeenCalledTimes(1)
  expect(api.saveEventRule).toHaveBeenCalledWith(expect.objectContaining({ id: 2, name: 'Second edited', probability: 1000000 }))
  expect(wrapper.findAll('article')).toHaveLength(2)
  expect(wrapper.text()).toContain('First trigger'); expect(wrapper.text()).toContain('Second edited')
})

it('blocks invalid probabilities and requires values for added conditions', async () => {
  const wrapper = render(); await flushPromises()
  await wrapper.get('main > div > div button').trigger('click')
  await wrapper.get('form input[maxlength="100"]').setValue('Conditional')
  for (const invalid of ['-1', '100.1']) {
    await wrapper.get('form input[step="0.0001"]').setValue(invalid)
    await wrapper.get('form').trigger('submit')
  }
  expect(api.saveEventRule).not.toHaveBeenCalled()
  await wrapper.get('form input[step="0.0001"]').setValue('0')
  const addCondition = wrapper.findAll('form button').find(button => button.text() === 'common.events.addCondition')!
  await addCondition.trigger('click')
  await wrapper.get('form').trigger('submit')
  expect(api.saveEventRule).not.toHaveBeenCalled()
  await wrapper.get('form input[maxlength="1024"]').setValue('2')
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(api.saveEventRule).toHaveBeenCalledWith(expect.objectContaining({ probability: 0, conditions: [{ field: 'user_id', operator: 'eq', value: '2' }] }))
})

it('loads execution pages independently and resets pagination when filtering', async () => {
  api.fetchEventLogs.mockResolvedValue({ items: [{ id: 1, event_type: 'billing.completed', user_id: 2, status: 'applied', created_at: '2026-11-28T00:00:00Z' }], total: 41, page: 1, page_size: 20 })
  const wrapper = render(); await flushPromises()
  await wrapper.findAll('[role="tab"]')[1].trigger('click'); await flushPromises()
  expect(api.fetchEventLogs).toHaveBeenLastCalledWith(true, '', '', 1)
  const next = wrapper.findAll('button').find(button => button.text() === 'common.next')!
  await next.trigger('click'); await flushPromises()
  expect(api.fetchEventLogs).toHaveBeenLastCalledWith(true, '', '', 2)
  await wrapper.get('select').setValue('billing.completed'); await flushPromises()
  expect(api.fetchEventLogs).toHaveBeenLastCalledWith(true, 'billing.completed', '', 1)
})

it('does not permit configuration when loading fails and preserves the retry path', async () => {
  api.fetchEventRules.mockRejectedValueOnce(new Error('offline'))
  const wrapper = render(); await flushPromises()
  expect(wrapper.find('form').exists()).toBe(false)
  expect(wrapper.findAll('article')).toHaveLength(0)
  expect(api.saveEventRule).not.toHaveBeenCalled()
  await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
  expect(wrapper.findAll('article')).toHaveLength(2)
})
