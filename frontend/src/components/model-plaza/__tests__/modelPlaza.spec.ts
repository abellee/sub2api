import { describe, expect, it } from 'vitest'
import { buildModelPlazaEntries, canonicalModelId } from '../modelPlaza'

describe('model plaza matching', () => {
  it('removes compact and dashed date suffixes', () => {
    expect(canonicalModelId('claude-sonnet-4-5-20250929')).toBe('claude-sonnet-4-5')
    expect(canonicalModelId('claude-sonnet-4-5-2025-09-29')).toBe('claude-sonnet-4-5')
    expect(canonicalModelId('gpt-5.2')).toBe('gpt-5.2')
  })

  it('keeps only models present for the same provider and group', () => {
    const entries = buildModelPlazaEntries([{
      id: 7,
      name: 'CC - Kiro',
      platform: 'anthropic',
      rate_multiplier: 0.12,
      models_list_config: { enabled: true, models: ['claude-sonnet-4-5-20250929', 'gpt-5.2'] },
    }], {
      'claude-sonnet-4-5': { provider: 'anthropic', input: 3, cache: 0.3, output: 15 },
      'gpt-5.2': { provider: 'openai', input: 1.75, cache: 0.175, output: 14 },
    })

    expect(entries).toEqual([expect.objectContaining({
      id: 'claude-sonnet-4-5', groupId: 7, groupName: 'CC - Kiro', rateMultiplier: 0.12,
    })])
  })
})
