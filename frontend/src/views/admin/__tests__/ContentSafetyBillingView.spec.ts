import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ContentSafetyBillingView from '../ContentSafetyBillingView.vue'
import { getContentSafetyBilling, updateContentSafetyBilling, getContentSafetyDebits } from '@/api/admin/contentSafetyBilling'

vi.mock('@/api/admin/contentSafetyBilling', () => ({
  getContentSafetyBilling: vi.fn(), updateContentSafetyBilling: vi.fn(), getContentSafetyDebits: vi.fn(),
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))

const config = { enabled: false, amount: 0.01, match_description: 'SAFETY_CHECK_TYPE', client_message: 'Content rejected' }

beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(getContentSafetyBilling).mockResolvedValue({ ...config })
  vi.mocked(getContentSafetyDebits).mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
})

describe('Content safety billing administration', () => {
  it('loads and saves all settings together with a fixed debit amount', async () => {
    vi.mocked(updateContentSafetyBilling).mockImplementation(async value => value)
    const wrapper = mount(ContentSafetyBillingView)
    await flushPromises()
    await wrapper.get('[data-test="enabled"]').setValue(true)
    await wrapper.get('[data-test="amount"]').setValue('0.05')
    await wrapper.get('[data-test="match"]').setValue('Failed check: SAFETY_CHECK_TYPE')
    await wrapper.get('[data-test="client-message"]').setValue('统一错误信息')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(updateContentSafetyBilling).toHaveBeenCalledWith({ enabled: true, amount: 0.05, match_description: 'Failed check: SAFETY_CHECK_TYPE', client_message: '统一错误信息' })
    expect(wrapper.text()).toContain('common.saved')
    wrapper.unmount()
  })

  it('does not expose a default form when loading the stored configuration fails', async () => {
    vi.mocked(getContentSafetyBilling).mockRejectedValue(new Error('offline'))
    const wrapper = mount(ContentSafetyBillingView)
    await flushPromises()
    expect(wrapper.find('form').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.contentSafetyBilling.loadError')
    expect(updateContentSafetyBilling).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('preserves the draft on save failure and renders the debit note safely', async () => {
    vi.mocked(updateContentSafetyBilling).mockRejectedValue(new Error('offline'))
    vi.mocked(getContentSafetyDebits).mockResolvedValue({
      items: [{ id: 'id-1', user_id: 1, api_key_id: 2, account_id: 3, upstream_request_id: 'xai-1', amount: 0.01, notes: '<script>unsafe</script>', client_message: 'Rejected', status: 'pending', created_at: '2026-10-24T00:00:00Z', charged_at: '', last_error: '' }],
      total: 1, page: 1, page_size: 20,
    })
    const wrapper = mount(ContentSafetyBillingView)
    await flushPromises()
    await wrapper.get('[data-test="client-message"]').setValue('Unsaved text')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('admin.contentSafetyBilling.saveError')
    expect((wrapper.get('[data-test="client-message"]').element as HTMLInputElement).value).toBe('Unsaved text')
    expect(wrapper.find('script').exists()).toBe(false)
    expect(wrapper.text()).toContain('<script>unsafe</script>')
    expect(wrapper.text()).toContain('admin.contentSafetyBilling.pending')
    wrapper.unmount()
  })
})
