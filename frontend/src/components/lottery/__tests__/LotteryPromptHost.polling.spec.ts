import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'

const mocks = vi.hoisted(() => ({
  lotteryCheck: vi.fn(), taskCheck: vi.fn(), sync: vi.fn(), enqueueWin: vi.fn(), refreshMenu: vi.fn(),
}))
vi.mock('@/api/lotteryClient', () => ({ createDefaultLotteryClient: () => ({ wsURL: 'ws://localhost' }) }))
vi.mock('@/composables/useLotteryPrompt', () => ({
  useLotteryPrompt: () => ({ check: mocks.lotteryCheck, promptedIds: () => [], acknowledge: vi.fn() }),
}))
vi.mock('@/composables/useTaskPrompt', () => ({
  useTaskPrompt: () => ({ check: mocks.taskCheck, promptedIds: () => [], acknowledge: vi.fn() }),
}))
vi.mock('@/composables/useLotteryWinPrompt', () => ({
  LOTTERY_WIN_EVENT: 'lottery-win',
  useLotteryWinPrompt: () => ({ current: ref(null), sync: mocks.sync, enqueue: mocks.enqueueWin, acknowledge: vi.fn() }),
}))
vi.mock('@/composables/useMenuStatus', () => ({ refreshMenuStatus: mocks.refreshMenu }))

import LotteryPromptHost from '../LotteryPromptHost.vue'

let sockets: FakeSocket[] = []
class FakeSocket {
  static OPEN = 1
  static CONNECTING = 0
  readyState = FakeSocket.CONNECTING
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onerror: (() => void) | null = null
  constructor() { sockets.push(this) }
  open() { this.readyState = FakeSocket.OPEN; this.onopen?.() }
  close() { this.readyState = 3; this.onclose?.() }
  send() { window.setTimeout(() => this.message({ type: 'pong' }), 0) }
  message(data: unknown) { this.onmessage?.({ data: JSON.stringify(data) }) }
}

describe('lottery prompt polling', () => {
  let wrapper: ReturnType<typeof mount> | undefined
  let hidden = false

  async function mountHost() {
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { template: '<div />' } }] })
    wrapper = mount(LotteryPromptHost, {
      global: { plugins: [router], stubs: { LotteryWinModal: true, LotteryPromptModal: true, TaskPromptModal: true } },
    })
    await flushPromises()
  }

  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    mocks.lotteryCheck.mockResolvedValue(null)
    mocks.taskCheck.mockResolvedValue(null)
    mocks.sync.mockResolvedValue(undefined)
    mocks.refreshMenu.mockResolvedValue(undefined)
    localStorage.setItem('auth_token', 'test-token')
    hidden = false
    sockets = []
    vi.spyOn(document, 'hidden', 'get').mockImplementation(() => hidden)
    vi.stubGlobal('WebSocket', FakeSocket)
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
    localStorage.clear()
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('uses live pushes with only a five-minute safety check while connected', async () => {
    await mountHost()
    sockets[0]!.open()
    await flushPromises()
    vi.clearAllMocks()
    await vi.advanceTimersByTimeAsync(120_000)
    expect(mocks.sync).not.toHaveBeenCalled()
    expect(mocks.lotteryCheck).not.toHaveBeenCalled()
    window.dispatchEvent(new Event('focus'))
    expect(mocks.lotteryCheck).not.toHaveBeenCalled()
    sockets[0]!.message({ type: 'lottery_win', winner: { id: 7 } })
    expect(mocks.enqueueWin).toHaveBeenCalledWith({ id: 7 })
    expect(mocks.refreshMenu).toHaveBeenCalledOnce()
    sockets[0]!.message({ type: 'task_prompt', task: { id: 8 } })
    expect(mocks.refreshMenu).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(180_000)
    expect(mocks.sync).toHaveBeenCalledOnce()
  })

  it('checks once per minute while disconnected and catches up on connection', async () => {
    await mountHost()
    expect(mocks.sync).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(59_999)
    expect(mocks.sync).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(mocks.sync).toHaveBeenCalledTimes(2)
    sockets[0]!.open()
    await flushPromises()
    expect(mocks.sync).toHaveBeenCalledTimes(3)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(mocks.sync).toHaveBeenCalledTimes(3)
  })

  it('pauses all background checks and resumes on returning to the page', async () => {
    hidden = true
    await mountHost()
    await vi.advanceTimersByTimeAsync(120_000)
    expect(sockets).toHaveLength(0)
    expect(mocks.sync).not.toHaveBeenCalled()
    expect(mocks.lotteryCheck).not.toHaveBeenCalled()
    hidden = false
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(sockets).toHaveLength(1)
    expect(mocks.sync).toHaveBeenCalledOnce()
    expect(mocks.lotteryCheck).toHaveBeenCalledOnce()
    wrapper!.unmount()
    wrapper = undefined
    vi.clearAllMocks()
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(120_000)
    expect(mocks.sync).not.toHaveBeenCalled()
    expect(mocks.lotteryCheck).not.toHaveBeenCalled()
  })

  it('retains fallback checks when WebSocket construction fails', async () => {
    vi.stubGlobal('WebSocket', class { constructor() { throw new Error('unavailable') } })
    await mountHost()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(mocks.sync).toHaveBeenCalledTimes(2)
    expect(mocks.lotteryCheck).toHaveBeenCalledTimes(2)
  })
})
