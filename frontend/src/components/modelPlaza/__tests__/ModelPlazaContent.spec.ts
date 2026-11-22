import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ModelPlazaContent from '../ModelPlazaContent.vue'
import PlazaFilterBar from '../PlazaFilterBar.vue'
import PlazaGroupSection from '../PlazaGroupSection.vue'
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ isAuthenticated: true })
}))

function model(name: string, billingMode: 'token' | 'image' | 'video'): PlazaModel {
  return {
    name,
    platform: 'grok',
    pricing: {
      billing_mode: billingMode,
      input_price: billingMode === 'token' ? 1e-6 : null,
      output_price: billingMode === 'token' ? 2e-6 : null,
      cache_write_price: null,
      cache_read_price: null,
      image_input_price: null,
      image_output_price: null,
      per_request_price: billingMode === 'token' ? null : 0.08,
      intervals: []
    },
    official_pricing: null
  }
}

function group(id: number, name: string, rate: number, models: PlazaModel[]): ModelPlazaGroup {
  return {
    id,
    name,
    description: '',
    platform: 'grok',
    subscription_type: 'standard',
    rate_multiplier: rate,
    peak_rate_enabled: false,
    peak_start: '',
    peak_end: '',
    peak_rate_multiplier: 1,
    is_exclusive: false,
    image_rate_independent: false,
    image_rate_multiplier: 1,
    video_rate_independent: false,
    video_rate_multiplier: 1,
    models
  }
}

describe('ModelPlazaContent defaults and ordering', () => {
  it('defaults to all providers and places media groups before cheaper token groups', () => {
    const wrapper = mount(ModelPlazaContent, {
      props: {
        loading: false,
        response: {
          description: '',
          groups: [
            group(1, 'token', 0.1, [model('grok-4.6', 'token')]),
            group(2, 'media', 1, [model('grok-imagine-image-quality', 'image')])
          ]
        }
      },
      global: {
        stubs: {
          Icon: true,
          PlazaFilterBar: true,
          PlazaGroupSection: true
        }
      }
    })

    expect(wrapper.findComponent(PlazaFilterBar).props('platform')).toBe('all')
    expect(wrapper.findAllComponents(PlazaGroupSection).map((section) => section.props('group').id))
      .toEqual([2, 1])
  })
})
