<script setup lang="ts">
/** 用户侧任务中心：进行中任务卡片 + 我的奖励（含兑换码）。 */
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { LotteryClient } from '@/api/lotteryClient'
import { formatValue } from '@/api/lottery'
import { fetchMyTaskRewards, fetchTaskPhase, fetchTaskVisibility, listTasks, type TaskReward, type TaskView } from '@/api/task'

const props = defineProps<{ client: LotteryClient }>()

const tasks = ref<TaskView[]>([])
const rewards = ref<TaskReward[]>([])
const loading = ref(true)
const error = ref('')
const copiedCode = ref('')
/** 任务中心对该用户是否可见（partial 模式白名单外显示未开放） */
const userVisible = ref<boolean | null>(null)

const hasActiveTask = computed(() => tasks.value.length > 0)

const sortedRewards = computed(() =>
  [...rewards.value].sort((a, b) => (b.created_at ?? '').localeCompare(a.created_at ?? ''))
)

function fmtDate(s: string): string {
  return s ? s.slice(0, 10).replace(/-/g, '/') : '—'
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
  window.removeEventListener('keydown', onLightboxKeydown)
  document.body.style.overflow = ''
})

async function refresh() {
  loading.value = true
  error.value = ''
  try {
    userVisible.value = await fetchTaskVisibility(props.client)
    if (userVisible.value) {
      const [t, r] = await Promise.all([listTasks(props.client), fetchMyTaskRewards(props.client)])
      tasks.value = t
      rewards.value = r
    } else {
      tasks.value = []
      rewards.value = []
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

onMounted(refresh)
// phase 接口仅用于侧边栏角标，此处调用一次以保证角标及时出现
onMounted(() => { void fetchTaskPhase(props.client).catch(() => {}) })
</script>

<template>
  <div class="mx-auto w-full max-w-4xl space-y-5">
    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400">
      {{ error }}
    </p>
    <p v-if="loading" class="py-8 text-center text-sm text-gray-400">加载中…</p>

    <!-- 未开放（partial 模式白名单外） -->
    <section v-if="!loading && userVisible === false" class="card">
      <div class="flex flex-col items-center gap-2 px-6 py-12 text-center">
        <span class="flex h-14 w-14 items-center justify-center rounded-full bg-gray-100 text-2xl dark:bg-dark-700">🚫</span>
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">任务中心未开放</h3>
        <p class="text-sm text-gray-500 dark:text-dark-400">任务功能尚未对你的账号开放，敬请期待</p>
      </div>
    </section>

    <!-- 任务卡片 -->
    <template v-if="userVisible !== false">
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
            <span class="inline-flex items-center gap-1.5 rounded-full bg-amber-100 px-3 py-1 text-xs font-semibold text-amber-700 dark:bg-amber-500/15 dark:text-amber-300">
              ⏳ 剩 {{ t.days_left }} 天
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
              <dt class="text-xs text-gray-400 dark:text-dark-500">结算时间</dt>
              <dd class="mt-0.5 text-sm font-medium text-gray-900 dark:text-gray-100">每日 {{ t.settle_time }} 结算前一天</dd>
            </div>
            <div>
              <dt class="text-xs text-gray-400 dark:text-dark-500">参与条件</dt>
              <dd class="mt-0.5 text-sm font-medium text-gray-900 dark:text-gray-100">自动参与，无需报名</dd>
            </div>
          </dl>
        </div>
      </div>
    </section>

    <!-- 暂无任务 -->
    <section v-if="!loading && !hasActiveTask" class="card">
      <div class="flex flex-col items-center gap-2 px-6 py-12 text-center">
        <span class="flex h-14 w-14 items-center justify-center rounded-full bg-primary-100 text-2xl dark:bg-primary-500/15">🎯</span>
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">暂无任务</h3>
        <p class="text-sm text-gray-500 dark:text-dark-400">有新任务时会在这里展示，敬请期待</p>
      </div>
    </section>
    </template>

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
