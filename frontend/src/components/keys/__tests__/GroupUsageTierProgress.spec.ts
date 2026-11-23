import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import GroupUsageTierProgress from '../GroupUsageTierProgress.vue'
import type { GroupUsageTierProgress as Progress } from '@/types/usageTiers'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => `${key} ${JSON.stringify(params ?? {})}` }) }))
const entry = (overrides: Partial<Progress> = {}): Progress => ({
  source_group_id: 1, source_group_name: 'Source', excluded: false, stage: 0, streak: 1, missed: 0, today_cost: 5, today_qualified: false, promoted_today: false,
  tiers: [ { daily_cost: 10, consecutive_days: 2, max_missed_days: 3, rate_multiplier: 0.8 }, { daily_cost: 20, consecutive_days: 3, max_missed_days: 2, rate_multiplier: 0.5 } ],
  ...overrides,
})

describe('usage tier progress', () => {
  it('shows rule details and progress before qualification', () => {
    const wrapper = mount(GroupUsageTierProgress, { props: { entries: [entry()] } })
    expect(wrapper.text()).toContain('usageTiers.rule')
    expect(wrapper.get('[role="progressbar"]').attributes('aria-valuenow')).toBe('75')
    expect(wrapper.find('[data-test="tier-maintenance"]').exists()).toBe(false)
  })
  it('shows the daily circle and resets the next-tier bar after promotion', () => {
    const wrapper = mount(GroupUsageTierProgress, { props: { entries: [entry({ stage: 1, streak: 0, rate_multiplier: 0.8, today_cost: 50, promoted_today: true, today_qualified: true })] } })
    expect(wrapper.get('svg[role="progressbar"]').attributes('aria-valuenow')).toBe('100')
    expect(wrapper.get('[data-test="tier-next"] [role="progressbar"]').attributes('aria-valuenow')).toBe('0')
    expect(wrapper.text()).toContain('usageTiers.nextTomorrow')
  })
  it('locks the last tier while keeping maintenance visible', () => {
    const wrapper = mount(GroupUsageTierProgress, { props: { entries: [entry({ stage: 2, rate_multiplier: 0.5 })] } })
    expect(wrapper.find('[data-test="tier-next"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="tier-locked"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="tier-maintenance"]').exists()).toBe(true)
  })
  it('shows excluded users the explanation without any progress', () => {
    const wrapper = mount(GroupUsageTierProgress, { props: { entries: [entry({ excluded: true })] } })
    expect(wrapper.find('details').exists()).toBe(true)
    expect(wrapper.find('[data-test="tier-excluded"]').exists()).toBe(true)
    expect(wrapper.find('[role="progressbar"]').exists()).toBe(false)
  })
})
