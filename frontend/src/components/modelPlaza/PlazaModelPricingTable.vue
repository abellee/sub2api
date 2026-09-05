<template>
  <div class="plaza-pricing-table overflow-x-auto" :style="accentStyle">
    <table class="w-full min-w-[1000px] table-fixed border-collapse text-sm tabular-nums">
      <colgroup>
        <col class="w-[25%]" />
        <col class="w-[11%]" />
        <col class="w-[9%]" />
        <col class="w-[14%]" />
        <col class="w-[11%]" />
        <col class="w-[8%]" />
        <col class="w-[14%]" />
        <col class="w-[8%]" />
      </colgroup>
      <thead>
        <tr
          class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400"
        >
          <th
            rowspan="2"
            class="border-r border-gray-100 py-2.5 pl-5 pr-4 text-left align-middle dark:border-dark-700/60"
          >
            {{ t('modelPlaza.table.model') }}
          </th>
          <th colspan="3" class="pz-bg pt-2 text-center">
            <div class="pz-title border-b pb-2 font-semibold">
              {{ t('modelPlaza.table.paidPrice') }}
              <span class="pz-unit ml-1 normal-case font-normal">{{ t('modelPlaza.table.unitPerMillion') }}</span>
            </div>
          </th>
          <th
            colspan="3"
            class="border-l border-gray-100 pt-2 text-center dark:border-dark-700/60"
          >
            <div class="border-b border-gray-200 pb-2 text-gray-400 dark:border-dark-600 dark:text-dark-500">
              {{ t('modelPlaza.table.officialPrice') }}
              <span class="ml-1 normal-case font-normal text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.unitPerMillion') }}</span>
            </div>
          </th>
          <th
            rowspan="2"
            class="border-l border-gray-100 py-2.5 pl-3 pr-5 text-right align-middle dark:border-dark-700/60"
          >
            {{ t('modelPlaza.table.rate') }}
          </th>
        </tr>
        <tr
          class="border-b border-gray-200 text-left text-[11px] font-medium uppercase leading-4 tracking-wide text-gray-400 dark:border-dark-700 dark:text-dark-500"
        >
          <th class="pz-bg px-3 py-2 font-medium">{{ t('modelPlaza.table.input') }}</th>
          <th class="pz-bg px-3 py-2 font-medium">{{ t('modelPlaza.table.output') }}</th>
          <th class="pz-bg px-3 py-2 font-medium">{{ t('modelPlaza.table.cache') }}</th>
          <th class="border-l border-gray-100 px-3 py-2 font-medium dark:border-dark-700/60">
            {{ t('modelPlaza.table.input') }}
          </th>
          <th class="px-3 py-2 font-medium">{{ t('modelPlaza.table.output') }}</th>
          <th class="px-3 py-2 font-medium">{{ t('modelPlaza.table.cache') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="{ model: m, period, key } in rows"
          :key="key"
          class="border-b border-gray-100 transition-colors last:border-b-0 hover:bg-gray-50/70 dark:border-dark-800 dark:hover:bg-dark-800/50"
        >
          <!-- 模型名 + 非 token 计费模式徽章;分时时段行额外标注时段 -->
          <td class="model-plaza-table__model-cell border-r border-gray-100 py-2.5 pl-5 pr-4 align-middle dark:border-dark-700/60">
            <div class="flex flex-wrap items-center gap-1.5">
              <span class="model-plaza-table__logo-wrap" aria-hidden="true">
                <ModelIcon :model="m.platform" size="18px" />
              </span>
              <span class="font-medium text-gray-900 dark:text-white">{{ m.name }}</span>
              <button
                v-if="hasLongContext(m)"
                type="button"
                class="model-plaza-table__long-context-toggle inline-flex items-center rounded-md border px-2 py-1 text-xs font-medium leading-tight transition"
                :class="isLongContextExpanded(m)
                  ? 'border-primary-500 bg-primary-50 text-primary-700 dark:border-primary-400 dark:bg-primary-500/15 dark:text-primary-300'
                  : 'border-gray-200 text-gray-500 hover:border-gray-300 dark:border-dark-600 dark:text-dark-400 dark:hover:border-dark-500'"
                :aria-pressed="isLongContextExpanded(m)"
                @click="toggleLongContext(m)"
              >
                {{ isLongContextExpanded(m) ? t('modelPlaza.table.closeLongContext') : t('modelPlaza.table.viewLongContext') }}
              </button>
                <div
                  v-if="priceTabs(m).length > 1"
                  class="model-plaza-table__price-tabs mt-2 flex gap-1.5"
                  role="tablist"
              >
                <button
                  v-for="tab in priceTabs(m)"
                  :key="tab.key"
                  type="button"
                  role="tab"
                  class="relative min-w-[88px] flex-none rounded-md border px-2 py-1 text-[10px] font-medium leading-tight whitespace-pre-line transition"
                  :class="activePriceTab(m).key === tab.key
                    ? 'border-primary-500 bg-primary-50 text-primary-700 dark:border-primary-400 dark:bg-primary-500/15 dark:text-primary-300'
                    : 'border-gray-200 text-gray-500 hover:border-gray-300 dark:border-dark-600 dark:text-dark-400 dark:hover:border-dark-500'"
                  :aria-selected="activePriceTab(m).key === tab.key"
                  @click="selectPriceTab(m, tab.key)"
                >
                  {{ tab.label }}
                  <span
                    v-if="tab.current"
                    class="absolute -right-1.5 -top-2 rounded bg-emerald-500 px-1 py-0.5 text-[8px] leading-none text-white"
                  >
                    • {{ t('modelPlaza.table.current') }}
                  </span>
                </button>
              </div>
              <div
                v-if="priceTabs(m).length > 1"
                class="model-plaza-table__price-select mt-2 w-full"
                :class="{ 'model-plaza-table__price-select--many': priceTabs(m).length >= 3 }"
              >
                <Select
                  :model-value="activePriceTab(m).key"
                  :options="priceSelectOptions(m)"
                  :aria-label="t('modelPlaza.table.standard')"
                  :searchable="false"
                  @update:model-value="selectPriceTab(m, String($event))"
                />
              </div>
              <!-- 分时时段通过模型下方子 Tab 切换 -->
              <span
                v-if="platform && m.platform !== platform"
                :class="[
                  'inline-flex items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium',
                  platformBadgeLightClass(m.platform)
                ]"
              >
                {{ platformLabel(m.platform) }}
              </span>
              <span
                v-if="billingMode(m) !== BILLING_MODE_TOKEN"
                class="rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-500 dark:bg-dark-700/70 dark:text-dark-300"
              >
                {{ billingModeLabel(m) }}
              </span>
              <span
                v-if="m.long_context_basis === 'marginal'"
                class="rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-500 dark:bg-dark-700/70 dark:text-dark-300"
                :title="t('modelPlaza.table.tierHintMarginal')"
              >
                {{ t('modelPlaza.table.marginalBadge') }}
              </span>
              <span
                v-if="m.pricing?.max_reasoning_effort_multiplier"
                class="rounded-md bg-amber-50 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-900/20 dark:text-amber-300"
                :title="t('modelPlaza.table.maxReasoningMultiplierHint', { multiplier: m.pricing.max_reasoning_effort_multiplier })"
              >
                {{ t('modelPlaza.table.maxReasoningMultiplierBadge', { multiplier: m.pricing.max_reasoning_effort_multiplier }) }}
              </span>
            </div>
          </td>

          <!-- token 计费:输入 / 输出 / 缓存(写/读) -->
          <template v-if="billingMode(m) === BILLING_MODE_TOKEN">
            <td class="pz-cell px-3 py-2.5 align-middle font-mono font-semibold text-gray-900 dark:text-gray-50">
              {{ paidPerMillion(priceFor(m, 'input_price'), period, currencyFor(m)) }}
              <div
                v-for="(iv, idx) in (isLongContextExpanded(m) ? tokenIntervals(m) : [])"
                :key="idx"
                class="mt-1.5 border-t border-gray-200 pt-1.5 text-xs font-normal dark:border-dark-600"
              >
                <span class="mr-1 font-sans text-xs text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.longContext') }} {{ tierLabel(iv) }}</span>
                {{ paidPerMillion(intervalPrice(m, iv, 'input_price'), period, currencyFor(m)) }}
              </div>
            </td>
            <td class="pz-cell px-3 py-2.5 align-middle font-mono font-semibold text-gray-900 dark:text-gray-50">
              {{ paidPerMillion(priceFor(m, 'output_price'), period, currencyFor(m)) }}
              <div
                v-for="(iv, idx) in (isLongContextExpanded(m) ? tokenIntervals(m) : [])"
                :key="idx"
                class="mt-1.5 border-t border-gray-200 pt-1.5 text-xs font-normal dark:border-dark-600"
              >
                <span class="mr-1 font-sans text-xs text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.longContext') }} {{ tierLabel(iv) }}</span>
                {{ paidPerMillion(intervalPrice(m, iv, 'output_price'), period, currencyFor(m)) }}
              </div>
            </td>
            <td class="pz-cell px-3 py-2.5 align-middle">
              <template v-if="hasTierCachePricingForModel(m) && isLongContextExpanded(m)">
                <div
                  v-for="(iv, idx) in (isLongContextExpanded(m) ? tokenIntervals(m) : [])"
                  :key="idx"
                  class="whitespace-nowrap font-mono text-xs leading-5 text-gray-800 dark:text-gray-200"
                  :title="tierHint(m)"
                >
                  <template v-if="intervalPrice(m, iv, 'cache_write_price') != null || intervalPrice(m, iv, 'cache_write_1h_price') != null || intervalPrice(m, iv, 'cache_read_price') != null">
                    <span class="font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.cacheWriteShort') }}</span>
                    {{ paidPerMillion(intervalPrice(m, iv, 'cache_write_price'), period, currencyFor(m)) }}
                    <template v-if="intervalPrice(m, iv, 'cache_write_1h_price') != null"
                      ><span class="font-sans font-normal text-gray-400 dark:text-dark-500"> (1h </span>{{ paidPerMillion(intervalPrice(m, iv, 'cache_write_1h_price'), period, currencyFor(m))
                      }}<span class="font-sans font-normal text-gray-400 dark:text-dark-500">)</span></template
                    >
                    <span class="ml-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.cacheReadShort') }}</span>
                    {{ paidPerMillion(intervalPrice(m, iv, 'cache_read_price'), period, currencyFor(m)) }}
                  </template>
                  <span v-else class="text-gray-400 dark:text-dark-500">-</span>
                </div>
              </template>
              <div v-else-if="hasCachePricing(m)"
                class="space-y-0.5 font-mono text-xs text-gray-800 dark:text-gray-200"
              >
                <div>
                  <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.cacheWrite') }}</span>
                  {{ paidPerMillion(priceFor(m, 'cache_write_price'), period, currencyFor(m))
                  }}<template v-if="priceFor(m, 'cache_write_1h_price') != null"
                    ><span class="font-sans font-normal text-gray-400 dark:text-dark-500"> (1h </span>{{ paidPerMillion(priceFor(m, 'cache_write_1h_price'), period, currencyFor(m))
                    }}<span class="font-sans font-normal text-gray-400 dark:text-dark-500">)</span></template
                  >
                </div>
                <div>
                  <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.cacheRead') }}</span>
                  {{ paidPerMillion(priceFor(m, 'cache_read_price'), period, currencyFor(m)) }}
                </div>
                <div
                  v-for="(iv, idx) in (isLongContextExpanded(m) ? tokenIntervals(m) : [])"
                  :key="idx"
                  class="mt-1.5 border-t border-gray-200 pt-1.5 dark:border-dark-600"
                >
                  <div class="font-sans text-xs text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.longContext') }} {{ tierLabel(iv) }}</div>
                  <div v-if="intervalPrice(m, iv, 'cache_write_price') != null">
                    <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.cacheWrite') }}</span>
                    {{ paidPerMillion(intervalPrice(m, iv, 'cache_write_price'), period, currencyFor(m)) }}
                  </div>
                  <div v-if="intervalPrice(m, iv, 'cache_read_price') != null">
                    <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.cacheRead') }}</span>
                    {{ paidPerMillion(intervalPrice(m, iv, 'cache_read_price'), period, currencyFor(m)) }}
                  </div>
                  <span v-if="intervalPrice(m, iv, 'cache_write_price') == null && intervalPrice(m, iv, 'cache_read_price') == null">-</span>
                </div>
              </div>
              <span v-else class="text-gray-400 dark:text-dark-500">-</span>
            </td>
          </template>

          <!-- 按次 / 按图片计费:实付区整体合并,阶梯芯片或单一按次价 -->
          <template v-else>
            <td colspan="3" class="pz-cell px-3 py-2.5 align-middle">
              <div
                v-if="requestIntervals(m).length"
                class="flex flex-wrap items-center gap-1.5"
              >
                <span
                  v-for="(iv, idx) in requestIntervals(m)"
                  :key="idx"
                  class="inline-flex items-center gap-1 rounded-md bg-gray-100 px-2 py-0.5 font-mono text-xs text-gray-800 dark:bg-dark-700/60 dark:text-gray-200"
                >
                  <span class="font-sans text-gray-400 dark:text-dark-500">{{ tierLabel(iv) }}</span>
                  {{ paidRequestPrice(m, iv.per_request_price, currencyFor(m))
                  }}<span class="font-sans text-gray-400 dark:text-dark-500">{{ perUnitSuffix(m) }}</span>
                </span>
              </div>
              <template v-else-if="m.pricing?.per_request_price != null">
                <span class="font-mono font-semibold text-gray-900 dark:text-gray-50">
                  {{ paidRequestPrice(m, m.pricing.per_request_price, currencyFor(m)) }}
                </span>
                <span class="ml-1 text-xs text-gray-400 dark:text-dark-500">{{ perUnitSuffix(m) }}</span>
              </template>
              <span v-else class="text-gray-400 dark:text-dark-500">-</span>
            </td>
          </template>

          <!-- 官方价格(参考价,不乘倍率)。非 token 行量纲不同,官方三列固定显示 - -->
          <td
            class="border-l border-gray-100 px-3 py-2.5 align-middle font-mono text-xs text-gray-500 dark:border-dark-700/60 dark:text-dark-400"
          >
            {{ billingMode(m) === BILLING_MODE_TOKEN ? official(m.official_pricing?.input_price, currencyFor(m)) : '-' }}
          </td>
          <td class="px-3 py-2.5 align-middle font-mono text-xs text-gray-500 dark:text-dark-400">
            {{ billingMode(m) === BILLING_MODE_TOKEN ? official(m.official_pricing?.output_price, currencyFor(m)) : '-' }}
          </td>
          <td class="px-3 py-2.5 align-middle">
            <template v-if="hasTierCachePricing(officialIntervals(m)) && isLongContextExpanded(m)">
              <div
                v-for="(iv, idx) in (isLongContextExpanded(m) ? officialIntervals(m) : [])"
                :key="idx"
                class="whitespace-nowrap font-mono text-xs leading-5 text-gray-500 dark:text-dark-400"
                :title="t('modelPlaza.table.tierHint')"
              >
                <template v-if="iv.cache_write_price != null || iv.cache_write_1h_price != null || iv.cache_read_price != null">
                  <span class="font-sans text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.cacheWriteShort') }}</span>
                  {{ official(iv.cache_write_price, currencyFor(m)) }}
                  <template v-if="iv.cache_write_1h_price != null"
                    ><span class="font-sans text-gray-400 dark:text-dark-500"> (1h </span>{{ official(iv.cache_write_1h_price, currencyFor(m))
                    }}<span class="font-sans text-gray-400 dark:text-dark-500">)</span></template
                  >
                  <span class="ml-1 font-sans text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.cacheReadShort') }}</span>
                  {{ official(iv.cache_read_price, currencyFor(m)) }}
                </template>
                <span v-else class="text-gray-400 dark:text-dark-500">-</span>
              </div>
            </template>
            <div
              v-else-if="m.official_pricing && hasOfficialCache(m.official_pricing)"
              class="space-y-0.5 font-mono text-xs text-gray-500 dark:text-dark-400"
            >
              <div>
                <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.cacheWrite') }}</span>
                {{ official(m.official_pricing.cache_write_price, currencyFor(m))
                }}<template v-if="m.official_pricing.cache_write_1h_price != null"
                  ><span class="font-sans text-gray-400 dark:text-dark-500"> (1h </span>{{ official(m.official_pricing.cache_write_1h_price, currencyFor(m))
                  }}<span class="font-sans text-gray-400 dark:text-dark-500">)</span></template
                >
              </div>
              <div>
                <span class="mr-1 font-sans font-normal text-gray-400 dark:text-dark-500">{{ t('modelPlaza.table.cacheRead') }}</span>
                {{ official(m.official_pricing.cache_read_price, currencyFor(m)) }}
              </div>
            </div>
            <span v-else class="text-gray-400 dark:text-dark-500">-</span>
          </td>

          <!-- 折扣倍率(分时时段行展示 生效倍率×时段倍率;生图/生视频独立倍率行展示独立倍率;专属倍率划线展示原倍率) -->
          <td
            class="border-l border-gray-100 py-2.5 pl-3 pr-5 text-right align-middle font-mono text-xs dark:border-dark-700/60"
          >
            <span
              v-if="period"
              class="font-bold text-primary-600 dark:text-primary-400"
              :title="t('modelPlaza.table.timePricingRateHint', { rate: effectiveRate, multiplier: period.multiplier })"
              >{{ periodRate(period) }}x</span
            >
            <span
              v-else-if="usesIndependentImageRate(m) || usesIndependentVideoRate(m)"
              class="font-bold text-gray-700 dark:text-gray-300"
              >{{ requestRate(m) }}x</span
            >
            <template v-else-if="hasCustomRate">
              <span class="mr-1 text-gray-400 line-through dark:text-dark-500">{{ rateMultiplier }}x</span>
              <span class="font-bold text-primary-600 dark:text-primary-400">{{ effectiveRate }}x</span>
            </template>
            <span v-else class="font-bold text-gray-700 dark:text-gray-300">{{ effectiveRate }}x</span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatScaled } from '@/utils/pricing'
import ModelIcon from '@/components/common/ModelIcon.vue'
import Select from '@/components/common/Select.vue'
import { platformAccentColor, platformBadgeLightClass, platformLabel } from '@/utils/platformColors'
import {
  BILLING_MODE_TOKEN,
  BILLING_MODE_IMAGE,
  BILLING_MODE_VIDEO,
  type BillingMode
} from '@/constants/channel'
import type { PlazaModel, PlazaTimePricingPeriod } from '@/api/modelPlaza'
import type { UserPricingInterval } from '@/api/channels'
import { hasModelPlazaChannelContextPricing } from '@/components/model-plaza/modelPlaza'

const props = defineProps<{
  models: PlazaModel[]
  /** 分组平台;实付分区底色随平台着色,未知平台回退品牌青。 */
  platform?: string
  /** 分组默认倍率。 */
  rateMultiplier: number
  /** 用户专属倍率;与默认不同,实付价按此计算并划线展示原倍率。 */
  userRateMultiplier?: number | null
  /** 生图独立倍率:true 时图片计费模型的实付倍率取 imageRateMultiplier,不取分组/专属倍率。 */
  imageRateIndependent?: boolean
  imageRateMultiplier?: number | null
  /** 生视频独立倍率:true 时视频计费模型的实付倍率取 videoRateMultiplier,不取分组/专属倍率。 */
  videoRateIndependent?: boolean
  videoRateMultiplier?: number | null
  /**
   * 高峰窗口描述(含倍率与服务器时区标注),空串/缺省 = 分组未启用高峰。
   * 表格所有价格均为不含高峰因子的口径,该窗口仅用于分时时段行的 tooltip 披露:
   * 与高峰重叠的部分实付还会再乘高峰倍率。
   */
  peakWindow?: string
  peakRateMultiplier?: number | null
}>()

const { t } = useI18n()

/** 实付分区只从平台拿一个主色,浅底/标题/下划线全部由 scoped CSS 用 color-mix 派生。 */
const accentStyle = computed(() => ({ '--plaza-accent': platformAccentColor(props.platform ?? '') }))

const PER_MILLION = 1_000_000

/**
 * 展示顺序:
 * 1. token 计费的排在前,按图/按次计费的沉到末尾——它们的官方 token 价与实付的按张/按次价不同量纲,混排无意义;
 * 2. 组内按官方输出价从高到低,无官方价的排最后;
 * 3. 同价按名称降序(新版本号在前,如 gpt-5.6 先于 gpt-5.5)。
 */
const sortedModels = computed(() => {
  return [...props.models].sort((a, b) => {
    const ta = billingMode(a) === BILLING_MODE_TOKEN
    const tb = billingMode(b) === BILLING_MODE_TOKEN
    if (ta !== tb) return ta ? -1 : 1
    const pa = a.official_pricing?.output_price ?? null
    const pb = b.official_pricing?.output_price ?? null
    if (pa != null && pb != null && pa !== pb) return pb - pa
    if (pa != null && pb == null) return -1
    if (pa == null && pb != null) return 1
    return b.name.localeCompare(a.name)
  })
})

const effectiveRate = computed(() => props.userRateMultiplier ?? props.rateMultiplier)
const hasCustomRate = computed(
  () => props.userRateMultiplier != null && props.userRateMultiplier !== props.rateMultiplier
)

function billingMode(m: PlazaModel): BillingMode {
  return (m.pricing?.billing_mode || BILLING_MODE_TOKEN) as BillingMode
}

function billingModeLabel(m: PlazaModel): string {
  const mode = billingMode(m)
  if (mode === BILLING_MODE_IMAGE) return t('modelPlaza.table.perImage')
  if (mode === BILLING_MODE_VIDEO) return t('modelPlaza.table.perVideo')
  return t('modelPlaza.table.perRequest')
}

/** 价格统一保底 2 位小数,更长的有效小数原样保留。 */
const MIN_DECIMALS = 2

const selectedPriceTabs = ref<Record<string, string>>({})
const expandedLongContext = ref<Record<string, boolean>>({})

type PriceTab = { key: string; label: string; selectLabel: string; period: PlazaTimePricingPeriod | null; current: boolean }

/** 表格行:每个模型一行,分时价格在模型单元格内用子 Tab 切换。 */
interface PlazaRow {
  model: PlazaModel
  period: PlazaTimePricingPeriod | null
  key: string
}

const rows = computed<PlazaRow[]>(() =>
  sortedModels.value.flatMap((m) => {
    return [{ model: m, period: activePriceTab(m).period, key: `${m.platform}:${m.name}` }]
  })
)

/** 时段行的生效倍率 = 生效倍率 × 时段倍率(去掉浮点噪声)。 */
function periodRate(period: PlazaTimePricingPeriod): number {
  return Math.round(effectiveRate.value * period.multiplier * 1000) / 1000
}

function modelKey(m: PlazaModel): string {
  return `${m.platform}:${m.name}`
}

function priceTabs(m: PlazaModel): PriceTab[] {
  const schedule = m.time_pricing
  const periods = schedule?.periods ?? []
  if (periods.length === 0) return [{ key: 'standard', label: t('modelPlaza.table.standard'), selectLabel: t('modelPlaza.table.standard'), period: null, current: true }]
  const current = periods.find((period) => isCurrentPeriod(period.start_time, period.end_time, schedule?.weekdays_only))
  return [
    { key: 'standard', label: t('modelPlaza.table.standard'), selectLabel: t('modelPlaza.table.standard'), period: null, current: !current },
    ...periods.map((period, index) => ({
      key: `period-${index}`,
      label: `${schedule?.weekdays_only ? `${t('modelPlaza.table.timePricingWeekdays')} ` : ''}${formatPeriodLabel(period.start_time, period.end_time)}`,
      selectLabel: `${schedule?.weekdays_only ? `${t('modelPlaza.table.timePricingWeekdays')} ` : ''}${formatPeriodLabel(period.start_time, period.end_time).replace(/\n/g, ' ')}`,
      period,
      current: period === current,
    })),
  ]
}

function activePriceTab(m: PlazaModel): PriceTab {
  const tabs = priceTabs(m)
  const selected = selectedPriceTabs.value[modelKey(m)]
  return tabs.find((tab) => tab.key === selected) ?? tabs.find((tab) => tab.current) ?? tabs[0]
}

function selectPriceTab(m: PlazaModel, key: string): void {
  selectedPriceTabs.value[modelKey(m)] = key
}

function hasLongContext(m: PlazaModel): boolean {
  return tokenIntervals(m).length > 0 || officialIntervals(m).length > 0
}

function isLongContextExpanded(m: PlazaModel): boolean {
  return expandedLongContext.value[modelKey(m)] === true
}

function toggleLongContext(m: PlazaModel): void {
  const key = modelKey(m)
  expandedLongContext.value[key] = !expandedLongContext.value[key]
}

function formatPeriodLabel(start: string, end: string): string {
  const startDate = periodDatePart(start)
  const endDate = periodDatePart(end)
  const startTime = periodTimePart(start)
  const endTime = periodTimePart(end)
  if (startDate && endDate && startDate === endDate) return `${startDate}\n${startTime}–${endTime}`
  if (startDate || endDate) return `${periodDisplayPart(start)}\n–\n${periodDisplayPart(end)}`
  return `${startTime}\n${t('modelPlaza.table.to')}\n${endTime}`
}

function periodDisplayPart(value: string): string {
  const date = periodDatePart(value)
  const time = periodTimePart(value)
  return date ? `${date} ${time}` : time
}

function periodDatePart(value: string): string {
  const match = value.trim().match(/^(\d{4})[-/](\d{1,2})[-/](\d{1,2})/)
  return match ? `${match[2].padStart(2, '0')}-${match[3].padStart(2, '0')}` : ''
}

function periodTimePart(value: string): string {
  const match = value.trim().match(/(?:T|\s|^)(\d{1,2}:\d{2})/)
  return match ? match[1].padStart(5, '0') : value.trim()
}

function priceSelectOptions(m: PlazaModel) {
  return priceTabs(m).map((tab) => ({
    value: tab.key,
    label: `${tab.selectLabel}${tab.current ? ` · ${t('modelPlaza.table.current')}` : ''}`,
  }))
}

function isCurrentPeriod(start: string, end: string, weekdaysOnly = false): boolean {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: 'Asia/Shanghai', weekday: 'short', hour: '2-digit', minute: '2-digit', hour12: false,
  }).formatToParts(new Date())
  const weekday = parts.find((part) => part.type === 'weekday')?.value
  if (weekdaysOnly && (weekday === 'Sat' || weekday === 'Sun')) return false
  const current = Number(parts.find((part) => part.type === 'hour')?.value ?? 0) % 24 * 60
    + Number(parts.find((part) => part.type === 'minute')?.value ?? 0)
  const from = timeToMinutes(start)
  const to = timeToMinutes(end)
  if (from == null || to == null || from === to) return false
  return from < to ? current >= from && current < to : current >= from || current < to
}

function timeToMinutes(value: string): number | null {
  const match = value.trim().match(/^(\d{1,2}):(\d{2})/)
  if (!match) return null
  const hour = Number(match[1]); const minute = Number(match[2])
  return hour < 24 && minute < 60 ? hour * 60 + minute : null
}

/** 实付价 = 渠道单价 × 生效倍率(时段行再乘时段倍率),按 $/1M token 展示。 */
type PricingField = 'input_price' | 'output_price' | 'cache_write_price' | 'cache_write_1h_price' | 'cache_read_price'

function priceFor(m: PlazaModel, field: PricingField): number | null {
  return m.pricing?.[field] ?? m.official_pricing?.[field] ?? null
}

function intervalPrice(m: PlazaModel, interval: UserPricingInterval, field: PricingField): number | null {
  if (interval[field] != null) return interval[field]
  const official = officialIntervals(m).find((candidate) => candidate.tier_label === interval.tier_label)
  return official?.[field] ?? null
}

function currencyFor(m: PlazaModel): 'USD' | 'CNY' {
  const identity = `${m.platform} ${m.name}`.toLowerCase()
  return /deepseek|glm|kimi|moonshot|minimax|doubao|zhipu/.test(identity) ? 'CNY' : 'USD'
}

function currencySymbol(currency: 'USD' | 'CNY'): string {
  return currency === 'CNY' ? '¥' : '$'
}

function paidPerMillion(value: number | null | undefined, period: PlazaTimePricingPeriod | null = null, currency: 'USD' | 'CNY' = 'USD'): string {
  if (value == null) return '-'
  const rate = period ? periodRate(period) : effectiveRate.value
  return `${currencySymbol(currency)}${formatScaled(value, PER_MILLION, MIN_DECIMALS, rate).replace(/^\$/, '')}`
}

/** 图片计费模型且分组开启生图独立倍率:实付倍率取独立倍率,与计费口径一致。 */
function usesIndependentImageRate(m: PlazaModel): boolean {
  return billingMode(m) === BILLING_MODE_IMAGE && props.imageRateIndependent === true
}

/** 视频计费模型且分组开启生视频独立倍率:实付倍率取独立倍率,与计费口径一致。 */
function usesIndependentVideoRate(m: PlazaModel): boolean {
  return billingMode(m) === BILLING_MODE_VIDEO && props.videoRateIndependent === true
}

/** 按次/按图片/按视频行的生效倍率。 */
function requestRate(m: PlazaModel): number {
  if (usesIndependentImageRate(m)) return props.imageRateMultiplier ?? 1
  if (usesIndependentVideoRate(m)) return props.videoRateMultiplier ?? 1
  return effectiveRate.value
}

/** 按次 / 按图片单价(乘该行生效倍率,不换算 1M)。 */
function paidRequestPrice(m: PlazaModel, value: number | null | undefined, currency: 'USD' | 'CNY' = 'USD'): string {
  if (value == null) return '-'
  return `${currencySymbol(currency)}${formatScaled(value, 1, MIN_DECIMALS, requestRate(m)).replace(/^\$/, '')}`
}

/** 官方参考价不乘倍率。 */
function official(value: number | null | undefined, currency: 'USD' | 'CNY' = 'USD'): string {
  if (value == null) return '-'
  return `${currencySymbol(currency)}${formatScaled(value, PER_MILLION, MIN_DECIMALS).replace(/^\$/, '')}`
}

/** 非 token 计费的单位后缀:按图片 → “/ 张”,视频 → “/ 秒”,按次 → “/ 次”。 */
function perUnitSuffix(m: PlazaModel): string {
  const mode = billingMode(m)
  if (mode === BILLING_MODE_IMAGE) return t('modelPlaza.table.perUnitImage')
  if (mode === BILLING_MODE_VIDEO) return t('modelPlaza.table.perUnitVideo')
  return t('modelPlaza.table.perUnitRequest')
}

function hasCachePricing(m: PlazaModel): boolean {
  return priceFor(m, 'cache_write_price') != null
    || priceFor(m, 'cache_write_1h_price') != null
    || priceFor(m, 'cache_read_price') != null
    || tokenIntervals(m).some((iv) => iv.cache_write_price != null || iv.cache_write_1h_price != null || iv.cache_read_price != null)
}

function hasOfficialCache(o: NonNullable<PlazaModel['official_pricing']>): boolean {
  return o.cache_write_price != null || o.cache_read_price != null || o.cache_write_1h_price != null
}

/** 上下文档位按下限升序展示(后端已升序,此处兜底)。 */
function sortByContext(intervals: UserPricingInterval[]): UserPricingInterval[] {
  return [...intervals].sort((a, b) => a.min_tokens - b.min_tokens)
}

/** 官方阶梯(后端按目录规则合成,不受分组开关影响)。 */
function officialIntervals(m: PlazaModel): UserPricingInterval[] {
  const channel = tokenIntervals(m)
  if (channel.length === 0) return []
  return sortByContext(m.official_pricing?.intervals ?? []).filter((official) => channel.some((configured) =>
    configured.tier_label === official.tier_label
    || (configured.min_tokens === official.min_tokens && configured.max_tokens === official.max_tokens)
  ))
}

/** 任一档带缓存价才按档渲染缓存列;否则沿用平价的写入/读取两行。 */
function hasTierCachePricing(intervals: UserPricingInterval[]): boolean {
  return intervals.some((iv) => iv.cache_write_price != null || iv.cache_write_1h_price != null || iv.cache_read_price != null)
}

function hasTierCachePricingForModel(m: PlazaModel): boolean {
  return tokenIntervals(m).some((iv) => intervalPrice(m, iv, 'cache_write_price') != null
    || intervalPrice(m, iv, 'cache_write_1h_price') != null
    || intervalPrice(m, iv, 'cache_read_price') != null)
}

/** 档位说明:整单按档计价,或(平台旧规则)仅超出部分按档计价。 */
function tierHint(m: PlazaModel): string {
  return m.long_context_basis === 'marginal'
    ? t('modelPlaza.table.tierHintMarginal')
    : t('modelPlaza.table.tierHint')
}
/** 按次/按图模式的阶梯定价(仅保留配了按次价的档位)。 */
function requestIntervals(m: PlazaModel): UserPricingInterval[] {
  return (m.pricing?.intervals ?? []).filter((iv) => iv.per_request_price != null)
}

/** token 模式的整单上下文档位；后端不会在此契约中输出边际计价规则。 */
function tokenIntervals(m: PlazaModel): UserPricingInterval[] {
  if (billingMode(m) !== BILLING_MODE_TOKEN) return []
  const intervals = hasModelPlazaChannelContextPricing(m) ? m.pricing?.intervals ?? [] : []
  return sortByContext(intervals.filter((iv) => iv.min_tokens > 0 && (
    iv.input_price != null
    || iv.output_price != null
    || iv.cache_write_price != null
    || iv.cache_write_1h_price != null
    || iv.cache_read_price != null
  )))
}

/**
 * 档位标签:优先后端/管理员给出的 tier_label,否则按区间生成统一形态——
 * 有上限为「≤上限」,末档为「>下限」;档位升序排列,相邻的 ≤100K / ≤200K 即表示 (100K,200K]。
 */
function tierLabel(iv: UserPricingInterval): string {
  if (iv.tier_label) return iv.tier_label
  const { min_tokens: min, max_tokens: max } = iv
  return max == null ? `>${formatTokenCount(min)}` : `≤${formatTokenCount(max)}`
}

function formatTokenCount(n: number): string {
  if (n >= 1_000_000) return `${trimZero(n / 1_000_000)}M`
  if (n >= 1_000) return `${trimZero(n / 1_000)}K`
  return String(n)
}

function trimZero(n: number): string {
  return String(Math.round(n * 100) / 100)
}
</script>

<style scoped>
/* 实付分区配色统一从 --plaza-accent(平台主色)派生,新增平台无需扩展样式 */
.plaza-pricing-table {
  --pz-title: color-mix(in srgb, var(--plaza-accent) 88%, black);
  --pz-bg: color-mix(in srgb, var(--plaza-accent) 7%, transparent);
  --pz-bg-hover: color-mix(in srgb, var(--plaza-accent) 13%, transparent);
}

.model-plaza-table__logo-wrap {
  display: inline-grid;
  width: 30px;
  height: 30px;
  flex: none;
  place-items: center;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  background: #fff;
  box-shadow: 0 1px 2px rgb(0 0 0 / 8%);
}

.model-plaza-table__price-tabs {
  max-width: 100%;
  overflow: hidden;
}

.model-plaza-table__model-cell {
  container-type: inline-size;
}

.model-plaza-table__price-select {
  display: none;
}

@media (max-width: 640px) {
  .model-plaza-table__price-tabs {
    display: none;
  }

  .model-plaza-table__price-select {
    display: block;
  }
}

@container (max-width: 300px) {
  .model-plaza-table__price-tabs--many {
    display: none;
  }

  .model-plaza-table__price-select--many {
    display: block;
  }
}

.dark .plaza-pricing-table {
  --pz-title: color-mix(in srgb, var(--plaza-accent) 70%, white);
  --pz-bg: color-mix(in srgb, var(--plaza-accent) 6%, transparent);
  --pz-bg-hover: color-mix(in srgb, var(--plaza-accent) 10%, transparent);
}

.pz-bg,
.pz-cell {
  background-color: var(--pz-bg);
}

.pz-cell {
  transition: background-color 150ms cubic-bezier(0.4, 0, 0.2, 1);
}

tbody tr:hover .pz-cell {
  background-color: var(--pz-bg-hover);
}

.pz-title {
  /* color-mix 不可用的老浏览器回退为平台原色 */
  color: var(--plaza-accent);
  color: var(--pz-title);
  border-color: color-mix(in srgb, var(--pz-title) 30%, transparent);
}

.pz-unit {
  color: color-mix(in srgb, var(--pz-title) 62%, transparent);
}
</style>
