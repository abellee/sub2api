import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import CreativeOverview from '../CreativeOverview.vue'

let wrapper: ReturnType<typeof mount>
let resize: () => void
const disconnect = vi.fn()
beforeEach(() => {
  vi.stubGlobal('ResizeObserver', class {
    constructor(callback: () => void) { resize = callback }
    observe() {}
    disconnect = disconnect
  })
})
afterEach(() => { wrapper?.unmount(); vi.restoreAllMocks(); vi.unstubAllGlobals(); vi.clearAllMocks() })

it('lets users start onboarding and pause the visual demo', async () => {
  wrapper = mount(CreativeOverview)
  await wrapper.get('.overview-start').trigger('click')
  expect(wrapper.emitted('start')).toHaveLength(1)
  const play = wrapper.get('[aria-label="播放流程演示"]')
  await play.trigger('click')
  expect(wrapper.get('.flow-result').classes()).toContain('is-playing')
  expect(play.attributes('aria-label')).toBe('暂停流程演示')
  expect(play.attributes('aria-pressed')).toBe('true')
  expect(wrapper.get('[role="status"]').text()).toBe('正在播放流程演示')
  await play.trigger('click')
  expect(wrapper.get('.flow-result').classes()).not.toContain('is-playing')
  expect(play.attributes('aria-pressed')).toBe('false')
})

it('connects the source port to both cards and follows their resized positions', async () => {
  wrapper = mount(CreativeOverview)
  const bounds = (x: number, y: number, width: number, height: number) => ({ x, y, width, height } as DOMRect)
  const canvas = vi.spyOn(wrapper.get('.creative-flow').element, 'getBoundingClientRect').mockReturnValue(bounds(100, 200, 580, 520))
  vi.spyOn(wrapper.get('.output-port').element, 'getBoundingClientRect').mockReturnValue(bounds(300, 450, 20, 20))
  const prompt = vi.spyOn(wrapper.get('.flow-prompt').element, 'getBoundingClientRect').mockReturnValue(bounds(440, 250, 230, 200))
  vi.spyOn(wrapper.get('.flow-result').element, 'getBoundingClientRect').mockReturnValue(bounds(450, 500, 240, 210))
  resize()
  await nextTick()
  expect(wrapper.get('.flow-wires').attributes('viewBox')).toBe('0 0 580 520')
  expect(wrapper.findAll('.flow-wires > path').map(path => path.attributes('d'))).toEqual([
    'M210 260 C276 260 276 150 342 150',
    'M210 260 C281 260 281 405 352 405',
  ])
  canvas.mockReturnValue(bounds(100, 200, 760, 520))
  prompt.mockReturnValue(bounds(620, 250, 230, 200))
  resize()
  await nextTick()
  expect(wrapper.get('.flow-wires').attributes('viewBox')).toBe('0 0 760 520')
  expect(wrapper.get('.flow-wires > path').attributes('d')).toBe('M210 260 C366 260 366 150 522 150')
  wrapper.unmount()
  expect(disconnect).toHaveBeenCalled()
})
