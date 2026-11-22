import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import { createDevModelPlazaResponse } from '@/api/modelPlazaMock'
import { buildModelPlazaEntries } from '../modelPlaza'
import ModelPlazaContent from '../ModelPlazaContent.vue'

const { getModelPlaza } = vi.hoisted(() => ({ getModelPlaza: vi.fn() }))
vi.mock('@/api/modelPlaza', () => ({ modelPlazaAPI: { getModelPlaza } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ locale: { value: 'zh' } }) }))

function requestModel(name = 'gemini-flash', platform = 'gemini', price: number | null = 0.025): PlazaModel {
  return {
    name, platform,
    pricing: {
      billing_mode: 'per_request', input_price: null, output_price: null,
      cache_write_price: null, cache_read_price: null, per_request_price: price, intervals: [],
    },
    official_pricing: {
      input_price: 2e-6, output_price: 10e-6, cache_write_price: null, cache_read_price: 0.5e-6,
    },
  }
}

function group(models: PlazaModel[]): ModelPlazaGroup {
  return {
    ...createDevModelPlazaResponse().groups[0],
    platform: models[0].platform, models,
    rate_multiplier: 0.2, user_rate_multiplier: 0.4,
    image_rate_independent: true, image_rate_multiplier: 0.01,
    video_rate_independent: true, video_rate_multiplier: 0.01,
  }
}

function tier(price: number | null, min = 0, max: number | null = null, label = '') {
  return {
    min_tokens: min, max_tokens: max, tier_label: label,
    input_price: null, output_price: null, cache_write_price: null, cache_read_price: null,
    per_request_price: price,
  }
}

describe('per-request model prices', () => {
  it.each([
    ['gemini', 'gemini-flash'], ['google', 'gemini-pro'], ['openai', 'gpt-5'],
    ['anthropic', 'claude-sonnet'], ['grok', 'grok-4'], ['deepseek', 'deepseek-chat'],
    ['kimi', 'kimi-k2'], ['zhipu', 'glm-5'], ['minimax', 'minimax-m2'],
    ['doubao', 'doubao-pro'], ['custom', 'unknown-model'],
    ['openai', 'gpt-image-1'], ['grok', 'grok-imagine-image'], ['grok', 'grok-imagine-video'],
  ])('uses the configured billing mode for %s / %s', (platform, name) => {
    const [entry] = buildModelPlazaEntries([group([requestModel(name, platform)])])
    expect(entry).toMatchObject({
      kind: 'per_request', rateMultiplier: 0.4, input: null, output: null, cache: null,
      tiers: [{ label: '', original: '0.025' }], tokenTiers: [], officialTokenTiers: [],
    })
  })

  it('preserves default and context tier prices, including free tiers', () => {
    const model = requestModel()
    model.pricing!.intervals = [tier(0, 0, 128000), tier(0.05, 128000), tier(null)]
    const [entry] = buildModelPlazaEntries([group([model])])
    expect(entry.tiers).toEqual([
      { label: '', original: '0.025' },
      { label: '<=128K', original: '0' },
      { label: '>128K', original: '0.05' },
    ])
  })

  it('keeps labeled tiers without requiring a default price', () => {
    const model = requestModel('custom-audio', 'custom', null)
    model.pricing!.intervals = [tier(0.001, 0, null, 'realtime')]
    expect(buildModelPlazaEntries([group([model])])[0].tiers)
      .toEqual([{ label: 'realtime', original: '0.001' }])
  })

  it('distinguishes a free request from a missing price without using official token prices', () => {
    const entries = buildModelPlazaEntries([group([
      requestModel('free', 'gemini', 0), requestModel('missing', 'gemini', null),
    ])])
    expect(entries[0].tiers).toEqual([{ label: '', original: '0' }])
    expect(entries[1].tiers).toEqual([{ label: '', original: null }])
    expect(entries[1].input).toBeNull()
  })
})

describe('per-request cards and list', () => {
  beforeEach(() => localStorage.removeItem('llmfree-model-view'))
  afterEach(() => {
    localStorage.removeItem('llmfree-model-view')
    vi.useRealTimers()
  })

  it.each(['home', 'console'] as const)('shows actual request prices in the %s card and list in both languages', async (variant) => {
    const paid = requestModel()
    paid.pricing!.intervals = [tier(0.05, 128000)]
    getModelPlaza.mockResolvedValue({ groups: [group([
      paid, requestModel('free', 'gemini', 0), requestModel('missing', 'gemini', null),
      requestModel('tiny', 'gemini', 0.000001), requestModel('deepseek-chat', 'deepseek'),
    ])] })
    const wrapper = mount(ModelPlazaContent, {
      props: { variant, language: 'zh' }, global: { stubs: { ModelIcon: true, Select: true } },
    })
    try {
      await flushPromises()
      const cards = wrapper.findAll('.model-plaza__card')
      expect(cards).toHaveLength(5)
      expect(cards[0].text()).toContain('按次')
      expect(cards[0].text()).toContain('USD / 次')
      expect(cards[0].text()).toContain('原价 $0.025')
      expect(cards[0].text()).toContain('实际 $0.01')
      expect(cards[0].text()).toContain('默认价格')
      expect(cards[0].text()).toContain('>128K')
      expect(cards[0].text()).toContain('实际 $0.02')
      expect(cards[1].text()).toContain('实际 $0.00')
      expect(cards[2].text()).toContain('实际 -')
      expect(cards[3].text()).toContain('实际 $0.0000004')
      expect(cards[4].text()).toContain('CNY / 次')
      expect(cards[4].text()).toContain('实际 ¥0.01')
      expect(wrapper.find('.model-plaza__prices').exists()).toBe(false)
      expect(wrapper.find('.model-plaza__long-context-toggle').exists()).toBe(false)

      await wrapper.setProps({ language: 'en' })
      expect(wrapper.find('.model-plaza__card').text()).toContain('Per request')
      expect(wrapper.find('.model-plaza__card').text()).toContain('USD / request')
      await wrapper.findAll('.model-plaza__view-toggle button')[1].trigger('click')
      const rows = wrapper.findAll('.model-plaza__table tbody tr')
      expect(rows).toHaveLength(5)
      expect(rows[0].text()).toContain('Actual $0.01')
      expect(rows[0].text()).toContain('Default price')
      expect(rows[0].text()).toContain('USD / request')
      expect(rows[3].text()).toContain('Actual $0.0000004')
      expect(rows[4].text()).toContain('CNY / request')
      expect(rows[0].find('td[colspan="3"]').exists()).toBe(true)
      await wrapper.setProps({ language: 'zh' })
      expect(wrapper.find('.model-plaza__table tbody tr').text()).toContain('USD / 次')
    } finally {
      wrapper.unmount()
    }
  })

  it('applies the selected time period multiplier to request prices', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-11-08T04:00:00Z'))
    const model = requestModel()
    model.time_pricing = {
      timezone: 'Asia/Shanghai', periods: [{ start_time: '10:00', end_time: '14:00', multiplier: 0.5 }],
    }
    getModelPlaza.mockResolvedValue({ groups: [group([model])] })
    const wrapper = mount(ModelPlazaContent, { global: { stubs: { ModelIcon: true, Select: true } } })
    try {
      await flushPromises()
      expect(wrapper.find('.model-plaza__card').text()).toContain('实际 $0.005')
      await wrapper.find('.model-plaza__price-tabs button').trigger('click')
      expect(wrapper.find('.model-plaza__card').text()).toContain('实际 $0.01')
    } finally {
      wrapper.unmount()
    }
  })
})
