import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import BatchDuplicateAccountModal from '../BatchDuplicateAccountModal.vue'
import type { Account } from '@/types'

const { batchDuplicate, probeUpstreamBillingBatch, showSuccess, showError } = vi.hoisted(() => ({ batchDuplicate: vi.fn(), probeUpstreamBillingBatch: vi.fn(), showSuccess: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: { batchDuplicate, probeUpstreamBillingBatch } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError }) }))
vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string, params?: unknown) => key + (params ? JSON.stringify(params) : '') }),
}))
enableAutoUnmount(afterEach)

const account = { id: 42, name: 'GPT-UltraFast1' } as Account
function openModal() {
  return mount(BatchDuplicateAccountModal, {
    props: { show: true, account },
    global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } },
  })
}

describe('batch duplicate account modal', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    batchDuplicate.mockResolvedValue(Array.from({ length: 10 }, (_, i) => ({ id: i + 43 })))
    probeUpstreamBillingBatch.mockImplementation(async (ids: number[]) => ids.map(id => ({ account_id: id, snapshot: { status: 'ok', synced_rate_multiplier: 0.15 } })))
  })

  it('previews the numbered names and submits the selected count', async () => {
    const wrapper = openModal()
    expect(wrapper.text()).toContain('GPT-UltraFast1-1')
    expect(wrapper.text()).toContain('GPT-UltraFast1-10')
    await wrapper.get('input').setValue('12')
    expect(wrapper.text()).toContain('GPT-UltraFast1-12')
    await wrapper.findAll('button')[1].trigger('click')
    await flushPromises()
    expect(batchDuplicate).toHaveBeenCalledWith(42, 12)
    expect(probeUpstreamBillingBatch.mock.calls).toEqual([[[43, 44, 45, 46]], [[47, 48, 49, 50]], [[51, 52]]])
    expect(showSuccess).toHaveBeenCalled()
    expect(wrapper.emitted('duplicated')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('reports partial sync failures and refreshes created accounts without offering to duplicate them again', async () => {
    probeUpstreamBillingBatch.mockRejectedValueOnce(new Error('timeout'))
      .mockResolvedValueOnce([
        { account_id: 47, snapshot: { status: 'unsupported' } },
        { account_id: 48, snapshot: { status: 'ok' } },
        { account_id: 49, error: 'failed' },
        { account_id: 50, snapshot: { status: 'ok', synced_rate_multiplier: 0.2 } },
      ])
    const wrapper = openModal()
    await wrapper.findAll('button')[1].trigger('click')
    await flushPromises()
    expect(showSuccess).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.accounts.batchDuplicateSyncIncomplete{"count":10,"failed":7}')
    expect(probeUpstreamBillingBatch).toHaveBeenCalledTimes(3)
    expect(wrapper.emitted('duplicated')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
    expect(batchDuplicate).toHaveBeenCalledTimes(1)
  })

  it('syncs every copy at the maximum count in bounded batches', async () => {
    batchDuplicate.mockResolvedValueOnce(Array.from({ length: 100 }, (_, id) => ({ id: id + 1 })))
    const wrapper = openModal()
    await wrapper.get('input').setValue('100')
    await wrapper.findAll('button')[1].trigger('click')
    await flushPromises()
    expect(probeUpstreamBillingBatch).toHaveBeenCalledTimes(25)
    expect(probeUpstreamBillingBatch.mock.calls.flat(2)).toEqual(Array.from({ length: 100 }, (_, id) => id + 1))
    expect(showSuccess).toHaveBeenCalledWith('admin.accounts.batchDuplicateSuccess{"count":100}')
  })

  it('waits for the initial sync before completing and prevents resubmission', async () => {
    let resolve!: (results: unknown[]) => void
    probeUpstreamBillingBatch.mockImplementationOnce(() => new Promise(resolvePromise => { resolve = resolvePromise }))
    const wrapper = openModal()
    await wrapper.findAll('button')[1].trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('admin.accounts.batchDuplicateSyncing')
    expect(wrapper.emitted('duplicated')).toBeUndefined()
    await wrapper.get('input').trigger('keyup', { key: 'Enter' })
    expect(batchDuplicate).toHaveBeenCalledTimes(1)
    resolve([43, 44, 45, 46].map(id => ({ account_id: id, snapshot: { status: 'ok', synced_rate_multiplier: 0.15 } })))
    await flushPromises()
    expect(showSuccess).toHaveBeenCalled()
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it.each(['', '0', '-1', '101', '1.5'])('rejects invalid copy counts: %s', async count => {
    const wrapper = openModal()
    await wrapper.get('input').setValue(count)
    expect(wrapper.findAll('button')[1].attributes('disabled')).toBeDefined()
    await wrapper.get('input').trigger('keyup', { key: 'Enter' })
    expect(batchDuplicate).not.toHaveBeenCalled()
  })

  it('prevents repeated submissions while saving and keeps the dialog open on failure', async () => {
    let reject!: (error: Error) => void
    batchDuplicate.mockImplementationOnce(() => new Promise((_resolve, rejectPromise) => { reject = rejectPromise }))
    const wrapper = openModal()
    await wrapper.findAll('button')[1].trigger('click')
    await wrapper.get('input').trigger('keyup', { key: 'Enter' })
    expect(batchDuplicate).toHaveBeenCalledTimes(1)
    expect(wrapper.findAll('button')[0].attributes('disabled')).toBeDefined()
    reject(new Error('copy failed'))
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('copy failed')
    expect(wrapper.emitted('close')).toBeUndefined()
    expect(wrapper.emitted('duplicated')).toBeUndefined()
    expect(wrapper.findAll('button')[1].attributes('disabled')).toBeUndefined()
  })
})
