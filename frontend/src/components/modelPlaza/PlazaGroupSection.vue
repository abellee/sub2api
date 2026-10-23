<template>
  <section
    class="overflow-hidden rounded-2xl border bg-white shadow-card dark:bg-dark-800/50"
    :class="[platformBorderStrongClass(group.platform)]"
  >
    <!-- 分组头部:品牌图标 | 名称与备注同行靠左 + 计费方式标签 | 倍率/专属/订阅徽章 -->
    <header class="border-b border-gray-100 px-5 py-4 dark:border-dark-700/60">
      <div class="flex items-start gap-3">
        <PlatformIcon :platform="group.platform as GroupPlatform" size="md" class="mt-0.5 shrink-0 text-gray-500 dark:text-gray-300" />
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
            <span class="text-base font-bold text-gray-900 dark:text-white">{{ group.name }}</span>
            <span
              v-if="group.user_rate_multiplier != null && group.user_rate_multiplier !== group.rate_multiplier"
              class="text-xs text-gray-400 line-through dark:text-dark-500"
            >
              {{ group.rate_multiplier }}x
            </span>
            <span
              class="rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-semibold text-gray-700 dark:bg-dark-700 dark:text-dark-200"
            >
              {{ group.user_rate_multiplier ?? group.rate_multiplier }}x
            </span>
            <span
              v-if="group.is_exclusive"
              class="inline-flex items-center gap-1 rounded-md bg-purple-50 px-2 py-0.5 text-xs font-medium text-purple-600 dark:bg-purple-900/20 dark:text-purple-400"
            >
              <Icon name="shield" size="xs" class="h-3 w-3" />
              {{ t('modelPlaza.badges.exclusive') }}
            </span>
            <span
              v-if="group.subscription_type === 'subscription'"
              class="inline-flex items-center rounded-md bg-violet-50 px-2 py-0.5 text-xs font-medium text-violet-600 dark:bg-violet-900/20 dark:text-violet-400"
            >
              {{ t('modelPlaza.badges.subscription') }}
            </span>
            <span
              class="ml-auto inline-flex shrink-0 items-center rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-dark-300"
            >
              {{ billingTypeLabel }}
            </span>
          </div>
          <p
            v-if="group.description"
            class="mt-1.5 whitespace-pre-line text-sm leading-relaxed text-gray-500 [overflow-wrap:anywhere] dark:text-dark-400"
          >
            {{ group.description }}
          </p>
          <p
            v-if="peakNote"
            class="mt-1.5 inline-flex items-center gap-1 text-xs text-amber-600 dark:text-amber-400"
          >
            <Icon name="clock" size="xs" class="h-3 w-3" />
            {{ peakNote }}
          </p>
          <p
            v-if="longContextNote"
            class="mt-1.5 flex items-center gap-1 text-xs text-gray-500 dark:text-dark-400"
          >
            <Icon name="infoCircle" size="xs" class="h-3 w-3" />
            {{ longContextNote }}
          </p>
        </div>
      </div>
    </header>

    <!-- 模型价格表:整行(含 hover 底色/分区底色)顶到卡片边缘,左右留白由表格首列/末列的 padding 提供 -->
    <div :style="scrollable ? { '--pz-body-max': 'min(52vh, 520px)' } : undefined">
      <PlazaModelPricingTable
        v-if="group.models.length > 0"
        :models="group.models"
        :platform="group.platform"
        :rate-multiplier="group.rate_multiplier"
        :user-rate-multiplier="group.user_rate_multiplier ?? null"
        :image-rate-independent="group.image_rate_independent"
        :image-rate-multiplier="group.image_rate_multiplier"
        :video-rate-independent="group.video_rate_independent"
        :video-rate-multiplier="group.video_rate_multiplier"
        :peak-window="peakWindow"
        :peak-rate-multiplier="group.peak_rate_multiplier"
        :compact-model="scrollable"
      />
      <p v-else class="px-5 py-4 text-center text-sm text-gray-400 dark:text-dark-500">
        {{ t('modelPlaza.detail.noModels') }}
      </p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import PlazaModelPricingTable from './PlazaModelPricingTable.vue'
import type { ModelPlazaGroup } from '@/api/modelPlaza'
import type { GroupPlatform } from '@/types'
import { platformBorderStrongClass } from '@/utils/platformColors'
import { hasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { useAppStore } from '@/stores/app'

const props = withDefaults(defineProps<{
  group: ModelPlazaGroup
  /** 模型较多时只在卡片内滚动(分组头部与表头固定),用于弹窗等受限空间。 */
  scrollable?: boolean
}>(), {
  scrollable: false
})

const { t } = useI18n()
const appStore = useAppStore()

/** 计费方式标签:组内存在按图/视频/按次模型 → 按次计费,否则按量计费。 */
const billingTypeLabel = computed(() => {
  const hasByRequest = props.group.models.some(
    (m) => m.pricing?.billing_mode && m.pricing.billing_mode !== 'token'
  )
  return hasByRequest ? t('modelPlaza.table.perRequest') : t('modelPlaza.group.billingByToken')
})

/** 高峰窗口描述(含倍率与服务器时区标注);分组未启用高峰为空串。 */
const peakWindow = computed(() => {
  if (!hasPeakRate(props.group)) return ''
  return formatPeakRateWindow(
    props.group,
    serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset)
  )
})

const peakNote = computed(() => {
  if (!peakWindow.value) return ''
  return t('modelPlaza.detail.peakNote', {
    window: peakWindow.value,
    multiplier: props.group.peak_rate_multiplier
  })
})

/**
 * 分组关闭了长上下文阶梯、但组内有模型官方带阶梯时提示:实付列只展示基础档,
 * 官方阶梯仅供参考。字段缺失(旧后端)不提示。
 */
const longContextNote = computed(() => {
  if (props.group.long_context_pricing_enabled !== false) return ''
  const hasOfficialLadder = props.group.models.some(
    (m) => (m.official_pricing?.intervals?.length ?? 0) > 1
  )
  return hasOfficialLadder ? t('modelPlaza.detail.longContextDisabledNote') : ''
})

</script>
