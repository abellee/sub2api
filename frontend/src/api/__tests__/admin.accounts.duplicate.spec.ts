import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({
  post: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { post }
}))

import { duplicate, batchDuplicate } from '@/api/admin/accounts'

describe('admin account duplicate API', () => {
  beforeEach(() => {
    sessionStorage.clear()
    post.mockReset()
    post.mockResolvedValue({ data: { id: 43, name: 'primary (Copy)' } })
    vi.spyOn(globalThis.crypto, 'randomUUID').mockReturnValue('11111111-1111-4111-8111-111111111111')
  })

  it('sends a stable idempotency key with the duplicate request', async () => {
    const account = await duplicate(42)

    expect(post).toHaveBeenCalledWith('/admin/accounts/42/duplicate', undefined, {
      headers: {
        'Idempotency-Key': 'account-duplicate-42-11111111-1111-4111-8111-111111111111'
      }
    })
    expect(account).toEqual({ id: 43, name: 'primary (Copy)' })
  })

  it('reuses the operation key after an ambiguous failed request', async () => {
    post.mockRejectedValueOnce(new Error('network timeout'))
    await expect(duplicate(99)).rejects.toThrow('network timeout')

    post.mockResolvedValueOnce({ data: { id: 100, name: 'retry (Copy)' } })
    await duplicate(99)

    expect(post).toHaveBeenCalledTimes(2)
    const firstHeaders = post.mock.calls[0][2].headers
    const secondHeaders = post.mock.calls[1][2].headers
    expect(secondHeaders).toEqual(firstHeaders)
  })

  it('reuses the operation key after a page reload', async () => {
    post.mockRejectedValueOnce(new Error('network timeout'))
    await expect(duplicate(77)).rejects.toThrow('network timeout')
    const firstHeaders = post.mock.calls[0][2].headers

    vi.resetModules()
    post.mockResolvedValueOnce({ data: { id: 78, name: 'reload (Copy)' } })
    const { duplicate: duplicateAfterReload } = await import('@/api/admin/accounts')
    await duplicateAfterReload(77)

    expect(post).toHaveBeenCalledTimes(2)
    expect(post.mock.calls[1][2].headers).toEqual(firstHeaders)
    expect(sessionStorage.length).toBe(0)
  })
})

describe('admin account batch duplicate API', () => {
  beforeEach(() => {
    sessionStorage.clear()
    post.mockReset()
    post.mockResolvedValue({ data: [{ id: 43, name: 'source-1' }] })
  })

  it('sends only a count and retries with the same key without copying credentials to the browser', async () => {
    post.mockRejectedValueOnce(new Error('network timeout'))
    await expect(batchDuplicate(42, 10)).rejects.toThrow('network timeout')
    await batchDuplicate(42, 10)
    expect(post.mock.calls[0][0]).toBe('/admin/accounts/42/batch-duplicate')
    expect(post.mock.calls[0][1]).toEqual({ count: 10 })
    expect(post.mock.calls[1][2]).toEqual(post.mock.calls[0][2])
    expect(sessionStorage.length).toBe(0)
  })

  it('keeps independent operation keys for different counts and single copies', async () => {
    post.mockRejectedValueOnce(new Error('timeout'))
    await expect(batchDuplicate(52, 10)).rejects.toThrow('timeout')
    await batchDuplicate(52, 5)
    expect(post.mock.calls[1][2].headers['Idempotency-Key']).not.toBe(post.mock.calls[0][2].headers['Idempotency-Key'])
    await duplicate(52)
    expect(post.mock.calls[2][0]).toBe('/admin/accounts/52/duplicate')
    expect(post.mock.calls[2][2].headers['Idempotency-Key']).not.toBe(post.mock.calls[0][2].headers['Idempotency-Key'])
  })

  it('keeps a pending batch key across reloads', async () => {
    post.mockRejectedValueOnce(new Error('timeout'))
    await expect(batchDuplicate(53, 10)).rejects.toThrow('timeout')
    const headers = post.mock.calls[0][2].headers
    vi.resetModules()
    const { batchDuplicate: retry } = await import('@/api/admin/accounts')
    await retry(53, 10)
    expect(post.mock.calls[1][2].headers).toEqual(headers)
  })
})
