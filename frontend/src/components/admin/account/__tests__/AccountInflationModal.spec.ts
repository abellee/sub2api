import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AccountInflationModal from '../AccountInflationModal.vue'
import { probeAccountInflation, supportsAccountInflation } from '@/api/admin/accountInflation'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: unknown) => key + (params ? JSON.stringify(params) : '') }) }))
vi.mock('@/api/client', () => ({ buildApiUrl: (path: string) => `/api/v1${path}`, apiClient: {} }))
vi.mock('@/api/admin/accountInflation', async importOriginal => ({
  ...await importOriginal<typeof import('@/api/admin/accountInflation')>(),
  probeAccountInflation: vi.fn(), supportsAccountInflation: vi.fn()
}))

function create() {
  return mount(AccountInflationModal, {
    props: { show: true, accountIds: [2, 3], accounts: [{ id: 2, name: 'Alpha' }] },
    global: { stubs: { BaseDialog: { template: '<div><slot/><slot name="footer"/></div>' } } }
  })
}
beforeEach(() => { vi.mocked(supportsAccountInflation).mockResolvedValue(true) })
afterEach(() => vi.resetAllMocks())

describe('AccountInflationModal', () => {
  it('uses the same prompt and model for every selected ID, including other pages, after an account failure', async () => {
    vi.mocked(probeAccountInflation)
      .mockRejectedValueOnce(new Error('upstream unavailable'))
      .mockImplementationOnce(async (_id, _options, event) => {
        event({ type: 'content', text: 'Hello world' })
        event({ type: 'inflation_result', data: { token_count: 2, output_tokens: 4, reasoning_tokens: 2, encoding: 'o200k_base' } })
        event({ type: 'test_complete', success: true })
      })
    const wrapper = create()
    await wrapper.find('input').setValue('same-model')
    await wrapper.find('textarea').setValue('same prompt')
    await wrapper.findAll('button').find(b => b.text().includes('inflation.start'))!.trigger('click')
    await flushPromises()
    expect(vi.mocked(probeAccountInflation).mock.calls.map(c => c[0])).toEqual([2, 3])
    for (const call of vi.mocked(probeAccountInflation).mock.calls) expect(call[1]).toEqual({ model: 'same-model', prompt: 'same prompt', encoding: 'o200k_base' })
    expect(wrapper.text()).toContain('100.00%')
    expect(wrapper.text()).toContain('upstream unavailable')
    wrapper.unmount()
  })

  it('does not send paid probes to an older backend', async () => {
    vi.mocked(supportsAccountInflation).mockResolvedValue(false)
    const wrapper = create()
    await wrapper.findAll('button').find(b => b.text().includes('inflation.start'))!.trigger('click')
    await flushPromises()
    expect(probeAccountInflation).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('inflation.backendRequired')
    wrapper.unmount()
  })

  it('stops the active request and does not start the next account', async () => {
    vi.mocked(probeAccountInflation).mockImplementation(async (_id, _options, _event, signal) => {
      await new Promise((_, reject) => signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError'))))
    })
    const wrapper = create()
    await wrapper.findAll('button').find(b => b.text().includes('inflation.start'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text().includes('inflation.stop'))!.trigger('click')
    await flushPromises()
    expect(probeAccountInflation).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('inflation.stopped')
    wrapper.unmount()
  })
})
