/**
 * useLotteryPrompt —— 全局资格引导钩子。
 *
 * 供任意宿主应用（开发壳或并入后的 Sub2API 后台）在路由切换/进入页面时调用：
 *  1. 查询 lotteryd /v1/eligibility，拿到用户符合条件且未参与的活动；
 *  2. 弹出引导动画提示用户进入抽奖页；
 *  3. 提示过后写 localStorage（活动级），之后不再为该活动弹窗；
 *  4. 仅做短间隔节流（避免路由切换频繁请求），以便随时发现新建的活动。
 */
import { fetchEligibility, type ActivityView } from '../api/lottery'
import type { LotteryClient } from '../api/lotteryClient'

const PROMPTED_PREFIX = 'lottery:prompted:'
const LAST_CHECK_KEY = 'lottery:last-check'
const CHECK_INTERVAL_MS = 60 * 1000

export interface LotteryPromptState {
  /** 当前应弹窗引导的活动（未确认前持有）。 */
  pending: ActivityView | null
}

export interface UseLotteryPrompt {
  /** 进入页面时调用；返回需要弹窗引导的活动（可能为空）。 */
  check: () => Promise<ActivityView | null>
  /** 用户点击「去参与」或关闭弹窗后确认，写入本地记录。 */
  acknowledge: (activityId: number) => void
  promptedIds: () => number[]
}

export function useLotteryPrompt(client: LotteryClient): UseLotteryPrompt {
  const storage: Storage | null =
    typeof localStorage !== 'undefined' ? localStorage : null

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

  const acknowledge = (activityId: number): void => {
    storage?.setItem(PROMPTED_PREFIX + activityId, JSON.stringify(Date.now()))
  }

  const check = async (): Promise<ActivityView | null> => {
    if (!storage) return null
    const lastCheck = Number(storage.getItem(LAST_CHECK_KEY) || 0)
    const prompted = promptedIds()
    // 节流：短时间内路由来回切换不重复查询；除此之外每次都查，保证新建活动能被及时发现。
    if (Date.now() - lastCheck < CHECK_INTERVAL_MS) return null

    try {
      const activities = await fetchEligibility(client)
      storage.setItem(LAST_CHECK_KEY, JSON.stringify(Date.now()))
      const next = activities.find((a) => !prompted.includes(a.id))
      return next ?? null
    } catch {
      // lotteryd 不可用时静默跳过，不影响宿主页面。
      return null
    }
  }

  return { check, acknowledge, promptedIds }
}
