import { defineComponent, ref } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AnthropicThinkingCompatField from '../AnthropicThinkingCompatField.vue'
import type { AnthropicThinkingCompat } from '@/types'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

function mountField(initial: AnthropicThinkingCompat) {
  return mount(defineComponent({
    components: { AnthropicThinkingCompatField },
    setup: () => ({ config: ref(initial) }),
    template: '<AnthropicThinkingCompatField v-model="config" id-prefix="test-thinking" />',
  }))
}

describe('AnthropicThinkingCompatField', () => {
  it('defaults to off and offers all or selected models when enabled', async () => {
    const wrapper = mountField({ enabled: false, all_models: true, models: [] })
    expect(wrapper.findAll('input[type=radio]')).toHaveLength(0)
    await wrapper.get('button[role=switch]').trigger('click')
    expect(wrapper.vm.config.enabled).toBe(true)
    expect(wrapper.findAll('input[type=radio]')).toHaveLength(2)
    expect(wrapper.find('textarea').exists()).toBe(false)
    await wrapper.findAll('input[type=radio]')[1].setValue()
    const field = wrapper.findComponent(AnthropicThinkingCompatField)
    expect(field.vm.validate()).toBe(false)
    await flushPromises()
    expect(wrapper.find('[role=alert]').exists()).toBe(true)
    await wrapper.get('textarea').setValue('claude-sonnet-5-5\n')
    await flushPromises()
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('claude-sonnet-5-5\n')
    await wrapper.get('textarea').setValue('claude-sonnet-5-5\nclaude-opus-*, claude-sonnet-5-5')
    await flushPromises()
    expect(wrapper.vm.config.models).toEqual(['claude-sonnet-5-5', 'claude-opus-*'])
    expect(field.vm.validate()).toBe(true)
    await wrapper.findAll('input[type=radio]')[0].setValue()
    expect(wrapper.vm.config.all_models).toBe(true)
    expect(wrapper.vm.config.models).toEqual(['claude-sonnet-5-5', 'claude-opus-*'])
    wrapper.unmount()
  })

  it('loads saved selections and follows external resets', async () => {
    const wrapper = mountField({ enabled: true, all_models: false, models: ['claude-sonnet-*'] })
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('claude-sonnet-*')
    wrapper.vm.config = { enabled: true, all_models: false, models: ['claude-opus-*'] }
    await flushPromises()
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('claude-opus-*')
    await wrapper.get('textarea').setValue(' , ， \n')
    await flushPromises()
    const field = wrapper.findComponent(AnthropicThinkingCompatField)
    expect(field.vm.validate()).toBe(false)
    field.vm.resetValidation()
    await flushPromises()
    expect(wrapper.find('[role=alert]').exists()).toBe(false)
    await wrapper.get('button[role=switch]').trigger('click')
    expect(field.vm.validate()).toBe(true)
    wrapper.unmount()
  })
})
