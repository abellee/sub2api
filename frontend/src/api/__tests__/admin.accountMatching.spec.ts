import { beforeEach, describe, expect, it, vi } from 'vitest'
import { accountMatchingAPI } from '../admin/accountMatching'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('../client', () => ({ apiClient: { get } }))

beforeEach(() => { vi.resetAllMocks() })

describe('account matching user search', () => {
  it('uses the existing production users endpoint and keeps email matches across pages', async () => {
    get.mockResolvedValueOnce({ data: { items: [{ id: 90, email: 'different@example.test', username: 'user2' }], pages: 2 } })
      .mockResolvedValueOnce({ data: { items: [{ id: 2, email: 'User2@example.test', username: 'demo', balance: 10 }], pages: 2 } })
    const signal = new AbortController().signal
    expect(await accountMatchingAPI.searchUsers(' user2 ', signal)).toEqual([{ id: 2, email: 'User2@example.test' }])
    expect(get).toHaveBeenCalledTimes(2)
    for (const [index, call] of get.mock.calls.entries()) {
      expect(call[0]).toBe('/admin/users')
      expect(call[1]).toEqual(expect.objectContaining({ signal, params: expect.objectContaining({ page: index + 1, search: 'user2', include_subscriptions: false }) }))
    }
  })

  it('does not fetch users for blank input and propagates a failed request', async () => {
    expect(await accountMatchingAPI.searchUsers(' ')).toEqual([])
    expect(get).not.toHaveBeenCalled()
    get.mockRejectedValue(new Error('unavailable'))
    await expect(accountMatchingAPI.searchUsers('user2')).rejects.toThrow('unavailable')
  })
})
