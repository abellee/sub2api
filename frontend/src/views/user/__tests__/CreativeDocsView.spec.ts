import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick, ref } from 'vue'
import CreativeDocsView from '../CreativeDocsView.vue'
import CreativeAgentPrompt from '@/components/group/CreativeAgentPrompt.vue'
import { referencePages } from '../../../../public/creative-docs/reference-content.js'

const { getAvailable, getCategories } = vi.hoisted(() => ({ getAvailable: vi.fn(), getCategories: vi.fn() }))
const theme = ref(true)
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main style="padding:32px"><slot /></main>' } }))
vi.mock('@/api/groups', () => ({ getAvailable, getCategories }))
vi.mock('@/components/group/CreativeAgentPrompt.vue', () => ({ default: { props: ['group'], template: '<section id="agent">文档太长不想看</section>' } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/composables/useTheme', () => ({ useTheme: () => ({ isDark: theme }) }))

let wrapper: ReturnType<typeof mount>
let resizeContent: () => void
let resizeGroups: () => void
const disconnect = vi.fn()
beforeEach(() => {
  getAvailable.mockResolvedValue([{ id: 70, name: '图片 A' }, { id: 71, name: '图片 B' }, { id: 72, name: '视频 C' }, { id: 73, name: '无文档' }])
  getCategories.mockResolvedValue({ documentation: { 70: 'banana', 71: 'banana', 72: 'sd-video', 999: 'video' } })
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  vi.stubGlobal('ResizeObserver', class {
    constructor(private callback: () => void) {}
    observe(target: Element) {
      if (target.tagName === 'BODY') resizeContent = this.callback
      if (target.classList.contains('creative-docs-groups')) resizeGroups = this.callback
    }
    disconnect = disconnect
  })
})
afterEach(() => {
  wrapper?.unmount()
  theme.value = true
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.clearAllMocks()
})

async function openDocs() {
  wrapper = mount(CreativeDocsView, { attachTo: document.body, global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
  await flushPromises()
  vi.spyOn(wrapper.get('.creative-docs-groups').element, 'getBoundingClientRect').mockReturnValue({ height: 80 } as DOMRect)
  const frame = wrapper.get('iframe')
  const doc = (frame.element as HTMLIFrameElement).contentDocument!
  const html = doc.appendChild(doc.createElement('html'))
  html.append(doc.createElement('body'))
  doc.addEventListener('creative-docs:select', (event) => {
    const detail = (event as CustomEvent).detail
    const page = referencePages.find(page => page.id === detail.documentId)!
    const headings = page.blocks.filter(block => block.type === 'heading')
    doc.body.innerHTML = `<section id="reference" data-doc-index="文档概览"></section>` + headings.map((heading, index) => `<section id="doc-${page.id}-${index + 1}" data-doc-index="${heading.value.replace(/^\d+\.\s*/, '')}"></section>`).join('')
    doc.querySelectorAll('section').forEach((section, index) => {
      vi.spyOn(section, 'getBoundingClientRect').mockReturnValue({ top: index * 200 } as DOMRect)
    })
    doc.dispatchEvent(new CustomEvent('creative-docs:rendered'))
  })
  vi.spyOn(doc.body, 'getBoundingClientRect').mockReturnValue({ height: 2400 } as DOMRect)
  await frame.trigger('load')
  return doc
}

async function chooseGroup(id: string) {
  await wrapper.get(`.creative-docs-groups button[data-group-id="${id}"]`).trigger('click')
  await flushPromises()
}

async function readChapter(doc: Document, label: string) {
  const section = [...doc.querySelectorAll<HTMLElement>('[data-doc-index]')].find(section => section.dataset.docIndex === label)!
  vi.spyOn(wrapper.get('iframe').element, 'getBoundingClientRect').mockReturnValue({ top: -section.getBoundingClientRect().top } as DOMRect)
  window.dispatchEvent(new Event('scroll'))
  await nextTick()
  return section
}

describe('Creative documentation groups', () => {
  it('shows every available bound group with its logo above one shared chapter navigation', async () => {
    await openDocs()
    const bar = wrapper.get('nav[aria-label="切换文档分组"]')
    expect(bar.findAll('button').map(button => button.text())).toEqual(['图片 A', '图片 B', '视频 C'])
    expect(bar.findAll('[role="group"]').map(group => group.attributes('aria-label'))).toEqual(['图片生成', '视频生成'])
    expect(bar.findAll('img').map(img => img.attributes('src'))).toEqual([
      '/creative-docs/assets/brands/gemini.svg', '/creative-docs/assets/brands/gemini.svg', '/creative-docs/assets/brands/bytedance.svg',
    ])
    expect(wrapper.find('.select-trigger').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('还没有分组绑定文档')
    expect(bar.element.previousElementSibling?.getAttribute('aria-label')).toBe('创作概览')
    const nav = wrapper.get('nav[aria-label="当前文档目录"]').element
    const links = wrapper.findAll('.creative-docs-index a').map(link => link.element)
    await chooseGroup('71')
    expect(bar.get('[aria-pressed="true"]').text()).toBe('图片 B')
    expect(wrapper.findAll('nav[aria-label="当前文档目录"]')).toHaveLength(1)
    expect(wrapper.get('.creative-docs-index').element).toBe(nav)
    wrapper.findAll('.creative-docs-index a').forEach((link, index) => expect(link.element).toBe(links[index]))
  })

  it('sends the bound document ID and group name, including when two groups share a document', async () => {
    const doc = await openDocs()
    const select = vi.fn()
    doc.addEventListener('creative-docs:select', select)
    await chooseGroup('71')
    expect((select.mock.lastCall![0] as CustomEvent).detail).toEqual({ documentId: 'banana', groupName: '图片 B' })
    await chooseGroup('72')
    expect((select.mock.lastCall![0] as CustomEvent).detail).toEqual({ documentId: 'sd-video', groupName: '视频 C' })
    expect(wrapper.get('.creative-docs-index').findAll('a')[2].attributes('href')).toBe('#doc-sd-video-1')
    expect(wrapper.get('.creative-docs-index').text()).toContain('文生视频')
    expect(wrapper.get('.creative-docs-index').text()).not.toContain('香蕉比例与输出尺寸')
    expect(wrapper.getComponent(CreativeAgentPrompt).props('group').id).toBe('72')
    await chooseGroup('72')
    expect(select).toHaveBeenCalledTimes(2)
  })

  it('reuses common chapter links, updates differing chapters and stays in the same chapter on group changes', async () => {
    getCategories.mockResolvedValue({ documentation: { 70: 'image-official', 71: 'image-1k', 72: 'sd-video' } })
    const doc = await openDocs()
    const oldSection = await readChapter(doc, '常见问题')
    const link = wrapper.get('.creative-docs-index [aria-current]').element
    const oldTop = oldSection.getBoundingClientRect().top
    vi.spyOn(window, 'scrollY', 'get').mockReturnValue(5000)
    await chooseGroup('71')
    const newSection = [...doc.querySelectorAll<HTMLElement>('[data-doc-index]')].find(section => section.dataset.docIndex === '常见问题')!
    expect(wrapper.get('.creative-docs-index a:last-child').element).toBe(link)
    expect(link.getAttribute('href')).toBe(`#${newSection.id}`)
    expect(wrapper.get('.creative-docs-index').text()).not.toContain('遮罩局部编辑')
    expect(window.scrollTo).toHaveBeenLastCalledWith({ top: 5000 + newSection.getBoundingClientRect().top - oldTop - 164, behavior: 'instant' })
  })

  it('falls back to the preceding shared chapter when the selected chapter is absent', async () => {
    getCategories.mockResolvedValue({ documentation: { 70: 'image-official', 71: 'image-1k' } })
    const doc = await openDocs()
    const oldSection = await readChapter(doc, '遮罩局部编辑')
    const oldTop = oldSection.getBoundingClientRect().top
    vi.spyOn(window, 'scrollY', 'get').mockReturnValue(5000)
    await chooseGroup('71')
    const target = [...doc.querySelectorAll<HTMLElement>('[data-doc-index]')].find(section => section.dataset.docIndex === '本地图片上传')!
    expect(window.scrollTo).toHaveBeenLastCalledWith({ top: 5000 + target.getBoundingClientRect().top - oldTop - 164, behavior: 'instant' })
  })

  it('does not jump into the document when switching groups above the reading area', async () => {
    await openDocs()
    vi.spyOn(document.getElementById('agent')!, 'getBoundingClientRect').mockReturnValue({ top: 800 } as DOMRect)
    vi.spyOn(wrapper.get('iframe').element, 'getBoundingClientRect').mockReturnValue({ top: 1600 } as DOMRect)
    window.dispatchEvent(new Event('scroll'))
    await nextTick()
    vi.mocked(window.scrollTo).mockClear()
    await chooseGroup('71')
    expect(window.scrollTo).not.toHaveBeenCalled()
  })

  it('follows the console theme and keeps one outer scroll area', async () => {
    const doc = await openDocs()
    expect(doc.documentElement.classList.contains('console-embedded')).toBe(true)
    expect(doc.documentElement.classList.contains('dark')).toBe(true)
    theme.value = false
    await nextTick()
    expect(doc.documentElement.classList.contains('dark')).toBe(false)
    vi.spyOn(window, 'scrollY', 'get').mockReturnValue(1600)
    vi.mocked(doc.body.getBoundingClientRect).mockReturnValue({ height: 4200 } as DOMRect)
    resizeContent()
    await nextTick()
    expect((wrapper.get('iframe').element as HTMLIFrameElement).style.height).toBe('4200px')
    expect(window.scrollTo).toHaveBeenLastCalledWith({ top: 1600, behavior: 'instant' })
  })

  it('retains the overview and updates the shared directory below the sticky group bar on outer scrolling', async () => {
    const doc = await openDocs()
    expect(wrapper.get('[aria-label="创作概览"]').text()).toContain('让灵感成像。')
    expect(wrapper.find('.creative-docs-nav').exists()).toBe(false)
    await readChapter(doc, '服务器内部异步与任务恢复')
    expect((wrapper.get('.creative-docs-sidebar').element as HTMLElement).style.top).toBe('164px')
    expect(wrapper.get('.creative-docs-index [aria-current]').attributes('href')).toBe('#doc-banana-1')
  })

  it('shows a retry state if group loading fails', async () => {
    getAvailable.mockRejectedValueOnce(new Error('offline'))
    wrapper = mount(CreativeDocsView, { global: { stubs: ['RouterLink'] } })
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('加载失败')
    expect(wrapper.find('iframe').exists()).toBe(false)
    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.find('iframe').exists()).toBe(true)
  })

  it('keeps the shared menu below the console header when its chapters exceed the viewport', async () => {
    await openDocs()
    const menu = wrapper.get('.creative-docs-sidebar').element as HTMLElement
    vi.spyOn(menu, 'offsetHeight', 'get').mockReturnValue(1400)
    vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(800)
    window.dispatchEvent(new Event('resize'))
    await nextTick()
    expect(menu.style.top).toBe('164px')
    await chooseGroup('71')
    window.dispatchEvent(new Event('scroll'))
    await nextTick()
    expect(menu.style.top).toBe('164px')
  })

  it('updates the sticky offsets and chapter highlighting when group buttons wrap to more rows', async () => {
    const doc = await openDocs()
    const bar = wrapper.get('.creative-docs-groups').element
    vi.mocked(bar.getBoundingClientRect).mockReturnValue({ height: 128 } as DOMRect)
    resizeGroups()
    await nextTick()
    expect((wrapper.get('.creative-docs').element as HTMLElement).style.getPropertyValue('--docs-group-top')).toBe('64px')
    expect((wrapper.get('.creative-docs-sidebar').element as HTMLElement).style.top).toBe('212px')
    await readChapter(doc, '接入方式与鉴权')
    expect(wrapper.get('.creative-docs-index [aria-current]').text()).toBe('模型与接口')
    const link = wrapper.findAll('.creative-docs-index a').find(link => link.text() === '接入方式与鉴权')!
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
    vi.mocked(window.scrollTo).mockClear()
    await link.trigger('click')
    expect(window.scrollTo).toHaveBeenCalledWith({ top: -212, behavior: 'smooth' })
    expect(wrapper.get('.creative-docs-index [aria-current]').text()).toBe('接入方式与鉴权')
    window.dispatchEvent(new Event('scroll'))
    await nextTick()
    expect(wrapper.get('.creative-docs-index [aria-current]').text()).toBe('接入方式与鉴权')
    document.dispatchEvent(new Event('scrollend'))
    await nextTick()
    expect(wrapper.get('.creative-docs-index [aria-current]').text()).toBe('模型与接口')
  })

  it('resumes scroll highlighting when the user interrupts smooth chapter navigation', async () => {
    const doc = await openDocs()
    await readChapter(doc, '接入方式与鉴权')
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
    await wrapper.get('.creative-docs-index a:last-child').trigger('click')
    expect(wrapper.get('.creative-docs-index [aria-current]').text()).toBe('常见问题')
    window.dispatchEvent(new Event('wheel'))
    window.dispatchEvent(new Event('scroll'))
    await nextTick()
    expect(wrapper.get('.creative-docs-index [aria-current]').text()).toBe('接入方式与鉴权')
  })
})
