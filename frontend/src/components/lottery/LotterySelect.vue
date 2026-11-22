<script setup lang="ts">
/**
 * LotterySelect —— 与 Sub2API 主站 Select 同款视觉的下拉选择。
 * 下拉面板内联渲染（不 Teleport），跟随触发器定位，规避跨层层级/命中问题。
 */
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

export interface LotterySelectOption {
  value: string
  label: string
}

const props = withDefaults(
  defineProps<{
    modelValue?: string
    options: LotterySelectOption[]
    placeholder?: string
    disabled?: boolean
  }>(),
  { placeholder: '请选择', disabled: false, modelValue: '' }
)

const emit = defineEmits<{ (e: 'update:modelValue', value: string): void }>()

const isOpen = ref(false)
const containerRef = ref<HTMLElement | null>(null)

const selectedLabel = computed(
  () => props.options.find((o) => o.value === props.modelValue)?.label ?? props.placeholder
)

function toggle() {
  if (props.disabled) return
  isOpen.value = !isOpen.value
}

function select(value: string) {
  emit('update:modelValue', value)
  isOpen.value = false
}

function onClickOutside(e: MouseEvent) {
  const target = e.target as HTMLElement
  if (isOpen.value && !containerRef.value?.contains(target)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', onClickOutside)
})
onBeforeUnmount(() => {
  document.removeEventListener('click', onClickOutside)
})
</script>

<template>
  <!-- @click.stop.prevent：阻断冒泡到外层 label，并阻止 label 的默认激活行为（转发点击给内部控件导致下拉反复开关） -->
  <div ref="containerRef" class="relative" @click.stop.prevent>
    <button
      type="button"
      :disabled="disabled"
      :aria-expanded="isOpen"
      :class="['select-trigger', isOpen && 'select-trigger-open', disabled && 'select-trigger-disabled']"
      @click="toggle"
    >
      <span class="select-value">{{ selectedLabel }}</span>
      <svg
        :class="['h-4 w-4 flex-shrink-0 text-gray-400 transition-transform duration-200 dark:text-dark-400', isOpen && 'rotate-180']"
        fill="none"
        viewBox="0 0 24 24"
        stroke-width="2"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
      </svg>
    </button>

    <!-- 内联下拉面板：同组件树渲染，无跨层问题 -->
    <Transition name="lottery-select">
      <div
        v-if="isOpen"
        class="select-dropdown"
        role="listbox"
      >
        <div
          v-for="opt in options"
          :key="opt.value"
          role="option"
          :aria-selected="opt.value === modelValue"
          :class="['select-option', opt.value === modelValue && 'select-option-selected']"
          @click="select(opt.value)"
        >
          <span class="flex-1 truncate text-left">{{ opt.label }}</span>
          <svg
            v-if="opt.value === modelValue"
            class="h-4 w-4 flex-shrink-0 text-primary-500"
            fill="none"
            viewBox="0 0 24 24"
            stroke-width="2"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" />
          </svg>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
/* 与 Sub2API 主站 Select.vue 相同的触发器样式 */
.select-trigger {
  @apply flex w-full items-center justify-between gap-2;
  @apply rounded-xl px-4 py-2.5 text-sm;
  @apply bg-white dark:bg-dark-800;
  @apply border border-gray-200 dark:border-dark-600;
  @apply text-gray-900 dark:text-gray-100;
  @apply transition-all duration-200;
  @apply focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/30;
  @apply hover:border-gray-300 dark:hover:border-dark-500;
  @apply cursor-pointer;
}
.select-trigger-open {
  @apply border-primary-500 ring-2 ring-primary-500/30;
}
.select-trigger-disabled {
  @apply cursor-not-allowed bg-gray-100 opacity-60 dark:bg-dark-900;
}
.select-value {
  @apply flex-1 truncate text-left;
}

/* 下拉面板：绝对定位于触发器下方 */
.select-dropdown {
  @apply absolute left-0 right-0 z-50 mt-1;
  @apply bg-white dark:bg-dark-800;
  @apply rounded-xl;
  @apply border border-gray-200 dark:border-dark-700;
  @apply shadow-lg shadow-black/10 dark:shadow-black/30;
  @apply overflow-hidden py-1;
  max-height: 240px;
  overflow-y: auto;
}
.select-option {
  @apply flex items-center justify-between gap-2;
  @apply px-4 py-2.5 text-sm;
  @apply text-gray-700 dark:text-gray-300;
  @apply cursor-pointer transition-colors duration-150;
  @apply hover:bg-gray-50 dark:hover:bg-dark-700;
}
.select-option-selected {
  @apply bg-primary-50 dark:bg-primary-900/20;
  @apply text-primary-700 dark:text-primary-300;
}
.lottery-select-enter-active,
.lottery-select-leave-active {
  transition: all 0.15s ease;
}
.lottery-select-enter-from,
.lottery-select-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
