<script setup lang="ts">
/**
 * LotteryDrawPanel —— 用户侧抽奖面板（独立组件，合并单元）。
 * 场次列表 + 手风琴展示：折叠态为紧凑卡片（状态色条区分），点击展开完整详情
 * （翻牌倒计时/说明/奖池/参与按钮）。进行中的场次在前（最早开奖优先），其后是
 * 未开始的场次；全部结束后回落展示最近一场已开奖。
 * 样式使用 Sub2API 主站的 Tailwind 组件类（card/btn/input），并入后零适配。
 */
import { computed, onMounted, ref, watch } from 'vue'
import {
  fetchMyVisibility,
  fetchMyWinnings,
  formatValue,
  listActivities,
  participate,
  type ActivityView,
  type WinnerRecord
} from '../../api/lottery'
import type { LotteryClient } from '../../api/lotteryClient'
import LotteryFlipCountdown from './LotteryFlipCountdown.vue'
import LotteryMarkdownCard from './LotteryMarkdownCard.vue'
import PrizeIcon from './PrizeIcon.vue'
import { formatBeijingDateTime } from '../../utils/beijingTime'

const props = defineProps<{ client: LotteryClient }>()

// 用户侧显隐：partial 模式下白名单外用户整个页面按"未开放"呈现。
// null = 未加载完成；接口失败按可见处理（服务端仍会强制校验）。
const userVisible = ref<boolean | null>(null)

const activities = ref<ActivityView[]>([])
const myWinnings = ref<WinnerRecord[]>([])
const loading = ref(true)
const error = ref('')
const joiningId = ref<number | null>(null)

/** 展示的场次：进行中（最早开奖优先）→ 未开始（最早开始优先）；
 * 都没有时回落最近一场已开奖（倒计时显示「-」），完全无数据为空。 */
const displayCards = computed<ActivityView[]>(() => {
  const joining = activities.value
    .filter((a) => a.phase === 'joining')
    .sort((a, b) => a.draws_at.localeCompare(b.draws_at))
  const upcoming = activities.value
    .filter((a) => a.phase === 'upcoming')
    .sort((a, b) => a.starts_at.localeCompare(b.starts_at))
  if (joining.length || upcoming.length) return [...joining, ...upcoming]
  const drawn = activities.value
    .filter((a) => a.phase === 'drawn' || a.phase === 'fulfilled')
    .sort((a, b) => (b.drawn_at ?? b.draws_at).localeCompare(a.drawn_at ?? a.draws_at))
  return drawn[0] ? [drawn[0]] : []
})

const lastDrawn = computed<ActivityView | null>(
  () =>
    activities.value
      .filter((a) => a.phase === 'drawn' || a.phase === 'fulfilled')
      .sort((a, b) => (b.drawn_at ?? '').localeCompare(a.drawn_at ?? ''))[0] ?? null
)

/** 手风琴展开的场次（默认展开优先级最高的一场）。 */
const expandedId = ref<number | null>(null)
function toggleCard(a: ActivityView) {
  expandedId.value = expandedId.value === a.id ? null : a.id
}

// 只有一场活动时始终默认展开；多场时保持用户手动选择
watch(
  () => displayCards.value.map((a) => a.id).join(','),
  (ids) => {
    if (!ids) {
      expandedId.value = null
      return
    }
    if (displayCards.value.length === 1) {
      expandedId.value = displayCards.value[0].id
    } else if (expandedId.value && !displayCards.value.some((a) => a.id === expandedId.value)) {
      expandedId.value = displayCards.value[0].id
    }
  },
  { immediate: true }
)

/** 每场活动的说明/条件 tab 状态（默认活动说明）。 */
const infoTabs = ref<Record<number, 'desc' | 'conditions'>>({})
function infoTabOf(a: ActivityView): 'desc' | 'conditions' {
  return infoTabs.value[a.id] ?? 'desc'
}

/** 倒计时目标与标签：未开始 → 距开始；进行中 → 距开奖；已开奖显示「-」。 */
function countdownOf(a: ActivityView) {
  if (a.phase === 'upcoming') {
    return { target: a.starts_at, label: '距开始', dash: false }
  }
  if (a.phase === 'joining') {
    return { target: a.draws_at, label: '距开奖', dash: false }
  }
  return { target: a.draws_at, label: '已开奖', dash: true }
}

/** 倒计时归零：自动刷新展示（距开奖需等调度器开奖，多等一个调度周期）。 */
function onCountdownFinish(label: string) {
  if (label === '已开奖') return // 已开奖占位不触发刷新
  const delay = label === '距开始' ? 2000 : 35_000
  window.setTimeout(() => {
    void refresh()
  }, delay)
}

function phaseBadgeOf(a: ActivityView) {
  if (a.phase === 'upcoming') return { text: '未开始', cls: 'bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-300' }
  if (a.phase === 'joining') return { text: '进行中', cls: 'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300' }
  return { text: '已开奖', cls: 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-400' }
}

/** 服务端没下发 participant_count 时，用户侧不展示当前人数。 */
function showsParticipantCount(a: ActivityView): boolean {
  return typeof a.participant_count === 'number'
}

function slotsLeftOf(a: ActivityView): number | null {
  if (!showsParticipantCount(a) || a.max_participants <= 0) return null
  return Math.max(0, a.max_participants - (a.participant_count ?? 0))
}

/** 参与人数进度（0-100）；不限名额或不显示人数时不展示进度条。 */
function participationPctOf(a: ActivityView): number {
  if (!showsParticipantCount(a) || a.max_participants <= 0) return 0
  return Math.min(100, Math.round(((a.participant_count ?? 0) / a.max_participants) * 100))
}

async function refresh() {
  error.value = ''
  try {
    activities.value = await listActivities(props.client)
    myWinnings.value = await fetchMyWinnings(props.client)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

async function join(a: ActivityView) {
  joiningId.value = a.id
  error.value = ''
  try {
    const updated = await participate(props.client, a.id)
    activities.value = activities.value.map((x) => (x.id === updated.id ? updated : x))
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    joiningId.value = null
  }
}

function fmtDate(v: string): string {
  return formatBeijingDateTime(v)
}

onMounted(async () => {
  try {
    userVisible.value = (await fetchMyVisibility(props.client)).visible
  } catch {
    userVisible.value = true // 探测失败按可见处理，服务端仍会强制校验
  }
  if (userVisible.value) await refresh()
})
</script>

<template>
  <div class="mx-auto w-full max-w-3xl space-y-6">
    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400">
      {{ error }}
    </p>

    <!-- 未开放：partial 模式且不在白名单（服务端同时强制校验） -->
    <div v-if="userVisible === false" class="card">
      <div class="card-body flex flex-col items-center gap-3 py-16 text-center">
        <div class="flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 text-3xl dark:bg-dark-800">🚫</div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">抽奖活动未开放</h2>
        <p class="text-sm text-gray-500 dark:text-dark-400">当前没有面向你开放的抽奖活动</p>
      </div>
    </div>

    <!-- 空状态：当前没有抽奖 -->
    <div v-else-if="!loading && !displayCards.length" class="card">
      <div class="card-body flex flex-col items-center gap-3 py-16 text-center">
        <div class="flex h-16 w-16 items-center justify-center rounded-full bg-primary-100 text-3xl dark:bg-primary-900/40">🎁</div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">暂无进行中的抽奖</h2>
        <p class="text-sm text-gray-500 dark:text-dark-400">下一场抽奖开启后，这里会第一时间通知你</p>
      </div>
    </div>

    <!-- 场次列表：手风琴样式，状态色条区分（进行中 teal / 未开始 amber / 已开奖 灰） -->
    <div v-else class="space-y-3">
      <div
        v-for="a in displayCards"
        :key="a.id"
        :class="['overflow-hidden rounded-2xl border shadow-card',
                 a.phase === 'joining'
                   ? 'border-primary-300 dark:border-primary-800'
                   : a.phase === 'upcoming'
                     ? 'border-amber-300 dark:border-amber-800'
                     : 'border-gray-200 dark:border-dark-700']"
      >
        <!-- 卡片头：渐变底 + 倒计时，整块可点击展开/收起 -->
        <button
          class="block w-full bg-gradient-primary px-5 py-5 text-left text-white transition-opacity hover:opacity-95 sm:px-6"
          @click="toggleCard(a)"
        >
          <div class="flex items-center gap-2">
            <span :class="['rounded-full px-2.5 py-0.5 text-xs font-semibold', phaseBadgeOf(a).cls]">
              {{ phaseBadgeOf(a).text }}
            </span>
            <span
              v-if="a.won"
              class="rounded-full bg-white/20 px-2.5 py-0.5 text-xs font-semibold"
            >🎉 中奖</span>
            <span class="ml-auto flex-shrink-0 text-xs text-white/70">{{ expandedId === a.id ? '收起 ▲' : '展开 ▼' }}</span>
          </div>

          <div class="mt-4 flex flex-col gap-5 lg:flex-row lg:items-center lg:justify-between">
            <div class="min-w-0 flex-1">
              <h1 class="text-xl font-bold leading-snug">{{ a.name }}</h1>
              <div class="mt-3 flex flex-wrap items-center gap-x-5 gap-y-1.5 text-sm text-primary-50/90">
                <span>开始 · {{ fmtDate(a.starts_at) }}</span>
                <span>开奖 · {{ fmtDate(a.draws_at) }}</span>
                <span v-if="showsParticipantCount(a)">
                  已有 <b class="font-semibold text-white">{{ a.participant_count }}</b> 人参与
                  <template v-if="slotsLeftOf(a) != null"> · 剩余 {{ slotsLeftOf(a) }} 名额</template>
                </span>
                <span v-else>
                  <template v-if="a.max_participants > 0">最多可参与 {{ a.max_participants }} 人</template>
                  <template v-else>最多可参与人数：不限</template>
                </span>
              </div>
            </div>
            <div class="flex-shrink-0 self-start rounded-2xl bg-white/10 p-4 ring-1 ring-white/15 backdrop-blur-sm lg:self-auto">
              <LotteryFlipCountdown
                :key="countdownOf(a).target + String(countdownOf(a).dash)"
                :target="countdownOf(a).target"
                :label="countdownOf(a).label"
                :dash="countdownOf(a).dash"
                @finish="onCountdownFinish"
              />
            </div>
          </div>

          <!-- 参与人数进度条：仅在服务端下发了人数、且限名额时显示 -->
          <div v-if="showsParticipantCount(a)" class="mt-5">
            <div v-if="a.max_participants > 0" class="h-2.5 w-full overflow-hidden rounded-full bg-white/25">
              <div
                class="h-2.5 rounded-full bg-white transition-all duration-500"
                :style="{ width: participationPctOf(a) + '%' }"
              ></div>
            </div>
            <div class="mt-1.5 flex justify-between text-xs text-primary-100/80">
              <span>{{ a.participant_count }} / {{ a.max_participants > 0 ? a.max_participants : '不限名额' }}</span>
              <span v-if="a.max_participants > 0">{{ participationPctOf(a) }}%</span>
            </div>
          </div>

          <!-- 底部参与提示：居中、醒目 -->
          <div class="mt-5 flex justify-center">
            <span
              :class="['inline-flex items-center gap-2 rounded-full px-6 py-2 text-sm font-bold shadow-lg',
                       a.phase === 'joining'
                         ? 'bg-white text-primary-700'
                         : 'bg-white/20 text-white ring-1 ring-white/30']"
            >
              <template v-if="a.phase === 'joining' && !a.joined">👆 点击参与抽奖</template>
              <template v-else-if="a.joined">✓ 已参与 · 点击查看详情</template>
              <template v-else>👆 点击查看详情</template>
            </span>
          </div>
        </button>

        <!-- 展开区：手风琴动画（grid rows 过渡） -->
        <div
          class="grid bg-white transition-[grid-template-rows] duration-300 ease-out dark:bg-dark-800"
          :class="expandedId === a.id ? 'grid-rows-[1fr]' : 'grid-rows-[0fr]'"
        >
          <div class="overflow-hidden">
            <div class="space-y-5 p-4 sm:p-5">
              <!-- 我的结果（该场中奖置顶展示） -->
              <section v-if="a.won" class="card border-primary-200 dark:border-primary-800">
                <div class="card-body">
                  <div class="flex items-center gap-3">
                    <span class="text-2xl">🎉</span>
                    <div>
                      <h2 class="font-semibold text-gray-900 dark:text-white">恭喜你中奖了！</h2>
                      <p class="text-sm text-gray-500 dark:text-dark-400">{{ a.won.prize_name }}</p>
                    </div>
                  </div>
                  <div class="mt-4 rounded-xl bg-primary-50 px-4 py-3 dark:bg-primary-900/30">
                    <template v-if="a.won.prize_type === 'balance'">
                      <p class="text-sm text-primary-800 dark:text-primary-200">
                        +{{ formatValue(a.won.value) }} 余额已到账
                        <span v-if="a.won.note" class="text-primary-600 dark:text-primary-300">（{{ a.won.note }}）</span>
                      </p>
                    </template>
                    <template v-else>
                      <p class="text-sm text-primary-800 dark:text-primary-200">兑换码</p>
                      <code class="mt-1 inline-block rounded-lg bg-white px-3 py-1.5 font-mono text-base font-semibold text-gray-900 dark:bg-dark-900 dark:text-primary-300">
                        {{ a.won.redeem_code || '发放中，请稍后刷新' }}
                      </code>
                      <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">前往「兑换」页面输入兑换码即可领取</p>
                    </template>
                  </div>
                </div>
              </section>

              <!-- 活动说明 / 参与条件 -->
              <section v-if="a.description || a.conditions?.length" class="card">
                <div class="card-header flex items-center gap-2">
                  <button
                    :class="['btn btn-sm', infoTabOf(a) === 'desc' ? 'btn-primary' : 'btn-ghost']"
                    @click="infoTabs = { ...infoTabs, [a.id]: 'desc' }"
                  >
                    活动说明
                  </button>
                  <button
                    v-if="a.conditions?.length"
                    :class="['btn btn-sm', infoTabOf(a) === 'conditions' ? 'btn-primary' : 'btn-ghost']"
                    @click="infoTabs = { ...infoTabs, [a.id]: 'conditions' }"
                  >
                    参与条件
                  </button>
                </div>
                <div class="card-body">
                  <template v-if="infoTabOf(a) === 'desc'">
                    <LotteryMarkdownCard v-if="a.description" embedded :source="a.description" />
                    <p v-else class="text-center text-sm text-gray-400 dark:text-dark-500">暂无说明</p>
                  </template>
                  <div v-else class="space-y-2.5">
                    <div
                      v-for="(c, i) in a.conditions"
                      :key="i"
                      class="flex items-center justify-between gap-3 rounded-xl px-4 py-3"
                      :class="c.satisfied ? 'bg-primary-50 dark:bg-primary-900/20' : 'bg-gray-50 dark:bg-dark-900/60'"
                    >
                      <div class="flex items-center gap-3">
                        <span
                          class="flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full text-xs font-bold text-white"
                          :class="c.satisfied ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600'"
                        >{{ c.satisfied ? '✓' : '✗' }}</span>
                        <span class="text-sm" :class="c.satisfied ? 'text-gray-900 dark:text-gray-100' : 'text-gray-500 dark:text-dark-400'">
                          {{ c.detail }}
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              </section>

              <!-- 奖池 -->
              <section class="card">
                <div class="card-header">
                  <h2 class="text-sm font-semibold text-gray-900 dark:text-white">奖池</h2>
                </div>
                <div class="card-body grid gap-3 sm:grid-cols-2">
                  <div
                    v-for="p in a.prizes"
                    :key="p.id"
                    class="flex items-center gap-3 rounded-xl border border-gray-100 px-4 py-3 dark:border-dark-700"
                  >
                    <span
                      :class="['flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl',
                               p.prize_type === 'balance'
                                 ? 'bg-amber-100 text-amber-600 dark:bg-amber-500/15 dark:text-amber-400'
                                 : 'bg-primary-100 text-primary-600 dark:bg-primary-500/15 dark:text-primary-400']"
                    >
                      <PrizeIcon :type="p.prize_type" class="h-5 w-5" />
                    </span>
                    <div class="min-w-0 flex-1">
                      <p class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ p.name }}</p>
                      <p class="text-xs text-gray-500 dark:text-dark-400">{{ p.prize_type === 'balance' ? '余额' : '兑换码' }} · {{ p.granted_count }}/{{ p.stock }} 已送出</p>
                    </div>
                    <span class="font-mono text-lg font-bold text-primary-600 dark:text-primary-400">{{ formatValue(p.value) }}</span>
                  </div>
                </div>
              </section>

              <!-- 参与操作 -->
              <div class="card p-4 shadow-glass">
                <button v-if="a.phase === 'joining' && !a.joined && a.eligible" class="btn btn-primary w-full !py-4 !text-base" :disabled="joiningId === a.id" @click="join(a)">
                  {{ joiningId === a.id ? '提交中…' : '立即参与抽奖' }}
                </button>
                <p v-else-if="a.won" class="flex items-center justify-center gap-2 py-1 text-sm font-medium text-primary-600 dark:text-primary-400">
                  🎉 恭喜中奖：{{ a.won.prize_name }}（详情见「我的中奖记录」）
                </p>
                <p v-else-if="a.phase === 'upcoming'" class="py-1 text-center text-sm text-gray-500 dark:text-dark-400">
                  活动尚未开始，{{ fmtDate(a.starts_at) }} 开放参与
                </p>
                <p v-else-if="a.joined" class="flex items-center justify-center gap-2 py-1 text-sm font-medium text-primary-600 dark:text-primary-400">
                  <span class="flex h-5 w-5 items-center justify-center rounded-full bg-primary-100 text-xs dark:bg-primary-900/40">✓</span>
                  已参与，开奖结果将在本页公布
                </p>
                <p v-else-if="a.phase === 'drawn' || a.phase === 'fulfilled'" class="py-1 text-center text-sm text-gray-500 dark:text-dark-400">
                  本期未中奖，期待下一场
                </p>
                <p v-else class="py-1 text-center text-sm text-gray-500 dark:text-dark-400">
                  {{ a.eligible_reason || '未满足参与条件' }}
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 我的中奖记录（跨活动；兑换码仅本人可见）。上期中奖名单不在用户侧展示。 -->
    <section v-if="myWinnings.length || lastDrawn" class="card">
      <div class="card-header">
        <h2 class="text-sm font-semibold text-gray-900 dark:text-white">我的中奖记录</h2>
      </div>
      <div class="card-body">
        <div class="space-y-3">
          <p v-if="!myWinnings.length" class="text-center text-sm text-gray-400 dark:text-dark-500">
            暂无中奖记录，参与抽奖试试手气吧
          </p>
          <div
            v-for="w in myWinnings"
            :key="w.id"
            class="rounded-xl border border-gray-100 px-4 py-3 dark:border-dark-700"
          >
          <div class="flex items-start gap-3">
            <span
              :class="['mt-0.5 flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg',
                       w.prize_type === 'balance'
                         ? 'bg-amber-100 text-amber-600 dark:bg-amber-500/15 dark:text-amber-400'
                         : 'bg-primary-100 text-primary-600 dark:bg-primary-500/15 dark:text-primary-400']"
            >
              <PrizeIcon :type="w.prize_type" class="h-4 w-4" />
            </span>
            <div class="min-w-0 flex-1">
              <div class="flex items-center justify-between gap-3">
                <span class="text-sm font-semibold text-gray-900 dark:text-white">{{ w.activity_name }}</span>
                <em
                  :class="['rounded-full px-2 py-0.5 text-xs not-italic',
                           w.fulfillment === 'done' ? 'bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300'
                             : w.fulfillment === 'pending' ? 'bg-amber-50 text-amber-600 dark:bg-amber-900/30 dark:text-amber-300'
                             : 'bg-red-50 text-red-600 dark:bg-red-900/30 dark:text-red-300']"
                >
                  {{ w.fulfillment === 'done' ? '已发放' : w.fulfillment === 'pending' ? '发放中' : '发放失败' }}
                </em>
              </div>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ w.prize_name }} · +{{ formatValue(w.value) }} {{ w.prize_type === 'balance' ? '余额' : '兑换码' }}
                <span v-if="w.created_at" class="ml-2 text-xs text-gray-400 dark:text-dark-500">{{ fmtDate(w.created_at) }}</span>
              </p>
              <p v-if="w.prize_type === 'balance' && w.note" class="mt-1 text-xs text-gray-400 dark:text-dark-500">
                到账备注：{{ w.note }}
              </p>
              <code
                v-if="w.prize_type === 'redeem_code' && w.redeem_code"
                class="mt-2 inline-block rounded-lg bg-gray-100 px-3 py-1.5 font-mono text-sm font-semibold text-gray-900 dark:bg-dark-900 dark:text-primary-300"
              >
                {{ w.redeem_code }}
              </code>
            </div>
          </div>
        </div>
        </div>
      </div>
    </section>
  </div>
</template>
