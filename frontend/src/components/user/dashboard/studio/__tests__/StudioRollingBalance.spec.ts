import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import StudioRollingBalance from '../StudioRollingBalance.vue'

describe('StudioRollingBalance', () => {
  it('shows the initial balance without rolling and only rolls changed digits on updates', async () => {
    const wrapper = mount(StudioRollingBalance, { props: { value: 12.5 } })
    expect(wrapper.text()).toBe('$12.50')
    expect(wrapper.findAll('.is-rolling')).toHaveLength(0)
    await wrapper.setProps({ value: 12.51 })
    const tracks = wrapper.findAll('.balance-digit-track')
    expect(tracks).toHaveLength(4)
    expect(wrapper.findAll('.is-rolling')).toHaveLength(1)
    expect(tracks.slice(0, 3).every(track => track.findAll('.balance-digit-step').length === 1)).toBe(true)
    expect(tracks[3].findAll('.balance-digit-step').length).toBeGreaterThanOrEqual(11)
    expect(tracks.map(track => track.findAll('.balance-digit-step').at(-1)!.attributes('data-digit')).join('')).toBe('1251')
    expect(wrapper.text()).toBe('$12.51')
    wrapper.unmount()
  })

  it('rolls changed places across carries and thousands separators', async () => {
    const wrapper = mount(StudioRollingBalance, { props: { value: 1299.99 } })
    await wrapper.setProps({ value: 1300 })
    const tracks = wrapper.findAll('.balance-digit-track')
    expect(tracks.map(track => track.classes().includes('is-rolling'))).toEqual([false, true, true, true, true, true])
    expect(wrapper.text()).toBe('$1,300.00')
    wrapper.unmount()
  })

  it('does not roll when the displayed balance is unchanged', async () => {
    const wrapper = mount(StudioRollingBalance, { props: { value: 12.501 } })
    await wrapper.setProps({ value: 12.502 })
    expect(wrapper.findAll('.is-rolling')).toHaveLength(0)
    expect(wrapper.text()).toBe('$12.50')
    wrapper.unmount()
  })

  it('lands on the latest digits when updates interrupt a roll or change the number of digits', async () => {
    const wrapper = mount(StudioRollingBalance, { props: { value: 9.99 } })
    for (const value of [10, 1024.56, 4.32, 0]) {
      await wrapper.setProps({ value })
      const digits = wrapper.findAll('.balance-digit-track').map(track => track.findAll('.balance-digit-step').at(-1)!.attributes('data-digit')).join('')
      expect(digits).toBe(value.toFixed(2).replace('.', ''))
    }
    wrapper.unmount()
  })
})
