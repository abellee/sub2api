<template>
  <div class="space-y-5">
    <!-- 页头(独立形态下展示标题;后台形态 AppHeader 已有页面标题) -->
    <div v-if="!embedded">
      <h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-white sm:text-3xl">{{ t('modelPlaza.title') }}</h1>
      <p class="mt-1.5 text-sm text-gray-500 dark:text-dark-400">{{ t('modelPlaza.description') }}</p>
    </div>

    <div class="flex gap-3 rounded-lg border border-amber-200 border-l-4 bg-amber-50 px-4 py-3 text-xs leading-6 text-amber-900 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-100" role="note">
      <Icon name="infoCircle" size="sm" class="mt-1 h-4 w-4 shrink-0" />
      <div class="min-w-0">
        <strong class="block font-semibold">{{ t('modelPlaza.pricingNotice.title') }}</strong>
        <span>{{ t('modelPlaza.pricingNotice.body') }}</span>
        <span class="mt-1 flex flex-wrap gap-x-3 gap-y-1">
          <a class="underline underline-offset-2" href="https://github.com/Wei-Shaw/model-price-repo" target="_blank" rel="noopener noreferrer">{{ t('modelPlaza.pricingNotice.data') }}</a>
          <span class="text-amber-700/70 dark:text-amber-200/70">{{ t('modelPlaza.pricingNotice.official') }}:</span>
          <a class="underline underline-offset-2" href="https://docs.anthropic.com/en/docs/about-claude/models/all-models" target="_blank" rel="noopener noreferrer">Anthropic</a>
          <a class="underline underline-offset-2" href="https://platform.openai.com/docs/pricing" target="_blank" rel="noopener noreferrer">OpenAI</a>
          <a class="underline underline-offset-2" href="https://ai.google.dev/gemini-api/docs/pricing" target="_blank" rel="noopener noreferrer">Gemini</a>
          <a class="underline underline-offset-2" href="https://docs.x.ai/docs/models" target="_blank" rel="noopener noreferrer">Grok</a>
          <a class="underline underline-offset-2" href="https://api-docs.deepseek.com/quick_start/pricing" target="_blank" rel="noopener noreferrer">DeepSeek</a>
          <a class="underline underline-offset-2" href="https://platform.moonshot.cn/docs/pricing" target="_blank" rel="noopener noreferrer">Kimi</a>
          <a class="underline underline-offset-2" href="https://open.bigmodel.cn/pricing" target="_blank" rel="noopener noreferrer">GLM</a>
        </span>
      </div>
    </div>

    <!-- 全局价格说明(管理员配置,Markdown) -->
    <div
      v-if="descriptionHtml"
      class="plaza-description rounded-2xl border border-gray-100 bg-white px-5 py-4 text-sm shadow-card dark:border-dark-700/50 dark:bg-dark-800/50"
      v-html="descriptionHtml"
    ></div>

    <!-- 未登录提示 -->
    <p
      v-if="!isAuthenticated"
      class="flex items-center gap-1.5 text-xs text-gray-400 dark:text-dark-500"
    >
      <Icon name="infoCircle" size="xs" class="h-3.5 w-3.5" />
      {{ t('modelPlaza.anonymousHint') }}
    </p>

    <!-- 加载/错误/空 -->
    <div v-if="loading" class="flex min-h-[240px] items-center justify-center">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-600/25 border-t-primary-600 dark:border-primary-400/25 dark:border-t-primary-400"></div>
    </div>
    <div
      v-else-if="error"
      class="rounded-2xl border border-red-200 bg-red-50 px-5 py-8 text-center text-sm text-red-600 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-300"
    >
      {{ t('modelPlaza.loadFailed') }}
    </div>
    <template v-else>
      <!-- 筛选区:平台 → 分组 → 倍率 -->
      <PlazaFilterBar
        :platforms="platforms"
        :groups="groupOptions"
        :rates="rates"
        :platform="selectedPlatform"
        :group-id="selectedGroupId"
        :rate="selectedRate"
        :search="searchQuery"
        @update:platform="selectedPlatform = $event"
        @update:group-id="selectedGroupId = $event"
        @update:rate="selectedRate = $event"
        @update:search="searchQuery = $event"
      />

      <!-- 分组分节的模型清单(默认按生效倍率升序) -->
      <div v-if="filteredGroups.length > 0" class="space-y-5">
        <PlazaGroupSection v-for="g in filteredGroups" :key="g.id" :group="g" />
      </div>
      <div
        v-else
        class="rounded-2xl border border-dashed border-gray-300 px-5 py-12 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-dark-400"
      >
        {{ searchActive ? t('modelPlaza.noSearchResult') : t('modelPlaza.empty') }}
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import Icon from '@/components/icons/Icon.vue'
import PlazaFilterBar from './PlazaFilterBar.vue'
import PlazaGroupSection from './PlazaGroupSection.vue'
import { modelPlazaEntryKind, modelPlazaProviderForGroup } from '@/components/model-plaza/modelPlaza'
import type { ModelPlazaGroup, ModelPlazaResponse } from '@/api/modelPlaza'
import { useAuthStore } from '@/stores/auth'

const props = defineProps<{
  response: ModelPlazaResponse | null
  loading: boolean
  error?: boolean
  /** 后台内嵌形态(AppLayout 内):隐藏页头。 */
  embedded?: boolean
}>()

const { t } = useI18n()
const authStore = useAuthStore()
const isAuthenticated = computed(() => authStore.isAuthenticated)

const selectedPlatform = ref<string>('all')
const selectedGroupId = ref<number | 'all'>('all')
const selectedRate = ref<number | 'all'>('all')
const searchQuery = ref('')

const searchActive = computed(() => searchQuery.value.trim() !== '')

const descriptionHtml = computed(() => {
  const md = props.response?.description?.trim()
  if (!md) return ''
  return DOMPurify.sanitize(marked.parse(md) as string)
})

/** 生效倍率 = 用户专属倍率 ?? 分组默认倍率。 */
function effectiveRate(g: ModelPlazaGroup): number {
  return g.user_rate_multiplier ?? g.rate_multiplier
}

const platforms = computed(() =>
  [...new Set((props.response?.groups ?? []).map(modelPlazaProviderForGroup).filter(Boolean))].sort()
)

function isMediaGroup(group: ModelPlazaGroup): boolean {
  return group.models.some((model) => {
    const kind = modelPlazaEntryKind(group, model)
    return kind === 'image' || kind === 'video'
  })
}

function compareGroups(a: ModelPlazaGroup, b: ModelPlazaGroup): number {
  const mediaOrder = Number(isMediaGroup(b)) - Number(isMediaGroup(a))
  return mediaOrder || effectiveRate(a) - effectiveRate(b) || a.name.localeCompare(b.name)
}

const orderedGroups = computed(() => [...(props.response?.groups ?? [])].sort(compareGroups))

const groupOptions = computed(() =>
  orderedGroups.value.map((g) => ({
    id: g.id,
    name: g.name,
    platform: modelPlazaProviderForGroup(g),
    rate: effectiveRate(g)
  }))
)

watch([selectedPlatform, groupOptions], () => {
  if (
    selectedGroupId.value !== 'all' &&
    selectedPlatform.value !== 'all' &&
    !groupOptions.value.some(
      (group) => group.id === selectedGroupId.value && group.platform === selectedPlatform.value
    )
  ) {
    selectedGroupId.value = 'all'
  }
})

/** 全量生效倍率;当前组合下不可用的项由 FilterBar 置灰而非隐藏。 */
const rates = computed(() =>
  [...new Set(
    (props.response?.groups ?? [])
      .filter((group) => selectedPlatform.value === 'all' || modelPlazaProviderForGroup(group) === selectedPlatform.value)
      .map(effectiveRate)
  )].sort((a, b) => a - b)
)

/** 数据刷新后选中的倍率可能不复存在,重置为全部。 */
watch(rates, (list) => {
  if (selectedRate.value !== 'all' && !list.includes(selectedRate.value)) {
    selectedRate.value = 'all'
  }
})

const filteredGroups = computed(() => {
  let groups = orderedGroups.value
  if (selectedPlatform.value !== 'all') {
    groups = groups.filter((g) => modelPlazaProviderForGroup(g) === selectedPlatform.value)
  }
  if (selectedGroupId.value !== 'all') {
    groups = groups.filter((g) => g.id === selectedGroupId.value)
  }
  if (selectedRate.value !== 'all') {
    groups = groups.filter((g) => effectiveRate(g) === selectedRate.value)
  }
  // 模型名搜索:分组内只留命中的模型,整组无命中则隐藏该分组。
  const q = searchQuery.value.trim().toLowerCase()
  if (q) {
    groups = groups
      .map((g) => ({ ...g, models: g.models.filter((m) => m.name.toLowerCase().includes(q)) }))
      .filter((g) => g.models.length > 0)
  }
  // 专属倍率会改变生效值；媒体分组始终优先于倍率排序。
  return [...groups].sort(compareGroups)
})
</script>

<style scoped>
.plaza-description {
  line-height: 1.7;
  overflow-wrap: anywhere;
}

.plaza-description :deep(h1),
.plaza-description :deep(h2),
.plaza-description :deep(h3) {
  @apply mb-2 mt-3 font-semibold text-gray-900 first:mt-0 dark:text-white;
}

.plaza-description :deep(p) {
  @apply mb-2 text-gray-700 last:mb-0 dark:text-dark-200;
}

.plaza-description :deep(a) {
  @apply text-primary-600 underline underline-offset-4 hover:text-primary-700 dark:text-primary-300;
}

.plaza-description :deep(ul) {
  @apply mb-2 list-disc pl-5;
}

.plaza-description :deep(ol) {
  @apply mb-2 list-decimal pl-5;
}

.plaza-description :deep(li) {
  @apply mb-0.5 text-gray-700 dark:text-dark-200;
}

.plaza-description :deep(code) {
  @apply rounded bg-gray-100 px-1.5 py-0.5 font-mono text-xs dark:bg-dark-800;
}

.plaza-description :deep(blockquote) {
  @apply my-2 border-l-4 border-gray-300 pl-3 text-gray-600 dark:border-dark-600 dark:text-dark-300;
}
</style>
