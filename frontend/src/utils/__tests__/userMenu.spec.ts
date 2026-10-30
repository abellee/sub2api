import { describe, expect, it } from 'vitest'
import { assignUserMenuItem, groupUserMenu } from '../userMenu'
import type { UserMenuSection } from '@/api/userMenu'

const items = [
  { path: '/dashboard', label: 'Dashboard' },
  { path: '/keys', label: 'Keys', badge: { text: 'New' } },
  { path: '/usage', label: 'Usage' },
  { path: '/profile', label: 'Profile' },
  { path: '/custom/docs', label: 'Docs', iconSvg: '<svg />' },
]

describe('groupUserMenu', () => {
  it('hides assigned and default entries without changing their saved placement', () => {
    const sections = [{ id: 'services', title: 'Services', items: ['/keys'] }]
    const hidden = ['/keys', '/custom/docs', '/dashboard']
    const result = groupUserMenu(items, sections, 'My Account', hidden)
    expect(result.map(section => section.id)).toEqual(['__unassigned__'])
    expect(result[0].items.map(item => item.path)).toEqual(['/usage', '/profile'])
    expect(groupUserMenu(items, sections, 'My Account')[0].items).toEqual([items[1]])
    expect(groupUserMenu(items, sections, 'My Account', items.map(item => item.path))).toEqual([])
  })
  it('preserves the original menu order in the default section without configuration', () => {
    expect(groupUserMenu(items, [], 'My Account')).toEqual([
      { id: '__unassigned__', title: 'My Account', items },
    ])
  })

  it('honors heading and item order, preserves metadata, and retains unassigned entries', () => {
    const sections: UserMenuSection[] = [
      { id: 'services', title: 'Services', items: ['/custom/docs', '/keys'] },
      { id: 'account', title: 'Account', items: ['/profile', '/usage'] },
    ]
    const result = groupUserMenu(items, sections, 'My Account')
    expect(result.map(section => section.title)).toEqual(['Services', 'Account', 'My Account'])
    expect(result[0].items.map(item => item.path)).toEqual(['/custom/docs', '/keys'])
    expect(result[0].items[1]).toBe(items[1])
    expect(result[2].items).toEqual([items[0]])
  })

  it('never resurrects filtered menus and omits empty headings and deleted custom pages', () => {
    const result = groupUserMenu(items.filter(item => item.path !== '/usage'), [
      { id: 'hidden', title: 'Hidden', items: ['/usage', '/admin/users', '/custom/deleted'] },
      { id: 'empty', title: 'Empty', items: [] },
      { id: 'visible', title: 'Visible', items: ['/keys'] },
    ], 'My Account')
    expect(result.map(section => section.id)).toEqual(['visible', '__unassigned__'])
    expect(result.flatMap(section => section.items).map(item => item.path)).not.toContain('/usage')
  })

  it('avoids duplicate links even if an older configuration repeats assignments', () => {
    const result = groupUserMenu(items, [
      { id: 'one', title: 'One', items: ['/keys', '/keys'] },
      { id: 'two', title: 'Two', items: ['/keys'] },
    ], 'My Account')
    expect(result.map(section => section.id)).toEqual(['one', '__unassigned__'])
    expect(result.flatMap(section => section.items).filter(item => item.path === '/keys')).toHaveLength(1)
  })

  it('moves assignments between headings and restores the default when unassigned', () => {
    const sections: UserMenuSection[] = [
      { id: 'one', title: 'One', items: ['/keys', '/usage'] },
      { id: 'two', title: 'Two', items: ['/profile'] },
    ]
    assignUserMenuItem(sections, '/keys', 'two')
    expect(sections.map(section => section.items)).toEqual([['/usage'], ['/profile', '/keys']])
    assignUserMenuItem(sections, '/keys', '')
    expect(sections.map(section => section.items)).toEqual([['/usage'], ['/profile']])
    expect(groupUserMenu(items, sections, 'My Account').at(-1)?.items).toContain(items[1])
  })
})
