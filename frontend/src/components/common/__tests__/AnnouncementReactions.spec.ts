import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import AnnouncementReactions from '../AnnouncementReactions.vue'

const { list, set } = vi.hoisted(() => ({ list: vi.fn(), set: vi.fn() }))
vi.mock('@/api/announcements', () => ({ default: { reactions: list, setReaction: set } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
const render = () => mount(AnnouncementReactions, { props: { announcementId: 1 }, global: { stubs: { LottieEmoji: true } } })
beforeEach(() => { vi.clearAllMocks(); list.mockResolvedValue([{ emoji_id: 2, count: 3, reacted: false }]) })

describe('announcement reactions', () => {
  it('opens thirty choices, saves a reaction, and removes it on the second click', async () => {
    const wrapper = render(); await flushPromises()
    expect(wrapper.get('.reaction-pill').text()).toBe('3')
    await wrapper.get('.reaction-add').trigger('click')
    expect(wrapper.findAll('.reaction-choice')).toHaveLength(30)
    set.mockResolvedValue([{ emoji_id: 2, count: 4, reacted: true }])
    await wrapper.findAll('.reaction-choice')[1].trigger('click'); await flushPromises()
    expect(set).toHaveBeenLastCalledWith(1, 2)
    expect(wrapper.get('.reaction-pill').attributes('aria-pressed')).toBe('true')
    set.mockResolvedValue([{ emoji_id: 2, count: 3, reacted: false }])
    await wrapper.get('.reaction-pill').trigger('click'); await flushPromises()
    expect(set).toHaveBeenLastCalledWith(1, null)
    expect(wrapper.get('.reaction-pill').text()).toBe('3')
  })
  it('updates immediately, restores the old count on failure, and allows retry', async () => {
    const wrapper = render(); await flushPromises()
    let fail!: (error: Error) => void
    set.mockImplementationOnce(() => new Promise((_resolve, reject) => { fail = reject }))
    await wrapper.get('.reaction-pill').trigger('click')
    expect(wrapper.get('.reaction-pill').text()).toBe('4')
    expect(wrapper.get('.reaction-pill').attributes('aria-pressed')).toBe('true')
    fail(new Error('offline')); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('saveFailed')
    expect(wrapper.get('.reaction-pill').text()).toBe('3')
    set.mockResolvedValue([{ emoji_id: 2, count: 4, reacted: true }])
    await wrapper.get('.reaction-pill').trigger('click'); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })
  it('switches and cancels immediately while waiting for the saved totals', async () => {
    list.mockResolvedValueOnce([{ emoji_id: 2, count: 3, reacted: true }, { emoji_id: 10, count: 2, reacted: false }])
    const wrapper = render(); await flushPromises()
    let finish!: (value: unknown) => void
    set.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    await wrapper.findAll('.reaction-pill')[1].trigger('click')
    expect(wrapper.findAll('.reaction-pill').map(button => button.text())).toEqual(['2', '3'])
    expect(wrapper.findAll('.reaction-pill')[0].attributes('aria-pressed')).toBe('false')
    expect(wrapper.findAll('.reaction-pill')[1].attributes('aria-pressed')).toBe('true')
    finish([{ emoji_id: 10, count: 5, reacted: true }, { emoji_id: 2, count: 2, reacted: false }]); await flushPromises()
    expect(wrapper.findAll('.reaction-pill')[0].text()).toBe('5')
    set.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    await wrapper.findAll('.reaction-pill')[0].trigger('click')
    expect(wrapper.findAll('.reaction-pill')[0].text()).toBe('4')
    expect(wrapper.findAll('.reaction-pill')[0].attributes('aria-pressed')).toBe('false')
    finish([{ emoji_id: 10, count: 4, reacted: false }, { emoji_id: 2, count: 2, reacted: false }]); await flushPromises()
  })
  it('does not apply a failed save to a different announcement', async () => {
    const wrapper = render(); await flushPromises()
    let fail!: (error: Error) => void
    set.mockImplementationOnce(() => new Promise((_resolve, reject) => { fail = reject }))
    await wrapper.get('.reaction-pill').trigger('click')
    list.mockResolvedValueOnce([{ emoji_id: 5, count: 8, reacted: false }])
    await wrapper.setProps({ announcementId: 2 }); await flushPromises()
    fail(new Error('offline')); await flushPromises()
    expect(wrapper.get('.reaction-pill').text()).toBe('8')
    expect(wrapper.get('.reaction-pill').attributes('aria-label')).toContain('fire')
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })
  it('ignores an old response after switching announcements', async () => {
    let finish!: (value: unknown) => void
    list.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const wrapper = render()
    list.mockResolvedValueOnce([])
    await wrapper.setProps({ announcementId: 2 }); await flushPromises()
    finish([{ emoji_id: 2, count: 9, reacted: true }]); await flushPromises()
    expect(wrapper.find('.reaction-pill').exists()).toBe(false)
  })
})
