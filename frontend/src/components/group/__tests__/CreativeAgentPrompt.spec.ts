import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import CreativeAgentPrompt from '../CreativeAgentPrompt.vue'
import Select from '@/components/common/Select.vue'
import defaults from '../../../../../backend/internal/service/documentation_defaults.json'
import type { DocumentationArticle } from '@/api/documentation'

const { list } = vi.hoisted(() => ({ list: vi.fn() }))
vi.mock('@/api/keys', () => ({ list }))
vi.mock('@/api/documentation', () => ({ downloadDocumentationSkill: vi.fn() }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const article = () => structuredClone(defaults.articles[1]) as DocumentationArticle
const key = (id: number, overrides = {}) => ({ id, group_id: 70, group: { name: '图片分组' }, name: 'Key ' + id, key: 'synthetic-secret-' + id, status: 'active', expires_at: null, quota: 0, quota_used: 0, ...overrides })
const response = (items: ReturnType<typeof key>[], pages = 1) => ({ items, pages })
let wrapper: ReturnType<typeof mount>
const writeText = vi.fn()
beforeEach(() => {
  list.mockResolvedValue(response([])); writeText.mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
})
afterEach(() => { wrapper?.unmount(); vi.clearAllMocks() })
async function openPrompt(value = article()) { wrapper = mount(CreativeAgentPrompt, { props: { article: value } }); await flushPromises() }

describe('Configurable Agent prompt and personal keys', () => {
  it('reads only the personal-key API and copies a placeholder if none are available', async () => {
    await openPrompt()
    expect(list).toHaveBeenCalledWith(1, 100, { status: 'active' }, { signal: expect.any(AbortSignal) })
    expect(wrapper.get('.agent-key-token').text()).toBe('<API_KEY>')
    await wrapper.get('.agent-copy').trigger('click')
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining('API Key：<API_KEY>'))
  })
  it('uses configured group filters, excludes unusable keys and reveals the selected secret only to the clipboard', async () => {
    const source = article(); source.agent.key_mode = 'groups'; source.agent.group_ids = [70]
    list.mockResolvedValue(response([key(1), key(2, { group_id: 71 }), key(3, { status: 'inactive' }), key(4, { expires_at: '2020-01-01' }), key(5, { quota: 2, quota_used: 2 }), key(6, { key: '' })]))
    await openPrompt(source)
    expect(wrapper.get('.agent-key-token').text()).toBe('图片分组 · Key 1')
    expect(wrapper.html()).not.toContain('synthetic-secret')
    await wrapper.get('.agent-copy').trigger('click')
    expect(writeText.mock.lastCall![0]).toContain('API Key：synthetic-secret-1')
    expect(writeText.mock.lastCall![0]).toContain(source.content)
    expect(wrapper.html()).not.toContain('synthetic-secret')
  })
  it('loads all pages, supports multiple groups and requires an explicit choice when multiple keys exist', async () => {
    const source = article(); source.agent.key_mode = 'groups'; source.agent.group_ids = [70, 71]
    list.mockResolvedValueOnce(response([key(1)], 2)).mockResolvedValueOnce(response([key(2, { group_id: 71 })], 2))
    await openPrompt(source)
    expect(list).toHaveBeenCalledTimes(2)
    const select = wrapper.getComponent(Select)
    expect(select.props('options')).toHaveLength(2)
    expect(select.props('searchable')).toBe(true)
    expect(wrapper.get('.agent-copy').attributes('disabled')).toBeDefined()
    select.vm.$emit('update:modelValue', 2); await flushPromises()
    await wrapper.get('.agent-copy').trigger('click')
    expect(writeText.mock.lastCall![0]).toContain('synthetic-secret-2')
    expect(wrapper.html()).not.toContain('synthetic-secret')
  })
  it.each(['manual', 'none'] as const)('does not fetch keys in %s mode', async mode => {
    const source = article(); source.agent.key_mode = mode
    await openPrompt(source)
    expect(list).not.toHaveBeenCalled()
    await wrapper.get('.agent-copy').trigger('click')
    expect(writeText.mock.lastCall![0]).toContain('<API_KEY>')
  })
  it('does not fetch keys when the template does not use them or the Agent section is disabled', async () => {
    const source = article(); source.agent.template = '{{article_name}} {{document}}'
    await openPrompt(source)
    expect(list).not.toHaveBeenCalled()
    source.agent.enabled = false; await wrapper.setProps({ article: { ...source } })
    expect(wrapper.find('section').exists()).toBe(false)
    expect(list).not.toHaveBeenCalled()
  })
  it('aborts stale requests when the article or its key filter changes', async () => {
    let resolveOld!: (value: ReturnType<typeof response>) => void
    list.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve })).mockResolvedValueOnce(response([key(9, { group_id: 71 })]))
    await openPrompt()
    const signal = list.mock.calls[0][3].signal as AbortSignal
    const next = article(); next.id = 'other'; next.agent.key_mode = 'groups'; next.agent.group_ids = [71]
    await wrapper.setProps({ article: next }); await flushPromises()
    expect(signal.aborted).toBe(true)
    resolveOld(response([key(1)])); await flushPromises()
    await wrapper.get('.agent-copy').trigger('click')
    expect(writeText.mock.lastCall![0]).toContain('synthetic-secret-9')
    expect(writeText.mock.lastCall![0]).not.toContain('synthetic-secret-1')
  })
  it('distinguishes failures from an empty key list and supports retry', async () => {
    list.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(response([key(1)]))
    await openPrompt()
    expect(wrapper.get('[role="alert"]').text()).toContain('Key 读取失败')
    expect(wrapper.get('.agent-copy').attributes('disabled')).toBeDefined()
    await wrapper.get('.agent-key-note button').trigger('click'); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    await wrapper.get('.agent-copy').trigger('click')
    expect(writeText.mock.lastCall![0]).toContain('synthetic-secret-1')
  })
  it('uses configured headings and text and reports clipboard failures', async () => {
    const source = article(); source.agent.title = '配置的标题'; source.agent.description = '配置的说明'; source.agent.template = 'Only {{article_name}}'
    await openPrompt(source)
    expect(wrapper.get('h2').text()).toBe('配置的标题')
    expect(wrapper.get('.agent-description').text()).toBe('配置的说明')
    expect(wrapper.find('.agent-details').exists()).toBe(false)
    writeText.mockRejectedValueOnce(new Error('denied'))
    await wrapper.get('.agent-copy').trigger('click'); await flushPromises()
    expect(wrapper.get('.agent-status').text()).toContain('复制失败')
  })
})
