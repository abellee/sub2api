import { describe, expect, it } from 'vitest'
import type { ModelPlazaGroup } from '@/api/modelPlaza'
import { createDevModelPlazaResponse } from '@/api/modelPlazaMock'
import { buildModelPlazaEntries, formatActualModelPrice, modelPlazaProviderForGroup } from '../modelPlaza'

describe('model plaza matching', () => {
  it('renders only priced media tiers and keeps configured Grok media names as media', () => {
    const group: ModelPlazaGroup = {
      id: 29,
      name: '生图/视频',
      description: '',
      platform: 'grok',
      subscription_type: 'standard',
      rate_multiplier: 0.2,
      peak_rate_enabled: false,
      peak_start: '',
      peak_end: '',
      peak_rate_multiplier: 1,
      is_exclusive: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      video_rate_independent: false,
      video_rate_multiplier: 1,
      models: [
        {
          name: 'grok-imagine',
          platform: 'grok',
          pricing: {
            billing_mode: 'image',
            input_price: null,
            output_price: null,
            cache_write_price: null,
            cache_read_price: null,
            per_request_price: null,
            intervals: [
              { min_tokens: 0, max_tokens: null, tier_label: '1K', input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: 0.03 },
              { min_tokens: 0, max_tokens: null, tier_label: '4K', input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: 0.08 },
            ],
          },
          official_pricing: null,
        },
        {
          name: 'grok-imagine-video',
          platform: 'grok',
          pricing: {
            billing_mode: 'video',
            input_price: null,
            output_price: null,
            cache_write_price: null,
            cache_read_price: null,
            per_request_price: null,
            intervals: [{ min_tokens: 0, max_tokens: null, tier_label: '720p', input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: 0.12 }],
          },
          official_pricing: null,
        },
        {
          name: 'grok-imagine-image-quality',
          platform: 'grok',
          pricing: {
            billing_mode: 'image',
            input_price: null,
            output_price: null,
            cache_write_price: null,
            cache_read_price: null,
            per_request_price: 0.08,
            intervals: [],
          },
          official_pricing: null,
        },
      ],
    }

    expect(buildModelPlazaEntries([group])).toEqual(expect.arrayContaining([
      expect.objectContaining({
        id: 'grok-imagine',
        kind: 'image',
        tiers: [{ label: '1K', original: '0.03' }, { label: '4K', original: '0.08' }],
      }),
      expect.objectContaining({
        id: 'grok-imagine-video',
        kind: 'video',
        tiers: [{ label: '720p', original: '0.12' }],
      }),
      expect.objectContaining({
        id: 'grok-imagine-image-quality',
        kind: 'image',
        tiers: [{ label: '', original: '0.08' }],
      }),
    ]))
  })

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
      video_rate_independent: false,
      video_rate_multiplier: 1,
      models: [{
        name: 'claude-sonnet-4-5-20250929',
        platform: 'anthropic',
        pricing: null,
        official_pricing: {
          input_price: 0.000003,
          output_price: 0.000015,
          cache_write_price: 0.00000375,
          cache_write_1h_price: 0.000006,
          cache_read_price: 0.0000003,
        },
      }],
    }])

    expect(entries).toEqual([expect.objectContaining({
      id: 'claude-sonnet-4-5-20250929', provider: 'anthropic', input: '3', cache: '0.3',
      cacheWrite: '3.75', cacheWrite1h: '6', cacheRead: '0.3', output: '15',
      groupId: 7, groupName: 'CC - Kiro', rateMultiplier: 0.12,
    })])
  })

  it('uses channel cache fields independently, preserves zero, and falls back per missing field', () => {
    const entries = buildModelPlazaEntries([{
      id: 8,
      name: 'Claude channel',
      description: '',
      platform: 'anthropic',
      subscription_type: 'standard',
      rate_multiplier: 0.2,
      peak_rate_enabled: false,
      peak_start: '',
      peak_end: '',
      peak_rate_multiplier: 1,
      is_exclusive: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      video_rate_independent: false,
      video_rate_multiplier: 1,
      models: [{
        name: 'claude-opus-4-6',
        platform: 'anthropic',
        pricing: {
          billing_mode: 'token',
          input_price: 5e-6,
          output_price: 25e-6,
          cache_write_price: 0,
          cache_write_1h_price: null,
          cache_read_price: 0.4e-6,
          image_input_price: null,
          image_output_price: null,
          per_request_price: null,
          intervals: [],
        },
        official_pricing: {
          input_price: 5e-6,
          output_price: 25e-6,
          cache_write_price: 6.25e-6,
          cache_write_1h_price: 10e-6,
          cache_read_price: 0.5e-6,
        },
      }],
    }])

    expect(entries[0]).toEqual(expect.objectContaining({
      cacheWrite: '0',
      cacheWrite1h: '10',
      cacheRead: '0.4',
    }))
  })

  it('prefers configured channel prices and marks domestic model prices as CNY', () => {
    const entries = buildModelPlazaEntries([{
      id: 31,
      name: '智谱渠道',
      description: '',
      platform: 'zhipu',
      subscription_type: 'standard',
      rate_multiplier: 0.5,
      peak_rate_enabled: false,
      peak_start: '',
      peak_end: '',
      peak_rate_multiplier: 1,
      is_exclusive: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      video_rate_independent: false,
      video_rate_multiplier: 1,
      models: [{
        name: 'glm-5',
        platform: 'zhipu',
        pricing: {
          billing_mode: 'token',
          input_price: 0.25e-6,
          output_price: null,
          cache_write_price: null,
          cache_read_price: null,
          image_input_price: null,
          image_output_price: null,
          per_request_price: null,
          intervals: [],
        },
        official_pricing: {
          input_price: 1e-6,
          output_price: 3.2e-6,
          cache_write_price: null,
          cache_read_price: 0.2e-6,
        },
        time_pricing: {
          timezone: 'Asia/Shanghai',
          weekdays_only: true,
          periods: [{ start_time: '09:00', end_time: '12:00', multiplier: 1.5 }],
        },
      }],
    }])

    expect(entries[0]).toEqual(expect.objectContaining({
      input: '0.25',
      output: '3.2',
      cache: '0.2',
      currency: 'CNY',
      timePricing: expect.objectContaining({ periods: [{ start_time: '09:00', end_time: '12:00', multiplier: 1.5 }] }),
    }))
  })

  it('maps configured token context tiers to per-million prices', () => {
    const entries = buildModelPlazaEntries([{
      id: 18,
      name: 'Grok Heavy',
      description: '',
      platform: 'grok',
      subscription_type: 'standard',
      rate_multiplier: 0.11,
      peak_rate_enabled: false,
      peak_start: '',
      peak_end: '',
      peak_rate_multiplier: 1,
      is_exclusive: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      video_rate_independent: false,
      video_rate_multiplier: 1,
      models: [{
        name: 'grok-4.6',
        platform: 'grok',
        has_channel_context_pricing: true,
        pricing: {
          billing_mode: 'token',
          input_price: 2e-6,
          output_price: 10e-6,
          cache_write_price: null,
          cache_read_price: 0.5e-6,
          per_request_price: null,
          intervals: [
            { min_tokens: 0, max_tokens: 128000, tier_label: '≤128K', input_price: 2e-6, output_price: 10e-6, cache_write_price: null, cache_read_price: 0.5e-6, per_request_price: null },
            { min_tokens: 128000, max_tokens: null, tier_label: '>128K', input_price: 4e-6, output_price: 20e-6, cache_write_price: null, cache_read_price: 1e-6, per_request_price: null },
          ],
        },
        official_pricing: {
          input_price: 2e-6,
          output_price: 10e-6,
          cache_write_price: null,
          cache_read_price: 0.5e-6,
        },
      }],
    }])

    expect(entries[0]).toEqual(expect.objectContaining({
      kind: 'token',
      tokenTiers: [
        {
          label: '>128K', input: '4', output: '20', cache: '1',
          cacheWrite: null, cacheWrite1h: null, cacheRead: '1',
        },
      ],
    }))
  })

  it('does not advertise official context tiers without channel intervals', () => {
    const entries = buildModelPlazaEntries([{
      id: 19,
      name: 'Official only',
      description: '',
      platform: 'anthropic',
      subscription_type: 'standard',
      rate_multiplier: 1,
      peak_rate_enabled: false,
      peak_start: '',
      peak_end: '',
      peak_rate_multiplier: 1,
      is_exclusive: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      video_rate_independent: false,
      video_rate_multiplier: 1,
      models: [{
        name: 'claude-sonnet-4-6',
        platform: 'anthropic',
        pricing: {
          billing_mode: 'token',
          input_price: 3e-6,
          output_price: 15e-6,
          cache_write_price: null,
          cache_read_price: 0.3e-6,
          per_request_price: null,
          intervals: [],
        },
        official_pricing: {
          input_price: 3e-6,
          output_price: 15e-6,
          cache_write_price: null,
          cache_read_price: 0.3e-6,
          intervals: [{ min_tokens: 200000, max_tokens: null, tier_label: '>200K', input_price: 6e-6, output_price: 30e-6, cache_write_price: null, cache_read_price: 0.6e-6, per_request_price: null }],
        },
      }],
    }])

    expect(entries[0].tokenTiers).toEqual([])
    expect(entries[0].officialTokenTiers).toEqual([
      expect.objectContaining({ label: '>200K', input: '6', output: '30', cacheRead: '0.6' }),
    ])
  })

  it('uses the official Grok 4.5 cache-read value for the displayed original price', () => {
    const entries = buildModelPlazaEntries([{
      id: 20,
      name: 'Grok',
      description: '',
      platform: 'grok',
      subscription_type: 'standard',
      rate_multiplier: 1,
      peak_rate_enabled: false,
      peak_start: '',
      peak_end: '',
      peak_rate_multiplier: 1,
      is_exclusive: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      video_rate_independent: false,
      video_rate_multiplier: 1,
      models: [{
        name: 'grok-4.5',
        platform: 'grok',
        pricing: { billing_mode: 'token', input_price: 2e-6, output_price: 6e-6, cache_write_price: 0, cache_read_price: 0.5e-6, per_request_price: null, intervals: [] },
        official_pricing: { input_price: 2e-6, output_price: 6e-6, cache_write_price: null, cache_read_price: 0.3e-6 },
      }],
    }])

    expect(entries[0]).toEqual(expect.objectContaining({ cacheRead: '0.5' }))
  })

  it('recognizes legacy Grok channel intervals when the ownership flag is absent', () => {
    const entries = buildModelPlazaEntries([{
      id: 20,
      name: 'Grok',
      description: '',
      platform: 'grok',
      subscription_type: 'standard',
      rate_multiplier: 1,
      peak_rate_enabled: false,
      peak_start: '',
      peak_end: '',
      peak_rate_multiplier: 1,
      is_exclusive: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      video_rate_independent: false,
      video_rate_multiplier: 1,
      models: [{
        name: 'grok-4.5',
        platform: 'grok',
        pricing: {
          billing_mode: 'token',
          input_price: 2e-6,
          output_price: 6e-6,
          cache_write_price: 0,
          cache_read_price: 0.5e-6,
          per_request_price: null,
          intervals: [{
            min_tokens: 200000,
            max_tokens: null,
            tier_label: '>200K',
            input_price: 4e-6,
            output_price: 12e-6,
            cache_write_price: 0,
            cache_read_price: 1e-6,
            per_request_price: null,
          }],
        },
        official_pricing: {
          input_price: 2e-6,
          output_price: 6e-6,
          cache_write_price: null,
          cache_read_price: 0.3e-6,
          intervals: [{
            min_tokens: 199999,
            max_tokens: null,
            tier_label: '≥200K',
            input_price: 4e-6,
            output_price: 12e-6,
            cache_write_price: null,
            cache_read_price: 0.6e-6,
            per_request_price: null,
          }],
        },
      }],
    }])

    expect(entries[0].tokenTiers).toEqual([
      expect.objectContaining({ label: '>200K', cacheRead: '1' }),
    ])
    expect(entries[0].officialTokenTiers).toEqual([])
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
      video_rate_independent: false,
      video_rate_multiplier: 1,
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
      video_rate_independent: false,
      video_rate_multiplier: 1,
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
      video_rate_independent: false,
      video_rate_multiplier: 1,
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
      video_rate_independent: false,
      video_rate_multiplier: 1,
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
