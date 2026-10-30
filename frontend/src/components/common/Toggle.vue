<template>
  <button
    type="button"
    @click="toggle"
    class="relative inline-flex h-6 w-11 flex-shrink-0 rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:focus:ring-offset-dark-800"
    :class="[
      modelValue ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600',
      disabled ? 'cursor-not-allowed opacity-50' : loading ? 'cursor-wait' : 'cursor-pointer',
    ]"
    role="switch"
    :aria-checked="modelValue"
    :aria-busy="loading || undefined"
    :disabled="disabled || loading"
  >
    <span
      class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
      :class="[modelValue ? 'translate-x-5' : 'translate-x-0']"
    >
      <svg v-if="loading" class="m-0.5 h-4 w-4 animate-spin text-primary-600" viewBox="0 0 24 24" fill="none" aria-hidden="true">
        <circle class="opacity-25" cx="12" cy="12" r="9" stroke="currentColor" stroke-width="3" />
        <path d="M12 3a9 9 0 0 1 9 9" stroke="currentColor" stroke-width="3" stroke-linecap="round" />
      </svg>
    </span>
  </button>
</template>

<script setup lang="ts">
const props = defineProps<{
  modelValue: boolean
  disabled?: boolean
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

function toggle() {
  if (props.disabled || props.loading) return
  emit('update:modelValue', !props.modelValue)
}
</script>
