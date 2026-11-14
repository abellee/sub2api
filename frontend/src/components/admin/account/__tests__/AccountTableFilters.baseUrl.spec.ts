import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountTableFilters from '../AccountTableFilters.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('upstream API address filter', () => {
  it('keeps other filters while changing and clearing the address', async () => {
    const filters = { platform: 'grok', group: '29', base_url: 'api.example.com' }
    const wrapper = mount(AccountTableFilters, { props: { searchQuery: '', filters } })
    const input = wrapper.get('input[placeholder="admin.accounts.filterByApiUrl"]')
    expect((input.element as HTMLInputElement).value).toBe('api.example.com')
    await input.setValue('https://api.example.com/v1')
    expect(wrapper.emitted('update:filters')?.at(-1)?.[0]).toEqual({ ...filters, base_url: 'https://api.example.com/v1' })
    await input.setValue('')
    expect(wrapper.emitted('update:filters')?.at(-1)?.[0]).toEqual({ ...filters, base_url: '' })
    wrapper.unmount()
  })
})
