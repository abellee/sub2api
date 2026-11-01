import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getDocumentation, getDocumentationCategories } from '../documentation'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('../client', () => ({ apiClient: { get } }))
beforeEach(() => vi.clearAllMocks())

describe('Documentation data API', () => {
  it('passes a category identifier to the article endpoint and preserves the returned data', async () => {
    const data = { revision: 3, categories: [{ id: 'custom-guides', name: 'Custom guides' }], articles: [{ id: 'guide-1', category_id: 'custom-guides', content: '# Guide', agent: { template: '{{document}}' } }] }
    get.mockResolvedValue({ data })
    expect(await getDocumentation('custom-guides')).toBe(data)
    expect(get).toHaveBeenCalledWith('/documentation', { params: { category: 'custom-guides' } })
  })

  it('returns category data without generating links or hiding empty categories', async () => {
    const categories = [{ id: 'image', name: 'Images' }, { id: 'empty', name: 'Empty category' }]
    get.mockResolvedValue({ data: categories })
    expect(await getDocumentationCategories()).toEqual(categories)
    expect(get).toHaveBeenCalledWith('/documentation/categories')
  })

  it('keeps an empty article response empty', async () => {
    const data = { revision: 3, categories: [], articles: [] }
    get.mockResolvedValue({ data })
    expect(await getDocumentation('missing')).toEqual(data)
  })
})
