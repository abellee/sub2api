<script setup lang="ts">
/**
 * LotteryPromptHost —— 全局抽奖资格引导弹窗宿主。
 * 挂在 AppLayout 内，覆盖后台所有页面：
 *  - 主通道：WebSocket（lotteryd /v1/ws?token=）实时推送，活动创建/资格变化立即弹窗；
 *    连接携带用户凭证，由 lotteryd 经主服务 introspect 校验，校验失败服务端直接拒绝升级。
 *  - 断线重连：连续尝试 5 次（间隔 3s），仍未成功则停 30s 后再来一轮，依次循环；
 *    页面失活导致定时器被节流时，依赖「重新可见/聚焦/网络恢复」事件立即补偿重连。
 *  - 回退：WS 断开期间退回轮询（路由切换 / 每 60s），双通道共用 localStorage 去重。
 */
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createDefaultLotteryClient } from '../../api/lotteryClient'
import { useLotteryPrompt } from '../../composables/useLotteryPrompt'
import type { ActivityView } from '../../api/lottery'
import LotteryPromptModal from './LotteryPromptModal.vue'

const route = useRoute()
const router = useRouter()
const client = createDefaultLotteryClient()
const prompt = useLotteryPrompt(client)
const pendingActivity = ref<ActivityView | null>(null)

async function runPromptCheck() {
  pendingActivity.value = await prompt.check()
}

function onConfirm(activityId: number) {
  prompt.acknowledge(activityId)
  pendingActivity.value = null
  // 「去参与」：跳转到抽奖活动页
  void router.push('/lottery')
}

function onDismiss(activityId: number) {
  prompt.acknowledge(activityId)
  pendingActivity.value = null
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
    schedulePolling()
    return
  }
  try {
    ws = new WebSocket(wsUrl())
  } catch {
    ws = null
    scheduleReconnect()
    return
  }
  ws.onopen = () => {
    attempt = 0
    awaitingPong = false
    stopHeartbeat()
    startHeartbeat()
    stopPolling() // WS 正常：停掉轮询，避免双通道重复请求
  }
  ws.onmessage = (ev) => {
    awaitingPong = false
    try {
      const msg = JSON.parse(ev.data) as { type?: string; activity?: ActivityView }
      if (msg.type === 'lottery_prompt' && msg.activity && !pendingActivity.value) {
        const prompted = prompt.promptedIds()
        if (!prompted.includes(msg.activity.id)) {
          pendingActivity.value = msg.activity
        }
      }
    } catch {
      // 忽略非法消息
    }
  }
  ws.onclose = () => {
    ws = null
    stopHeartbeat()
    if (!closedByUs) {
      scheduleReconnect()
      schedulePolling() // 断线期间轮询兜底
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
    if (!pendingActivity.value) void runPromptCheck()
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
  if (!pendingActivity.value) void runPromptCheck()
}

function onWindowFocus() {
  reconnectNow()
  if (!pendingActivity.value) void runPromptCheck()
}

function onOnline() {
  reconnectNow()
}

onMounted(() => {
  void runPromptCheck()
  wsConnect()
  window.addEventListener('focus', onWindowFocus)
  window.addEventListener('online', onOnline)
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onBeforeUnmount(() => {
  closedByUs = true
  if (reconnectTimer) window.clearTimeout(reconnectTimer)
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
    if (!pendingActivity.value) void runPromptCheck()
  }
})
</script>

<template>
  <LotteryPromptModal :activity="pendingActivity" @confirm="onConfirm" @dismiss="onDismiss" />
</template>
