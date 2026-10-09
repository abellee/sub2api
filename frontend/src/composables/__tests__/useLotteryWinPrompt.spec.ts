import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { LotteryClient } from '@/api/lotteryClient'
import type { WinnerRecord } from '@/api/lottery'
import { useAuthStore } from '@/stores/auth'

const list = vi.hoisted(() => vi.fn())
vi.mock('@/api/lottery', () => ({ fetchMyWinnings: list }))

import { useLotteryWinPrompt } from '../useLotteryWinPrompt'

const client = {} as LotteryClient

function winner(id: number, createdAt: string): WinnerRecord {
  return {
    id,
    activity_id: 4,
    activity_name: '晚场',
    user_id: 9,
    email: 'a***@b.c',
    prize_id: 1,
    prize_name: '余额奖',
    prize_type: 'balance',
    value: 2,
    fulfillment: 'done',
    created_at: createdAt,
  }
}

describe('useLotteryWinPrompt', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    list.mockReset()
    useAuthStore().user = { id: 9 } as never
  })

  it('remembers older wins and only queues a recent one on the first check', async () => {
    const prompt = useLotteryWinPrompt(client)
    list.mockResolvedValueOnce([
      winner(2, new Date().toISOString()),
      winner(1, '2020-01-01T00:00:00Z'),
    ])
    await prompt.sync()
    expect(prompt.current.value?.id).toBe(2)
    expect(JSON.parse(localStorage.getItem('lottery_win_popup_9') || '[]')).toEqual([1])
  })

  it('queues a win that appears after the first check and stays quiet after it is closed', async () => {
    const prompt = useLotteryWinPrompt(client)
    list.mockResolvedValueOnce([winner(1, '2020-01-01T00:00:00Z')])
    await prompt.sync()
    expect(prompt.current.value).toBeNull()

    list.mockResolvedValueOnce([
      winner(3, new Date().toISOString()),
      winner(1, '2020-01-01T00:00:00Z'),
    ])
    await prompt.sync()
    expect(prompt.current.value?.id).toBe(3)

    prompt.acknowledge(3)
    expect(prompt.current.value).toBeNull()
    list.mockResolvedValueOnce([
      winner(3, new Date().toISOString()),
      winner(1, '2020-01-01T00:00:00Z'),
    ])
    await prompt.sync()
    expect(prompt.current.value).toBeNull()
  })

  it('ignores a live win that was already closed', async () => {
    const prompt = useLotteryWinPrompt(client)
    list.mockResolvedValueOnce([])
    await prompt.sync()
    prompt.enqueue(winner(8, new Date().toISOString()))
    prompt.acknowledge(8)
    prompt.enqueue(winner(8, new Date().toISOString()))
    expect(prompt.current.value).toBeNull()
  })
})
