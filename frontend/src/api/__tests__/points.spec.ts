import { beforeEach, expect, it, vi } from 'vitest'
import { fetchMyPoints, fetchMyPointsHistory, fetchCheckInStatus, claimDailyCheckIn, fetchPointsSettings, savePointsSettings, fetchAdminUserPoints, rechargeUserPoints } from '../points'

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }))
vi.mock('../lotteryClient', async () => ({ ...await vi.importActual('../lotteryClient'), createDefaultLotteryClient: () => ({ http: client }) }))
beforeEach(() => vi.clearAllMocks())

it('reads points and paginated usage for the authenticated user without accepting a user ID', async () => {
  const summary = { balance: '0', total_earned: '30', total_spent: '30' }
  client.get.mockResolvedValueOnce({ data: { code: 0, data: summary } })
  expect(await fetchMyPoints()).toEqual(summary)
  expect(client.get).toHaveBeenNthCalledWith(1, '/v1/me/points')
  const history = { items: [], total: 0, page: 2, page_size: 10 }
  client.get.mockResolvedValueOnce({ data: { code: 0, data: history } })
  expect(await fetchMyPointsHistory(2, 'spend')).toEqual(history)
  expect(client.get).toHaveBeenNthCalledWith(2, '/v1/me/points/transactions', { params: { page: 2, page_size: 10, kind: 'spend' } })
})

it('uses server-owned check-in state and admin-only points configuration and recharge endpoints', async () => {
  client.get.mockResolvedValue({ data: { code: 0, data: {} } })
  client.post.mockResolvedValue({ data: { code: 0, data: {} } })
  client.put.mockResolvedValue({ data: { code: 0, data: {} } })
  await fetchCheckInStatus()
  expect(client.get).toHaveBeenLastCalledWith('/v1/me/points/check-in')
  await claimDailyCheckIn()
  expect(client.post).toHaveBeenLastCalledWith('/v1/me/points/check-in')
  await fetchPointsSettings()
  expect(client.get).toHaveBeenLastCalledWith('/v1/admin/points/settings')
  await savePointsSettings({ daily_checkin_points: 15 })
  expect(client.put).toHaveBeenLastCalledWith('/v1/admin/points/settings', { daily_checkin_points: 15 })
  await fetchAdminUserPoints(2)
  expect(client.get).toHaveBeenLastCalledWith('/v1/admin/users/2/points')
  await rechargeUserPoints(2, '123', 'credit', 'uuid')
  expect(client.post).toHaveBeenLastCalledWith('/v1/admin/users/2/points', { amount: '123', notes: 'credit', request_id: 'uuid' })
})
