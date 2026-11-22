<template>
  <BaseDialog :show="show" :title="title" width="narrow" @close="handleCancel">
    <div :class="studio ? 'key-dialog key-confirm' : 'space-y-4'">
      <span v-if="studio && danger" class="key-confirm-symbol"><Icon name="trash" size="md" /></span>
      <p class="text-sm text-gray-600 dark:text-gray-400" :class="{ 'key-confirm-message': studio }">{{ message }}</p>
      <slot></slot>
    </div>

    <template #footer>
      <div class="flex justify-end space-x-3" :class="{ 'key-dialog-actions': studio }">
        <button
          @click="handleCancel"
          type="button"
          :class="{ 'key-dialog-button key-dialog-cancel': studio }"
          class="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600 dark:focus:ring-offset-dark-800"
        >
          {{ cancelText }}
        </button>
        <button
          @click="handleConfirm"
          type="button"
          :class="[
            { 'key-dialog-button': studio, 'key-dialog-danger': studio && danger, 'key-dialog-primary': studio && !danger },
            'rounded-md px-4 py-2 text-sm font-medium text-white focus:outline-none focus:ring-2 focus:ring-offset-2 dark:focus:ring-offset-dark-800',
            danger
              ? 'bg-red-600 hover:bg-red-700 focus:ring-red-500'
              : 'bg-primary-600 hover:bg-primary-700 focus:ring-primary-500'
          ]"
        >
          {{ confirmText }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from './BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import '@/styles/key-dialog.css'

const { t } = useI18n()

interface Props {
  show: boolean
  title: string
  message: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
  studio?: boolean
}

interface Emits {
  (e: 'confirm'): void
  (e: 'cancel'): void
}

const props = withDefaults(defineProps<Props>(), {
  danger: false
})

const confirmText = computed(() => props.confirmText || t('common.confirm'))
const cancelText = computed(() => props.cancelText || t('common.cancel'))

const emit = defineEmits<Emits>()

const handleConfirm = () => {
  emit('confirm')
}

const handleCancel = () => {
  emit('cancel')
}
</script>
