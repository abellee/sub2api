import { describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get } }))
import { exportData, list, listWithEtag, getUpstreamBillingRatesWithEtag } from '@/api/admin/accounts'

describe('account upstream API filter', () => {
  it('passes the address to list, refresh, billing snapshots and export', async () => {
    get.mockResolvedValue({ data: { items: [], total: 0 }, status: 200, headers: {} })
    const filters = { base_url: 'api.example.com', platform: 'openai' }
    await list(2, 20, filters)
    await listWithEtag(2, 20, filters)
    await getUpstreamBillingRatesWithEtag(2, 20, filters)
    await exportData({ filters })
    expect(get).toHaveBeenCalledTimes(4)
    for (const [, options] of get.mock.calls) expect(options.params).toMatchObject(filters)
  })
})
