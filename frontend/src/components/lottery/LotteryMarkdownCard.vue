<script setup lang="ts">
/**
 * LotteryMarkdownCard —— Markdown 说明卡片（独立组件）。
 * 长内容默认折叠（约 260px），可展开/收起；Markdown 经 marked + DOMPurify 消毒。
 */
import { computed, onMounted, ref, watch } from 'vue'
import { renderMarkdown } from '../../utils/markdown'

const props = defineProps<{
  title?: string
  source: string
  /** 折叠阈值（px），0 = 不折叠。 */
  collapseHeight?: number
  /** 嵌入模式：不渲染卡片外壳（供 tab 容器内使用）。 */
  embedded?: boolean
}>()

const html = computed(() => renderMarkdown(props.source ?? ''))
const container = ref<HTMLElement | null>(null)
const overflowing = ref(false)
const expanded = ref(false)

const limit = computed(() => props.collapseHeight ?? 260)

function measure() {
  const el = container.value
  if (!el || !limit.value) {
    overflowing.value = false
    return
  }
  overflowing.value = el.scrollHeight > limit.value + 24
}

onMounted(measure)
watch(
  () => [props.source, html.value],
  () => {
    expanded.value = false
    measure()
  }
)
</script>

<template>
  <!-- 嵌入模式：只有可折叠的 Markdown 正文 -->
  <div v-if="embedded" class="relative">
    <div
      ref="container"
      class="lottery-markdown text-sm leading-relaxed text-gray-600 dark:text-gray-300"
      :class="{ collapsed: overflowing && !expanded }"
      :style="overflowing && !expanded ? { maxHeight: limit + 'px' } : undefined"
      v-html="html"
    ></div>
    <div
      v-if="overflowing && !expanded"
      class="pointer-events-none absolute inset-x-0 bottom-0 h-16 bg-gradient-to-t from-white to-transparent dark:from-dark-800"
    ></div>
    <div v-if="overflowing" class="mt-3 text-center">
      <button class="btn btn-ghost btn-sm" @click="expanded = !expanded">
        {{ expanded ? '收起' : '展开全文' }}
      </button>
    </div>
  </div>

  <!-- 卡片模式：带标题外壳 -->
  <section v-else class="card">
    <div class="card-header">
      <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ title }}</h2>
    </div>
    <div class="card-body">
      <div class="relative">
        <div
          ref="container"
          class="lottery-markdown text-sm leading-relaxed text-gray-600 dark:text-gray-300"
          :class="{ collapsed: overflowing && !expanded }"
          :style="overflowing && !expanded ? { maxHeight: limit + 'px' } : undefined"
          v-html="html"
        ></div>
        <div
          v-if="overflowing && !expanded"
          class="pointer-events-none absolute inset-x-0 bottom-0 h-16 bg-gradient-to-t from-white to-transparent dark:from-dark-800"
        ></div>
      </div>
      <div v-if="overflowing" class="mt-3 text-center">
        <button class="btn btn-ghost btn-sm" @click="expanded = !expanded">
          {{ expanded ? '收起' : '展开全文' }}
        </button>
      </div>
    </div>
  </section>
</template>

<style scoped>
/* Markdown 内容排版（与主站自定义页面同风格） */
.lottery-markdown :deep(h1),
.lottery-markdown :deep(h2),
.lottery-markdown :deep(h3) {
  @apply mt-4 mb-2 font-semibold text-gray-900 first:mt-0 dark:text-white;
}
.lottery-markdown :deep(h1) {
  @apply text-lg;
}
.lottery-markdown :deep(h2) {
  @apply text-base;
}
.lottery-markdown :deep(h3) {
  @apply text-sm;
}
.lottery-markdown :deep(p) {
  @apply my-2 first:mt-0 last:mb-0;
}
.lottery-markdown :deep(ul) {
  @apply my-2 list-disc space-y-1 pl-5;
}
.lottery-markdown :deep(ol) {
  @apply my-2 list-decimal space-y-1 pl-5;
}
.lottery-markdown :deep(a) {
  @apply text-primary-600 underline underline-offset-2 dark:text-primary-400;
}
.lottery-markdown :deep(code) {
  @apply rounded-md bg-gray-100 px-1.5 py-0.5 font-mono text-xs text-gray-800 dark:bg-dark-700 dark:text-gray-200;
}
.lottery-markdown :deep(pre) {
  @apply my-2 overflow-x-auto rounded-xl bg-gray-900 p-3 dark:bg-dark-900;
}
.lottery-markdown :deep(pre code) {
  @apply bg-transparent p-0 text-gray-100;
}
.lottery-markdown :deep(blockquote) {
  @apply my-2 border-l-4 border-primary-300 pl-3 text-gray-500 dark:border-primary-700 dark:text-dark-400;
}
.lottery-markdown :deep(table) {
  @apply my-2 w-full border-collapse text-xs;
}
.lottery-markdown :deep(th),
.lottery-markdown :deep(td) {
  @apply border border-gray-200 px-2 py-1 dark:border-dark-600;
}
.lottery-markdown :deep(img) {
  @apply my-2 max-w-full rounded-lg;
}
.lottery-markdown :deep(hr) {
  @apply my-3 border-gray-100 dark:border-dark-700;
}
/* 折叠态底部留白，避免文字贴着渐变遮罩 */
.lottery-markdown.collapsed {
  overflow: hidden;
}
</style>
