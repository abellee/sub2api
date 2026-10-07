/**
 * useTaskPrompt —— 任务资格引导钩子。
 *
 * 与抽奖引导同一套节奏：查询符合可参与条件的进行中任务，弹过后按任务 ID
 * 写入 localStorage，短间隔节流避免路由切换重复请求。
 */
import { fetchTaskPrompts, type TaskPrompt } from '../api/task'
import type { LotteryClient } from '../api/lotteryClient'

const PROMPTED_PREFIX = 'task:prompted:'
const LAST_CHECK_KEY = 'task:last-check'
const RESET_KEY = 'task:prompt-reset'
const RESET_VERSION = '2'
const CHECK_INTERVAL_MS = 60 * 1000

/** 清掉已经写过的任务弹框标记，让当前任务可以再弹一次。只执行一轮。 */
function clearPromptMarks(storage: Storage) {
  if (storage.getItem(RESET_KEY) === RESET_VERSION) return
  const keys: string[] = []
  for (let i = 0; i < storage.length; i++) {
    const key = storage.key(i)
    if (key && (key.startsWith(PROMPTED_PREFIX) || key === LAST_CHECK_KEY)) keys.push(key)
  }
  for (const key of keys) storage.removeItem(key)
  storage.setItem(RESET_KEY, RESET_VERSION)
}

export interface UseTaskPrompt {
  check: () => Promise<TaskPrompt | null>
  acknowledge: (taskId: number) => void
  promptedIds: () => number[]
}

export function useTaskPrompt(client: LotteryClient): UseTaskPrompt {
  const storage: Storage | null = typeof localStorage !== 'undefined' ? localStorage : null
  if (storage) clearPromptMarks(storage)

  const promptedIds = (): number[] => {
    if (!storage) return []
    const ids: number[] = []
    for (let i = 0; i < storage.length; i++) {
      const key = storage.key(i)
      if (key && key.startsWith(PROMPTED_PREFIX)) {
        const id = Number(key.slice(PROMPTED_PREFIX.length))
        if (!Number.isNaN(id)) ids.push(id)
      }
    }
    return ids
  }

  const acknowledge = (taskId: number): void => {
    storage?.setItem(PROMPTED_PREFIX + taskId, JSON.stringify(Date.now()))
  }

  const check = async (): Promise<TaskPrompt | null> => {
    if (!storage) return null
    const lastCheck = Number(storage.getItem(LAST_CHECK_KEY) || 0)
    const prompted = promptedIds()
    if (Date.now() - lastCheck < CHECK_INTERVAL_MS) return null

    try {
      const tasks = await fetchTaskPrompts(client)
      storage.setItem(LAST_CHECK_KEY, JSON.stringify(Date.now()))
      const next = tasks.find((task) => !prompted.includes(task.id))
      return next ?? null
    } catch {
      return null
    }
  }

  return { check, acknowledge, promptedIds }
}
