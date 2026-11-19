import { afterEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { useBalancePreview } from '../useBalancePreview'

const Probe = defineComponent({
  setup() { return useBalancePreview(() => 77) },
  template: '<span>{{ balance }}</span>'
})
afterEach(() => { vi.useRealTimers(); window.history.replaceState({}, '', '/dashboard') })
describe('useBalancePreview', () => {
  it('keeps real data outside the explicit demo', () => {
    const wrapper = mount(Probe)
    expect(wrapper.text()).toBe('77')
    wrapper.unmount()
  })
  it('shares a three-second demo timer and cleans it up after the last consumer leaves', async () => {
    vi.useFakeTimers()
    window.history.replaceState({}, '', '/dashboard?balanceDemo=1')
    const first = mount(Probe)
    const second = mount(Probe)
    await vi.advanceTimersByTimeAsync(3000)
    expect(first.text()).toBe('8.76')
    expect(second.text()).toBe('8.76')
    first.unmount()
    await vi.advanceTimersByTimeAsync(3000)
    expect(second.text()).toBe('4.32')
    second.unmount()
    expect(vi.getTimerCount()).toBe(0)
  })
})
