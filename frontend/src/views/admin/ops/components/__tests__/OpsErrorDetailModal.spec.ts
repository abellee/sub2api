import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import OpsErrorDetailModal from '../OpsErrorDetailModal.vue'

const mocks = vi.hoisted(() => ({
  getRequestErrorDetail: vi.fn(),
  listRequestErrorUpstreamErrors: vi.fn()
}))

vi.mock('@/api/admin/ops', () => ({
  opsAPI: {
    getRequestErrorDetail: mocks.getRequestErrorDetail,
    getUpstreamErrorDetail: vi.fn(),
    listRequestErrorUpstreamErrors: mocks.listRequestErrorUpstreamErrors
  }
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: vi.fn() })
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

describe('OpsErrorDetailModal', () => {
  beforeEach(() => {
    mocks.getRequestErrorDetail.mockReset()
    mocks.listRequestErrorUpstreamErrors.mockReset()
    mocks.listRequestErrorUpstreamErrors.mockResolvedValue({ items: [] })
  })

  it('prioritizes upstream root cause and deduplicates diagnostic payloads', async () => {
    mocks.getRequestErrorDetail.mockResolvedValue({
      id: 1,
      created_at: '2026-08-19T00:00:00Z',
      phase: 'request',
      type: 'upstream_error',
      error_owner: 'provider',
      error_source: 'gateway',
      severity: 'P1',
      status_code: 502,
      upstream_status_code: 429,
      platform: 'openai',
      model: 'gpt-5.6',
      resolved: false,
      request_id: 'rid-1',
      message: 'All available accounts exhausted',
      error_body: '{"error":"same"}',
      upstream_error_message: 'provider rate limit exhausted',
      upstream_error_detail: '{"error":"same"}',
      upstream_errors: '[]',
      account_name: 'account',
      group_name: 'group',
      is_business_limited: false
    })

    const wrapper = shallowMount(OpsErrorDetailModal, {
      props: { show: true, errorId: 1, errorType: 'request' },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /></div>' },
          Icon: true
        }
      }
    })
    await flushPromises()

    expect(wrapper.text()).toContain('provider rate limit exhausted')
    expect(wrapper.text()).toContain('admin.ops.errorDetail.upstreamStatus')
    expect(wrapper.text()).toContain('429')
    expect(wrapper.findAll('pre')).toHaveLength(2)
    expect(wrapper.text()).not.toContain('admin.ops.errorDetail.payloads.upstream_detail')
  })

  it('shows the account on request errors and account/model on linked upstream errors', async () => {
    mocks.getRequestErrorDetail.mockResolvedValue({
      id: 2, phase: 'request', error_owner: 'provider',
      created_at: '2026-11-13T00:00:00Z', status_code: 502,
      request_id: 'request-2', user_email: 'user@example.test',
      account_id: 305, account_name: 'gemini-ultra7', model: 'gemini-3.8-flash'
    })
    mocks.listRequestErrorUpstreamErrors.mockResolvedValue({ items: [{
      id: 3, phase: 'upstream', error_owner: 'provider', status_code: 429,
      account_id: 273, account_name: 'gemini6',
      requested_model: 'gemini-alias', upstream_model: 'gemini-3.8-flash'
    }, {
      id: 4, phase: 'upstream', error_owner: 'provider', status_code: 502,
      account_id: 400, model: 'gemini-fallback'
    }] })

    const wrapper = shallowMount(OpsErrorDetailModal, {
      props: { show: true, errorId: 2, errorType: 'request' },
      global: { stubs: { BaseDialog: { template: '<div><slot /></div>' }, Icon: true } }
    })
    await flushPromises()

    expect(wrapper.text()).toContain('user@example.test')
    expect(wrapper.get('[data-testid="error-detail-account"]').text()).toContain('gemini-ultra7')
    expect(wrapper.get('[data-testid="error-detail-account"]').text()).toContain('#305')
    const accounts = wrapper.findAll('[data-testid="upstream-error-account"]')
    expect(accounts[0].text()).toContain('gemini6')
    expect(accounts[0].text()).toContain('#273')
    expect(accounts[1].text()).toContain('#400')
    expect(wrapper.findAll('[data-testid="upstream-error-model"]').map(node => node.text())).toEqual([
      expect.stringContaining('gemini-alias → gemini-3.8-flash'), expect.stringContaining('gemini-fallback')
    ])
  })
})
