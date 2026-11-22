import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import CreativeSkillExport from '../CreativeSkillExport.vue'
import defaults from '../../../../../backend/internal/service/documentation_defaults.json'
import type { DocumentationArticle } from '@/api/documentation'

const { download, saveAs } = vi.hoisted(() => ({ download: vi.fn(), saveAs: vi.fn() }))
vi.mock('@/api/documentation', () => ({ downloadDocumentationSkill: download }))
vi.mock('file-saver', () => ({ saveAs }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
let wrapper: ReturnType<typeof mount>
const article = () => structuredClone(defaults.articles[0]) as DocumentationArticle
beforeEach(() => { vi.resetAllMocks(); download.mockResolvedValue(new Blob(['zip'])) })
afterEach(() => wrapper?.unmount())
function open() { wrapper = mount(CreativeSkillExport, { props: { article: article() } }) }

describe('Documentation Skill two-step import', () => {
  it('shows download first, seven logo choices second, without auto-opening apps', () => {
    open()
    expect(wrapper.find('details').exists()).toBe(false)
    expect(wrapper.get('h3').text()).toContain('TokenFree Creative')
    expect(wrapper.findAll('.skill-step h4').map(item => item.text())).toEqual(['第一步：下载 ZIP 包', '第二步：选择 Agent，查看添加方法'])
    expect(wrapper.findAll('.skill-client').map(item => item.text())).toEqual(['Codex', 'Claude Code', 'TRAE', 'Qoder', 'CodeBuddy', 'Kimi Code', 'ZCode'])
    expect(wrapper.findAll('.skill-client img')).toHaveLength(7)
    expect(wrapper.findAll('.skill-client').every(item => item.element.tagName === 'BUTTON')).toBe(true)
    expect(wrapper.find('.skill-guide').exists()).toBe(false)
    expect(download).not.toHaveBeenCalled()
  })

  it('downloads the saved article only when clicking the download button', async () => {
    open()
    await wrapper.get('.skill-download').trigger('click'); await flushPromises()
    expect(download).toHaveBeenCalledTimes(1)
    expect(download).toHaveBeenCalledWith('image-1k')
    expect(saveAs).toHaveBeenCalledWith(expect.any(Blob), 'TokenFree Creative.zip')
    expect(wrapper.get('[role="status"]').text()).toContain('已开始下载')
    expect(wrapper.find('.skill-guide').exists()).toBe(false)
  })

  it('changes only the guide when selecting each Agent logo', async () => {
    open()
    for (const [id, directory] of [['codex', '.agents'], ['claude', '.claude'], ['trae', '.trae'], ['qoder', '.qoder'], ['codebuddy', '.codebuddy'], ['kimi', '.agents'], ['zcode', '.agents']]) {
      await wrapper.get(`[data-client="${id}"]`).trigger('click')
      expect(wrapper.get(`[data-client="${id}"]`).attributes('aria-pressed')).toBe('true')
      expect(wrapper.get('.skill-guide').text()).toContain(directory + '/skills/')
      expect(wrapper.get('.skill-guide').text()).toContain('tokenfree-creative')
      expect(wrapper.get('.skill-guide a').attributes('href')).toMatch(/^https:\/\//)
    }
    expect(download).not.toHaveBeenCalled(); expect(saveAs).not.toHaveBeenCalled()
    expect(wrapper.findAll('a').some(link => /^(codex|claude|qoder):/.test(link.attributes('href') ?? ''))).toBe(false)
  })

  it('allows retry after failure and never claims the Skill was installed', async () => {
    download.mockRejectedValueOnce(new Error('offline'))
    open()
    await wrapper.get('.skill-download').trigger('click'); await flushPromises()
    expect(wrapper.get('[role="status"]').text()).toContain('下载失败')
    expect(saveAs).not.toHaveBeenCalled()
    await wrapper.get('.skill-download').trigger('click'); await flushPromises()
    expect(saveAs).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).not.toContain('已安装')
  })

  it('blocks duplicate downloads and discards stale results after switching articles or unmounting', async () => {
    let resolve!: (value: Blob) => void
    download.mockImplementation(() => new Promise(done => { resolve = done }))
    open()
    await wrapper.get('.skill-download').trigger('click')
    expect(wrapper.get('.skill-download').attributes('disabled')).toBeDefined()
    await wrapper.get('.skill-download').trigger('click')
    expect(download).toHaveBeenCalledTimes(1)
    await wrapper.get('[data-client="claude"]').trigger('click')
    await wrapper.setProps({ article: { ...article(), id: 'other' } })
    resolve(new Blob(['old'])); await flushPromises()
    expect(saveAs).not.toHaveBeenCalled()
    expect(wrapper.find('.skill-guide').exists()).toBe(false)
    await wrapper.get('.skill-download').trigger('click')
    wrapper.unmount(); resolve(new Blob(['left'])); await flushPromises()
    expect(saveAs).not.toHaveBeenCalled()
  })
})
