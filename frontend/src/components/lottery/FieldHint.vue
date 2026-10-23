<script setup lang="ts">
/**
 * FieldHint —— 表单项标签 + ⓘ 帮助图标（悬停/聚焦显示说明气泡）。
 * 与 Sub2API 主站 input-label 样式一致，纯 CSS tooltip 无依赖。
 */
import { ref } from 'vue'

defineProps<{
  /** 字段名。 */
  label: string
  /** 悬停说明。 */
  hint: string
}>()

const show = ref(false)
</script>

<template>
  <span class="field-hint-label">
    <span class="input-label !mb-0">{{ label }}</span>
    <span
      class="hint-anchor"
      tabindex="0"
      role="note"
      :aria-label="hint"
      @mouseenter="show = true"
      @mouseleave="show = false"
      @focus="show = true"
      @blur="show = false"
    >
      <!-- Heroicons outline: information-circle -->
      <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor" aria-hidden="true">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M11.25 11.25l.041-.02a.75.75 0 011.063.852l-.708 2.836a.75.75 0 001.063.853l.041-.021M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9-3.75h.008v.008H12V8.25z"
        />
      </svg>
      <transition name="hint-pop">
        <span v-if="show" class="hint-bubble" role="tooltip">{{ hint }}</span>
      </transition>
    </span>
  </span>
</template>

<style scoped>
.field-hint-label {
  @apply mb-1.5 flex items-center gap-1 text-sm font-medium text-gray-700 dark:text-gray-300;
}
.hint-anchor {
  @apply relative flex h-4 w-4 cursor-help items-center justify-center text-gray-400;
  @apply transition-colors hover:text-primary-500 dark:text-dark-500 dark:hover:text-primary-400;
}
.hint-bubble {
  @apply absolute bottom-full left-1/2 z-10 mb-2 w-56 -translate-x-1/2;
  @apply rounded-lg bg-gray-900 px-3 py-2 text-xs font-normal leading-relaxed text-gray-100;
  @apply shadow-lg dark:bg-dark-700 dark:text-gray-200;
}
.hint-bubble::after {
  content: '';
  position: absolute;
  top: 100%;
  left: 50%;
  transform: translateX(-50%);
  border: 5px solid transparent;
  border-top-color: #111827;
}
:root.dark .hint-bubble::after {
  border-top-color: #334155;
}
.hint-pop-enter-active,
.hint-pop-leave-active {
  transition: opacity 120ms ease, transform 120ms ease;
}
.hint-pop-enter-from,
.hint-pop-leave-to {
  opacity: 0;
  transform: translate(-50%, 4px);
}
</style>
