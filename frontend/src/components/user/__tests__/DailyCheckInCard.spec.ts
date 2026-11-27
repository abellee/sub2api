import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import DailyCheckInCard from '../DailyCheckInCard.vue'
import type { CheckInStatus } from '@/api/points'

const mocks = vi.hoisted(() => ({ refresh: vi.fn(), claim: vi.fn() }))
const status = ref<CheckInStatus | null>(null)
const loading = ref(false), claiming = ref(false), error = ref(false)
vi.mock('@/composables/useDailyCheckIn', () => ({ useDailyCheckIn: () => ({ status, loading, claiming, error, ...mocks }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: unknown) => key + (params ? JSON.stringify(params) : ''), locale: ref('zh') }) }))
enableAutoUnmount(afterEach)
const initial = { date: '2026-11-27', day: 6, completed_days: 5, claimed: false, rewards: [10,25,12,40,15,7,9], today_reward: 7, next_reset_at: '2026-11-28T00:00:00+08:00' }
beforeEach(() => { vi.clearAllMocks(); status.value = initial; loading.value = claiming.value = error.value = false })
const render = () => mount(DailyCheckInCard, { props: { show: true, userId: 2 }, global: { stubs: { Icon: true } } })

it('shows the configured rewards without fixed double days, then claims and updates the parent', async () => {
  const points = { balance: '1300', total_earned: '1600', total_spent: '300' }
  mocks.claim.mockImplementation(async () => { status.value = { ...initial, claimed: true, completed_days: 6 }; return { status: status.value, points, granted: true } })
  const wrapper = render()
  expect(wrapper.findAll('.checkin-day')).toHaveLength(7)
  expect(wrapper.findAll('.checkin-double')).toHaveLength(0)
  expect(wrapper.findAll('.checkin-day strong').map(day => day.text())).toEqual(initial.rewards.map(reward => '+' + reward))
  expect(wrapper.find('.is-today').text()).toContain('+7')
  expect(wrapper.get('.checkin-claim').text()).toContain('"points":"7"')
  expect(wrapper.text()).not.toContain('common.checkIn.double')
  await wrapper.get('.checkin-claim').trigger('click')
  await flushPromises()
  expect(wrapper.emitted('claimed')?.[0]).toEqual([points])
  expect(wrapper.get('.checkin-claim').attributes('disabled')).toBeDefined()
  expect(wrapper.findAll('.is-complete')).toHaveLength(6)
})

it('reconciles with the server after a lost response and never invents a successful grant', async () => {
  mocks.claim.mockRejectedValue(new Error('connection lost'))
  const wrapper = render()
  await wrapper.get('.checkin-claim').trigger('click')
  await flushPromises()
  expect(wrapper.get('[role="alert"]').text()).toContain('common.checkIn.claimError')
  expect(wrapper.emitted('claimed')).toBeUndefined()
  expect(mocks.refresh).toHaveBeenLastCalledWith(true)
})

it('keeps all seven styled days and a disabled claim button when the endpoint is unavailable', async () => {
  status.value = null; error.value = true
  const wrapper = render()
  expect(wrapper.findAll('.checkin-day')).toHaveLength(7)
  expect(wrapper.findAll('.checkin-double')).toHaveLength(0)
  expect(wrapper.findAll('.checkin-day strong').map(day => day.text())).toEqual(Array(7).fill('—'))
  expect(wrapper.get('.checkin-claim').attributes('disabled')).toBeDefined()
  await wrapper.get('[role="alert"] button').trigger('click')
  expect(mocks.refresh).toHaveBeenCalledWith(true)
})
