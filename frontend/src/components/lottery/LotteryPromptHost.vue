<script setup lang="ts">
/**
 * LotteryPromptHost —— 全局抽奖 / 任务资格引导弹窗宿主。
 * 挂在 App.vue，登录后只挂载一次。菜单切换不会卸载它，已建立的 WebSocket 保持不动。
 *  - 主通道：WebSocket（lotteryd /v1/ws?token=）实时推送。活动或任务发布且用户符合
 *    可参与条件时立即弹窗。同一时刻只显示一个，其余排队。中奖消息优先弹出庆祝框。
 *  - 断线重连：连续尝试 5 次（间隔 3s），仍未成功则停 30s 后再来一轮，依次循环；
 *    页面失活导致定时器被节流时，依赖「重新可见/聚焦/网络恢复」事件立即补偿重连。
 *  - 回退：WS 断开期间退回轮询（路由切换 / 每 60s），双通道共用 localStorage 去重。
 */
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createDefaultLotteryClient } from '../../api/lotteryClient'
import { useLotteryPrompt } from '../../composables/useLotteryPrompt'
import { useTaskPrompt } from '../../composables/useTaskPrompt'
import type { ActivityView, WinnerRecord } from '../../api/lottery'
import type { TaskPrompt } from '../../api/task'
import { entryPopupsHeld } from '../../composables/entryPopupGate'
import { setLotterySocketStatus } from '../../composables/useLotterySocketStatus'
import { LOTTERY_WIN_EVENT, useLotteryWinPrompt } from '../../composables/useLotteryWinPrompt'
import LotteryPromptModal from './LotteryPromptModal.vue'
import LotteryWinModal from './LotteryWinModal.vue'
import TaskPromptModal from '../task/TaskPromptModal.vue'

const route = useRoute()
const router = useRouter()
const client = createDefaultLotteryClient()
const prompt = useLotteryPrompt(client)
const taskPrompt = useTaskPrompt(client)
const winPrompt = useLotteryWinPrompt(client)
const onLotteryPage = computed(() => route.name === 'Lottery')
const shownWin = computed(() => winPrompt.current.value)

type PromptItem =
  | { kind: 'lottery'; id: number; activity: ActivityView }
  | { kind: 'task'; id: number; task: TaskPrompt }

const queue = ref<PromptItem[]>([])
const current = computed(() => queue.value[0] ?? null)
const currentActivity = computed(() => (current.value?.kind === 'lottery' ? current.value.activity : null))
const currentTask = computed(() => (current.value?.kind === 'task' ? current.value.task : null))
const shownActivity = computed(() => (entryPopupsHeld.value || shownWin.value ? null : currentActivity.value))
const shownTask = computed(() => (entryPopupsHeld.value || shownWin.value ? null : currentTask.value))

function onCloseWin(id: number) {
  winPrompt.acknowledge(id)
}

function onViewWin(id: number) {
  winPrompt.acknowledge(id)
  if (route.name !== 'Lottery') void router.push('/lottery')
}

function enqueue(item: PromptItem) {
  const prompted = item.kind === 'lottery' ? prompt.promptedIds() : taskPrompt.promptedIds()
  if (prompted.includes(item.id)) return
  if (queue.value.some((queued) => queued.kind === item.kind && queued.id === item.id)) return
  queue.value = [...queue.value, item]
}

function drop(kind: PromptItem['kind'], id: number) {
  queue.value = queue.value.filter((item) => !(item.kind === kind && item.id === id))
}

async function runPromptCheck() {
  const activity = await prompt.check()
  if (activity) enqueue({ kind: 'lottery', id: activity.id, activity })
  const task = await taskPrompt.check()
  if (task) enqueue({ kind: 'task', id: task.id, task })
}

function onConfirm(activityId: number) {
  prompt.acknowledge(activityId)
  drop('lottery', activityId)
  void router.push('/lottery')
}

function onDismiss(activityId: number) {
  prompt.acknowledge(activityId)
  drop('lottery', activityId)
}

function onConfirmTask(taskId: number) {
  taskPrompt.acknowledge(taskId)
  drop('task', taskId)
  void router.push('/tasks')
}

function onDismissTask(taskId: number) {
  taskPrompt.acknowledge(taskId)
  drop('task', taskId)
}

// ---- WebSocket 实时通道 ----
const WS_STEP_MS = 3_000 // 连续重试间隔
const WS_BURST = 5 // 每轮连续尝试次数
const WS_PAUSE_MS = 30_000 // 一轮失败后的长停顿
const WS_HEARTBEAT_MS = 25_000 // 应用层心跳（页面可见时才发送）
const WS_PONG_TIMEOUT_MS = 10_000

let ws: WebSocket | null = null
let closedByUs = false
let attempt = 0 // 当前轮内已失败次数
let reconnectTimer = 0
let pollTimer = 0
let heartbeatTimer = 0
let pongCheckTimer = 0
let awaitingPong = false
let winSyncTimer = 0
const WIN_SYNC_MS = 15_000

function wsUrl(): string {
  const token = localStorage.getItem('auth_token') ?? ''
  return `${client.wsURL}/v1/ws?token=${encodeURIComponent(token)}`
}

/** 断线后的重连调度：5 次快试 → 停 30s → 再 5 次，循环；页面重新可见/网络恢复时立即补偿。 */
function scheduleReconnect() {
  if (closedByUs || reconnectTimer) return
  attempt++
  let delay = WS_STEP_MS
  if (attempt >= WS_BURST) {
    attempt = 0
    delay = WS_PAUSE_MS
  }
  reconnectTimer = window.setTimeout(() => {
    reconnectTimer = 0
    wsConnect()
  }, delay)
}

/** 立即重连（页面重新可见/聚焦/网络恢复时调用），并清掉挂起的延迟重试。 */
function reconnectNow() {
  if (closedByUs) return
  if (reconnectTimer) {
    window.clearTimeout(reconnectTimer)
    reconnectTimer = 0
  }
  if (!ws || ws.readyState !== WebSocket.OPEN) wsConnect()
}

function wsConnect() {
  if (closedByUs) return
  if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) return
  const token = localStorage.getItem('auth_token')
  if (!token) {
    // 未登录：不连 WS，走轮询兜底（登录后路由切换会再次尝试）
    setLotterySocketStatus('closed')
    schedulePolling()
    return
  }
  setLotterySocketStatus('connecting')
  try {
    ws = new WebSocket(wsUrl())
  } catch {
    ws = null
    setLotterySocketStatus('closed')
    scheduleReconnect()
    return
  }
  ws.onopen = () => {
    attempt = 0
    awaitingPong = false
    setLotterySocketStatus('open')
    stopHeartbeat()
    startHeartbeat()
    stopPolling() // WS 正常：停掉轮询，避免双通道重复请求
  }
  ws.onmessage = (ev) => {
    awaitingPong = false
    try {
      const msg = JSON.parse(ev.data) as {
        type?: string
        activity?: ActivityView
        task?: TaskPrompt
        winner?: WinnerRecord
      }
      if (msg.type === 'lottery_prompt' && msg.activity) {
        enqueue({ kind: 'lottery', id: msg.activity.id, activity: msg.activity })
      } else if (msg.type === 'task_prompt' && msg.task) {
        enqueue({ kind: 'task', id: msg.task.id, task: msg.task })
      } else if (msg.type === 'lottery_win' && msg.winner) {
        winPrompt.enqueue(msg.winner)
      }
    } catch {
      // 忽略非法消息
    }
  }
  ws.onclose = () => {
    ws = null
    stopHeartbeat()
    if (!closedByUs) {
      setLotterySocketStatus('closed')
      scheduleReconnect()
      schedulePolling() // 断线期间轮询兜底
    } else {
      setLotterySocketStatus('idle')
    }
  }
  ws.onerror = () => {
    // 统一交给 onclose 走重连调度
  }
}

function wsClose() {
  closedByUs = true
  ws?.close()
  ws = null
  stopHeartbeat()
}

/** 应用层心跳：检测死连接（协议层 ping/pong 由浏览器自动应答，服务端负责保活）。 */
function startHeartbeat() {
  stopHeartbeat()
  heartbeatTimer = window.setInterval(() => {
    if (document.hidden) return // 页面失活时不发送，也跳过超时判定，避免误断
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    if (awaitingPong) return
    try {
      ws.send(JSON.stringify({ type: 'ping' }))
      awaitingPong = true
      pongCheckTimer = window.setTimeout(() => {
        if (awaitingPong && ws) {
          ws.close() // 无响应视为死连接，触发重连
        }
      }, WS_PONG_TIMEOUT_MS)
    } catch {
      ws?.close()
    }
  }, WS_HEARTBEAT_MS)
}

function stopHeartbeat() {
  if (heartbeatTimer) {
    window.clearInterval(heartbeatTimer)
    heartbeatTimer = 0
  }
  if (pongCheckTimer) {
    window.clearTimeout(pongCheckTimer)
    pongCheckTimer = 0
  }
  awaitingPong = false
}

/** WS 未就绪时的轮询回退。 */
function schedulePolling() {
  if (pollTimer) return
  pollTimer = window.setInterval(() => {
    if (!current.value) void runPromptCheck()
  }, 60_000)
}

function stopPolling() {
  if (pollTimer) {
    window.clearInterval(pollTimer)
    pollTimer = 0
  }
}

function onVisibilityChange() {
  if (document.hidden) return
  // 页面重新可见：立即补偿重连/检查（后台标签页的定时器可能被浏览器节流）
  if (!ws || ws.readyState !== WebSocket.OPEN) reconnectNow()
  if (!current.value) void runPromptCheck()
  void winPrompt.sync()
}

function onWindowFocus() {
  reconnectNow()
  if (!current.value) void runPromptCheck()
}

function onOnline() {
  reconnectNow()
}

watch(() => winPrompt.current.value?.id, (id) => {
  if (id) window.dispatchEvent(new CustomEvent(LOTTERY_WIN_EVENT))
})

onMounted(() => {
  void runPromptCheck()
  void winPrompt.sync()
  winSyncTimer = window.setInterval(() => {
    if (!document.hidden) void winPrompt.sync()
  }, WIN_SYNC_MS)
  wsConnect()
  window.addEventListener('focus', onWindowFocus)
  window.addEventListener('online', onOnline)
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onBeforeUnmount(() => {
  closedByUs = true
  setLotterySocketStatus('idle')
  if (reconnectTimer) window.clearTimeout(reconnectTimer)
  if (winSyncTimer) window.clearInterval(winSyncTimer)
  wsClose()
  stopPolling()
  window.removeEventListener('focus', onWindowFocus)
  window.removeEventListener('online', onOnline)
  document.removeEventListener('visibilitychange', onVisibilityChange)
})

// 路由切换时：WS 已连接则无需处理（服务端实时推），断线时补偿重连 + 轮询检查
watch(() => route.path, () => {
  if (!ws || ws.readyState !== WebSocket.OPEN) {
    reconnectNow()
    if (!current.value) void runPromptCheck()
  }
})
</script>

<template>
  <LotteryWinModal
    :win="shownWin"
    :on-lottery-page="onLotteryPage"
    @close="onCloseWin"
    @view="onViewWin"
  />
  <LotteryPromptModal :activity="shownActivity" @confirm="onConfirm" @dismiss="onDismiss" />
  <TaskPromptModal :task="shownTask" @confirm="onConfirmTask" @dismiss="onDismissTask" />
</template>
