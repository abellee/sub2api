import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick, reactive } from 'vue'
import CreativeDocsView from '../CreativeDocsView.vue'
import CreativeAgentPrompt from '@/components/group/CreativeAgentPrompt.vue'
import defaults from '../../../../../backend/internal/service/documentation_defaults.json'
const { getDocumentation, replace } = vi.hoisted(() => ({ getDocumentation: vi.fn(), replace: vi.fn() }))
const route = reactive<{ params: Record<string, string | undefined>; query: Record<string, string | undefined> }>({ params: {}, query: {} })
vi.mock('@/api/documentation', () => ({ getDocumentation }))
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => ({ replace }) }))
vi.mock('@/components/user/dashboard/studio/DashboardStudioLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/components/group/CreativeOverview.vue', () => ({ default: { template: '<section aria-label="创作概览" />' } }))
vi.mock('@/components/group/CreativeAgentPrompt.vue', () => ({ default: { props: ['article'], template: '<section v-if="article.agent.enabled" id="agent">Agent</section>' } }))
let wrapper: ReturnType<typeof mount>
let disconnect: ReturnType<typeof vi.fn>
const writeText = vi.fn()
beforeEach(() => {
  route.params = {}; route.query = {}
  getDocumentation.mockResolvedValue(structuredClone(defaults))
  replace.mockImplementation(async ({ query }) => { route.query = query })
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  vi.stubGlobal('matchMedia', () => ({ matches: false }))
  disconnect = vi.fn()
  vi.stubGlobal('ResizeObserver', class { observe = vi.fn(); disconnect = disconnect })
  writeText.mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
})
afterEach(() => { wrapper?.unmount(); vi.unstubAllGlobals(); vi.restoreAllMocks(); vi.clearAllMocks() })
async function openDocs() { wrapper = mount(CreativeDocsView, { attachTo: document.body }); await flushPromises() }

describe('Managed documentation', () => {
  it('shows all published articles independently of API groups with configured logos and one chapter list', async () => {
    await openDocs()
    expect(getDocumentation).toHaveBeenCalledOnce()
    expect(wrapper.findAll('button[data-article-id]')).toHaveLength(10)
    expect(wrapper.findAll('.creative-docs-group-category').map(item => item.attributes('aria-label'))).toEqual(['图片生成', '视频生成'])
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.get('.creative-docs-markdown').html()).toContain('<table>')
    expect(wrapper.findAll('.creative-docs-index a').length).toBeGreaterThan(4)
    expect(wrapper.getComponent(CreativeAgentPrompt).props('article').id).toBe('image-1k')
    await wrapper.get('[data-article-id="banana"]').trigger('click'); await flushPromises()
    expect(wrapper.getComponent(CreativeAgentPrompt).props('article').id).toBe('banana')
    expect(replace).toHaveBeenCalledWith({ query: { article: 'banana' }, hash: '' })
    expect(wrapper.findAll('nav[aria-label="当前文档目录"]')).toHaveLength(1)
    expect(wrapper.get('[data-article-id="banana"] img').attributes('src')).toContain('gemini.svg')
  })
  it('selects a direct article link in the existing reader without category page filtering', async () => {
    route.query = { article: 'sd-video' }
    await openDocs()
    expect(wrapper.findAll('button[data-article-id]')).toHaveLength(10)
    expect(wrapper.getComponent(CreativeAgentPrompt).props('article').id).toBe('sd-video')
    route.query = { article: 'image-1k' }; await nextTick(); await flushPromises()
    expect(wrapper.findAll('button[data-article-id]')).toHaveLength(10)
    expect(wrapper.getComponent(CreativeAgentPrompt).props('article').id).toBe('image-1k')
  })
  it('never shows drafts in the existing reader', async () => {
    const data = structuredClone(defaults); data.articles[0].published = false
    getDocumentation.mockResolvedValue(data)
    await openDocs()
    expect(wrapper.findAll('button[data-article-id]')).toHaveLength(9)
    expect(wrapper.find('[data-article-id="image-1k"]').exists()).toBe(false)
  })
  it('handles an empty article response without showing baked-in documents', async () => {
    getDocumentation.mockResolvedValue({ revision: 1, categories: [], articles: [] }); await openDocs()
    expect(wrapper.text()).toContain('暂无已发布的文章')
    expect(wrapper.find('.creative-docs-workspace').exists()).toBe(false)
  })
  it('shows a loading error and retries without replacing it with baked-in documents', async () => {
    getDocumentation.mockRejectedValueOnce(new Error('offline'))
    await openDocs()
    expect(wrapper.get('[role="alert"]').text()).toContain('文档加载失败')
    expect(wrapper.find('button[data-article-id]').exists()).toBe(false)
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(wrapper.findAll('button[data-article-id]')).toHaveLength(10)
  })
  it('keeps the clicked chapter highlighted throughout smooth scrolling and resumes tracking on scrollend', async () => {
    await openDocs()
    const headings = wrapper.findAll('.creative-docs-markdown h1,.creative-docs-markdown h2,.creative-docs-markdown h3')
    headings.forEach((heading, index) => vi.spyOn(heading.element, 'getBoundingClientRect').mockReturnValue({ top: 400 + index * 200 } as DOMRect))
    const target = wrapper.findAll('.creative-docs-index a')[3]
    await target.trigger('click')
    expect(window.scrollTo).toHaveBeenLastCalledWith(expect.objectContaining({ behavior: 'smooth' }))
    expect(target.attributes('aria-current')).toBe('location')
    window.dispatchEvent(new Event('scroll')); await nextTick()
    expect(target.attributes('aria-current')).toBe('location')
    document.dispatchEvent(new Event('scrollend')); await nextTick()
    expect(wrapper.get('.creative-docs-index a[aria-current]').attributes('href')).toBe('#agent')
  })
  it('supports disabling the Agent section and copies literal code', async () => {
    const data = structuredClone(defaults); data.articles[0].agent.enabled = false
    getDocumentation.mockResolvedValue(data); await openDocs()
    expect(wrapper.find('#agent').exists()).toBe(false)
    expect(wrapper.find('.creative-docs-index a[href="#agent"]').exists()).toBe(false)
    const code = wrapper.get('.docs-code-block pre code').text()
    await wrapper.get('[data-copy-code]').trigger('click'); await flushPromises()
    expect(writeText).toHaveBeenCalledWith(code + '\n')
    expect(wrapper.get('[data-copy-code]').text()).toBe('已复制')
  })
  it('disconnects content observations when unmounted', async () => { await openDocs(); wrapper.unmount(); expect(disconnect).toHaveBeenCalled() })
})
