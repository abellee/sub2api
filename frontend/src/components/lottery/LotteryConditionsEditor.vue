<script setup lang="ts">
/**
 * LotteryConditionsEditor —— 可参与条件编辑器（活动表单 / 日常抽奖配置共用）。
 * 直接原地修改 model 数组元素（对象引用共享），增删通过数组变更同步。
 */
import { computed, defineModel } from 'vue'
import {
  conditionEffectiveBonus,
  totalWeightMultiplier,
  type ConditionDef
} from '../../api/lottery'
import LotterySelect from './LotterySelect.vue'

const conditions = defineModel<ConditionDef[]>({ required: true })

// 「自动叠加」模式下使用的全局默认加成比例（由父级传入当前表单值）。
// showBonus=false 时隐藏全部加成 UI（任务等无权重概念的场景）。
const props = withDefaults(
  defineProps<{ autoBonusPercent?: number; showBonus?: boolean }>(),
  { autoBonusPercent: 0, showBonus: true }
)

/** 该条件当前的实际加成百分比：none=0，manual=手填值，auto=表单默认值。 */
function effectiveBonus(c: ConditionDef): number {
  return conditionEffectiveBonus(c, props.autoBonusPercent ?? 0)
}

const totalMultiplierText = computed(() => {
  const m = totalWeightMultiplier(conditions.value, props.autoBonusPercent ?? 0)
  return `×${Math.round(m * 100) / 100}`
})

const dimensionOptions = [
  { value: 'token_usage', label: 'Token 消耗量' },
  { value: 'activity_days', label: '活跃度' },
  { value: 'registered_days', label: '注册时长' }
]
const tokenModeOptions = [
  { value: 'per_day', label: '每日≥阈值' },
  { value: 'total', label: '累计≥阈值' }
]
const bonusModeOptions = [
  { value: 'none', label: '无加成' },
  { value: 'manual', label: '手动加成 %' },
  { value: 'auto', label: '自动叠加' }
]

function addCondition() {
  conditions.value.push({
    dimension: 'token_usage', window_days: 7, mode: 'per_day',
    threshold: 100_000_000, bonus_mode: 'none'
  })
}

function removeCondition(i: number) {
  conditions.value.splice(i, 1)
}

/** 切换维度时补齐该维度的默认参数，避免遗留字段造成校验失败。 */
function onDimensionChange(c: ConditionDef, dimension: string) {
  c.dimension = dimension as ConditionDef['dimension']
  if (dimension === 'token_usage') {
    c.window_days = c.window_days || 7
    c.mode = c.mode === 'total' ? 'total' : 'per_day'
    c.threshold = c.threshold && c.threshold > 0 ? c.threshold : 100_000_000
  } else if (dimension === 'activity_days') {
    c.window_days = c.window_days || 7
    c.min_active_days = c.min_active_days && c.min_active_days > 0 ? c.min_active_days : 1
  } else if (dimension === 'registered_days') {
    // 默认：新用户（注册 ≤ 7 天）
    if (!c.min_registered_days && !c.max_registered_days) c.max_registered_days = 7
  }
}
</script>

<template>
  <div class="space-y-3">
    <div
      v-for="(c, i) in conditions"
      :key="i"
      class="rounded-xl border border-gray-100 bg-white p-3 dark:border-dark-700 dark:bg-dark-800/60"
    >
      <!-- 第一行：维度 + 删除 -->
      <div class="flex items-center gap-2">
        <LotterySelect
          :model-value="c.dimension"
          :options="dimensionOptions"
          class="!w-36"
          @update:model-value="onDimensionChange(c, $event)"
        />
        <button
          class="ml-auto flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-900/20"
          title="删除条件"
          @click="removeCondition(i)"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
          </svg>
        </button>
      </div>
      <!-- 第二行：参数 -->
      <div class="mt-2.5 flex flex-wrap items-center gap-2 pl-9">
        <label
          v-if="c.dimension !== 'registered_days'"
          class="flex items-center gap-1 text-xs text-gray-500 dark:text-dark-400"
          title="统计用量的回看天数（窗口以昨天为终点）"
        >
          窗口
          <input v-model.number="c.window_days" type="number" min="1" class="input !w-16 !px-2" />
          天
        </label>
        <template v-if="c.dimension === 'token_usage'">
          <LotterySelect v-model="c.mode" :options="tokenModeOptions" class="!w-36" />
          <label
            class="flex items-center gap-1 text-xs text-gray-500 dark:text-dark-400"
            title="每天（或窗口累计）需要达到的最少 token 数，单位 M（百万 tokens）"
          >
            ≥
            <input
              :value="c.threshold ? c.threshold / 1e6 : ''"
              @input="c.threshold = Math.round(Number(($event.target as HTMLInputElement).value || 0) * 1e6)"
              type="number"
              min="0.1"
              step="0.1"
              class="input !w-24 !px-2"
            />
            M
          </label>
        </template>
        <template v-else-if="c.dimension === 'activity_days'">
          <label class="flex items-center gap-1 text-xs text-gray-500 dark:text-dark-400" title="窗口内「有用量」的天数下限">
            活跃≥
            <input v-model.number="c.min_active_days" type="number" min="1" class="input !w-16 !px-2" />
            天
          </label>
          <label class="flex items-center gap-1 text-xs text-gray-500 dark:text-dark-400" title="当天 token 消耗达到该值才算「活跃」，0 = 有任何用量即算，单位 M（百万 tokens）">
            日最低
            <input
              :value="c.daily_min_tokens ? c.daily_min_tokens / 1e6 : 0"
              @input="c.daily_min_tokens = Math.round(Number(($event.target as HTMLInputElement).value || 0) * 1e6)"
              type="number"
              min="0"
              step="0.1"
              class="input !w-24 !px-2"
            />
            M
          </label>
        </template>
        <template v-else-if="c.dimension === 'registered_days'">
          <label class="flex items-center gap-1 text-xs text-gray-500 dark:text-dark-400" title="注册天数下限（自然日），0 = 不设下限。注册当天 = 第 1 天，当天注册即满足「注册满 1 天」">
            注册满
            <input v-model.number="c.min_registered_days" type="number" min="0" class="input !w-16 !px-2" />
            天
          </label>
          <label class="flex items-center gap-1 text-xs text-gray-500 dark:text-dark-400" title="注册天数上限（自然日），0 = 不设上限。注册当天 = 第 1 天，≤ 7 天即「注册 7 天内的新用户」">
            至多
            <input v-model.number="c.max_registered_days" type="number" min="0" placeholder="不限" class="input !w-16 !px-2" />
            天
          </label>
        </template>
        <LotterySelect v-if="showBonus" v-model="c.bonus_mode" :options="bonusModeOptions" class="!w-32" title="概率加成" />
        <label v-if="showBonus && c.bonus_mode === 'manual'" class="flex items-center gap-1 text-xs text-gray-500 dark:text-dark-400">
          +
          <input v-model.number="c.bonus_percent" type="number" min="0" class="input !w-20 !px-2" />
          %
        </label>
        <span
          v-if="showBonus"
          class="rounded-full px-2 py-0.5 text-xs font-semibold"
          :class="effectiveBonus(c) > 0
            ? 'bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300'
            : 'bg-gray-100 text-gray-400 dark:bg-dark-700 dark:text-dark-400'"
          title="该条件满足时用户中奖权重的实际放大比例（自动叠加 = 表单里的默认加成比例）"
        >
          实际 +{{ effectiveBonus(c) }}%
        </span>
      </div>
    </div>
    <button class="btn btn-ghost btn-sm" @click="addCondition">+ 添加条件</button>
    <p v-if="showBonus && conditions.length" class="text-xs text-gray-400">
      全部满足条件的用户权重合计倍率：<span class="font-semibold text-gray-600 dark:text-dark-300">{{ totalMultiplierText }}</span>
    </p>
    <p v-if="!conditions.length" class="text-xs text-gray-400">不添加条件时，任何登录用户均可参与</p>
  </div>
</template>
