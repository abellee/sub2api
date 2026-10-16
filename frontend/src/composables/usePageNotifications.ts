/**
 * 页面内通知。浏览器推送出现时也会走这里。
 * 新通知排在最下面，旧通知被顶上去，5 秒后自行关闭。
 */
import { ref } from 'vue'

export type PageNotification = {
  id: number
  title: string
  body: string
  image: string
  url: string
}

const AUTO_CLOSE_MS = 5000
const items = ref<PageNotification[]>([])
const timers = new Map<number, number>()
const seenKeys = new Set<string>()
let nextId = 1

export function showPageNotification(input: {
  title: string
  body?: string
  image?: string
  url?: string
  key?: string
}): number {
  const title = input.title.trim()
  if (!title) return 0
  const key = input.key?.trim() || ''
  if (key) {
    if (seenKeys.has(key)) return 0
    seenKeys.add(key)
    if (seenKeys.size > 200) {
      const oldest = seenKeys.values().next().value
      if (oldest) seenKeys.delete(oldest)
    }
  }
  const id = nextId++
  items.value = [...items.value, {
    id,
    title,
    body: input.body?.trim() || '',
    image: input.image?.trim() || '',
    url: input.url?.trim() || '',
  }]
  timers.set(id, window.setTimeout(() => dismissPageNotification(id), AUTO_CLOSE_MS))
  return id
}

export function dismissPageNotification(id: number) {
  const timer = timers.get(id)
  if (timer) window.clearTimeout(timer)
  timers.delete(id)
  items.value = items.value.filter((item) => item.id !== id)
}

export function usePageNotifications() {
  return { items, showPageNotification, dismissPageNotification }
}
