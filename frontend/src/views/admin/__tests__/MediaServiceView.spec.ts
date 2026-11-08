import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import MediaServiceView from '../MediaServiceView.vue'

const api = vi.hoisted(() => ({ config: vi.fn(), saveConfig: vi.fn(), agent: vi.fn(), bindings: vi.fn(), saveBinding: vi.fn(), deleteBinding: vi.fn(), records: vi.fn() }))
vi.mock('@/api/admin/mediaService', () => ({ mediaServiceAPI: api }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...(await importOriginal<typeof import('vue-i18n')>()), useI18n: () => ({ t: (key: string) => key }) }))

const view = { settings: { mediad_base_url: 'http://127.0.0.1:18090', management_token_configured: true, relay_secret_configured: true, callback_secret_configured: true }, active: {}, restart_required: false, pending_fields: [] }
enableAutoUnmount(afterEach)
function mountView() { return mount(MediaServiceView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' } } } }) }

describe('Media service administration', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.config.mockResolvedValue(view)
    api.saveConfig.mockResolvedValue({ ...view, restart_required: true, pending_fields: ['mediad_base_url'] })
    api.bindings.mockResolvedValue({ bindings: [] })
    api.records.mockResolvedValue({ records: [] })
    api.agent.mockImplementation((_method, path) => Promise.resolve(path === '/schemas' ? { schemas: [{ id: 'image-default', version: 1 }] } : {}))
  })
  it('does not round-trip saved secret flags and clears entered secrets after save', async () => {
    const wrapper = mountView(); await flushPromises()
    expect((wrapper.get('[data-testid="main-management_token"]').element as HTMLInputElement).value).toBe('')
    await wrapper.get('[data-testid="main-management_token"]').setValue('new-secret')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.saveConfig).toHaveBeenCalledWith(expect.objectContaining({ management_token: 'new-secret', relay_secret: '', callback_secret: '' }))
    expect(api.saveConfig.mock.calls[0][0]).not.toHaveProperty('management_token_configured')
    expect((wrapper.get('[data-testid="main-management_token"]').element as HTMLInputElement).value).toBe('')
    expect(wrapper.get('[data-testid="main-restart"]').text()).toContain('mediaService.mainRestart')
  })
  it('shows an info tooltip for each setting and removes main-site file storage controls', async () => {
    const wrapper = mountView(); await flushPromises()
    expect(wrapper.find('[data-testid="main-storage_root"]').exists()).toBe(false)
    const infos = wrapper.findAll('[data-testid="setting-info"]')
    expect(infos).toHaveLength(8)
    await infos[1].trigger('mouseenter'); await flushPromises()
    expect(document.body.textContent).toContain('mediaService.help.mediad_base_url')
    expect((document.querySelectorAll('[role="tooltip"]')[1] as HTMLElement).style.display).not.toBe('none')
    wrapper.unmount()
  })
  it('queries download records with an explicit user ID', async () => {
    const wrapper = mountView(); await flushPromises()
    await wrapper.get('[data-testid="tab-records"]').trigger('click'); await flushPromises()
    await wrapper.get('input').setValue(42)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.records).toHaveBeenCalledWith(42)
    expect(wrapper.text()).toContain('mediaService.noRecords')
    wrapper.unmount()
  })
  it('keeps unsaved input and displays failed saves', async () => {
    api.saveConfig.mockRejectedValue({ message: 'disk unavailable' })
    const wrapper = mountView(); await flushPromises()
    await wrapper.get('[data-testid="main-mediad_base_url"]').setValue('http://new.example')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('disk unavailable')
    expect((wrapper.get('[data-testid="main-mediad_base_url"]').element as HTMLInputElement).value).toBe('http://new.example')
    expect(wrapper.find('[role="status"]').exists()).toBe(false)
  })
  it('shows microservice errors instead of claiming a successful runtime load', async () => {
    api.agent.mockRejectedValue({ message: 'service offline' })
    const wrapper = mountView(); await flushPromises()
    await wrapper.get('[data-testid="tab-runtime"]').trigger('click'); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('service offline')
    expect(wrapper.get('form button').attributes('disabled')).toBeDefined()
  })
  it('saves zero file retention and shows changes as immediately applied', async () => {
    const runtime = { settings: { file_retention_hours: 72 }, active: { file_retention_hours: 72 }, restart_required: false, pending_fields: [] }
    api.agent.mockResolvedValue(runtime)
    const wrapper = mountView(); await flushPromises()
    await wrapper.get('[data-testid="tab-runtime"]').trigger('click'); await flushPromises()
    const input = wrapper.findAll('input[type="number"]').find(input => input.attributes('min') === '0')!
    expect(input.exists()).toBe(true)
    expect(input.attributes('max')).toBeUndefined()
    await input.setValue(0)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.agent).toHaveBeenCalledWith('PUT', '/config', expect.objectContaining({ file_retention_hours: 0 }))
    expect(wrapper.get('[role="status"]').text()).toBe('mediaService.saved')
  })
  it('saves a group binding with the selected rule', async () => {
    const wrapper = mountView(); await flushPromises()
    await wrapper.get('[data-testid="tab-bindings"]').trigger('click'); await flushPromises()
    await wrapper.get('select').setValue('image-default')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(api.saveBinding).toHaveBeenCalledWith({ group_id: 1, schema_id: 'image-default', estimate_kind: 'image', enabled: true })
  })
  it('reports an unsuccessful callback test as a failure', async () => {
    api.agent.mockImplementation((_method, path) => Promise.resolve(path === '/callback' ? { callback_url: 'http://main/internal/media/result', enabled: true } : { delivered: false, error: 'signature rejected' }))
    const wrapper = mountView(); await flushPromises()
    await wrapper.get('[data-testid="tab-callback"]').trigger('click'); await flushPromises()
    const testButton = wrapper.findAll('button').find(button => button.text() === 'mediaService.testCallback')!
    await testButton.trigger('click'); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('signature rejected')
    expect(wrapper.find('[role="status"]').exists()).toBe(false)
  })
})
