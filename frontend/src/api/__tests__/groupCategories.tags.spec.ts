import { beforeEach, describe, expect, it, vi } from 'vitest'
import { list, setGroupTags } from '../admin/groupCategories'
import { getCategories } from '../groups'

const { get, put } = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }))
vi.mock('../client', () => ({ apiClient: { get, put } }))

describe('group tag API', () => {
  beforeEach(() => vi.resetAllMocks())

  it.each([list, getCategories])('preserves tags when loading group configuration', async load => {
    const tags = { '42': [{ name: '快速', description: '说明' }] }
    get.mockResolvedValue({ data: { categories: [], assignments: {}, group_caps: { '42': 0.8 }, group_tags: tags } })
    expect(await load()).toEqual({ categories: [], assignments: {}, group_caps: { '42': 0.8 }, group_tags: tags })
  })

  it.each([list, getCategories])('accepts existing configurations without tags', async load => {
    get.mockResolvedValue({ data: { categories: [], assignments: {} } })
    expect((await load()).group_tags).toEqual({})
  })

  it('sends updates and explicit clearing to the group tag endpoint', async () => {
    const tags = [{ name: '快速', description: '说明' }]
    await setGroupTags(42, tags)
    expect(put).toHaveBeenLastCalledWith('/admin/group-categories/group-tags/42', { tags })
    await setGroupTags(42, [])
    expect(put).toHaveBeenLastCalledWith('/admin/group-categories/group-tags/42', { tags: [] })
  })
})
