import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { refreshUserPoints, setUserPointsIdentity, updateUserPoints, useUserPoints } from '../useUserPoints'
import type { PointsSummary } from '@/api/points'

const api = vi.hoisted(() => ({ fetch: vi.fn() }))
vi.mock('@/api/points', () => ({ fetchMyPoints: api.fetch }))
const initial = { balance: '120', total_earned: '150', total_spent: '30' }
beforeEach(() => {
  setUserPointsIdentity(null)
  vi.resetAllMocks()
  api.fetch.mockResolvedValue(initial)
})
afterEach(() => setUserPointsIdentity(null))

describe('current user points', () => {
  it('shares the current user balance and deduplicates concurrent loads', async () => {
    const sidebar = useUserPoints()
    const card = useUserPoints()
    expect(sidebar.balance.value).toBe('—')
    setUserPointsIdentity(2)
    await Promise.all([refreshUserPoints(), refreshUserPoints()])
    expect(api.fetch).toHaveBeenCalledOnce()
    expect(sidebar.balance.value).toBe('120')
    expect(card.balance.value).toBe('120')
    updateUserPoints({ ...initial, balance: '0' }, 2)
    expect(sidebar.balance.value).toBe('0')
    expect(card.balance.value).toBe('0')
    await refreshUserPoints()
    expect(api.fetch).toHaveBeenCalledOnce()
    await refreshUserPoints(true)
    expect(api.fetch).toHaveBeenCalledTimes(2)
  })

  it('does not expose another user balance after switching identity or logging out', async () => {
    let resolveOld!: (value: PointsSummary) => void
    api.fetch.mockReturnValueOnce(new Promise<PointsSummary>(resolve => { resolveOld = resolve }))
    setUserPointsIdentity(2)
    const oldRequest = refreshUserPoints()
    api.fetch.mockResolvedValue({ ...initial, balance: '5' })
    setUserPointsIdentity(3)
    expect(useUserPoints().balance.value).toBe('—')
    await refreshUserPoints()
    resolveOld({ ...initial, balance: '999' })
    await oldRequest
    expect(useUserPoints().balance.value).toBe('5')
    updateUserPoints({ ...initial, balance: '777' }, 2)
    expect(useUserPoints().balance.value).toBe('5')
    setUserPointsIdentity(null)
    expect(useUserPoints().balance.value).toBe('—')
    await refreshUserPoints()
    expect(api.fetch).toHaveBeenCalledTimes(2)
  })

  it('retains newer dialog data when an earlier background request finishes', async () => {
    let resolveOld!: (value: PointsSummary) => void
    api.fetch.mockReturnValueOnce(new Promise<PointsSummary>(resolve => { resolveOld = resolve }))
    setUserPointsIdentity(2)
    const oldRequest = refreshUserPoints()
    updateUserPoints({ ...initial, balance: '90' }, 2)
    resolveOld(initial)
    await oldRequest
    expect(useUserPoints().balance.value).toBe('90')
  })

  it('keeps unavailable data distinct from zero and recovers on retry without losing precision', async () => {
    api.fetch.mockRejectedValueOnce(new Error('offline'))
    setUserPointsIdentity(2)
    await refreshUserPoints()
    expect(useUserPoints().balance.value).toBe('—')
    api.fetch.mockResolvedValue({ ...initial, balance: '9007199254740993' })
    await refreshUserPoints(true)
    expect(useUserPoints().balance.value).toBe('9007199254740993')
  })
})
