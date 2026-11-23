import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import KeyInflationHistory from '../KeyInflationHistory.vue'
const { history } = vi.hoisted(() => ({ history: vi.fn() }))
vi.mock('@/api/inflationMonitor', async importOriginal => ({ ...await importOriginal<typeof import('@/api/inflationMonitor')>(), inflationMonitorAPI: { history } }))
vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } }) }))
describe('KeyInflationHistory', () => {
  it('shows token counts and prompt for a selected historical candle', async () => {
    history.mockResolvedValue({ aggregation: 'median', yellow_threshold: 10, red_threshold: 30, results: [
      { measured_at: '2026-11-24T00:00:00Z', prompt: 'First prompt', encoding: 'o200k_base', reasoning_effort: 'none', samples: [{ model: 'first-model', input_tokens: 100, output_tokens: 110, text_tokens: 100, reasoning_tokens: 0, rate: 10 }] },
      { measured_at: '2026-11-24T01:00:00Z', prompt: 'Second prompt', encoding: 'cl100k_base', reasoning_effort: 'low', samples: [{ model: 'second-model', input_tokens: 200, output_tokens: 120, text_tokens: 100, reasoning_tokens: 10, rate: 20 }] },
    ] })
    const wrapper = mount(KeyInflationHistory, { props: { keyId: 89 } })
    await flushPromises()
    expect(history).toHaveBeenCalledWith(89, expect.any(AbortSignal))
    expect(wrapper.text()).toContain('second-model')
    expect(wrapper.get('.inflation-prompt').text()).toBe('Second prompt')
    expect(wrapper.text()).toContain('inflationMonitor.formula')
    expect(wrapper.get('.inflation-sample').text()).toContain('200')
    expect(wrapper.text()).not.toContain('inflationMonitor.valid')
    await wrapper.findAll('g[role="button"]')[0]!.trigger('click')
    expect(wrapper.text()).toContain('first-model')
    expect(wrapper.get('.inflation-prompt').text()).toBe('First prompt')
    expect(wrapper.text()).toContain('none')
    wrapper.unmount()
  })
})
