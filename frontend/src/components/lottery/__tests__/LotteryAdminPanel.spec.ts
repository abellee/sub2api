import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import LotteryAdminPanel from '../LotteryAdminPanel.vue'
import type { LotteryClient } from '@/api/lotteryClient'

const envelope = (data: unknown) => Promise.resolve({ data: { code: 0, data } })
const activity = {
  id: 10, name: '测试抽奖', description: '', starts_at: '2026-10-18T00:00:00Z', draws_at: '2026-10-19T00:00:00Z',
  max_participants: 0, show_participant_count: true, participant_count: 1, condition_match: 'all', phase: 'joining', status: 'active',
  conditions: [], auto_bonus_percent: 0, blacklist: ['old@test.com'],
  prizes: [{ id: 1, name: '余额奖', prize_type: 'balance', value: 1, stock: 1, weight: 1 }]
}

function setup() {
  let removed = false
  const get = vi.fn((path: string) => {
    if (path.endsWith('/participants')) return envelope({ participants: removed ? [] : [{ id: 1, user_id: 7, email: 'user@test.com', weight: 1 }] })
    if (path === '/v1/admin/activities') return envelope({ activities: [activity] })
    if (path === '/v1/admin/daily-configs') return envelope({ configs: [] })
    return envelope({ activity, winners: [] })
  })
  const put = vi.fn(() => envelope({ id: 10 }))
  const del = vi.fn(() => { removed = true; return envelope({ removed: true }) })
  const client = { http: { get, put, delete: del }, baseURL: '/lotteryd', wsURL: '' } as unknown as LotteryClient
  const wrapper = mount(LotteryAdminPanel, { props: { client }, global: { stubs: { Teleport: true } } })
  return { wrapper, get, put, del }
}

afterEach(() => { document.body.style.overflow = ''; vi.useRealTimers() })

describe('LotteryAdminPanel moderation', () => {
  it('searches, adds, deduplicates and removes blacklist users before saving', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    const { wrapper, get, put } = setup()
    await flushPromises()
    await wrapper.findAll('button').find((b) => b.text() === '编辑活动')!.trigger('click')
    await flushPromises()
    expect(wrapper.get('[aria-label="已添加的黑名单用户"]').text()).toContain('old@test.com')
    expect(wrapper.text()).toContain('满足参与条件时可正常报名，开奖时不会中奖')
    get.mockImplementation((path: string) => {
      if (path === '/v1/admin/users/search') return envelope({ users: [{ id: 2, email: ' A@TEST.COM ', username: 'Alice' }] })
      if (path === '/v1/admin/activities') return envelope({ activities: [activity] })
      return envelope({ configs: [], participants: [] })
    })
    await wrapper.get('[aria-label="搜索黑名单用户"]').setValue('Alice')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(get).toHaveBeenCalledWith('/v1/admin/users/search', { params: { q: 'Alice', limit: 20 } })
    expect(wrapper.text()).toContain('Alice')
    await wrapper.get('[aria-label="添加到黑名单  A@TEST.COM "]').trigger('click')
    expect(wrapper.get('[aria-label="已添加的黑名单用户"]').text()).toContain('a@test.com')
    await wrapper.get('[aria-label="搜索黑名单用户"]').setValue('Alice')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(wrapper.get('[aria-label="已添加  A@TEST.COM "]').attributes('disabled')).toBeDefined()
    await wrapper.get('[aria-label="移出黑名单 old@test.com"]').trigger('click')
    await wrapper.findAll('button').find((b) => b.text() === '保存')!.trigger('click')
    await flushPromises()
    expect(put).toHaveBeenCalledWith('/v1/admin/activities/10', expect.objectContaining({ blacklist: ['a@test.com'] }))
    wrapper.unmount()
  })

  it('ignores stale search results and reports empty results and failures', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    const { wrapper, get } = setup()
    await flushPromises()
    await wrapper.findAll('button').find((b) => b.text() === '编辑活动')!.trigger('click')
    await flushPromises()
    let resolveOld!: (value: Awaited<ReturnType<typeof envelope>>) => void
    get.mockImplementation(() => new Promise((resolve) => { resolveOld = resolve }))
    await wrapper.get('[aria-label="搜索黑名单用户"]').setValue('old')
    await vi.advanceTimersByTimeAsync(300)
    await wrapper.get('[aria-label="搜索黑名单用户"]').setValue('')
    resolveOld({ data: { code: 0, data: { users: [{ id: 2, email: 'stale@test.com', username: 'stale' }] } } })
    await flushPromises()
    expect(wrapper.find('[aria-label="添加到黑名单 stale@test.com"]').exists()).toBe(false)
    get.mockImplementation(() => envelope({ users: [] }))
    await wrapper.get('[aria-label="搜索黑名单用户"]').setValue('unknown')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(wrapper.text()).toContain('未找到匹配的用户')
    get.mockRejectedValueOnce(new Error('网络错误'))
    await wrapper.get('[aria-label="搜索黑名单用户"]').setValue('error')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(wrapper.text()).toContain('搜索失败：网络错误')
    expect(wrapper.get('[aria-label="已添加的黑名单用户"]').text()).toContain('old@test.com')
    wrapper.unmount()
  })

  it('confirms removal and refreshes the participant list', async () => {
    const { wrapper, del } = setup()
    await flushPromises()
    await wrapper.findAll('button').find((b) => b.text() === '参与名单')!.trigger('click')
    await flushPromises()
    const dialog = wrapper.get('[role="dialog"][aria-label="参与名单"]')
    expect(dialog.text()).toContain('user@test.com')
    await dialog.findAll('button').find((b) => b.text() === '踢出')!.trigger('click')
    expect(del).not.toHaveBeenCalled()
    await wrapper.findAll('.btn-danger').find((b) => b.text() === '踢出')!.trigger('click')
    await flushPromises()
    expect(del).toHaveBeenCalledWith('/v1/admin/activities/10/participants/7')
    expect(wrapper.text()).toContain('已踢出 user@test.com')
    expect(wrapper.get('[role="dialog"][aria-label="参与名单"]').text()).toContain('还没有人参与')
    wrapper.unmount()
  })
})
