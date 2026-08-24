import { describe, expect, it } from 'vitest'
import type { ModelPlazaGroup } from '@/api/modelPlaza'
import { createDevModelPlazaResponse } from '@/api/modelPlazaMock'
import { buildModelPlazaEntries, formatActualModelPrice, modelPlazaProviderForGroup } from '../modelPlaza'

describe('model plaza matching', () => {
  it('maps Sub2API official token prices to per-million model plaza entries', () => {
    const entries = buildModelPlazaEntries([{
      id: 7,
      name: 'CC - Kiro',
      description: '',
      platform: 'anthropic',
      subscription_type: 'standard',
      rate_multiplier: 0.12,
      peak_rate_enabled: false,
      peak_start: '',
      peak_end: '',
      peak_rate_multiplier: 1,
      is_exclusive: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      models: [{
        name: 'claude-sonnet-4-5-20250929',
        platform: 'anthropic',
        pricing: null,
        official_pricing: {
          input_price: 0.000003,
          output_price: 0.000015,
          cache_write_price: 0.00000375,
          cache_read_price: 0.0000003,
        },
      }],
    }])

    expect(entries).toEqual([expect.objectContaining({
      id: 'claude-sonnet-4-5-20250929', provider: 'anthropic', input: '3', cache: '0.3', output: '15',
      groupId: 7, groupName: 'CC - Kiro', rateMultiplier: 0.12,
    })])
  })

  it('normalizes Google pricing to the Gemini provider', () => {
    const entries = buildModelPlazaEntries([{
      id: 12,
      name: 'Gemini',
      description: '',
      platform: 'gemini',
      subscription_type: 'standard',
      rate_multiplier: 0.15,
      peak_rate_enabled: false,
      peak_start: '',
      peak_end: '',
      peak_rate_multiplier: 1,
      is_exclusive: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      models: [{
        name: 'gemini-2.5-flash',
        platform: 'google',
        pricing: null,
        official_pricing: { input_price: 0.0000003, output_price: 0.0000025, cache_write_price: null, cache_read_price: null },
      }],
    }])

    expect(entries[0]).toEqual(expect.objectContaining({ provider: 'gemini', input: '0.3', output: '2.5', cache: null }))
  })

  it('uses the native DeepSeek platform for DeepSeek groups', () => {
    const deepSeekGroup: ModelPlazaGroup = {
      id: 18,
      name: 'DeepSeek 高速线路',
      description: '',
      platform: 'deepseek',
      subscription_type: 'standard',
      rate_multiplier: 0.35,
      peak_rate_enabled: false,
      peak_start: '',
      peak_end: '',
      peak_rate_multiplier: 1,
      is_exclusive: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      models: [{
        name: 'deepseek-v3.2',
        platform: 'deepseek',
        pricing: null,
        official_pricing: null,
      }],
    }

    expect(modelPlazaProviderForGroup(deepSeekGroup)).toBe('deepseek')
    expect(buildModelPlazaEntries([deepSeekGroup])[0]).toEqual(expect.objectContaining({
      provider: 'deepseek',
      groupName: 'DeepSeek 高速线路',
    }))
  })

  it('does not introduce floating point artifacts while converting prices per million', () => {
    const entries = buildModelPlazaEntries([{
      id: 2,
      name: 'Welfare',
      description: '',
      platform: 'openai',
      subscription_type: 'standard',
      rate_multiplier: 0.08,
      peak_rate_enabled: false,
      peak_start: '',
      peak_end: '',
      peak_rate_multiplier: 1,
      is_exclusive: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      models: [{
        name: 'gpt-low-price',
        platform: 'openai',
        pricing: null,
        official_pricing: {
          input_price: 0.0000002,
          output_price: 0.0000012,
          cache_write_price: null,
          cache_read_price: 0.00000002,
        },
      }],
    }])

    expect(entries[0]).toEqual(expect.objectContaining({ input: '0.2', cache: '0.02', output: '1.2' }))
    expect(formatActualModelPrice(entries[0].input, entries[0].rateMultiplier)).toBe('0.016')
    expect(formatActualModelPrice(entries[0].cache, entries[0].rateMultiplier)).toBe('0.0016')
    expect(formatActualModelPrice(entries[0].output, entries[0].rateMultiplier)).toBe('0.096')
  })

  it('keeps configured models even when Sub2API has no official price', () => {
    const entries = buildModelPlazaEntries([{
      id: 12,
      name: 'Gemini',
      description: '',
      platform: 'gemini',
      subscription_type: 'standard',
      rate_multiplier: 0.15,
      peak_rate_enabled: false,
      peak_start: '',
      peak_end: '',
      peak_rate_multiplier: 1,
      is_exclusive: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      models: [{ name: 'gemini-unpriced', platform: 'gemini', pricing: null, official_pricing: null }],
    }])

    expect(entries).toEqual([expect.objectContaining({
      id: 'gemini-unpriced', provider: 'gemini', input: null, output: null, cache: null,
    })])
  })
})

describe('model plaza actual price formatting', () => {
  it('does not inject floating-point artifacts into generated demo cache prices', () => {
    const entries = buildModelPlazaEntries(createDevModelPlazaResponse().groups)
    const terra = entries.find((entry) => (
      entry.id === 'gpt-5.6-terra' && entry.groupName === 'GPT 经济线路'
    ))

    expect(terra?.cache).toBe('0.15')
    expect(formatActualModelPrice(terra?.cache ?? null, terra?.rateMultiplier ?? 0)).toBe('0.0975')
  })

  it('keeps every meaningful decimal place for low prices', () => {
    expect(formatActualModelPrice(0.03, 0.08)).toBe('0.0024')
    expect(formatActualModelPrice(0.000001, 0.08)).toBe('0.00000008')
  })

  it('avoids binary floating point artifacts while retaining two minimum decimals', () => {
    expect(formatActualModelPrice(0.1, 0.2)).toBe('0.02')
    expect(formatActualModelPrice(2.5, 0.15)).toBe('0.375')
    expect(formatActualModelPrice(5, 2)).toBe('10.00')
  })

  it('keeps long decimal products without rounding or floating-point noise', () => {
    expect(formatActualModelPrice('0.000000000123456789', '0.123456789')).toBe('0.000000000015241578750190521')
    expect(formatActualModelPrice('1234567.890123456789', '0.00000001')).toBe('0.01234567890123456789')
  })

  it('keeps missing prices as a dash', () => {
    expect(formatActualModelPrice(null, 0.08)).toBe('-')
  })
})
