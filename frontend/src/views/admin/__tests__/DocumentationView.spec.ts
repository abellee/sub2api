import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import DocumentationView from '../DocumentationView.vue'
import defaults from '../../../../../backend/internal/service/documentation_defaults.json'
const mocks = vi.hoisted(() => ({ get: vi.fn(), save: vi.fn(), groups: vi.fn(), success: vi.fn() }))
vi.mock('@/api/documentation', () => ({ getAdminDocumentation: mocks.get, saveDocumentation: mocks.save, getDocumentationCategories: vi.fn() }))
vi.mock('@/api/admin/groups', () => ({ getAllIncludingInactive: mocks.groups }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: mocks.success }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('vue-router', async () => ({ ...await vi.importActual<typeof import('vue-router')>('vue-router'), onBeforeRouteLeave: vi.fn() }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
let wrapper: ReturnType<typeof mount>
beforeEach(() => {
  setActivePinia(createPinia()); vi.clearAllMocks()
  mocks.get.mockResolvedValue(structuredClone(defaults))
  mocks.save.mockImplementation(async value => ({ ...structuredClone(value), revision: value.revision + 1 }))
  mocks.groups.mockResolvedValue([{ id: 70, name: 'Image keys' }])
  vi.spyOn(window, 'confirm').mockReturnValue(true)
})
afterEach(() => { wrapper?.unmount(); vi.restoreAllMocks() })
async function open() {
  wrapper = mount(DocumentationView, { global: { stubs: { RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' } } } })
  await flushPromises()
}

describe('Documentation management', () => {
  it('loads the existing articles and saves names, Markdown and prompt configuration with their revision', async () => {
    await open()
    expect(wrapper.findAll('button[data-article-id]')).toHaveLength(10)
    expect(wrapper.get('[data-testid="save-documents"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="article-name"]').setValue('Changed name')
    await wrapper.get('[data-testid="article-content"]').setValue('# Changed\n\n## Chapter\n\nMarkdown content')
    await wrapper.get('[data-testid="agent-template"]').setValue('Use {{article_name}} {{api_key}}\n{{document}}')
    await wrapper.get('[data-testid="save-documents"]').trigger('click'); await flushPromises()
    expect(mocks.save.mock.lastCall![0]).toMatchObject({ revision: 0, articles: [expect.objectContaining({ name: 'Changed name', content: '# Changed\n\n## Chapter\n\nMarkdown content', agent: expect.objectContaining({ template: 'Use {{article_name}} {{api_key}}\n{{document}}' }) }), ...defaults.articles.slice(1)] })
    expect(mocks.success).toHaveBeenCalledWith('文档已保存')
    expect(wrapper.get('[data-testid="save-documents"]').attributes('disabled')).toBeDefined()
  })
  it('adds a custom category and a draft article and exposes data APIs without category pages', async () => {
    await open()
    await wrapper.get('input[aria-label="新分类名称"]').setValue('Custom guides')
    await wrapper.get('form').trigger('submit')
    const categoryIDs = wrapper.findAll('input[aria-label="分类标识"]')
    const articleAPIs = wrapper.findAll('input[aria-label="文章数据接口"]')
    expect(categoryIDs).toHaveLength(3)
    expect(articleAPIs.map(input => (input.element as HTMLInputElement).value)).toEqual([
      '/api/v1/documentation?category=image', '/api/v1/documentation?category=video',
      '/api/v1/documentation?category=' + (categoryIDs[2].element as HTMLInputElement).value,
    ])
    expect(wrapper.find('input[value="/api/v1/documentation/categories"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('打开分类页面')
    expect(wrapper.find('a[href^="/docs/creative/"]').exists()).toBe(false)
    await wrapper.get('[data-testid="add-article"]').trigger('click')
    expect(wrapper.findAll('button[data-article-id]')).toHaveLength(11)
    expect(wrapper.get('[data-testid="article-name"]').element).toHaveProperty('value', '新文章')
    await wrapper.get('[data-testid="save-documents"]').trigger('click'); await flushPromises()
    expect(mocks.save.mock.lastCall![0].articles.at(-1)).toMatchObject({ published: false, agent: { key_mode: 'all' } })
    expect(mocks.save.mock.lastCall![0].categories.at(-1).name).toBe('Custom guides')
  })
  it('preserves the unsaved draft and displays a specific conflict message after a failed save', async () => {
    mocks.save.mockRejectedValue({ status: 409, message: '文档已被其他管理员更新，请重新加载后再保存' })
    await open()
    await wrapper.get('[data-testid="article-name"]').setValue('Unsaved')
    await wrapper.get('[data-testid="save-documents"]').trigger('click'); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('文档已被其他管理员更新')
    expect(wrapper.get('[data-testid="article-name"]').element).toHaveProperty('value', 'Unsaved')
    expect(wrapper.get('[data-testid="save-documents"]').attributes('disabled')).toBeUndefined()
    expect(mocks.success).not.toHaveBeenCalled()
  })
  it('blocks deletion of a category that still contains articles', async () => {
    await open()
    await wrapper.findAll('button').find(button => button.text() === '删除分类')!.trigger('click')
    expect(wrapper.get('[role="alert"]').text()).toContain('请先移走或删除')
    expect(wrapper.findAll('input[aria-label="分类名称"]')).toHaveLength(2)
  })
  it('keeps loading failures explicit and does not allow saving an empty fallback', async () => {
    mocks.get.mockRejectedValue(new Error('offline'))
    await open()
    expect(wrapper.get('[role="alert"]').text()).toContain('offline')
    expect(wrapper.find('[data-testid="article-content"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="save-documents"]').attributes('disabled')).toBeDefined()
  })
})
