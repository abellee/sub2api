import { describe, expect, it } from 'vitest'
import { buildModelPlazaEntries } from '../modelPlaza'

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
      id: 'claude-sonnet-4-5-20250929', provider: 'anthropic', input: 3, cache: 0.3, output: 15,
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

    expect(entries[0]).toEqual(expect.objectContaining({ provider: 'gemini', input: 0.3, output: 2.5, cache: null }))
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
