import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import ModelPieScene from '../ModelPieScene.vue'
enableAutoUnmount(afterEach)
vi.mock('vue-i18n', async () => ({ ...await vi.importActual('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('three', async () => ({ ...await vi.importActual('three'), WebGLRenderer: class { constructor() { throw new Error('No WebGL in this test') } } }))
describe('ModelPieScene details', () => {
  it('keeps the same three detail fields mounted when selecting and clearing a model', async () => {
    const wrapper = mount(ModelPieScene, { props: { models: [{ model: 'example', total_tokens: 100, requests: 8, input_tokens: 100, output_tokens: 0, cache_creation_tokens: 0, cache_read_tokens: 0, actual_cost: 0.25, cost: 0.5 }] } })
    const facts = wrapper.find('.model-facts').element
    expect(wrapper.findAll('.model-facts b').map(node => node.text())).toEqual(['-', '-', '-'])
    await wrapper.find('.model-row').trigger('click')
    expect(wrapper.find('.model-facts').element).toBe(facts)
    expect(wrapper.findAll('.model-facts b').map(node => node.text())).toEqual(['8', '$0.2500', '$0.5000'])
    await wrapper.find('.model-row').trigger('click')
    expect(wrapper.find('.model-facts').element).toBe(facts)
    expect(wrapper.findAll('.model-facts b').map(node => node.text())).toEqual(['-', '-', '-'])
  })
})
