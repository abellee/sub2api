import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import LotteryWinModal from '../LotteryWinModal.vue'
import type { WinnerRecord } from '@/api/lottery'

const win: WinnerRecord = {
  id: 7,
  activity_id: 4,
  activity_name: '晚场',
  user_id: 9,
  email: 'a***@b.c',
  prize_id: 1,
  prize_name: '余额奖',
  prize_type: 'balance',
  value: 2,
  fulfillment: 'done',
  created_at: '2026-10-10T00:00:00Z',
}

describe('LotteryWinModal', () => {
  it('keeps only a close button on the lottery page', () => {
    const wrapper = mount(LotteryWinModal, { props: { win, onLotteryPage: true } })
    expect(wrapper.get('[data-testid="lottery-win-svg"]').exists()).toBe(true)
    const pieces = wrapper.findAll('[data-testid="lottery-win-confetti"] i')
    expect(pieces).toHaveLength(160)
    expect(pieces.some((piece) => piece.classes().includes('from-left'))).toBe(true)
    expect(pieces.some((piece) => piece.classes().includes('from-right'))).toBe(true)
    expect(pieces[0].attributes('style')).toContain('animation-iteration-count: 1')
    expect(wrapper.find('[data-testid="lottery-win-view"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="lottery-win-close"]').text()).toContain('关闭')
  })

  it('offers a way to open the lottery page from anywhere else', async () => {
    const wrapper = mount(LotteryWinModal, { props: { win, onLotteryPage: false } })
    expect(wrapper.get('[data-testid="lottery-win-view"]').text()).toContain('去查看')
    await wrapper.get('[data-testid="lottery-win-close"]').trigger('click')
    expect(wrapper.emitted('close')?.[0]).toEqual([7])
  })
})
