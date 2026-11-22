import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import KeyCardGrid from '../KeyCardGrid.vue'
import type { ApiKey } from '@/types'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const rows = [{ id: 1, name: 'First', key: 'sk-one' }, { id: 2, name: 'Second', key: 'sk-two' }] as ApiKey[]
const render = (selectedKeys: number[] = []) => mount(KeyCardGrid, {
  props: {
    data: rows, loading: false, selectedKeys, sortBy: 'created_at', sortOrder: 'desc',
    columns: [{ key: 'name', label: 'Name' }, { key: 'key', label: 'Key' }, { key: 'id', label: 'ID', sortable: true }, { key: 'actions', label: 'Actions' }]
  },
  slots: { 'cell-key': '<template #default="{ value }"><code>masked {{ value.slice(-3) }}</code></template>' }
})

describe('KeyCardGrid', () => {
  it('renders a card for every key with the supplied field renderer', () => {
    const wrapper = render()
    expect(wrapper.findAll('[data-key-id]')).toHaveLength(2)
    expect(wrapper.find('table').exists()).toBe(false)
    expect(wrapper.get('[data-key-id="1"] h2').text()).toBe('First')
    expect(wrapper.get('[data-key-id="1"] code').text()).toBe('masked one')
    expect(wrapper.text()).not.toContain('sk-one')
  })

  it('selects loaded cards and toggles an individual selection', async () => {
    const wrapper = render([1])
    expect((wrapper.get('.key-selection input').element as HTMLInputElement).indeterminate).toBe(true)
    await wrapper.get('[data-key-id="2"] input').setValue(true)
    expect(wrapper.emitted('update:selectedKeys')?.[0]).toEqual([[1, 2]])
    await wrapper.get('.key-selection input').setValue(true)
    expect(wrapper.emitted('update:selectedKeys')?.[1]).toEqual([[1, 2]])
  })

  it('preserves sorting and hides fields removed by the settings', async () => {
    const wrapper = render()
    await wrapper.get('[data-key-id="1"] button[title="ID"]').trigger('click')
    expect(wrapper.emitted('sort')?.[0]).toEqual(['id', 'asc'])
    await wrapper.setProps({ columns: [{ key: 'name', label: 'Name' }], sortBy: 'id', sortOrder: 'asc' })
    expect(wrapper.find('[data-field="id"]').exists()).toBe(false)
  })

  it('shows skeletons only while the first page is loading', async () => {
    const wrapper = render()
    await wrapper.setProps({ data: [], loading: true })
    expect(wrapper.findAll('.key-card-skeleton')).toHaveLength(6)
    await wrapper.setProps({ data: rows })
    expect(wrapper.find('.key-card-skeleton').exists()).toBe(false)
    expect(wrapper.findAll('[data-key-id]')).toHaveLength(2)
  })
})
