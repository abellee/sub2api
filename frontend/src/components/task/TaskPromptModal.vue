<script setup lang="ts">
/**
 * TaskPromptModal —— 新任务引导弹窗。
 * 展示封面、任务名称和任务说明。样式对齐抽奖引导弹窗。
 */
import type { TaskPrompt } from '../../api/task'

defineProps<{ task: TaskPrompt | null }>()
const emit = defineEmits<{ (e: 'confirm', taskId: number): void; (e: 'dismiss', taskId: number): void }>()
</script>

<template>
  <transition name="task-modal">
    <div
      v-if="task"
      class="fixed inset-0 z-[2000] flex items-center justify-center bg-gray-900/60 px-4 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      aria-labelledby="task-prompt-title"
    >
      <div class="card flex max-h-[92vh] w-full max-w-sm flex-col overflow-y-auto p-0 text-left shadow-glass">
        <div v-if="task.cover" class="flex justify-center bg-gray-100 dark:bg-dark-800">
          <img :src="task.cover" :alt="task.name" class="task-cover" />
        </div>
        <div
          v-else
          class="flex h-32 items-center justify-center bg-gradient-to-br from-teal-50 to-indigo-50 text-sm text-gray-400 dark:from-dark-800 dark:to-dark-700 dark:text-dark-400"
        >
          暂无封面
        </div>
        <div class="p-6">
          <p class="text-xs font-medium tracking-wide text-primary-500">新任务</p>
          <h3 id="task-prompt-title" class="mt-1 text-lg font-bold text-gray-900 dark:text-white">
            {{ task.name }}
          </h3>
          <p
            v-if="task.description"
            class="mt-2 max-h-40 overflow-y-auto whitespace-pre-line text-sm leading-relaxed text-gray-500 dark:text-dark-400"
          >
            {{ task.description }}
          </p>
          <p v-else class="mt-2 text-sm text-gray-400 dark:text-dark-500">暂无任务说明</p>
          <div class="mt-5 flex justify-center gap-3">
            <button class="btn btn-secondary flex-1" @click="emit('dismiss', task.id)">稍后再说</button>
            <button class="btn btn-primary flex-1" @click="emit('confirm', task.id)">去看看</button>
          </div>
        </div>
      </div>
    </div>
  </transition>
</template>

<style scoped>
.task-cover {
  display: block;
  width: auto;
  height: auto;
  max-width: 100%;
  max-height: 68vh;
  margin-inline: auto;
  object-fit: contain;
}
.task-modal-enter-active,
.task-modal-leave-active {
  transition: opacity 200ms ease;
}
.task-modal-enter-from,
.task-modal-leave-to {
  opacity: 0;
}
</style>
