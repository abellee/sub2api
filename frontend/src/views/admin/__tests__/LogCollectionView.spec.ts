import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LogCollectionView from '../LogCollectionView.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'

const api = vi.hoisted(() => ({ list: vi.fn(), create: vi.fn(), stop: vi.fn(), records: vi.fn() }))
vi.mock('@/api/admin/latencyCollections', () => ({ latencyCollectionsAPI: api }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key, te: () => false }) }))
const session = { sequence: 1, id: 'fba293e2-1476-43b7-bc18-3596b6ab4a89', user_id: 2, group_id: 145, account_id: 748, created_by: 1, started_at: '2026-11-17T10:00:00Z', stopped_at: null, stop_reason: '', record_count: 0 }
beforeEach(() => {
  vi.resetAllMocks()
  api.list.mockResolvedValue({ items: [], next_before: 0, health: { ready: true, queue_depth: 0, dropped: 0, write_failed: 0, refresh_failed: 0 } })
  api.create.mockResolvedValue(session)
  api.stop.mockResolvedValue(undefined)
  api.records.mockResolvedValue({ items: [], next_before: 0 })
})
async function render() {
  const wrapper = mount(LogCollectionView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, ConfirmDialog: true } } })
  await flushPromises()
  return wrapper
}
function button(wrapper: ReturnType<typeof mount>, text: string) { return wrapper.findAll('button').find(item => item.text() === text)! }

describe('latency collection administration', () => {
  it('confirms the exact scope, shows its unique ID and stops without removing history', async () => {
    const wrapper = await render()
    const inputs = wrapper.findAll('input[type="number"]')
    for (const [index, value] of [2, 145, 748].entries()) await inputs[index]!.setValue(value)
    await wrapper.findAll('form')[0]!.trigger('submit')
    expect(api.create).not.toHaveBeenCalled()
    expect(wrapper.findComponent(ConfirmDialog).props('show')).toBe(true)
    api.list.mockResolvedValue({ items: [session], next_before: 0 })
    wrapper.findComponent(ConfirmDialog).vm.$emit('confirm')
    await flushPromises()
    expect(api.create).toHaveBeenCalledTimes(1)
    expect(api.create).toHaveBeenCalledWith({ user_id: 2, group_id: 145, account_id: 748 })
    expect(wrapper.text()).toContain(session.id)
    expect(api.records).toHaveBeenCalledWith(session.id, '', 0)
    api.list.mockResolvedValue({ items: [{ ...session, stopped_at: '2026-11-17T10:05:00Z', record_count: 1 }], next_before: 0 })
    await button(wrapper, 'admin.logCollection.stop').trigger('click'); await flushPromises()
    expect(api.stop).toHaveBeenCalledTimes(1)
    expect(api.stop).toHaveBeenCalledWith(session.id)
    expect(wrapper.text()).toContain('admin.logCollection.stopped')
    expect(wrapper.text()).toContain(session.id)
    wrapper.unmount()
  })

  it('rejects invalid scope without calling the API', async () => {
    const wrapper = await render()
    await wrapper.findAll('form')[0]!.trigger('submit')
    expect(api.create).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.logCollection.invalid')
    wrapper.unmount()
  })

  it('queries a collection and accepts client request IDs', async () => {
    api.list.mockResolvedValue({ items: [session], next_before: 0 })
    const wrapper = await render()
    await wrapper.get('input[aria-label="admin.logCollection.collectionId"]').setValue(session.id)
    await wrapper.findAll('form')[1]!.trigger('submit'); await flushPromises()
    expect(api.list).toHaveBeenLastCalledWith(session.id, 0)
    await button(wrapper, session.id).trigger('click'); await flushPromises()
    await wrapper.get('input[aria-label="admin.logCollection.requestId"]').setValue('client:example')
    await wrapper.findAll('form')[2]!.trigger('submit'); await flushPromises()
    expect(api.records).toHaveBeenLastCalledWith(session.id, 'client:example', 0)
    wrapper.unmount()
  })
})
