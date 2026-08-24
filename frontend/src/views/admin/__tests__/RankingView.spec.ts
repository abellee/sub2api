import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import RankingView from '../RankingView.vue'

const push = vi.fn()

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('vue-router', () => ({
  useRouter: () => ({ push }),
}))

describe('RankingView', () => {
  it('switches between token and cost rankings and opens user usage', async () => {
    const wrapper = mount(RankingView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DateRangePicker: true,
          UserTokenRanking: {
            name: 'UserTokenRanking',
            props: {
              metric: String,
              resultLimit: Number,
              showLimit: Boolean,
              metricOnly: Boolean,
            },
            emits: ['select-user'],
            template: '<button data-testid="ranking" @click="$emit(\'select-user\', 17, \'u@test.com\')">ranking</button>',
          },
        },
      },
    })

    const ranking = wrapper.findComponent({ name: 'UserTokenRanking' })
    expect(ranking.props('metric')).toBe('tokens')
    expect(ranking.props('resultLimit')).toBe(0)
    expect(ranking.props('showLimit')).toBe(false)
    expect(ranking.props('metricOnly')).toBe(true)

    await wrapper.findAll('[role="tab"]')[1].trigger('click')
    expect(ranking.props('metric')).toBe('cost')

    await wrapper.get('[data-testid="ranking"]').trigger('click')
    expect(push).toHaveBeenCalledWith(expect.objectContaining({
      path: '/admin/usage',
      query: expect.objectContaining({ user_id: '17' }),
    }))
  })
})
