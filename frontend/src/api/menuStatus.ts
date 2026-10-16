import type { LotteryPhase } from '@/api/lottery'
import { createDefaultLotteryClient, unwrap, type LotteryClient } from '@/api/lotteryClient'

/** 侧边栏一次拿到抽奖、任务的显隐和角标。 */
export interface MenuStatus {
  lottery_visible: boolean
  lottery_phase: LotteryPhase
  task_visible: boolean
  task_phase: 'active' | 'none'
}

export async function fetchMenuStatus(client: LotteryClient = createDefaultLotteryClient()): Promise<MenuStatus> {
  return unwrap<MenuStatus>(client.http.get('/v1/me/menu'))
}
