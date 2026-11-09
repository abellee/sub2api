import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import { flushPromises, mount } from '@vue/test-utils'
import MediaLibraryView from '../MediaLibraryView.vue'
import MediaLibraryCard from '@/components/media/MediaLibraryCard.vue'
import { mediaLibraryAPI, type MediaFile } from '@/api/mediaLibrary'

vi.mock('@/api/mediaLibrary', () => ({ mediaLibraryAPI: { list: vi.fn(), delete: vi.fn() } }))
vi.mock('vue-router', () => ({ onBeforeRouteLeave: vi.fn() }))
vi.mock('@/components/user/dashboard/studio/DashboardStudioLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
let wrapper: ReturnType<typeof mount>
let observers: Array<{ callback: IntersectionObserverCallback; targets: Set<Element> }>
function file(index: number): MediaFile {
  return { id: `mf${index}`, task_id: 'task', file_id: `f${index}`, model: `model-${index}`, download_url: `/download/${index}`, content_type: index % 2 ? 'video/mp4' : 'image/png', size_bytes: 100, created_at: new Date(2026, 10, 9, 0, index).toISOString(), expires_at: new Date(Date.now() + 7200000).toISOString() }
}
async function open(files: MediaFile[]) {
  vi.mocked(mediaLibraryAPI.list).mockResolvedValue({ items: files, next_cursor: '' })
  wrapper = mount(MediaLibraryView, { attachTo: document.body, global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })] } })
  await flushPromises()
}
async function confirm() {
  const button = document.querySelector<HTMLButtonElement>('[data-confirm]')!
  button.click()
  await flushPromises()
}
beforeEach(() => {
  vi.resetAllMocks()
  vi.spyOn(Date, 'now').mockReturnValue(new Date('2026-11-09T01:00:00Z').getTime())
  observers = []
  vi.stubGlobal('IntersectionObserver', class {
    targets = new Set<Element>()
    constructor(public callback: IntersectionObserverCallback) { observers.push(this) }
    observe(target: Element) { this.targets.add(target) }
    disconnect() { this.targets.clear() }
  })
})
afterEach(() => { wrapper?.unmount(); document.body.innerHTML = ''; vi.restoreAllMocks(); vi.unstubAllGlobals() })

function reachBottom() {
  const target = wrapper.get('.library-load-more').element
  const observer = observers.find(item => item.targets.has(target))!
  observer.callback([{ target, isIntersecting: true } as IntersectionObserverEntry], observer as unknown as IntersectionObserver)
}

describe('user media library', () => {
	 it('updates remaining time and handles expiry and indefinite retention', async () => {
		const media = file(0)
		wrapper = mount(MediaLibraryCard, { props: { file: media, now: Date.now(), disabled: false }, global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })] } })
		expect(wrapper.text()).toContain('剩余 02:00:00')
		await wrapper.setProps({ now: Date.now() + 1000 })
		expect(wrapper.text()).toContain('剩余 01:59:59')
		await wrapper.setProps({ now: Date.now() + 7200000 })
		expect(wrapper.text()).toContain('已到期')
		await wrapper.setProps({ file: { ...media, expires_at: null } })
		expect(wrapper.text()).toContain('长期保存')
	 })
  it('opens image and video lightboxes, updates the countdown and confirms single deletion', async () => {
    await open([file(0), file(1)])
    expect(wrapper.findAll('.media-card')).toHaveLength(2)
    expect(wrapper.text()).toContain('剩余 02:00:')
    await wrapper.get('[data-media-id="mf0"] .card-preview').trigger('click')
    expect(document.querySelector('.library-lightbox img')?.getAttribute('src')).toBe('/download/0')
    expect(document.querySelector('.library-lightbox .modal-content')).toBeNull()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    await wrapper.get('[data-media-id="mf1"] .card-preview').trigger('click')
    expect(document.querySelector('.library-lightbox video[controls]')).not.toBeNull()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    await wrapper.get('[data-media-id="mf0"] .card-delete').trigger('click')
    expect(mediaLibraryAPI.delete).not.toHaveBeenCalled()
    vi.mocked(mediaLibraryAPI.delete).mockResolvedValue({ deleted_ids: ['mf0'], failed_ids: [] })
    await confirm()
    expect(mediaLibraryAPI.delete).toHaveBeenCalledWith(['mf0'])
    expect(wrapper.find('[data-media-id="mf0"]').exists()).toBe(false)
  })

  it('zooms and rotates on the overlay, resets on navigation and restores focus on close', async () => {
    await open([file(0), file(1), file(2)])
    const origin = wrapper.get('[data-media-id="mf0"] .card-preview').element as HTMLButtonElement
    origin.focus()
    await wrapper.get('[data-media-id="mf0"] .card-preview').trigger('click')
    const lightbox = () => document.querySelector<HTMLElement>('.library-lightbox')!
    async function click(label: string) {
      lightbox().querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`)!.click()
      await flushPromises()
    }
    const transform = () => lightbox().querySelector<HTMLElement>('.lightbox-media')!.style.transform
    expect(document.activeElement).toBe(lightbox().querySelector('.lightbox-close'))
    expect(document.body.classList.contains('media-lightbox-open')).toBe(true)
    await click('放大')
    await click('顺时针旋转')
    expect(transform()).toContain('scale(1.25) rotate(90deg)')
    await click('缩小')
    expect(transform()).toContain('scale(1) rotate(90deg)')
    await click('重置缩放和旋转')
    expect(transform()).toContain('scale(1) rotate(0deg)')
    for (let i = 0; i < 15; i++) await click('放大')
    expect(transform()).toContain('scale(4)')
    expect(lightbox().querySelector<HTMLButtonElement>('[aria-label="放大"]')!.disabled).toBe(true)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight' }))
    await flushPromises()
    expect(lightbox().querySelector('video[controls]')?.getAttribute('src')).toBe('/download/1')
    expect(transform()).toContain('scale(1) rotate(0deg)')
    await click('下一个作品')
    expect(lightbox().querySelector('img')?.getAttribute('src')).toBe('/download/2')
    expect(lightbox().querySelector<HTMLButtonElement>('[aria-label="下一个作品"]')!.disabled).toBe(true)
    await click('上一个作品')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft' }))
    await flushPromises()
    for (let i = 0; i < 5; i++) await click('缩小')
    expect(transform()).toContain('scale(0.25)')
    expect(lightbox().querySelector<HTMLButtonElement>('[aria-label="缩小"]')!.disabled).toBe(true)
    const last = lightbox().querySelector<HTMLButtonElement>('[aria-label="顺时针旋转"]')!
    last.focus()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', cancelable: true }))
    expect(document.activeElement).toBe(lightbox().querySelector('.lightbox-close'))
    lightbox().querySelector<HTMLElement>('.lightbox-stage')!.click()
    await flushPromises()
    expect(document.querySelector('.library-lightbox')).toBeNull()
    expect(document.body.classList.contains('media-lightbox-open')).toBe(false)
    expect(document.activeElement).toBe(origin)
  })

  it('clears unloaded files of all types in batches of 10 and retains failures for retry', async () => {
    await open([file(0)])
    await wrapper.get('.library-tabs button:nth-child(2)').trigger('click')
    await flushPromises()
    const all = Array.from({ length: 121 }, (_, i) => file(i))
    vi.mocked(mediaLibraryAPI.list).mockResolvedValueOnce({ items: all.slice(0, 50), next_cursor: 'next' })
      .mockResolvedValueOnce({ items: all.slice(50, 100), next_cursor: 'last' })
      .mockResolvedValueOnce({ items: all.slice(100), next_cursor: '' })
    let resolveFirst!: (result: { deleted_ids: string[]; failed_ids: string[] }) => void
    vi.mocked(mediaLibraryAPI.delete).mockImplementationOnce(() => new Promise(resolve => { resolveFirst = resolve }))
      .mockImplementation(async ids => ({ deleted_ids: ids.filter(id => id !== 'mf12'), failed_ids: ids.filter(id => id === 'mf12') }))
    await wrapper.get('[data-clear]').trigger('click')
    expect(mediaLibraryAPI.delete).not.toHaveBeenCalled()
    await confirm()
    expect(mediaLibraryAPI.delete).toHaveBeenCalledTimes(1)
    expect(document.querySelector('[role="progressbar"]')?.getAttribute('aria-valuenow')).toBe('0')
    const first = vi.mocked(mediaLibraryAPI.delete).mock.calls[0][0]
    resolveFirst({ deleted_ids: first, failed_ids: [] })
    await flushPromises()
    expect(vi.mocked(mediaLibraryAPI.delete).mock.calls.map(call => call[0].length)).toEqual([...Array(12).fill(10), 1])
    expect(mediaLibraryAPI.list).toHaveBeenLastCalledWith('last', expect.any(AbortSignal))
    expect(document.querySelector('[role="progressbar"]')?.getAttribute('aria-valuenow')).toBe('121')
    expect(document.body.textContent).toContain('1 个文件删除失败')
    expect(wrapper.findAll('.media-card')).toHaveLength(1)
    expect(wrapper.find('[data-media-id="mf12"]').exists()).toBe(true)
    vi.mocked(mediaLibraryAPI.delete).mockResolvedValue({ deleted_ids: ['mf12'], failed_ids: [] })
    const retry = [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find(button => button.textContent === '请重试')!
    retry.click()
    await flushPromises()
    expect(mediaLibraryAPI.delete).toHaveBeenLastCalledWith(['mf12'])
    expect(wrapper.findAll('.media-card')).toHaveLength(0)
  })

  it('only loads the next page on scroll, preserves existing cards and can retry a page failure', async () => {
    vi.mocked(mediaLibraryAPI.list).mockResolvedValueOnce({ items: [file(0)], next_cursor: 'next' })
    wrapper = mount(MediaLibraryView, { attachTo: document.body, global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })] } }); await flushPromises()
    expect(mediaLibraryAPI.list).toHaveBeenCalledTimes(1)
    expect(wrapper.findAll('.media-card')).toHaveLength(1)
    expect(wrapper.find('h2').exists()).toBe(false)
    const originalCard = wrapper.get('[data-media-id="mf0"]').element
    vi.mocked(mediaLibraryAPI.list).mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ items: [file(1)], next_cursor: '' })
    reachBottom()
    await flushPromises()
    expect(wrapper.findAll('.media-card')).toHaveLength(1)
    await wrapper.get('.library-load-more button').trigger('click')
    await flushPromises()
    expect(mediaLibraryAPI.list).toHaveBeenLastCalledWith('next', expect.any(AbortSignal), 'all')
    expect(wrapper.findAll('.media-card')).toHaveLength(2)
    expect(wrapper.get('[data-media-id="mf0"]').element).toBe(originalCard)
    expect(wrapper.get('.library-load-more').isVisible()).toBe(false)
  })

  it('leaves a network-failed delete batch available', async () => {
    await open([file(0), file(1)])
    vi.mocked(mediaLibraryAPI.delete).mockRejectedValue(new Error('offline'))
    await wrapper.get('[data-clear]').trigger('click'); await confirm()
    expect(wrapper.findAll('.media-card')).toHaveLength(2)
    expect(document.body.textContent).toContain('2 个文件删除失败')
  })

  it('restarts pagination when switching media type and blocks overlapping page requests', async () => {
    await open([file(0)])
    let resolve!: (result: { items: MediaFile[]; next_cursor: string }) => void
    vi.mocked(mediaLibraryAPI.list).mockImplementationOnce(() => new Promise(done => { resolve = done }))
    await wrapper.get('.library-tabs button:nth-child(3)').trigger('click')
    expect(mediaLibraryAPI.list).toHaveBeenLastCalledWith('', expect.any(AbortSignal), 'video')
    expect(wrapper.get('[data-clear]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('.library-tabs button:first-child').attributes('disabled')).toBeDefined()
    resolve({ items: [file(1)], next_cursor: '' })
    await flushPromises()
    expect(wrapper.find('[data-media-id="mf0"]').exists()).toBe(false)
    expect(wrapper.find('[data-media-id="mf1"]').exists()).toBe(true)
  })

  it('does not start clearing if collecting an unloaded page fails and can retry', async () => {
    await open([file(0)])
    vi.mocked(mediaLibraryAPI.list).mockResolvedValueOnce({ items: [file(0)], next_cursor: 'next' }).mockRejectedValueOnce(new Error('offline'))
    await wrapper.get('[data-clear]').trigger('click'); await confirm()
    expect(mediaLibraryAPI.delete).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('尚未开始删除')
    vi.mocked(mediaLibraryAPI.list).mockResolvedValue({ items: [file(0), file(1)], next_cursor: '' })
    vi.mocked(mediaLibraryAPI.delete).mockResolvedValue({ deleted_ids: ['mf0', 'mf1'], failed_ids: [] })
    const retry = [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find(button => button.textContent === '请重试')!
    retry.click(); await flushPromises()
    expect(mediaLibraryAPI.delete).toHaveBeenCalledWith(['mf0', 'mf1'])
    expect(wrapper.findAll('.media-card')).toHaveLength(0)
  })

  it('disables clearing when loading fails and allows retry', async () => {
    vi.mocked(mediaLibraryAPI.list).mockRejectedValueOnce(new Error('offline'))
    wrapper = mount(MediaLibraryView, { attachTo: document.body, global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })] } }); await flushPromises()
    expect(wrapper.get('[data-clear]').attributes('disabled')).toBeDefined()
    vi.mocked(mediaLibraryAPI.list).mockResolvedValue({ items: [file(0)], next_cursor: '' })
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(wrapper.findAll('.media-card')).toHaveLength(1)
  })
})
