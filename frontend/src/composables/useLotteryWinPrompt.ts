/**
 * 中奖弹框队列。
 * 第一次拿到中奖记录时，只记住较早的记录，避免把历史中奖全部弹出来。
 * 近 15 分钟内的新中奖，以及之后出现的中奖，会进入队列，关掉后不再弹。
 */
import { computed, ref } from 'vue'
import { fetchMyWinnings, type WinnerRecord } from '@/api/lottery'
import type { LotteryClient } from '@/api/lotteryClient'
import { useAuthStore } from '@/stores/auth'

export const LOTTERY_WIN_EVENT = 'lottery-win'
const RECENT_WIN_MS = 15 * 60 * 1000

function storageKey(userId: number) {
  return `lottery_win_popup_${userId}`
}

function readSeen(userId: number): number[] | null {
  const raw = localStorage.getItem(storageKey(userId))
  if (raw == null) return null
  try {
    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return null
    return parsed.filter((id): id is number => typeof id === 'number')
  } catch {
    return null
  }
}

function writeSeen(userId: number, ids: number[]) {
  localStorage.setItem(storageKey(userId), JSON.stringify(ids))
}

function isRecentWin(createdAt: string, now: number) {
  const created = Date.parse(createdAt)
  if (Number.isNaN(created)) return false
  const age = now - created
  return age <= RECENT_WIN_MS && age >= -60_000
}

export function useLotteryWinPrompt(client: LotteryClient) {
  const auth = useAuthStore()
  const queue = ref<WinnerRecord[]>([])
  const current = computed(() => queue.value[0] ?? null)
  let syncing = false

  function enqueue(win: WinnerRecord) {
    if (!win?.id) return
    const userId = auth.user?.id
    if (userId) {
      const seen = readSeen(userId)
      if (seen?.includes(win.id)) return
    }
    if (queue.value.some((item) => item.id === win.id)) return
    queue.value = [...queue.value, win]
  }

  function acknowledge(id: number) {
    queue.value = queue.value.filter((item) => item.id !== id)
    const userId = auth.user?.id
    if (!userId) return
    const seen = new Set(readSeen(userId) ?? [])
    seen.add(id)
    writeSeen(userId, [...seen])
  }

  async function sync() {
    const userId = auth.user?.id
    if (!userId || syncing) return
    syncing = true
    try {
      const wins = await fetchMyWinnings(client)
      if (auth.user?.id !== userId) return
      const seen = readSeen(userId)
      if (seen == null) {
        const now = Date.now()
        const baseline: number[] = []
        const recent: WinnerRecord[] = []
        for (const win of wins) {
          if (isRecentWin(win.created_at, now)) recent.push(win)
          else baseline.push(win.id)
        }
        writeSeen(userId, baseline)
        for (const win of [...recent].reverse()) enqueue(win)
        return
      }
      const known = new Set(seen)
      for (const win of [...wins].reverse()) {
        if (!known.has(win.id)) enqueue(win)
      }
    } catch {
      // 失败不写入已读，下次再判断。
    } finally {
      syncing = false
    }
  }

  return { current, enqueue, acknowledge, sync }
}
