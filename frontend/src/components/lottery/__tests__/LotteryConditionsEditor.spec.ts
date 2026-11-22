import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { reactive } from 'vue'
import LotteryConditionsEditor from '../LotteryConditionsEditor.vue'
import type { ConditionDef } from '@/api/lottery'

describe('LotteryConditionsEditor historical recharge', () => {
  it('switches between token and dollar amounts without mixing units', async () => {
    const conditions = reactive<ConditionDef[]>([{ dimension: 'token_usage', window_days: 7, mode: 'total', threshold: 100_000_000 }])
    const wrapper = mount(LotteryConditionsEditor, { props: { modelValue: conditions, allowHistoricalRecharge: true, showBonus: false } })
    await wrapper.findAll('.select-trigger')[0]!.trigger('click')
    await wrapper.findAll('[role="option"]').find((o) => o.text() === '历史充值')!.trigger('click')
    expect(conditions[0]).toMatchObject({ dimension: 'historical_recharge', threshold: 10, window_days: 0 })
    expect(wrapper.text()).not.toContain('窗口')
    await wrapper.get('[aria-label="累计充值金额（美元）"]').setValue('12.34')
    expect(conditions[0]!.threshold).toBe(12.34)
    await wrapper.get('.select-trigger').trigger('click')
    await wrapper.findAll('[role="option"]').find((o) => o.text() === 'Token 消耗量')!.trigger('click')
    expect(conditions[0]).toMatchObject({ dimension: 'token_usage', threshold: 100_000_000, window_days: 7 })
    wrapper.unmount()
  })

  it('keeps an existing cents threshold and opts other editors out', async () => {
    const wrapper = mount(LotteryConditionsEditor, { props: { modelValue: [{ dimension: 'historical_recharge', window_days: 0, threshold: 0.01 }], allowHistoricalRecharge: true } })
    expect((wrapper.get('[aria-label="累计充值金额（美元）"]').element as HTMLInputElement).value).toBe('0.01')
    wrapper.unmount()
    const other = mount(LotteryConditionsEditor, { props: { modelValue: [{ dimension: 'token_usage', window_days: 7 }] } })
    await other.findAll('.select-trigger')[0]!.trigger('click')
    expect(other.findAll('[role="option"]').some((o) => o.text() === '历史充值')).toBe(false)
    other.unmount()
  })
})

describe('LotteryConditionsEditor token conditions', () => {
  it('defaults activity to one qualifying day and converts M to raw tokens', async () => {
    const conditions = reactive<ConditionDef[]>([{ dimension: 'token_usage', window_days: 7, mode: 'per_day', threshold: 100_000_000 }])
    const wrapper = mount(LotteryConditionsEditor, { props: { modelValue: conditions, showBonus: false } })
    await wrapper.findAll('.select-trigger')[0]!.trigger('click')
    await wrapper.findAll('[role="option"]').find((o) => o.text() === '活跃度')!.trigger('click')
    expect(conditions[0]).toMatchObject({ dimension: 'activity_days', window_days: 7, min_active_days: 1 })
    const minimum = wrapper.findAll('label').find((label) => label.text().includes('日最低'))!.get('input')
    await minimum.setValue('1')
    expect(conditions[0]!.daily_min_tokens).toBe(1_000_000)
    await minimum.setValue('0.1')
    expect(conditions[0]!.daily_min_tokens).toBe(100_000)
    // The active-day count remains independent of the token amount.
    expect(conditions[0]!.min_active_days).toBe(1)
    expect(conditions[0]!.window_days).toBe(7)
    wrapper.unmount()
  })

  it('preserves the window and raw threshold when switching daily and total modes', async () => {
    const conditions = reactive<ConditionDef[]>([{ dimension: 'token_usage', window_days: 7, mode: 'per_day', threshold: 1_000_000 }])
    const wrapper = mount(LotteryConditionsEditor, { props: { modelValue: conditions, showBonus: false } })
    const amount = wrapper.findAll('input[type="number"]')[1]!
    expect((amount.element as HTMLInputElement).value).toBe('1')
    await wrapper.findAll('.select-trigger')[1]!.trigger('click')
    await wrapper.findAll('[role="option"]').find((o) => o.text() === '累计≥阈值')!.trigger('click')
    expect(conditions[0]).toMatchObject({ mode: 'total', threshold: 1_000_000, window_days: 7 })
    await amount.setValue('1.1')
    expect(conditions[0]!.threshold).toBe(1_100_000)
    await wrapper.findAll('.select-trigger')[1]!.trigger('click')
    await wrapper.findAll('[role="option"]').find((o) => o.text() === '每日≥阈值')!.trigger('click')
    expect(conditions[0]).toMatchObject({ mode: 'per_day', threshold: 1_100_000, window_days: 7 })
    wrapper.unmount()
  })
})
