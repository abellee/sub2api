/**
 * 抽奖和任务菜单的显隐、角标。页面可见时每 30 秒补查，推送到达时立即更新。
 * 两个菜单默认隐藏，拿到结果后再决定是否显示。
 */
import { ref } from 'vue'
import { fetchMenuStatus, type MenuStatus } from '@/api/menuStatus'
import type { LotteryPhase } from '@/api/lottery'

const POLL_MS = 30_000

const lotteryVisible = ref(false)
const lotteryPhase = ref<LotteryPhase>('')
const taskVisible = ref(false)
const taskPhase = ref<MenuStatus['task_phase']>('none')
const menuReady = ref(false)

let timer: number | undefined
let started = false
let generation = 0
let inflight: Promise<void> | null = null
let inflightGen = -1

function applyMenuStatus(data: MenuStatus) {
  lotteryVisible.value = !!data.lottery_visible
  lotteryPhase.value = data.lottery_phase || ''
  taskVisible.value = !!data.task_visible
  taskPhase.value = data.task_phase === 'active' ? 'active' : 'none'
  menuReady.value = true
}

export function refreshMenuStatus(): Promise<void> {
  const gen = generation
  if (inflight && inflightGen === gen) return inflight
  inflightGen = gen
  const run = fetchMenuStatus()
    .then((data) => {
      if (gen !== generation) return
      applyMenuStatus(data)
    })
    .catch(() => {
      // 失败时保留上一次结果。还没成功过时菜单继续隐藏。
    })
    .finally(() => {
      if (inflight === run) inflight = null
    })
  inflight = run
  return run
}

export function startMenuStatusPolling() {
  if (started) return
  started = true
  refreshVisibleMenuStatus()
  timer = window.setInterval(refreshVisibleMenuStatus, POLL_MS)
  document.addEventListener('visibilitychange', refreshVisibleMenuStatus)
}

function refreshVisibleMenuStatus() {
  if (started && !document.hidden) void refreshMenuStatus()
}

export function stopMenuStatusPolling() {
  generation += 1
  started = false
  document.removeEventListener('visibilitychange', refreshVisibleMenuStatus)
  if (timer !== undefined) {
    window.clearInterval(timer)
    timer = undefined
  }
  lotteryVisible.value = false
  lotteryPhase.value = ''
  taskVisible.value = false
  taskPhase.value = 'none'
  menuReady.value = false
}

export function useMenuStatus() {
  return { lotteryVisible, lotteryPhase, taskVisible, taskPhase, menuReady }
}
