import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AccountMatchingView from '../AccountMatchingView.vue'

const mocks = vi.hoisted(() => ({ list: vi.fn(), getInputRouting: vi.fn(), saveInputRouting: vi.fn(), searchUsers: vi.fn(), save: vi.fn(), delete: vi.fn(), accounts: vi.fn(), groups: vi.fn(), showSuccess: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/admin/accountMatching', () => ({ accountMatchingAPI: mocks }))
vi.mock('@/api/admin/accounts', () => ({ default: { list: mocks.accounts } }))
vi.mock('@/api/admin/groups', () => ({ default: { getAllIncludingInactive: mocks.groups } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/constants/platforms', () => ({ CONCRETE_PLATFORM_OPTIONS: [{ value: 'openai', label: 'OpenAI' }, { value: 'gemini', label: 'Gemini' }] }))

const rule = { id: 7, user_id: 2, email: 'user2@example.test', group_ids: [10], account_ids: [20], created_at: '', updated_at: '' }
beforeEach(() => {
  vi.resetAllMocks()
  vi.useFakeTimers()
  mocks.list.mockResolvedValue([])
  mocks.getInputRouting.mockResolvedValue({ enabled: false, threshold: 100000, group_ids: [], account_ids: [] })
  mocks.saveInputRouting.mockImplementation(async input => ({ ...input, revision: 'new' }))
  mocks.groups.mockResolvedValue([{ id: 10, name: 'Group A', platform: 'openai' }, { id: 11, name: 'Group B', platform: 'gemini' }])
  mocks.accounts.mockImplementation(async (page: number) => ({ items: page === 1 ? [{ id: 20, name: 'Account A', platform: 'openai' }] : [{ id: 21, name: 'Account B', platform: 'gemini' }], pages: 2, page, total: 2 }))
  mocks.searchUsers.mockResolvedValue([{ id: 2, email: rule.email }])
  mocks.save.mockImplementation(async (input) => ({ ...rule, ...input }))
})
afterEach(() => { vi.useRealTimers() })
async function render() {
  const wrapper = mount(AccountMatchingView, { global: { stubs: {
    AppLayout: { template: '<div><slot /></div>' },
    TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /></div>' },
    BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
    ConfirmDialog: true, EmptyState: true, Icon: true
  } } })
  await flushPromises()
  return wrapper
}
function button(wrapper: ReturnType<typeof mount>, text: string) { return wrapper.findAll('button').find(b => b.text() === text)! }

describe('Account matching', () => {
  it('loads every account page, searches users and saves multiple groups and accounts across platforms', async () => {
    const wrapper = await render()
    expect(mocks.accounts).toHaveBeenCalledTimes(2)
    await button(wrapper, 'common.add').trigger('click')
    await wrapper.get('#matching-email').setValue('user2@')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(mocks.searchUsers).toHaveBeenCalledWith('user2@', expect.any(AbortSignal))
    await wrapper.get('input[type="radio"]').setValue(true)
    await wrapper.get('input[type="checkbox"][value="10"]').setValue(true)
    await wrapper.get('input[type="checkbox"][value="11"]').setValue(true)
    await wrapper.get('input[type="checkbox"][value="20"]').setValue(true)
    await wrapper.get('select').setValue('gemini')
    expect(wrapper.find('input[type="checkbox"][value="20"]').exists()).toBe(false)
    await wrapper.get('input[type="checkbox"][value="21"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.save).toHaveBeenCalledWith({ user_id: 2, group_ids: [10, 11], account_ids: [20, 21] }, undefined)
    expect(wrapper.text()).toContain(rule.email)
    wrapper.unmount()
  })

  it('clears the selected user when the search changes and ignores stale search replies', async () => {
    let resolveOld!: (value: unknown) => void
    mocks.searchUsers.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    const wrapper = await render()
    await button(wrapper, 'common.add').trigger('click')
    await wrapper.get('#matching-email').setValue('old@')
    await vi.advanceTimersByTimeAsync(300)
    await wrapper.get('#matching-email').setValue('user2@')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    resolveOld([{ id: 99, email: 'old@example.test' }])
    await flushPromises()
    expect(wrapper.text()).not.toContain('old@example.test')
    await wrapper.get('input[type="radio"]').setValue(true)
    await wrapper.get('input[type="checkbox"][value="10"]').setValue(true)
    await wrapper.get('input[type="checkbox"][value="20"]').setValue(true)
    await wrapper.get('#matching-email').setValue('different@')
    await wrapper.get('form').trigger('submit')
    expect(mocks.save).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('edits without changing owner and retains the form on failed save', async () => {
    mocks.list.mockResolvedValue([rule])
    mocks.save.mockRejectedValue(new Error('conflict'))
    const wrapper = await render()
    await button(wrapper, 'common.edit').trigger('click')
    expect(wrapper.get('#matching-email').attributes('disabled')).toBeDefined()
    await wrapper.get('input[type="checkbox"][value="21"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.save).toHaveBeenCalledWith({ user_id: 2, group_ids: [10], account_ids: [20, 21] }, 7)
    expect(wrapper.find('form').exists()).toBe(true)
    wrapper.unmount()
  })
})

describe('Input token restriction settings', () => {
  it('saves the switch, threshold, groups and exactly two accounts independently of manual matching', async () => {
    const wrapper = await render()
    await button(wrapper, 'admin.accountMatching.inputRoutingTitle').trigger('click')
    const form = wrapper.get('#input-routing-form')
    await form.get('#input-routing-enabled').setValue(true)
    await form.get('#input-routing-threshold').setValue(120000)
    await form.get('input[value="10"]').setValue(true)
    await form.get('input[value="20"]').setValue(true)
    await form.trigger('submit')
    expect(mocks.saveInputRouting).not.toHaveBeenCalled()
    await form.get('input[value="21"]').setValue(true)
    await form.trigger('submit')
    await flushPromises()
    expect(mocks.saveInputRouting).toHaveBeenCalledWith({ enabled: true, threshold: 120000, group_ids: [10], account_ids: [20, 21] })
    expect(mocks.save).not.toHaveBeenCalled()
    expect(wrapper.find('#input-routing-form').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.accountMatching.inputRoutingActive')
    wrapper.unmount()
  })

  it('can turn the feature off after selected groups or accounts were deleted', async () => {
    mocks.getInputRouting.mockResolvedValue({ enabled: true, threshold: 100000, group_ids: [99], account_ids: [98, 97], revision: 'old' })
    const wrapper = await render()
    await button(wrapper, 'admin.accountMatching.inputRoutingTitle').trigger('click')
    const form = wrapper.get('#input-routing-form')
    await form.get('#input-routing-enabled').setValue(false)
    await form.trigger('submit')
    await flushPromises()
    expect(mocks.saveInputRouting).toHaveBeenCalledWith(expect.objectContaining({ enabled: false }))
    wrapper.unmount()
  })

  it('retains edits and shows errors when saving fails', async () => {
    mocks.saveInputRouting.mockRejectedValue(new Error('save failed'))
    const wrapper = await render()
    await button(wrapper, 'admin.accountMatching.inputRoutingTitle').trigger('click')
    await wrapper.get('#input-routing-threshold').setValue(150000)
    await wrapper.get('#input-routing-form').trigger('submit')
    await flushPromises()
    expect(wrapper.find('#input-routing-form').exists()).toBe(true)
    expect((wrapper.get('#input-routing-threshold').element as HTMLInputElement).value).toBe('150000')
    expect(wrapper.get('#input-routing-form [role="alert"]').text()).toContain('save failed')
    wrapper.unmount()
  })
})
