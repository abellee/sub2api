import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h, ref } from 'vue'
import { mount } from '@vue/test-utils'
import type { StudioGroupCard } from '@/features/channel-monitor-v2-studio/useStudioGroupCards'

const isV2 = vi.fn(() => true)
const groupCards = ref<StudioGroupCard[]>([])

vi.mock('@/utils/featureFlags', () => ({
  isChannelMonitorV2Mode: () => isV2(),
}))

vi.mock('@/features/channel-monitor-v2-studio/useStudioGroupCards', () => ({
  useStudioGroupCards: () => ({
    loading: ref(false),
    groupCards,
    coverage: ref(null),
    showThroughput: ref(false),
  }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
      locale: { value: 'zh' },
    }),
  }
})

import UserDashboardActiveGroups from '../UserDashboardActiveGroups.vue'

function metricCard(
  label: string,
  options: { active?: boolean; state?: StudioGroupCard['state']; rpm?: number } = {},
): StudioGroupCard {
  const state = options.state ?? (options.active === false ? 'unknown' : 'healthy')
  const active = state !== 'unknown'
  const rpm = options.rpm ?? (active ? 12 : 0)
  return {
    key: label,
    label,
    platform: 'openai',
    brandLabel: 'OpenAI',
    rateLabel: '',
    successLabel: '成功率',
    successRate: active ? '99%' : '-',
    ttftLabel: '首 TOKEN',
    ttft: active ? '200ms' : '-',
    cacheLabel: '缓存率',
    cacheRate: active ? '10%' : '-',
    statusHeading: '当前状态',
    statusLabel: state === 'healthy' ? '健康' : state === 'warning' ? '波动' : state === 'critical' ? '异常' : '样本不足',
    title: '',
    state,
    successState: state,
    ttftState: state,
    cacheState: state,
    accent: state === 'healthy' ? 'teal' : state === 'warning' ? 'amber' : state === 'critical' ? 'coral' : 'slate',
    buckets: [],
    metrics: {
      success_requests: active ? 10 : 0,
      error_requests: 0,
      request_count: active ? 10 : 0,
      token_count: active ? 100 : 0,
      rpm,
      tpm: active ? 10 : 0,
      error_rate: 0,
      cache_rate: active ? 0.1 : 0,
      cache_rate_numerator: active ? 1 : 0,
      cache_rate_denominator: active ? 10 : 0,
      ttft: { sample_count: active ? 10 : 0, p50_ms: active ? 200 : null, p95_ms: active ? 300 : null, avg_ms: active ? 220 : null },
      duration: { sample_count: active ? 10 : 0, p50_ms: active ? 400 : null, p95_ms: active ? 500 : null, avg_ms: active ? 420 : null },
    },
    health: {
      overall: state,
      error_rate: state,
      ttft: state,
      minimum_sample: 1,
    },
  }
}

const mountWidget = () =>
  mount(UserDashboardActiveGroups, {
    global: {
      stubs: {
        StudioKpiCard: defineComponent({
          name: 'StudioKpiCard',
          props: ['label', 'statusLabel'],
          setup: (props) => () => h('div', { 'data-test': 'kpi-card' }, `${props.label} ${props.statusLabel}`),
        }),
        StudioStatusFaces: true,
        Icon: true,
        RouterLink: defineComponent({
          name: 'RouterLink',
          props: ['to'],
          setup: (props, { slots }) => () =>
            h('a', { 'data-test': 'view-all', href: JSON.stringify(props.to) }, slots.default?.()),
        }),
      },
    },
  })

describe('UserDashboardActiveGroups', () => {
  beforeEach(() => {
    isV2.mockReturnValue(true)
    groupCards.value = [
      metricCard('默认组'),
      metricCard('波动组', { state: 'warning' }),
      metricCard('异常组', { state: 'critical', rpm: 80 }),
      metricCard('备用组', { active: false }),
    ]
  })

  it('hides when channel monitor V2 is off', () => {
    isV2.mockReturnValue(false)
    const wrapper = mountWidget()
    expect(wrapper.find('[data-test="dashboard-active-groups"]').exists()).toBe(false)
  })

  it('shows a single healthy card and skips 异常', () => {
    groupCards.value = [metricCard('默认组'), metricCard('异常组', { state: 'critical', rpm: 80 })]
    const wrapper = mountWidget()
    expect(wrapper.findAll('[data-test="kpi-card"]').map((card) => card.text())).toEqual(['默认组 健康'])
  })

  it('shows an odd number of 健康/波动 cards', () => {
    groupCards.value = [metricCard('默认组'), metricCard('波动组', { state: 'warning' }), metricCard('第三组')]
    const wrapper = mountWidget()
    expect(wrapper.findAll('[data-test="kpi-card"]').map((card) => card.text())).toEqual([
      '默认组 健康',
      '第三组 健康',
      '波动组 波动',
    ])
  })

  it('renders 健康/波动 cards only, with a text link to all channel statuses', () => {
    const wrapper = mountWidget()
    const section = wrapper.get('[data-test="dashboard-active-groups"]')
    expect(section.text()).toContain('channelMonitorV2.studio.groups.active')
    expect(wrapper.get('[data-test="view-all"]').text()).toBe('dashboard.viewAllChannelStatus')
    expect(wrapper.get('[data-test="view-all"]').attributes('href')).toContain('ChannelStatus')
    expect(wrapper.findAll('[data-test="kpi-card"]').map((card) => card.text())).toEqual(['默认组 健康', '波动组 波动'])
    expect(wrapper.findComponent({ name: 'StudioStatusFaces' }).exists()).toBe(false)
  })
})
