import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import CreativeAgentPrompt from '../CreativeAgentPrompt.vue'
import Select from '@/components/common/Select.vue'

const { list } = vi.hoisted(() => ({ list: vi.fn() }))
vi.mock('@/api/keys', () => ({ list }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const group = { id: '70', name: '图片分组', description: '', documentId: 'banana', kind: 'image' as const }
const key = (id: number, overrides = {}) => ({ id, group_id: 70, name: `Key ${id}`, key: `synthetic-secret-${id}`, status: 'active', expires_at: null, quota: 0, quota_used: 0, ...overrides })
const response = (items: ReturnType<typeof key>[], pages = 1) => ({ items, pages })
let wrapper: ReturnType<typeof mount>
const writeText = vi.fn()

beforeEach(() => {
  list.mockResolvedValue(response([]))
  writeText.mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
})
afterEach(() => { wrapper?.unmount(); vi.clearAllMocks() })

async function openPrompt(value = group) {
  wrapper = mount(CreativeAgentPrompt, { props: { group: value } })
  await flushPromises()
}

describe('Creative Agent prompt and own-group keys', () => {
  it('uses a placeholder when no matching key exists and never requests keys for template previews', async () => {
    await openPrompt()
    expect(list).toHaveBeenCalledWith(1, 100, { group_id: 70, status: 'active' }, { signal: expect.any(AbortSignal) })
    expect(wrapper.get('.agent-key-token').text()).toBe('<API_KEY>')
    await wrapper.get('.agent-copy').trigger('click')
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining('API Key：<API_KEY>'))
    await wrapper.setProps({ group: { ...group, id: 'preview:banana' } })
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(1)
  })

  it('automatically selects one usable exact-group key, displays only the group name and copies the real key', async () => {
    list.mockResolvedValue(response([
      key(1), key(2, { group_id: 71 }), key(3, { status: 'inactive' }), key(4, { expires_at: '2020-01-01' }), key(5, { quota: 2, quota_used: 2 }), key(6, { key: '' }),
    ]))
    await openPrompt()
    expect(wrapper.get('.agent-key-token').text()).toBe(group.name)
    expect(wrapper.html()).not.toContain('synthetic-secret')
    await wrapper.get('.agent-copy').trigger('click')
    expect(writeText.mock.lastCall![0]).toContain('API Key：synthetic-secret-1')
    expect(writeText.mock.lastCall![0]).toContain('nano-banana-2')
    expect(writeText.mock.lastCall![0]).not.toContain('文档标识')
    expect(wrapper.html()).not.toContain('synthetic-secret')
  })

  it('loads all pages and requires an explicit choice with the shared Select and Input search', async () => {
    list.mockResolvedValueOnce(response([key(1)], 2)).mockResolvedValueOnce(response([key(2)], 2))
    await openPrompt()
    expect(list).toHaveBeenCalledTimes(2)
    const select = wrapper.getComponent(Select)
    expect(select.props('options')).toEqual([{ value: 1, label: '图片分组 · Key 1' }, { value: 2, label: '图片分组 · Key 2' }])
    expect(wrapper.get('.agent-copy').attributes('disabled')).toBeDefined()
    select.vm.$emit('update:modelValue', 2)
    await flushPromises()
    await wrapper.get('.agent-copy').trigger('click')
    expect(writeText.mock.lastCall![0]).toContain('API Key：synthetic-secret-2')
    expect(wrapper.html()).not.toContain('synthetic-secret')
    expect(select.props('searchable')).toBe(true)
  })

  it('cancels stale requests when switching groups and never reuses another group’s key', async () => {
    let resolveOld!: (value: ReturnType<typeof response>) => void
    list.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve })).mockResolvedValueOnce(response([key(9, { group_id: 71 })]))
    await openPrompt()
    const signal = list.mock.calls[0][3].signal as AbortSignal
    await wrapper.setProps({ group: { ...group, id: '71', name: '另一个分组' } })
    await flushPromises()
    expect(signal.aborted).toBe(true)
    resolveOld(response([key(1)]))
    await flushPromises()
    await wrapper.get('.agent-copy').trigger('click')
    expect(writeText.mock.lastCall![0]).toContain('API Key：synthetic-secret-9')
    expect(writeText.mock.lastCall![0]).not.toContain('synthetic-secret-1')
  })

  it('distinguishes loading failures from no key and allows retry', async () => {
    list.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(response([key(1)]))
    await openPrompt()
    expect(wrapper.get('[role="alert"]').text()).toContain('读取失败')
    expect(wrapper.get('.agent-copy').attributes('disabled')).toBeDefined()
    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.get('.agent-key-token').text()).toBe(group.name)
  })

  it('does not reveal the secret if clipboard access fails', async () => {
    list.mockResolvedValue(response([key(1)]))
    writeText.mockRejectedValueOnce(new Error('denied'))
    await openPrompt()
    await wrapper.get('.agent-copy').trigger('click')
    await flushPromises()
    expect(wrapper.get('.agent-status').text()).toContain('复制失败')
    expect(wrapper.html()).not.toContain('synthetic-secret')
  })
})
