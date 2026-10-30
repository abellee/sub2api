import { beforeEach, expect, it, vi } from 'vitest'
import { getUserMenu, saveUserMenu } from '../userMenu'

const client = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }))
vi.mock('../client', () => ({ apiClient: client }))
vi.mock('@/i18n', () => ({ i18n: { global: { t: (key: string) => key } } }))
beforeEach(() => vi.clearAllMocks())

it('rejects success responses that dropped visibility settings', async () => {
  client.put.mockResolvedValue({ data: { sections: [] } })
  await expect(saveUserMenu({ sections: [], hidden_items: ['/keys'] }))
    .rejects.toThrow('admin.menuManagement.visibilityNotSaved')
})

it('accepts persisted visibility regardless of order and loads it back', async () => {
  const data = { sections: [], hidden_items: ['/custom/docs', '/keys'] }
  client.put.mockResolvedValue({ data })
  expect(await saveUserMenu({ sections: [], hidden_items: ['/keys', '/custom/docs'] })).toEqual(data)
  client.get.mockResolvedValue({ data })
  expect(await getUserMenu()).toEqual(data)
  expect(client.get).toHaveBeenCalledWith('/user/menu')
})

it('supports old configurations and restoring all menus', async () => {
  client.get.mockResolvedValue({ data: { sections: [] } })
  expect(await getUserMenu(true)).toEqual({ sections: [], hidden_items: [] })
  client.put.mockResolvedValue({ data: { sections: [] } })
  await expect(saveUserMenu({ sections: [], hidden_items: [] })).resolves.toEqual({ sections: [] })
})
