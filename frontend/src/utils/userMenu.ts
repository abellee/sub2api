import type { UserMenuSection } from '@/api/userMenu'

export const USER_MENU_ITEMS = [
  { path: '/dashboard', labelKey: 'nav.dashboard' },
  { path: '/keys', labelKey: 'nav.apiKeys' },
  { path: '/usage', labelKey: 'nav.usage' },
  { path: '/available-channels', labelKey: 'nav.availableChannels' },
  { path: '/monitor', labelKey: 'nav.channelStatus' },
  { path: '/subscriptions', labelKey: 'nav.mySubscriptions' },
  { path: '/purchase', labelKey: 'nav.buySubscription' },
  { path: '/orders', labelKey: 'nav.myOrders' },
  { path: '/lottery', labelKey: 'nav.lottery' },
  { path: '/tasks', labelKey: 'nav.tasks' },
  { path: '/affiliate', labelKey: 'nav.affiliate' },
  { path: '/profile', labelKey: 'nav.profile' },
  { path: '/model-plaza', labelKey: 'nav.modelPlaza' },
  { path: '/redeem', labelKey: 'nav.redeem' },
] as const

export interface UserNavSection<T> {
  id: string
  title: string
  items: T[]
}

/** Group only entries already allowed by the sidebar's feature and permission filters. */
export function groupUserMenu<T extends { path: string }>(
  items: T[], sections: UserMenuSection[], defaultTitle: string,
): UserNavSection<T>[] {
  const available = new Map(items.map(item => [item.path, item]))
  const used = new Set<string>()
  const grouped: UserNavSection<T>[] = []
  for (const section of sections) {
    const members: T[] = []
    for (const path of section.items) {
      const item = available.get(path)
      if (item && !used.has(path)) {
        used.add(path)
        members.push(item)
      }
    }
    if (members.length) grouped.push({ id: section.id, title: section.title, items: members })
  }
  const unassigned = items.filter(item => !used.has(item.path))
  if (unassigned.length) grouped.push({ id: '__unassigned__', title: defaultTitle, items: unassigned })
  return grouped
}

/** Moving an entry removes its prior assignment and appends it under the chosen heading. */
export function assignUserMenuItem(sections: UserMenuSection[], path: string, sectionId: string): void {
  for (const section of sections) section.items = section.items.filter(item => item !== path)
  sections.find(section => section.id === sectionId)?.items.push(path)
}
