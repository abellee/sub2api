import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { fetchMenuStatus, type MenuStatus } from '@/api/menuStatus'
import { refreshMenuStatus, startMenuStatusPolling, stopMenuStatusPolling, useMenuStatus } from '../useMenuStatus'

vi.mock('@/api/menuStatus', () => ({ fetchMenuStatus: vi.fn() }))

const status: MenuStatus = {
  lottery_visible: true, lottery_phase: 'joining', task_visible: true, task_phase: 'active',
}
let hidden = false

describe('menu status polling', () => {
  beforeEach(() => {
    stopMenuStatusPolling()
    vi.useFakeTimers()
    hidden = false
    vi.spyOn(document, 'hidden', 'get').mockImplementation(() => hidden)
    vi.mocked(fetchMenuStatus).mockReset().mockResolvedValue(status)
  })

  afterEach(() => {
    stopMenuStatusPolling()
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('polls once per 30 seconds, pauses in the background and catches up on return', async () => {
    startMenuStatusPolling()
    startMenuStatusPolling()
    await flushPromises()
    expect(fetchMenuStatus).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(29_999)
    expect(fetchMenuStatus).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(fetchMenuStatus).toHaveBeenCalledTimes(2)

    hidden = true
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(90_000)
    expect(fetchMenuStatus).toHaveBeenCalledTimes(2)
    hidden = false
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(fetchMenuStatus).toHaveBeenCalledTimes(3)

    stopMenuStatusPolling()
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(60_000)
    expect(fetchMenuStatus).toHaveBeenCalledTimes(3)
  })

  it('coalesces overlapping refreshes and ignores a response after logout', async () => {
    let resolve!: (value: MenuStatus) => void
    vi.mocked(fetchMenuStatus).mockReturnValueOnce(new Promise((done) => { resolve = done }))
    startMenuStatusPolling()
    const manual = refreshMenuStatus()
    await vi.advanceTimersByTimeAsync(30_000)
    expect(fetchMenuStatus).toHaveBeenCalledTimes(1)
    stopMenuStatusPolling()
    resolve(status)
    await manual
    expect(useMenuStatus().menuReady.value).toBe(false)
    expect(useMenuStatus().lotteryVisible.value).toBe(false)
  })

  it('retains the last status on failure and retries on the next interval', async () => {
    startMenuStatusPolling()
    await flushPromises()
    vi.mocked(fetchMenuStatus).mockRejectedValueOnce(new Error('offline'))
    await vi.advanceTimersByTimeAsync(30_000)
    expect(useMenuStatus().lotteryVisible.value).toBe(true)
    vi.mocked(fetchMenuStatus).mockResolvedValueOnce({ ...status, lottery_visible: false })
    await vi.advanceTimersByTimeAsync(30_000)
    expect(useMenuStatus().lotteryVisible.value).toBe(false)
  })
})
