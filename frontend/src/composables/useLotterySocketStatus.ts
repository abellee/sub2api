/**
 * 抽奖 / 任务推送 WebSocket 的连接状态。连接本身挂在 App.vue，
 * 顶栏只读这个状态，不负责建立或断开连接。
 */
import { ref } from 'vue'

export type LotterySocketStatus = 'idle' | 'connecting' | 'open' | 'closed'

const status = ref<LotterySocketStatus>('idle')

export function setLotterySocketStatus(next: LotterySocketStatus) {
  status.value = next
}

export function useLotterySocketStatus() {
  return { status }
}
