<script setup lang="ts">
/**
 * LotteryPrizesEditor —— 奖池编辑器（活动表单 / 日常抽奖配置共用）。
 * 奖品草稿带 codesText（码池文本），提交前由调用方 parseCodes 转成 codes 数组。
 */
import { defineModel } from 'vue'
import type { PrizeDraft } from '../../api/lottery'
import LotterySelect from './LotterySelect.vue'
import PrizeIcon from './PrizeIcon.vue'

const prizes = defineModel<PrizeDraft[]>({ required: true })

// 概率预估入参：合计权重倍率（满足全部条件）/ 人数上限（0=不限）/ 是否配置了条件。
const props = defineProps<{
  weightMultiplier?: number
  maxParticipants?: number
  hasConditions?: boolean
}>()

const prizeTypeOptions = [
  { value: 'balance', label: '余额' },
  { value: 'redeem_code', label: '兑换码' }
]

function addPrize() {
  prizes.value.push({ name: '', prize_type: 'balance', value: 1, weight: 0, stock: 1, codesText: '' })
}

function removePrize(i: number) {
  prizes.value.splice(i, 1)
}

function codesCount(p: PrizeDraft): number {
  return p.codesText.split('\n').map((s) => s.trim()).filter(Boolean).length
}
// pct 格式化：保留 1 位小数并去尾零。
function fmtPct(v: number): string {
  const r = Math.round(v * 1000) / 10
  return (Number.isInteger(r) ? r.toFixed(0) : r.toFixed(1)) + '%'
}

/**
 * 单个奖品的中奖概率预估文案（与开奖算法同口径）：
 * 预计发出份数按权重占比分配 min(人数, 总库存)；
 * 普通用户率 = 份数/人数；满足条件用户率 = 份数×m / (人数-1+m)。
 */
function chanceText(p: PrizeDraft): string {
  const list = prizes.value
  const totalStock = list.reduce((s, q) => s + Math.max(0, q.stock || 0), 0)
  if (totalStock <= 0) return ''
  const eff = (q: PrizeDraft) => (q.weight > 0 ? q.weight : 1) // 与后端 EffectiveWeight 一致
  const weightSum = list.reduce((s, q) => s + eff(q), 0)
  const n = props.maxParticipants ?? 0
  const m = props.weightMultiplier ?? 1
  const capped = n > 0 && n < totalStock
  const issues = n > 0 ? Math.min(n, totalStock) : totalStock
  const count = issues * (eff(p) / weightSum)
  const stockPart = `预计发出约 ${Math.round(count * 10) / 10} 份`
  if (n <= 0) {
    return `${stockPart}（人数不限，无法估算单人中奖率）`
  }
  const base = count / n
  let text = `${stockPart} · 单人中奖率 ≈ ${fmtPct(base)}`
  if (props.hasConditions && m > 1) {
    const boosted = (count * m) / (n - 1 + m)
    text += `（满足全部条件 ×${Math.round(m * 100) / 100} ≈ ${fmtPct(boosted)}）`
  }
  if (capped) text += ' · 人数少于总库存，人人有份'
  return text
}
</script>

<template>
  <div>
    <div class="mb-3 flex items-center justify-end">
      <button class="btn btn-ghost btn-sm" @click="addPrize">+ 添加奖品</button>
    </div>
    <div class="space-y-3">
      <div
        v-for="(p, i) in prizes"
        :key="i"
        class="rounded-xl border border-gray-100 bg-white p-3 dark:border-dark-700 dark:bg-dark-800/60"
      >
        <!-- 第一行：图标 + 名称 + 删除 -->
        <div class="flex items-center gap-2">
          <span
            :class="['flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg',
                     p.prize_type === 'balance'
                       ? 'bg-amber-100 text-amber-600 dark:bg-amber-500/15 dark:text-amber-400'
                       : 'bg-primary-100 text-primary-600 dark:bg-primary-500/15 dark:text-primary-400']"
          >
            <PrizeIcon :type="p.prize_type" class="h-4.5 w-4.5" />
          </span>
          <input v-model="p.name" class="input !w-40" placeholder="奖品名称" />
          <button
            class="ml-auto flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-900/20"
            title="删除奖品"
            @click="removePrize(i)"
          >
            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
            </svg>
          </button>
        </div>
        <!-- 第二行：类型 / 面额 / 权重 / 库存 -->
        <div class="mt-2.5 flex flex-wrap items-center gap-2 pl-11">
          <LotterySelect v-model="p.prize_type" :options="prizeTypeOptions" class="!w-28" />
          <label class="flex items-center gap-1 text-xs text-gray-500 dark:text-dark-400" title="余额奖 = 入账金额（USD）；兑换码奖 = 生成兑换码的面额">
            面额
            <input v-model.number="p.value" type="number" min="0" class="input !w-24 !px-2" />
          </label>
          <label class="flex items-center gap-1 text-xs text-gray-500 dark:text-dark-400" title="相对概率权重；0 = 与其他奖品均等随机。数值越大越容易被抽到">
            权重
            <input v-model.number="p.weight" type="number" min="0" class="input !w-16 !px-2" />
          </label>
          <label class="flex items-center gap-1 text-xs text-gray-500 dark:text-dark-400" title="该奖品的发放数量；兑换码奖需要预填等量兑换码">
            库存
            <input v-model.number="p.stock" type="number" min="1" class="input !w-16 !px-2" />
          </label>
        </div>
        <!-- 中奖概率预估（按当前权重/库存/人数上限/合计倍率估算） -->
        <p v-if="chanceText(p)" class="mt-1.5 pl-11 text-xs text-gray-400 dark:text-dark-500">
          {{ chanceText(p) }}
        </p>
        <!-- 兑换码奖：预存码池 -->
        <div v-if="p.prize_type === 'redeem_code'" class="mt-3 border-t border-gray-100 pt-3 dark:border-dark-700">
          <div class="mb-1 flex items-center justify-between">
            <span class="text-xs font-medium text-gray-500 dark:text-dark-400">
              兑换码码池（每行一个，开奖时按序发给中奖用户）
            </span>
            <span
              :class="['text-xs font-semibold',
                       codesCount(p) >= p.stock ? 'text-primary-600 dark:text-primary-400' : 'text-red-500']"
            >
              已填 {{ codesCount(p) }} / 需 {{ p.stock }}
            </span>
          </div>
          <textarea
            v-model="p.codesText"
            class="input font-mono !text-xs"
            :rows="Math.min(6, Math.max(3, p.stock))"
            placeholder="REDEEM-CODE-0001&#10;REDEEM-CODE-0002"
          ></textarea>
          <p v-if="codesCount(p) < p.stock" class="mt-1 text-xs text-red-500">
            还差 {{ p.stock - codesCount(p) }} 个；码池不足时中奖用户将无法即时拿到兑换码
          </p>
        </div>
      </div>
    </div>
  </div>
</template>
