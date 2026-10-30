import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import MenuManagementView from '../MenuManagementView.vue'
import { useUserMenuStore } from '@/stores/userMenu'
import type { UserMenuConfig } from '@/api/userMenu'

const mocks = vi.hoisted(() => ({
  getUserMenu: vi.fn(), saveUserMenu: vi.fn(), getSettings: vi.fn(),
  showError: vi.fn(), showSuccess: vi.fn(),
}))
vi.mock('@/api/userMenu', () => ({ getUserMenu: mocks.getUserMenu, saveUserMenu: mocks.saveUserMenu }))
vi.mock('@/api/admin/settings', () => ({ getSettings: mocks.getSettings }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: mocks.showError, showSuccess: mocks.showSuccess }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))

let wrapper: ReturnType<typeof mount>
const initial: UserMenuConfig = {
  sections: [
    { id: 'api', title: 'API Services', items: ['/keys', '/usage'] },
    { id: 'account', title: 'Account', items: ['/profile'] },
  ],
}

beforeEach(() => {
  vi.clearAllMocks()
  setActivePinia(createPinia())
  mocks.getUserMenu.mockResolvedValue(structuredClone(initial))
  mocks.saveUserMenu.mockImplementation(async config => structuredClone(config))
  mocks.getSettings.mockResolvedValue({ custom_menu_items: [
    { id: 'docs', label: 'User docs', visibility: 'user', sort_order: 0 },
    { id: 'internal', label: 'Admin-only docs', visibility: 'admin', sort_order: 1 },
  ] })
})
afterEach(() => wrapper?.unmount())

function render() {
  wrapper = mount(MenuManagementView, { global: { stubs: { Icon: true } } })
  return wrapper
}

describe('MenuManagementView', () => {
  it('saves hidden builtin and custom menus, reloads them, and restores their placement', async () => {
    render()
    await flushPromises()
    const store = useUserMenuStore()
    await wrapper.get('[data-testid="visible-/keys"]').trigger('click')
    await wrapper.get('[data-testid="visible-/custom/docs"]').trigger('click')
    expect(store.config).toEqual(initial)
    await wrapper.get('[data-testid="save-menu"]').trigger('click')
    await flushPromises()
    expect(store.config.hidden_items).toEqual(['/keys', '/custom/docs'])
    expect(store.config.sections).toEqual(initial.sections)
    mocks.getUserMenu.mockResolvedValueOnce(JSON.parse(JSON.stringify(store.config)))
    wrapper.unmount()
    render()
    await flushPromises()
    expect(wrapper.get('[data-testid="visible-/keys"]').attributes('aria-checked')).toBe('false')
    await wrapper.get('[data-testid="visible-/keys"]').trigger('click')
    await wrapper.get('[data-testid="save-menu"]').trigger('click')
    await flushPromises()
    expect(store.config.hidden_items).toEqual(['/custom/docs'])
    expect(store.config.sections[0].items).toEqual(['/keys', '/usage'])
  })

  it('retains hidden menus after a failed save and discards the draft on reload', async () => {
    mocks.getUserMenu.mockResolvedValue({ ...structuredClone(initial), hidden_items: ['/keys'] })
    render()
    await flushPromises()
    await wrapper.get('[data-testid="visible-/keys"]').trigger('click')
    mocks.saveUserMenu.mockRejectedValueOnce(new Error('storage unavailable'))
    await wrapper.get('[data-testid="save-menu"]').trigger('click')
    await flushPromises()
    expect(useUserMenuStore().config.hidden_items).toEqual(['/keys'])
    expect(wrapper.get('[data-testid="visible-/keys"]').attributes('aria-checked')).toBe('true')
    await wrapper.get('header .btn-secondary').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="visible-/keys"]').attributes('aria-checked')).toBe('false')
    expect(wrapper.get('[data-testid="save-menu"]').attributes('disabled')).toBeDefined()
  })
  it('loads editable headings and includes user custom pages without admin-only pages', async () => {
    render()
    await flushPromises()
    expect(wrapper.text()).toContain('User docs')
    expect(wrapper.text()).not.toContain('Admin-only docs')
    expect((wrapper.get('[data-testid="assign-/keys"]').element as HTMLSelectElement).value).toBe('api')
    expect(wrapper.get('[data-testid="save-menu"]').attributes('disabled')).toBeDefined()
  })

  it('adds a heading, reassigns a menu, and updates live sidebar configuration only after saving', async () => {
    render()
    await flushPromises()
    const store = useUserMenuStore()
    await wrapper.get('[data-testid="new-heading"]').setValue('Tools')
    await wrapper.get('form').trigger('submit')
    const select = wrapper.get('[data-testid="assign-/keys"]')
    const newId = select.findAll('option').find(option => option.text() === 'Tools')!.attributes('value')!
    await select.setValue(newId)
    expect(store.config).toEqual(initial, 'draft edits must not mutate the live sidebar')
    await wrapper.get('[data-testid="save-menu"]').trigger('click')
    await flushPromises()
    expect(mocks.saveUserMenu).toHaveBeenCalledOnce()
    expect(store.config.sections[0].items).toEqual(['/usage'])
    expect(store.config.sections[2]).toEqual({ id: newId, title: 'Tools', items: ['/keys'] })
    expect(mocks.showSuccess).toHaveBeenCalledOnce()
    expect(wrapper.get('[data-testid="save-menu"]').attributes('disabled')).toBeDefined()
  })

  it('deleting a heading returns its menu items to the default section', async () => {
    render()
    await flushPromises()
    await wrapper.get('[data-testid="delete-heading-api"]').trigger('click')
    expect((wrapper.get('[data-testid="assign-/keys"]').element as HTMLSelectElement).value).toBe('')
    await wrapper.get('[data-testid="save-menu"]').trigger('click')
    await flushPromises()
    expect(useUserMenuStore().config.sections).toEqual([initial.sections[1]])
  })

  it('keeps the saved menu and unsaved draft intact when saving fails', async () => {
    render()
    await flushPromises()
    await wrapper.get('[data-testid="assign-/keys"]').setValue('account')
    mocks.saveUserMenu.mockRejectedValueOnce(new Error('storage unavailable'))
    await wrapper.get('[data-testid="save-menu"]').trigger('click')
    await flushPromises()
    expect(useUserMenuStore().config).toEqual(initial)
    expect((wrapper.get('[data-testid="assign-/keys"]').element as HTMLSelectElement).value).toBe('account')
    expect(wrapper.get('[data-testid="save-menu"]').attributes('disabled')).toBeUndefined()
    expect(mocks.showError).toHaveBeenCalledOnce()
  })

  it('cannot overwrite a configuration that failed to load', async () => {
    mocks.getUserMenu.mockRejectedValueOnce(new Error('unavailable'))
    render()
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="save-menu"]').attributes('disabled')).toBeDefined()
    expect(mocks.saveUserMenu).not.toHaveBeenCalled()
  })
})
