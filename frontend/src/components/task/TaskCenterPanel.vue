<script setup lang="ts">
/** 用户侧任务中心：进行中任务卡片 + 我的奖励（含兑换码）。 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { LotteryClient } from '@/api/lotteryClient'
import { formatValue } from '@/api/lottery'
import { fetchMyTaskRewards, listTasks, type TaskRepeatPolicy, type TaskReward, type TaskView } from '@/api/task'
import { refreshMenuStatus, useMenuStatus } from '@/composables/useMenuStatus'
import { sanitizeSvg } from '@/utils/sanitize'
import { addCalendarDays, beijingToday } from '@/utils/beijingTime'

const props = defineProps<{ client: LotteryClient }>()

const tasks = ref<TaskView[]>([])
const rewards = ref<TaskReward[]>([])
const { taskVisible, menuReady } = useMenuStatus()
const showTaskBody = computed(() => !menuReady.value || taskVisible.value)
const loading = ref(true)
const error = ref('')
const copiedCode = ref('')

const hasActiveTask = computed(() => tasks.value.length > 0)

const sortedRewards = computed(() =>
  [...rewards.value].sort((a, b) => (b.created_at ?? '').localeCompare(a.created_at ?? ''))
)

function fmtDate(s: string): string {
  return s ? s.slice(0, 10).replace(/-/g, '/') : '—'
}

/** 最后一天仍在进行，不能写成剩 0 天。已结束的任务不展示剩余。 */
function remainLabel(t: TaskView): string {
  if (t.status !== 'active') return ''
  const today = beijingToday()
  const end = (t.end_date ?? '').slice(0, 10)
  if (!end || end < today) return ''
  if (end === today) return '最后一天'
  const left = t.days_left ?? 0
  return left > 0 ? `剩 ${left} 天` : '最后一天'
}

/** 进行中且今天落在任务周期内时，展示今天这份消耗的结算时刻。 */
function settlesTodayUsage(t: TaskView): boolean {
  if (t.status !== 'active' || !t.settle_time) return false
  const today = beijingToday()
  const start = (t.start_date ?? '').slice(0, 10)
  const end = (t.end_date ?? '').slice(0, 10)
  return Boolean(start && end && today >= start && today <= end)
}

/** 当天消耗的结算时刻：次日 settle_time（北京时间）。 */
function settleMomentText(t: TaskView): string {
  const time = t.settle_time || ''
  if (settlesTodayUsage(t)) return `${fmtDate(addCalendarDays(beijingToday(), 1))} ${time}`
  return time ? `次日 ${time}` : '—'
}

const repeatPolicyLabel: Record<TaskRepeatPolicy, string> = {
  unlimited: '不限（每日达标即发）',
  join_once: '限参与一次',
  achieved_once: '达成后不能再参与'
}

function tokensM(n: number | undefined): string {
  const m = (n ?? 0) / 1e6
  if (!Number.isFinite(m)) return '0'
  const rounded = Math.round(m * 10) / 10
  return Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1)
}

function progressPercent(t: TaskView): number {
  const cap = t.progress_cap ?? 0
  if (cap <= 0) return 0
  const value = ((t.progress_tokens ?? 0) / cap) * 100
  if (!Number.isFinite(value) || value < 0) return 0
  return Math.min(100, value)
}

function stageRewardText(t: TaskView): string {
  const units = t.progress_units && t.progress_units > 0 ? t.progress_units : 1
  if (t.reward_type === 'balance') return `余额 ${formatValue(t.reward_value * units)}`
  return `兑换码 ×${units}`
}

// 进度条颜色：管理端配置值按后端同款规则（#RGB / #RRGGBB）校验，非法或未配置时保持默认渐变。
const HEX_COLOR = /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/

function barStyle(t: TaskView): Record<string, string> {
  const style: Record<string, string> = { width: (shownPercent.value[t.id] ?? 0) + '%' }
  const color = t.progress_color ?? ''
  if (HEX_COLOR.test(color)) {
    style['--progress-color'] = color
    style['--progress-color-soft'] = `color-mix(in srgb, ${color} 70%, #ffffff)`
  }
  return style
}

/** 进度条小动画：管理端上传的 SVG 源码，清洗后按任务 ID 缓存，避免每次渲染重复过 DOMPurify。 */
const progressSvg = computed<Record<number, string>>(() => {
  const map: Record<number, string> = {}
  for (const task of tasks.value) {
    if (task.progress_svg) map[task.id] = sanitizeSvg(task.progress_svg)
  }
  return map
})

const PROGRESS_REFRESH_MS = 10 * 60 * 1000
const PROGRESS_GROW_EPSILON = 0.05

/** 当前展示宽度。进入页面时直接等于结果，不从 0 过渡。 */
const shownPercent = ref<Record<number, number>>({})
/** 仅停留刷新且百分比上升时为 true，用来打开宽度过渡。 */
const progressAnimate = ref<Record<number, boolean>>({})

let progressEpoch = 0
let progressTimer: number | undefined
let panelAlive = true

/**
 * 写入进度宽度。animateGrowth 为 false 时立刻停在结果上。
 * 为 true 时，只有比上次展示值更高才在下一帧拉长；持平、下降或新出现的任务直接跳到结果。
 */
function applyShownProgress(list: TaskView[], animateGrowth: boolean, epoch: number): Promise<void> {
  const previous = shownPercent.value
  const nextShown: Record<number, number> = {}
  const animateFlags: Record<number, boolean> = {}
  const grown: { id: number; value: number }[] = []
  for (const task of list) {
    const next = progressPercent(task)
    const old = previous[task.id]
    if (animateGrowth && old !== undefined && next > old + PROGRESS_GROW_EPSILON) {
      nextShown[task.id] = old
      animateFlags[task.id] = true
      grown.push({ id: task.id, value: next })
    } else {
      nextShown[task.id] = next
      animateFlags[task.id] = false
    }
  }
  shownPercent.value = nextShown
  progressAnimate.value = animateFlags
  if (grown.length === 0) return Promise.resolve()
  return nextTick().then(() => new Promise((resolve) => {
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        if (!panelAlive || epoch !== progressEpoch) {
          resolve()
          return
        }
        const filled = { ...shownPercent.value }
        for (const item of grown) filled[item.id] = item.value
        shownPercent.value = filled
        resolve()
      })
    })
  }))
}

function copyCode(code: string) {
  void navigator.clipboard?.writeText(code)
  copiedCode.value = code
  setTimeout(() => { copiedCode.value = '' }, 1500)
}

// 封面灯箱：点击查看大图，点击遮罩 / ✕ / Esc 关闭
const lightboxSrc = ref('')
const lightboxOpen = ref(false)

function openLightbox(src: string) {
  if (!src) return
  lightboxSrc.value = src
  lightboxOpen.value = true
}

function closeLightbox() {
  lightboxOpen.value = false
}

function onLightboxKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') closeLightbox()
}

watch(lightboxOpen, (open) => {
  document.body.style.overflow = open ? 'hidden' : ''
  if (open) window.addEventListener('keydown', onLightboxKeydown)
  else window.removeEventListener('keydown', onLightboxKeydown)
})

onBeforeUnmount(() => {
  panelAlive = false
  if (progressTimer !== undefined) window.clearInterval(progressTimer)
  window.removeEventListener('keydown', onLightboxKeydown)
  document.body.style.overflow = ''
})

async function loadTaskCenter(animateGrowth: boolean) {
  const epoch = ++progressEpoch
  // 菜单可见范围由全局轮询更新，不挡任务列表和进度。
  void refreshMenuStatus()
  const [t, r] = await Promise.all([
    listTasks(props.client),
    fetchMyTaskRewards(props.client)
  ])
  if (!panelAlive || epoch !== progressEpoch) return
  rewards.value = r
  const paint = applyShownProgress(t, animateGrowth, epoch)
  tasks.value = t
  if (animateGrowth) await paint
}

async function refresh() {
  loading.value = true
  error.value = ''
  try {
    await loadTaskCenter(false)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

async function refreshWhileStaying() {
  try {
    await loadTaskCenter(true)
    if (panelAlive) error.value = ''
  } catch {
    // 停留期间刷新失败时保留当前卡片和进度。
  }
}

onMounted(() => {
  void refresh()
  progressTimer = window.setInterval(() => {
    void refreshWhileStaying()
  }, PROGRESS_REFRESH_MS)
})
</script>

<template>
  <div class="mx-auto w-full max-w-4xl space-y-5">
    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400">
      {{ error }}
    </p>
    <p v-if="loading" class="py-8 text-center text-sm text-gray-400">加载中…</p>

    <section v-if="!loading && menuReady && !taskVisible" class="card">
      <div class="flex flex-col items-center gap-2 px-6 py-12 text-center">
        <span class="flex h-14 w-14 items-center justify-center rounded-full bg-gray-100 text-2xl dark:bg-dark-700">🎯</span>
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">任务中心菜单对当前账号未开放</h3>
        <p class="text-sm text-gray-500 dark:text-dark-400">这由任务管理右上角设置里的菜单可见范围决定。调整后，这里会显示你能看到的任务</p>
      </div>
    </section>

    <!-- 任务卡片：进行中与已结束都展示，已归档由服务端不下发 -->
    <template v-if="showTaskBody">
    <section v-for="t in tasks" :key="t.id" class="relative card">
      <span
        class="absolute right-4 top-4 z-10 inline-flex items-center gap-1.5 rounded-full bg-primary-100 px-3 py-1 text-xs font-semibold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300"
      >
        <span
          class="h-1.5 w-1.5 rounded-full"
          :class="t.status === 'active' ? 'animate-pulse bg-primary-500' : 'bg-gray-400'"
        ></span>
        {{ t.status === 'active' ? '进行中' : '已结束' }}
      </span>
      <div class="card-body flex flex-col gap-4 sm:flex-row">
        <!-- 左侧封面：高度跟随右侧内容（窄屏时固定高度置顶）；点击查看大图 -->
        <div class="flex-none sm:w-44">
          <img
            v-if="t.cover"
            :src="t.cover"
            alt=""
            class="h-40 w-full cursor-zoom-in rounded-2xl object-cover ring-1 ring-gray-200 transition-opacity hover:opacity-90 sm:h-full sm:w-full dark:ring-dark-700"
            title="点击查看大图"
            @click="openLightbox(t.cover)"
          />
          <span
            v-else
            class="flex h-40 w-full items-center justify-center rounded-2xl bg-gradient-primary text-3xl sm:h-full sm:w-full"
          >🎯</span>
        </div>

        <!-- 右侧任务信息 -->
        <div class="min-w-0 flex-1 space-y-3">
          <div>
            <h2 class="truncate text-lg font-bold text-gray-900 dark:text-white">{{ t.name }}</h2>
          </div>

          <!-- 分组 / 模型 / 剩余天数标签 -->
          <div class="flex flex-wrap items-center gap-2">
            <span class="inline-flex items-center gap-1.5 rounded-full bg-primary-100 px-3 py-1 text-xs font-semibold text-primary-700 dark:bg-primary-500/15 dark:text-primary-300">
              📁 {{ t.group_name ? `限定分组：${t.group_name}` : '不限分组' }}
            </span>
            <span
              class="inline-flex items-center gap-1.5 rounded-full bg-violet-100 px-3 py-1 text-xs font-semibold text-violet-700 dark:bg-violet-500/15 dark:text-violet-300"
            >🤖 {{ t.model ? `限定模型：${t.model}` : '不限模型' }}</span>
            <span
              v-if="remainLabel(t)"
              class="inline-flex items-center gap-1.5 rounded-full bg-amber-100 px-3 py-1 text-xs font-semibold text-amber-700 dark:bg-amber-500/15 dark:text-amber-300"
            >
              ⏳ {{ remainLabel(t) }}
            </span>
          </div>

          <!-- 任务说明 -->
          <p v-if="t.description" class="whitespace-pre-line text-sm leading-relaxed text-gray-600 dark:text-gray-300">{{ t.description }}</p>

          <dl class="grid grid-cols-2 gap-x-3 gap-y-2 sm:grid-cols-3">
            <div>
              <dt class="text-xs text-gray-400 dark:text-dark-500">任务周期</dt>
              <dd class="mt-0.5 text-sm font-medium text-gray-900 dark:text-gray-100">{{ fmtDate(t.start_date) }} ~ {{ fmtDate(t.end_date ?? '') }}</dd>
            </div>
            <div>
              <dt class="text-xs text-gray-400 dark:text-dark-500">今日消耗结算时间</dt>
              <dd class="mt-0.5 text-sm font-medium text-gray-900 dark:text-gray-100">{{ settleMomentText(t) }}</dd>
            </div>
            <div>
              <dt class="text-xs text-gray-400 dark:text-dark-500">达成条件</dt>
              <dd class="mt-0.5 text-sm font-medium text-gray-900 dark:text-gray-100">每结算日 {{ tokensM(t.threshold_tokens) }}M</dd>
            </div>
            <div>
              <dt class="text-xs text-gray-400 dark:text-dark-500">任务奖励</dt>
              <dd class="mt-0.5 text-sm font-medium text-gray-900 dark:text-gray-100">
                每 {{ tokensM(t.threshold_tokens) }}M → {{ t.reward_type === 'balance' ? '余额' : '兑换码' }} {{ formatValue(t.reward_value) }}
              </dd>
            </div>
            <div>
              <dt class="text-xs text-gray-400 dark:text-dark-500">重复参与策略</dt>
              <dd class="mt-0.5 text-sm font-medium text-gray-900 dark:text-gray-100">{{ repeatPolicyLabel[t.repeat_policy] || '不限' }}</dd>
            </div>
            <div>
              <dt class="text-xs text-gray-400 dark:text-dark-500">参与条件</dt>
              <dd class="mt-0.5 text-sm font-medium text-gray-900 dark:text-gray-100">自动参与，无需报名</dd>
            </div>
          </dl>

          <template v-if="t.status === 'active'">
          <div v-if="t.progress_ready === false" class="text-xs text-gray-400 dark:text-dark-500">今日进度暂时无法获取</div>
          <div v-else :class="progressSvg[t.id] ? 'pt-8' : ''">
            <p class="relative z-0 mb-1.5 text-right text-xs text-gray-500 dark:text-dark-400">
              今日进度 · 第 {{ t.progress_units || 1 }} 阶段
            </p>
            <div class="relative h-2.5">
              <div class="h-2.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                <div
                  class="task-stage-progress h-full rounded-full"
                  :class="{ 'task-stage-progress-grow': progressAnimate[t.id] }"
                  :style="barStyle(t)"
                ></div>
              </div>
              <!-- 底部与进度条对齐。层级高于两侧文字，跟着进度头部移动。 -->
              <span
                v-if="progressSvg[t.id]"
                class="task-stage-marker absolute bottom-0 z-30 h-16 w-16 -translate-x-1/2"
                :class="{ 'task-stage-marker-grow': progressAnimate[t.id] }"
                :style="{ left: (shownPercent[t.id] ?? 0) + '%' }"
                v-html="progressSvg[t.id]"
              ></span>
            </div>
            <div class="relative z-0 mt-1.5 flex items-center justify-between gap-3 text-xs">
              <p class="text-gray-500 dark:text-dark-400">达成本阶段可获得 {{ stageRewardText(t) }}</p>
              <p class="shrink-0 font-medium text-gray-700 dark:text-gray-200">
                {{ tokensM(t.progress_tokens) }}M / {{ tokensM(t.progress_cap || t.threshold_tokens) }}M
              </p>
            </div>
          </div>
          </template>
        </div>
      </div>
    </section>
    </template>

    <!-- 暂无任务 -->
    <section v-if="!loading && showTaskBody && !hasActiveTask" class="card">
      <div class="flex flex-col items-center gap-2 px-6 py-12 text-center">
        <span class="flex h-14 w-14 items-center justify-center rounded-full bg-primary-100 text-2xl dark:bg-primary-500/15">🎯</span>
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">暂无任务</h3>
        <p class="text-sm text-gray-500 dark:text-dark-400">有新任务时会在这里展示，敬请期待</p>
      </div>
    </section>

    <!-- 我的奖励 -->
    <section v-if="!loading && sortedRewards.length" class="card">
      <div class="card-header">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">我的奖励</h2>
      </div>
      <div class="card-body space-y-3">
        <div
          v-for="r in sortedRewards"
          :key="r.id"
          class="rounded-xl border border-gray-100 px-4 py-3 dark:border-dark-700"
        >
          <div class="flex items-center justify-between gap-2">
            <p class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ r.task_name || '任务奖励' }}</p>
            <span
              :class="['rounded-full px-2.5 py-0.5 text-xs font-semibold',
                       r.fulfillment === 'done'
                         ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
                         : r.fulfillment === 'failed'
                           ? 'bg-red-100 text-red-600 dark:bg-red-500/15 dark:text-red-300'
                           : 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300']"
            >
              {{ r.fulfillment === 'done' ? '已发放' : r.fulfillment === 'failed' ? '发放失败' : '发放中' }}
            </span>
          </div>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
            结算日 {{ r.settle_date }} · 消耗 {{ (r.tokens / 1e6).toFixed(1) }}M · 达成 {{ r.units }} 单位 ·
            {{ r.reward_type === 'balance' ? '余额 ' + formatValue(r.reward_value * r.units) : '兑换码 ×' + r.units }}
          </p>
          <div v-if="r.codes?.length" class="mt-2 flex flex-wrap gap-2">
            <button
              v-for="code in r.codes"
              :key="code"
              class="inline-flex items-center gap-2 rounded-lg bg-gray-100 px-3 py-1.5 font-mono text-xs text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600"
              :title="copiedCode === code ? '已复制' : '点击复制'"
              @click="copyCode(code)"
            >
              {{ code }}
              <span class="text-[10px] text-gray-400">{{ copiedCode === code ? '✓ 已复制' : '复制' }}</span>
            </button>
          </div>
        </div>
      </div>
    </section>
    <!-- 封面灯箱 -->
    <Teleport to="body">
      <div
        v-if="lightboxOpen"
        class="fixed inset-0 z-[2000] flex items-center justify-center bg-gray-900/85 p-8 backdrop-blur-sm"
        @click="closeLightbox"
      >
        <button
          class="absolute right-5 top-5 flex h-10 w-10 items-center justify-center rounded-full bg-white/10 text-xl text-white transition-colors hover:bg-white/20"
          aria-label="关闭"
          @click.stop="closeLightbox"
        >✕</button>
        <img
          :src="lightboxSrc"
          alt=""
          class="max-h-[90vh] max-w-[90vw] rounded-2xl object-contain shadow-glass"
          @click.stop
        />
      </div>
    </Teleport>
  </div>
</template>

<style>
.task-stage-progress {
  background-image: linear-gradient(
    90deg,
    var(--progress-color, #6366f1) 0%,
    var(--progress-color-soft, #a5b4fc) 25%,
    var(--progress-color, #6366f1) 50%,
    var(--progress-color-soft, #a5b4fc) 75%,
    var(--progress-color, #6366f1) 100%
  );
  background-size: 200% 100%;
  animation: task-stage-progress-flow 2.4s linear infinite;
}

@keyframes task-stage-progress-flow {
  from {
    background-position: 0% 0;
  }
  to {
    background-position: -200% 0;
  }
}

.task-stage-marker {
  z-index: 30;
  line-height: 0;
  pointer-events: none;
}

/* 动画铺满 64px 方框。viewBox 仍按自身比例放进这个方框，不另做拉伸。 */
.task-stage-marker > svg {
  display: block;
  height: 100%;
  width: 100%;
}

.task-stage-marker-grow {
  transition: left 0.9s cubic-bezier(0.22, 1, 0.36, 1);
}

.task-stage-progress-grow {
  transition: width 0.9s cubic-bezier(0.22, 1, 0.36, 1);
}

@media (prefers-reduced-motion: reduce) {
  .task-stage-progress {
    animation: none;
  }
  .task-stage-progress-grow,
  .task-stage-marker-grow {
    transition: none;
  }
}
</style>
