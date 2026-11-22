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

  it('shows every attempted account and the final outcome', async () => {
    mocks.getRequestErrorDetail.mockResolvedValue({
      id: 19, phase: 'request', status_code: 200,
      upstream_errors: JSON.stringify([{
        account_id: 31, failover_result: {
          result: 'success', final_account_id: 37,
          attempts: [
            { account_id: 31, account_name: 'First account', result: 'failed', status_code: 502, message: 'Upstream unavailable' },
            { account_id: 37, result: 'success', status_code: 200 }
          ]
        }
      }])
    })
    const wrapper = shallowMount(OpsErrorDetailModal, {
      props: { show: true, errorId: 19, errorType: 'request' },
      global: { stubs: { BaseDialog: { template: '<div><slot /></div>' }, Icon: true } }
    })
    await flushPromises()
    const rows = wrapper.findAll('[data-testid="failover-chain"] li')
    expect(rows).toHaveLength(2)
    expect(wrapper.get('[data-testid="account-switch-summary"]').text()).toContain('admin.ops.errorDetail.accountSwitched')
    expect(rows[0].text()).toContain('#31')
    expect(rows[0].text()).toContain('Upstream unavailable')
    expect(rows[1].text()).toContain('#37')
    expect(rows[1].text()).toContain('admin.ops.errorDetail.failoverOutcome.success')
    expect(wrapper.find('[data-testid="failover-outcome"]').text()).toContain('admin.ops.errorDetail.successAccount')
  })

  it.each([false, true])('shows an unswitched failure even with same-account retries (%s)', async (sameAccountRetry) => {
    const attempt = { account_id: 403, result: 'failed', status_code: 502 }
    mocks.getRequestErrorDetail.mockResolvedValue({
      id: 22, phase: 'request', status_code: 502,
      upstream_errors: JSON.stringify([{ account_id: 403, failover_result: {
        result: 'failed', final_account_id: 403,
        attempts: sameAccountRetry ? [attempt, attempt] : [attempt]
      } }])
    })
    const wrapper = shallowMount(OpsErrorDetailModal, {
      props: { show: true, errorId: 22, errorType: 'request' },
      global: { stubs: { BaseDialog: { template: '<div><slot /></div>' }, Icon: true } }
    })
    await flushPromises()
    expect(wrapper.get('[data-testid="account-switch-summary"]').text()).toContain('admin.ops.errorDetail.noAccountSwitch')
    expect(wrapper.get('[data-testid="failover-outcome"]').text()).toContain('admin.ops.errorDetail.lastAttemptAccount')
    expect(wrapper.get('[data-testid="failover-outcome"]').text()).not.toContain('admin.ops.errorDetail.successAccount')
  })

  it('marks old traces incomplete rather than inventing missing attempts', async () => {
    mocks.getRequestErrorDetail.mockResolvedValue({
      id: 20, phase: 'request', status_code: 502,
      upstream_errors: JSON.stringify([{ account_id: 31, failover_result: { result: 'failed' } }])
    })
    const wrapper = shallowMount(OpsErrorDetailModal, {
      props: { show: true, errorId: 20, errorType: 'request' },
      global: { stubs: { BaseDialog: { template: '<div><slot /></div>' }, Icon: true } }
    })
    await flushPromises()
    expect(wrapper.find('[data-testid="incomplete-failover-chain"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="failover-chain"]').exists()).toBe(false)
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
