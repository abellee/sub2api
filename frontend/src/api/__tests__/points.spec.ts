import { beforeEach, expect, it, vi } from 'vitest'
import { fetchMyPoints, fetchMyPointsHistory } from '../points'

const client = vi.hoisted(() => ({ get: vi.fn() }))
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
