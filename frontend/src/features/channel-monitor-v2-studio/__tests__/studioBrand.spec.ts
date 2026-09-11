import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import StudioBrandIcon from '../StudioBrandIcon.vue'
import { studioBrandFill } from '../studioBrand'

describe('studioBrand', () => {
  it('uses official brand fills for logos and names', () => {
    expect(studioBrandFill('anthropic')).toBe('#D97757')
    expect(studioBrandFill('openai')).toBe('#10A37F')
    expect(studioBrandFill('gemini')).toBe('#3186FF')
    expect(studioBrandFill('deepseek')).toBe('#4D6BFE')
  })

  it('renders the official platform mark in the brand color', () => {
    const wrapper = mount(StudioBrandIcon, { props: { platform: 'openai', size: 'md' } })
    expect(wrapper.findComponent(PlatformIcon).exists()).toBe(true)
    expect(wrapper.attributes('style') || '').toMatch(/rgb\(16,\s*163,\s*127\)|#10A37F/i)
  })
})
