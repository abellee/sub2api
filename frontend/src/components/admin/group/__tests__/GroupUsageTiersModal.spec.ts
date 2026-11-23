import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import GroupUsageTiersModal from '../GroupUsageTiersModal.vue'
import type { AdminGroup } from '@/types'

const mocks = vi.hoisted(() => ({ getUsageTiers: vi.fn(), saveUsageTiers: vi.fn(), getAll: vi.fn(), models: vi.fn(), users: vi.fn(), user: vi.fn(), showError: vi.fn(), showSuccess: vi.fn() }))
vi.mock('@/api/admin/groups', () => ({ getUsageTiers: mocks.getUsageTiers, saveUsageTiers: mocks.saveUsageTiers, getModelAllowlistCandidates: mocks.models }))
vi.mock('@/api/admin/users', () => ({ default: { list: mocks.users, getById: mocks.user } }))
vi.mock('@/api/admin', () => ({ adminAPI: { groups: { getAllIncludingInactive: mocks.getAll } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const config = () => ({ enabled: true, models: [], tiers: [{ daily_tokens: 1000000, consecutive_days: 2, max_missed_days: 3, rate_multiplier: 0.8 }], linked_group_ids: [2], excluded_user_ids: [3] })
const render = () => mount(GroupUsageTiersModal, {
  props: { show: true, group: { id: 1, name: 'Source', rate_multiplier: 1, platform: 'openai' } as AdminGroup },
  global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' } } },
})

describe('group usage tier settings', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    mocks.getUsageTiers.mockResolvedValue(config())
    mocks.getAll.mockResolvedValue([{ id: 1, name: 'Source' }, { id: 2, name: 'Linked' }])
    mocks.models.mockResolvedValue(['model-a', 'model-b'])
    mocks.user.mockResolvedValue({ id: 3, username: 'Saved', email: 'saved@example.com' })
    mocks.users.mockResolvedValue({ items: [{ id: 4, username: 'New', email: 'new@example.com' }], pages: 1 })
    mocks.saveUsageTiers.mockResolvedValue(undefined)
  })
  afterEach(() => vi.useRealTimers())

  it('searches users, renders saved and new selections as removable tags, and saves IDs', async () => {
    vi.useFakeTimers()
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[data-test="excluded-user-tag"]').text()).toContain('Saved · saved@example.com')
    await wrapper.get('#tier-excluded-users').setValue('new')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(mocks.users).toHaveBeenCalledWith(1, 30, { search: 'new', include_subscriptions: false }, { signal: expect.any(AbortSignal) })
    await wrapper.get('[data-test="excluded-user-result"]').trigger('click')
    expect(wrapper.findAll('[data-test="excluded-user-tag"]')).toHaveLength(2)
    await wrapper.get('[data-test="excluded-user-tag"] button').trigger('click')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.saveUsageTiers).toHaveBeenCalledWith(1, { ...config(), excluded_user_ids: [4] })
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })

  it('selects specific models and saves token thresholds', async () => {
    const wrapper = render()
    await wrapper.setProps({ group: { id: 1, name: 'Source', platform: 'openai', model_allowlist: { enabled: true, models: ['model-b'] } } as AdminGroup })
    await flushPromises()
    await wrapper.get('#tier-model-scope').setValue('selected')
    expect(wrapper.findAll('label').some(label => label.text() === 'model-a')).toBe(false)
    const option = wrapper.findAll('label').find(label => label.text() === 'model-b')!
    await option.get('input').setValue(true)
    await wrapper.get('#tier-tokens-0').setValue('2000000')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.saveUsageTiers).toHaveBeenCalledWith(1, { ...config(), models: ['model-b'], tiers: [{ ...config().tiers[0], daily_tokens: 2000000 }] })
    wrapper.unmount()
  })

  it('rejects fractional tokens and empty model selection, and retains the draft on save failure', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('#tier-tokens-0').setValue('1.5')
    await wrapper.get('form').trigger('submit')
    expect(mocks.saveUsageTiers).not.toHaveBeenCalled()
    await wrapper.get('#tier-tokens-0').setValue('100')
    await wrapper.get('#tier-model-scope').setValue('selected')
    await wrapper.get('form').trigger('submit')
    expect(mocks.saveUsageTiers).not.toHaveBeenCalled()
    await wrapper.get('#tier-model-scope').setValue('all')
    mocks.saveUsageTiers.mockRejectedValue(new Error('offline'))
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.emitted('close')).toBeUndefined()
    expect(mocks.showError).toHaveBeenCalled()
    wrapper.unmount()
  })

  it('prevents older group loads and user searches from replacing the current group', async () => {
    vi.useFakeTimers()
    let resolveSearch!: (value: unknown) => void
    mocks.users.mockReturnValueOnce(new Promise(resolve => { resolveSearch = resolve }))
    const wrapper = render()
    await flushPromises()
    await wrapper.get('#tier-excluded-users').setValue('old')
    await vi.advanceTimersByTimeAsync(300)
    await wrapper.setProps({ group: { id: 2, name: 'New' } as AdminGroup })
    await flushPromises()
    resolveSearch({ items: [{ id: 999, email: 'old@example.com' }] })
    await flushPromises()
    expect(wrapper.find('[data-test="excluded-user-result"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="excluded-user-tag"]').text()).toContain('saved@example.com')
    wrapper.unmount()
  })

  it('discards a stale configuration load', async () => {
    let resolveOld!: (value: ReturnType<typeof config>) => void
    mocks.getUsageTiers.mockReturnValueOnce(new Promise(resolve => { resolveOld = resolve }))
    const wrapper = render()
    await wrapper.setProps({ group: { id: 2, name: 'New' } as AdminGroup })
    await flushPromises()
    resolveOld({ ...config(), excluded_user_ids: [999] })
    await flushPromises()
    expect(wrapper.findAll('[data-test="excluded-user-tag"]')).toHaveLength(1)
    expect(wrapper.get('[data-test="excluded-user-tag"]').text()).toContain('saved@example.com')
    wrapper.unmount()
  })
})
