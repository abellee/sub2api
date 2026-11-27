import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import UserBalanceModal from '../UserBalanceModal.vue'
import type { AdminUser } from '@/types'
const mocks = vi.hoisted(() => ({ updateBalance: vi.fn(), showError: vi.fn(), showSuccess: vi.fn(), fetchAdminUserPoints: vi.fn(), rechargeUserPoints: vi.fn(), updateUserPoints: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { users: mocks } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: 'en' }) }))
vi.mock('@/api/points', () => ({ fetchAdminUserPoints: mocks.fetchAdminUserPoints, rechargeUserPoints: mocks.rechargeUserPoints }))
vi.mock('@/composables/useUserPoints', () => ({ updateUserPoints: mocks.updateUserPoints }))
enableAutoUnmount(afterEach)
beforeEach(() => { vi.clearAllMocks(); vi.spyOn(console, 'error').mockImplementation(() => {}) })
afterEach(() => vi.restoreAllMocks())
describe('balance operation error feedback', () => {
  it.each([
    [{ message: 'User balance changed; retry' }, 'User balance changed; retry'],
    [{ response: { data: { detail: 'Legacy detail' } } }, 'Legacy detail'],
    [{}, 'common.error'],
  ])('shows the available API error and keeps the dialog open', async (error, message) => {
    mocks.updateBalance.mockRejectedValueOnce(error)
    const w = mount(UserBalanceModal, { props: { show: true, operation: 'add', user: { id: 1, email: 'a@example.com', balance: 5 } as AdminUser },
      global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } } })
    await w.get('input[type="number"]').setValue('1')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.showError).toHaveBeenCalledWith(message)
    expect(w.emitted('success')).toBeUndefined(); expect(w.emitted('close')).toBeUndefined()
    expect(w.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })
})

describe('points recharge', () => {
  async function render() {
    mocks.fetchAdminUserPoints.mockResolvedValue({ balance: '9007199254740993' })
    const wrapper = mount(UserBalanceModal, { props: { show: true, operation: 'add', user: { id: 2, email: 'user2@example.com', balance: 5 } as AdminUser }, global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } } })
    await wrapper.get('select').setValue('points')
    await flushPromises()
    return wrapper
  }
  it('keeps integer points separate from dollars and preserves large balance precision', async () => {
    mocks.rechargeUserPoints.mockResolvedValue({ points: { balance: '9007199254741093', total_earned: '9007199254741093', total_spent: '0' }, granted: true })
    const wrapper = await render()
    expect(wrapper.text()).toContain('9,007,199,254,740,993')
    await wrapper.get('input[type="number"]').setValue('100')
    expect(wrapper.text()).toContain('9,007,199,254,741,093')
    await wrapper.get('textarea').setValue('support credit')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.rechargeUserPoints).toHaveBeenCalledWith(2, '100', 'support credit', expect.any(String))
    expect(mocks.updateBalance).not.toHaveBeenCalled()
    expect(wrapper.emitted('success')).toHaveLength(1)
  })
  it('rejects decimal points and reuses the request ID after a lost response', async () => {
    const wrapper = await render()
    await wrapper.get('input[type="number"]').setValue('1.5')
    await wrapper.get('form').trigger('submit')
    expect(mocks.showError).toHaveBeenCalledWith('common.pointsAdmin.invalidAmount')
    expect(mocks.rechargeUserPoints).not.toHaveBeenCalled()
    mocks.rechargeUserPoints.mockRejectedValue(new Error('connection lost'))
    await wrapper.get('input[type="number"]').setValue('100')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(mocks.rechargeUserPoints).toHaveBeenCalledTimes(2)
    expect(mocks.rechargeUserPoints.mock.calls[0][3]).toBe(mocks.rechargeUserPoints.mock.calls[1][3])
    expect(wrapper.emitted('success')).toBeUndefined()
  })
  it('does not expose points in the withdrawal dialog', () => {
    const wrapper = mount(UserBalanceModal, { props: { show: true, operation: 'subtract', user: { id: 2, email: 'user2@example.com', balance: 5 } as AdminUser }, global: { stubs: { BaseDialog: { template: '<div><slot /></div>' } } } })
    expect(wrapper.find('select').exists()).toBe(false)
  })
})
