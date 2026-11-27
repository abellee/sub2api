import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import AnnouncementContent from '../AnnouncementContent.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
afterEach(() => { document.body.innerHTML = '' })

describe('announcement images', () => {
  it('opens Markdown and HTML images, navigates and zooms, and closes only the lightbox on Escape', async () => {
    const wrapper = mount(AnnouncementContent, {
      attachTo: document.body,
      props: { content: '![First](/first.png)\n\n<a href="/elsewhere"><img src="/second.png" alt="Second"></a>' },
    })
    await wrapper.vm.$nextTick()
    const images = wrapper.findAll('.markdown-body img')
    expect(images[0].attributes()).toMatchObject({ tabindex: '0', role: 'button' })
    await images[0].trigger('keydown', { key: 'Enter' })
    const lightbox = () => document.body.querySelector('.library-lightbox')!
    expect(lightbox().querySelector('img')?.getAttribute('src')).toContain('/first.png')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }))
    await wrapper.vm.$nextTick()
    expect(lightbox().querySelector('img')?.getAttribute('src')).toContain('/second.png')
    lightbox().querySelector<HTMLButtonElement>('[aria-label="mediaLibrary.zoomIn"]')!.click()
    await wrapper.vm.$nextTick()
    expect(lightbox().textContent).toContain('125%')
    const parentEscape = vi.fn()
    document.addEventListener('keydown', parentEscape)
    try {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }))
      await wrapper.vm.$nextTick()
      expect(document.body.querySelector('.library-lightbox')).toBeNull()
      expect(parentEscape).not.toHaveBeenCalled()
      expect(document.activeElement).toBe(images[0].element)
      expect(document.body.classList.contains('media-lightbox-open')).toBe(false)
      await images[1].trigger('click')
      expect(lightbox().querySelector('img')?.getAttribute('src')).toContain('/second.png')
      await wrapper.setProps({ content: 'Next announcement' })
      expect(document.body.querySelector('.library-lightbox')).toBeNull()
    } finally {
      document.removeEventListener('keydown', parentEscape)
    }
  })

  it('sanitizes images and HTML and leaves ordinary links alone', async () => {
    const wrapper = mount(AnnouncementContent, {
      props: { content: '<script>alert(1)</script><img src="javascript:alert(1)" onerror="alert(1)">\n\n[Link](#docs)' },
    })
    await wrapper.vm.$nextTick()
    expect(wrapper.find('script').exists()).toBe(false)
    expect(wrapper.get('img').attributes('onerror')).toBeUndefined()
    expect(wrapper.get('img').attributes('src')).toBeUndefined()
    await wrapper.get('a').trigger('click')
    expect(document.body.querySelector('.library-lightbox')).toBeNull()
  })
})
