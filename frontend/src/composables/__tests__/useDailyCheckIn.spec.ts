import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { claimDailyCheckIn, fetchCheckInStatus, type CheckInStatus } from '@/api/points'
import { setDailyCheckInIdentity, useDailyCheckIn } from '../useDailyCheckIn'
import { updateUserPoints } from '../useUserPoints'

vi.mock('@/api/points', () => ({ fetchCheckInStatus: vi.fn(), claimDailyCheckIn: vi.fn() }))
vi.mock('../useUserPoints', () => ({ updateUserPoints: vi.fn() }))
const initial: CheckInStatus = { date: '2026-11-27', day: 6, completed_days: 5, claimed: false, rewards: [10,10,10,10,10,20,20], today_reward: 20, next_reset_at: '2026-11-28T00:00:00+08:00' }
const points = { balance: '1300', total_earned: '1600', total_spent: '300' }
async function settle() { for (let i = 0; i < 6; i++) await Promise.resolve() }
beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-11-27T10:00:00+08:00'))
  vi.clearAllMocks()
  setDailyCheckInIdentity(null)
  vi.mocked(fetchCheckInStatus).mockResolvedValue(initial)
  vi.mocked(claimDailyCheckIn).mockResolvedValue({ status: { ...initial, claimed: true, completed_days: 6 }, points, granted: true })
})
afterEach(() => { setDailyCheckInIdentity(null); vi.useRealTimers() })

describe('shared daily check-in', () => {
  it('loads, claims once while pending, updates the points shared by all entrances', async () => {
    setDailyCheckInIdentity(2)
    await settle()
    const state = useDailyCheckIn()
    expect(state.status.value?.day).toBe(6)
    const claim = state.claim()
    expect(await state.claim()).toBeNull()
    await claim
    expect(claimDailyCheckIn).toHaveBeenCalledTimes(1)
    expect(state.status.value?.claimed).toBe(true)
    expect(updateUserPoints).toHaveBeenCalledWith(points, 2)
  })
  it('discards a claim that finishes after logout', async () => {
    setDailyCheckInIdentity(2)
    await settle()
    let finish!: (value: Awaited<ReturnType<typeof claimDailyCheckIn>>) => void
    vi.mocked(claimDailyCheckIn).mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const result = useDailyCheckIn().claim()
    setDailyCheckInIdentity(null)
    finish({ status: { ...initial, claimed: true }, points, granted: true })
    expect(await result).toBeNull()
    expect(useDailyCheckIn().status.value).toBeNull()
    expect(updateUserPoints).not.toHaveBeenCalled()
  })
  it('ignores a status read started before a successful claim', async () => {
    setDailyCheckInIdentity(2)
    await settle()
    let finish!: (value: CheckInStatus) => void
    vi.mocked(fetchCheckInStatus).mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const old = useDailyCheckIn().refresh(true)
    await useDailyCheckIn().claim()
    finish(initial)
    await old
    expect(useDailyCheckIn().status.value?.claimed).toBe(true)
  })
  it('refreshes at Beijing midnight and clears stale data if the read fails', async () => {
    setDailyCheckInIdentity(2)
    await settle()
    vi.mocked(fetchCheckInStatus).mockRejectedValueOnce(new Error('offline'))
    await vi.advanceTimersByTimeAsync(14 * 60 * 60 * 1000 + 100)
    expect(useDailyCheckIn().status.value).toBeNull()
    expect(useDailyCheckIn().error.value).toBe(true)
    expect(fetchCheckInStatus).toHaveBeenCalledTimes(2)
  })
})
