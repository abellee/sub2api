<template>
  <div class="w-full min-w-0 space-y-4 rounded-2xl border border-gray-100 bg-white p-4 shadow-card dark:border-dark-700/60 dark:bg-dark-800/50 sm:p-5">
    <div class="space-y-4">
      <!-- 一级目录:模型厂商。横向排列，空间不足时自动换行。 -->
      <section class="relative w-full min-w-0 pb-4 after:absolute after:inset-x-0 after:bottom-0 after:h-px after:bg-gray-100 after:content-[''] dark:after:bg-dark-700/60">
        <div class="mb-2 flex items-center gap-3">
          <div>
            <div class="flex items-center gap-2">
              <p class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
                {{ t('modelPlaza.filters.providerLabel') }}
              </p>
              <span class="inline-grid h-5 min-w-5 place-items-center rounded-full bg-gray-100 px-1.5 text-[10px] font-bold tabular-nums text-gray-500 dark:bg-dark-700 dark:text-dark-300">{{ platforms.length }}</span>
            </div>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              {{ t('modelPlaza.filters.providerHint') }}
            </p>
          </div>
        </div>
        <div class="flex w-full min-w-0 flex-wrap gap-2">
          <button
            v-for="p in ['all', ...platforms]"
            :key="`platform-${p}`"
            type="button"
            class="inline-flex min-h-10 items-center justify-between gap-3 rounded-lg px-3 py-2 text-left text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-40"
            :class="p === 'all' ? providerClass(platform === 'all') : platform === p ? 'provider-tinted-active' : 'provider-tinted'"
            :style="p === 'all' ? undefined : { '--provider-accent': platformAccentColor(p) }"
            :disabled="p !== 'all' && !providerEnabled(p)"
            :aria-pressed="platform === p"
            @click="$emit('update:platform', p)"
          >
            <span class="flex min-w-0 items-center gap-2.5">
              <PlatformIcon v-if="p !== 'all'" :platform="p as GroupPlatform" size="xs" />
              <span class="truncate">{{ p === 'all' ? t('modelPlaza.filters.allProviders') : p }}</span>
            </span>
            <span v-if="p !== 'all'" class="shrink-0 text-xs tabular-nums opacity-70">{{ providerGroupCount(p) }}</span>
          </button>
        </div>
      </section>

      <!-- 二级目录:只显示当前厂商的分组，同样横向自动换行。 -->
      <section class="w-full min-w-0">
        <div class="mb-2 flex flex-wrap items-end gap-2">
          <div>
            <div class="flex items-center gap-2">
              <p class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
                {{ t('modelPlaza.filters.groupLabel') }}
              </p>
              <span class="inline-grid h-5 min-w-5 place-items-center rounded-full bg-gray-100 px-1.5 text-[10px] font-bold tabular-nums text-gray-500 dark:bg-dark-700 dark:text-dark-300">{{ visibleGroups.length }}</span>
            </div>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              {{ selectedProviderLabel }}
            </p>
          </div>
        </div>
        <div v-if="visibleGroups.length" class="flex w-full min-w-0 flex-wrap gap-x-2 gap-y-4 pt-1">
          <button
            type="button"
            class="flex min-h-10 items-center justify-between gap-2 rounded-lg px-3 py-2 text-left text-sm font-medium transition"
            :class="chipClass(groupId === 'all')"
            @click="$emit('update:groupId', 'all')"
          >
            <span class="truncate">{{ t('modelPlaza.filters.allGroups') }}</span>
            <span class="shrink-0 text-xs opacity-70">{{ visibleGroups.length }}</span>
          </button>
          <button
            v-for="g in visibleGroups"
            :key="`group-${g.id}`"
            type="button"
            class="relative flex min-h-10 items-center justify-between gap-2 rounded-lg px-3 py-2 text-left text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-40"
            :class="[groupId === g.id ? 'chip-tinted-active' : 'chip-tinted', { 'is-lowest-rate': isLowestRate(g) }]"
            :style="{ '--chip-accent': platformAccentColor(g.platform) }"
            :disabled="!groupEnabled(g)"
            @click="$emit('update:groupId', g.id)"
          >
            <span v-if="isLowestRate(g)" class="lowest-rate-badge">{{ t('modelPlaza.filters.lowest') }}</span>
            <span class="truncate">{{ g.name }}</span>
            <span class="flex shrink-0 items-center gap-1.5">
              <span class="group-rate-badge text-xs">{{ formatRate(g.rate) }}</span>
            </span>
          </button>
        </div>
        <p v-else class="rounded-lg border border-dashed border-gray-200 px-3 py-6 text-center text-sm text-gray-400 dark:border-dark-600 dark:text-dark-500">
          {{ t('modelPlaza.filters.noGroupsForProvider') }}
        </p>
      </section>
    </div>

    <!-- 三级筛选:倍率与模型搜索。 -->
    <div class="grid gap-3 border-t border-gray-100 pt-4 dark:border-dark-700/60 md:grid-cols-[minmax(0,1fr)_minmax(220px,288px)] md:items-end">
      <div>
        <span class="mb-2 block text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
          {{ t('modelPlaza.filters.rateLabel') }}
        </span>
        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            class="rounded-lg px-3 py-1.5 text-sm font-medium transition"
            :class="chipClass(rate === 'all')"
            @click="$emit('update:rate', 'all')"
          >
            {{ t('modelPlaza.filters.all') }}
          </button>
          <button
            v-for="r in rates"
            :key="`rate-${r}`"
            type="button"
            class="rounded-lg px-3 py-1.5 font-mono text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-40"
            :class="chipClass(rate === r)"
            :disabled="!rateEnabled(r)"
            @click="$emit('update:rate', r)"
          >
            {{ formatRate(r) }}
          </button>
        </div>
      </div>

      <div>
        <span class="mb-2 block text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
          {{ t('modelPlaza.filters.modelLabel') }}
        </span>
        <div class="relative">
          <Icon
            name="search"
            size="sm"
            class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-dark-500"
          />
          <input
            :value="search"
            type="text"
            :placeholder="t('modelPlaza.filters.searchPlaceholder')"
            class="input rounded-lg py-1.5 pl-9 pr-9"
            @input="$emit('update:search', ($event.target as HTMLInputElement).value)"
          />
          <button
            v-if="search"
            type="button"
            class="absolute right-2.5 top-1/2 -translate-y-1/2 text-gray-400 transition-colors hover:text-gray-600 dark:text-dark-500 dark:hover:text-gray-300"
            :aria-label="t('modelPlaza.filters.clearSearch')"
            @click="$emit('update:search', '')"
          >
            <Icon name="x" size="xs" class="h-3.5 w-3.5" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { platformAccentColor } from '@/utils/platformColors'
import type { GroupPlatform } from '@/types'

const props = defineProps<{
  /** 数据中出现的平台(去重排序后)。 */
  platforms: string[]
  /** 全量分组(含平台与生效倍率),三个维度的置灰联动由此推导。 */
  groups: Array<{ id: number; name: string; platform: string; rate: number }>
  /** 全量生效倍率去重升序。 */
  rates: number[]
  platform: string
  groupId: number | 'all'
  rate: number | 'all'
  /** 模型名搜索词(纯前端过滤)。 */
  search: string
}>()

defineEmits<{
  'update:platform': [value: string]
  'update:groupId': [value: number | 'all']
  'update:rate': [value: number | 'all']
  'update:search': [value: string]
}>()

const { t } = useI18n()

const visibleGroups = computed(() =>
  props.platform === 'all' ? props.groups : props.groups.filter((g) => g.platform === props.platform)
)

/**
 * 三个维度互为约束(faceted):某选项可点 ⟺ 在「其他两维」当前选择下仍有分组命中。
 * 「全部」永远可点,作为解除本维约束的出口;可点项组合恒有结果,无需选择修正。
 */
function providerEnabled(p: string): boolean {
  return props.groups.some(
    (g) =>
      g.platform === p &&
      (props.rate === 'all' || g.rate === props.rate)
  )
}

function groupEnabled(g: { platform: string; rate: number }): boolean {
  return (
    (props.platform === 'all' || g.platform === props.platform) &&
    (props.rate === 'all' || g.rate === props.rate)
  )
}

function providerGroupCount(provider: string): number {
  return props.groups.filter((g) => g.platform === provider).length
}

function formatRate(value: number): string {
  return `${value}x`
}

const selectedProviderLabel = computed(() => {
  if (props.platform === 'all') return t('modelPlaza.filters.allProviders')
  return props.platform
})

const lowestRate = computed(() =>
  visibleGroups.value.length ? Math.min(...visibleGroups.value.map((g) => g.rate)) : null
)

function isLowestRate(g: { rate: number }): boolean {
  return lowestRate.value !== null && Math.abs(g.rate - lowestRate.value) < Number.EPSILON
}

function providerClass(active: boolean): string {
  return active
    ? 'bg-gradient-to-r from-primary-500 to-primary-600 text-white shadow-sm shadow-primary-500/30'
    : 'bg-white text-gray-600 ring-1 ring-inset ring-gray-200 hover:bg-gray-50 hover:text-gray-900 dark:bg-dark-800/60 dark:text-dark-300 dark:ring-dark-700 dark:hover:bg-dark-800 dark:hover:text-white'
}

function rateEnabled(r: number): boolean {
  return props.groups.some(
    (g) =>
      g.rate === r &&
      (props.platform === 'all' || g.platform === props.platform) &&
      (props.groupId === 'all' || g.id === props.groupId)
  )
}

function chipClass(active: boolean): string {
  return active
    ? 'bg-gradient-to-r from-primary-500 to-primary-600 text-white shadow-sm shadow-primary-500/30'
    : 'bg-white text-gray-600 ring-1 ring-inset ring-gray-200 enabled:hover:bg-gray-50 enabled:hover:text-gray-900 enabled:hover:ring-gray-300 dark:bg-dark-800/60 dark:text-dark-300 dark:ring-dark-700 dark:enabled:hover:bg-dark-800 dark:enabled:hover:text-white'
}
</script>

<style scoped>
/* 平台/分组 chip 的配色统一从 --chip-accent(平台主色)派生,新增平台无需扩展样式。
   激活态与非激活态在模板上互斥挂载,避免选择器优先级互相覆盖。 */
.chip-tinted {
  color: var(--chip-accent);
  color: color-mix(in srgb, var(--chip-accent) 78%, black);
  background-color: color-mix(in srgb, var(--chip-accent) 9%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--chip-accent) 25%, transparent);
}

.provider-tinted {
  color: var(--provider-accent, #4b5563);
  background-color: color-mix(in srgb, var(--provider-accent, #64748b) 8%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--provider-accent, #64748b) 22%, transparent);
}

.provider-tinted:not(:disabled):hover {
  background-color: color-mix(in srgb, var(--provider-accent, #64748b) 14%, transparent);
}

.provider-tinted-active {
  color: #fff;
  background-color: color-mix(in srgb, var(--provider-accent, #0ea5e9) 84%, black);
  box-shadow: 0 1px 2px 0 color-mix(in srgb, var(--provider-accent, #0ea5e9) 32%, transparent);
}

.provider-tinted-active:not(:disabled):hover {
  background-color: color-mix(in srgb, var(--provider-accent, #0ea5e9) 74%, black);
}

.dark .provider-tinted {
  color: color-mix(in srgb, var(--provider-accent, #94a3b8) 74%, white);
  background-color: color-mix(in srgb, var(--provider-accent, #94a3b8) 12%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--provider-accent, #94a3b8) 28%, transparent);
}

.dark .provider-tinted-active {
  background-color: color-mix(in srgb, var(--provider-accent, #0ea5e9) 78%, transparent);
}

.group-rate-badge {
  padding: 0.3rem 0.45rem;
  border-radius: 0.375rem;
  color: var(--chip-accent);
  background-color: color-mix(in srgb, var(--chip-accent) 14%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--chip-accent) 26%, transparent);
  font-weight: 800;
  line-height: 1;
}

.lowest-rate-badge {
  position: absolute;
  top: -0.75rem;
  left: -0.3rem;
  z-index: 1;
  padding: 0.3rem 0.4rem;
  border: 2px solid white;
  border-radius: 0.375rem;
  color: #9a3412;
  background: #ffedd5;
  font-size: 9px;
  font-weight: 800;
  line-height: 1;
}

.is-lowest-rate {
  padding-left: 0.75rem !important;
}

.dark .lowest-rate-badge {
  border-color: rgb(30 41 59);
}

.chip-tinted-active .group-rate-badge {
  color: var(--chip-accent);
  background-color: #fff;
  box-shadow: none;
}

.chip-tinted:not(:disabled):hover {
  background-color: color-mix(in srgb, var(--chip-accent) 16%, transparent);
}

.dark .chip-tinted {
  color: color-mix(in srgb, var(--chip-accent) 72%, white);
  background-color: color-mix(in srgb, var(--chip-accent) 12%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--chip-accent) 30%, transparent);
}

.dark .chip-tinted:not(:disabled):hover {
  background-color: color-mix(in srgb, var(--chip-accent) 18%, transparent);
}

.chip-tinted-active {
  color: #fff;
  background-color: var(--chip-accent);
  background-color: color-mix(in srgb, var(--chip-accent) 85%, black);
  box-shadow: 0 1px 2px 0 color-mix(in srgb, var(--chip-accent) 35%, transparent);
}

.chip-tinted-active:not(:disabled):hover {
  background-color: color-mix(in srgb, var(--chip-accent) 75%, black);
}

.dark .chip-tinted-active {
  background-color: color-mix(in srgb, var(--chip-accent) 80%, transparent);
}

.dark .chip-tinted-active:not(:disabled):hover {
  background-color: var(--chip-accent);
}
</style>
