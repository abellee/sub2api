import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AccountInflationModal from '../AccountInflationModal.vue'
import { list as listAccounts } from '@/api/admin/accounts'
import type { AccountListItem } from '@/types'
import { probeAccountInflation, supportsAccountInflation } from '@/api/admin/accountInflation'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: unknown) => key + (params ? JSON.stringify(params) : '') }) }))
vi.mock('@/api/admin/accounts', () => ({ list: vi.fn() }))
vi.mock('@/api/client', () => ({ buildApiUrl: (path: string) => `/api/v1${path}`, apiClient: {} }))
vi.mock('@/api/admin/accountInflation', async importOriginal => ({
  ...await importOriginal<typeof import('@/api/admin/accountInflation')>(),
  probeAccountInflation: vi.fn(), supportsAccountInflation: vi.fn()
}))

function create() {
  return mount(AccountInflationModal, {
    props: { show: true, groups: [{ id: 27, name: 'GPT', platform: 'openai' }] },
    global: { stubs: {
      BaseDialog: { template: '<div><slot/><slot name="footer"/></div>' },
      Select: {
        props: ['modelValue', 'options', 'disabled', 'ariaLabel'],
        emits: ['update:modelValue'],
        template: '<select :aria-label="ariaLabel" :disabled="disabled" :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>'
      }
    } }
  })
}
const sampleAccounts = [
  { id: 2, name: 'Alpha', platform: 'openai', type: 'oauth' },
  { id: 3, name: 'Beta', platform: 'openai', type: 'apikey' }
] as AccountListItem[]
const page = (items = sampleAccounts, total = items.length, pages = 1) => ({ items, total, page: 1, page_size: 1000, pages })
const button = (wrapper: ReturnType<typeof create>, name: string) => wrapper.findAll('button').find(b => b.text().includes(name))!
async function loadAndSelect(wrapper: ReturnType<typeof create>) {
  await button(wrapper, 'inflation.loadAccounts').trigger('click')
  await flushPromises()
  await wrapper.get('input[type="checkbox"]').setValue(true)
}
beforeEach(() => {
  vi.mocked(supportsAccountInflation).mockResolvedValue(true)
  vi.mocked(listAccounts).mockResolvedValue(page())
})
afterEach(() => vi.resetAllMocks())

describe('AccountInflationModal', () => {
  it('loads every matching page with both filters and checks only the accounts selected inside the modal', async () => {
    vi.mocked(listAccounts)
      .mockResolvedValueOnce(page([sampleAccounts[0]!], 2, 2))
      .mockResolvedValueOnce(page([sampleAccounts[1]!]))
    vi.mocked(probeAccountInflation).mockResolvedValue()
    const wrapper = create()
    expect(listAccounts).not.toHaveBeenCalled()
    expect(button(wrapper, 'inflation.start').attributes('disabled')).toBeDefined()
    await wrapper.get('select[aria-label="admin.accounts.platform"]').setValue('openai')
    await wrapper.get('select[aria-label="admin.accounts.inflation.group"]').setValue('27')
    await button(wrapper, 'inflation.loadAccounts').trigger('click')
    await flushPromises()
    expect(listAccounts).toHaveBeenCalledTimes(2)
    expect(listAccounts).toHaveBeenNthCalledWith(2, 2, 1000, expect.objectContaining({ platform: 'openai', group: '27', lite: '1' }), expect.objectContaining({ signal: expect.any(AbortSignal) }))
    expect(wrapper.text()).toContain('Alpha')
    expect(wrapper.text()).toContain('Beta')
    await wrapper.findAll('input[type="checkbox"]')[2]!.setValue(true)
    await button(wrapper, 'inflation.start').trigger('click')
    await flushPromises()
    expect(probeAccountInflation).toHaveBeenCalledTimes(1)
    expect(vi.mocked(probeAccountInflation).mock.calls[0]![0]).toBe(3)
    wrapper.unmount()
  })

  it('clears the previous selection when filters change', async () => {
    const wrapper = create()
    await loadAndSelect(wrapper)
    expect(button(wrapper, 'inflation.start').attributes('disabled')).toBeUndefined()
    await wrapper.get('select[aria-label="admin.accounts.inflation.group"]').setValue('27')
    expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(0)
    expect(button(wrapper, 'inflation.start').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).not.toContain('Alpha')
    expect(probeAccountInflation).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('ignores an old response after changing filters and loading again', async () => {
    let finishOld!: (result: ReturnType<typeof page>) => void
    vi.mocked(listAccounts)
      .mockImplementationOnce(() => new Promise(resolve => { finishOld = resolve }))
      .mockResolvedValueOnce(page([sampleAccounts[1]!]))
    const wrapper = create()
    await button(wrapper, 'inflation.loadAccounts').trigger('click')
    const oldSignal = vi.mocked(listAccounts).mock.calls[0]![3]!.signal!
    await wrapper.get('select[aria-label="admin.accounts.platform"]').setValue('openai')
    expect(oldSignal.aborted).toBe(true)
    await button(wrapper, 'inflation.loadAccounts').trigger('click')
    await flushPromises()
    finishOld(page([sampleAccounts[0]!]))
    await flushPromises()
    expect(wrapper.text()).toContain('Beta')
    expect(wrapper.text()).not.toContain('Alpha')
    wrapper.unmount()
  })

  it('shows an error without exposing a partial selection when a later page fails', async () => {
    vi.mocked(listAccounts)
      .mockResolvedValueOnce(page([sampleAccounts[0]!], 2, 2))
      .mockRejectedValueOnce(new Error('load failed'))
    const wrapper = create()
    await button(wrapper, 'inflation.loadAccounts').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('load failed')
    expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(0)
    expect(button(wrapper, 'inflation.start').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('uses the same prompt and model for selected accounts and continues after an account failure', async () => {
    vi.mocked(probeAccountInflation)
      .mockRejectedValueOnce(new Error('upstream unavailable'))
      .mockImplementationOnce(async (_id, _options, event) => {
        event({ type: 'reasoning', text: 'Considering ' })
        event({ type: 'reasoning', text: '<script>plain text</script>' })
        event({ type: 'content', text: 'Hello world' })
        event({ type: 'inflation_result', data: { token_count: 2, output_tokens: 4, reasoning_tokens: 2, reasoning_text_tokens: 8, encoding: 'o200k_base' } })
        event({ type: 'test_complete', success: true })
      })
    const wrapper = create()
    await loadAndSelect(wrapper)
    await wrapper.find('input:not([type="checkbox"])').setValue('same-model')
    await wrapper.find('textarea').setValue('same prompt')
    await wrapper.findAll('button').find(b => b.text().includes('inflation.start'))!.trigger('click')
    await flushPromises()
    expect(vi.mocked(probeAccountInflation).mock.calls.map(c => c[0])).toEqual([2, 3])
    for (const call of vi.mocked(probeAccountInflation).mock.calls) expect(call[1]).toEqual({ model: 'same-model', prompt: 'same prompt', encoding: 'o200k_base' })
    expect(wrapper.text()).toContain('100.00%')
    expect(wrapper.get('details').text()).toContain('Considering <script>plain text</script>')
    expect(wrapper.find('script').exists()).toBe(false)
    expect(wrapper.text()).toContain('inflation.reasoningCount=8')
    expect(wrapper.text()).toContain('upstream unavailable')
    wrapper.unmount()
  })

  it('explains missing reasoning and redacted reasoning without inventing text', async () => {
    vi.mocked(probeAccountInflation).mockImplementation(async (id, _options, event) => {
      if (id === 3) event({ type: 'reasoning_redacted' })
      event({ type: 'inflation_result', data: { token_count: 2, output_tokens: 4, reasoning_tokens: null, encoding: 'o200k_base' } })
      event({ type: 'test_complete', success: true })
    })
    const wrapper = create()
    await loadAndSelect(wrapper)
    await button(wrapper, 'inflation.start').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('inflation.noReasoning')
    expect(wrapper.text()).toContain('inflation.reasoningRedacted')
    expect(wrapper.find('details').exists()).toBe(false)
    wrapper.unmount()
  })

  it('does not send paid probes to an older backend', async () => {
    vi.mocked(supportsAccountInflation).mockResolvedValue(false)
    const wrapper = create()
    await loadAndSelect(wrapper)
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
    await loadAndSelect(wrapper)
    await wrapper.findAll('button').find(b => b.text().includes('inflation.start'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text().includes('inflation.stop'))!.trigger('click')
    await flushPromises()
    expect(probeAccountInflation).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('inflation.stopped')
    wrapper.unmount()
  })
})
