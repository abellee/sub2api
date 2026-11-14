<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.batchDuplicate')"
    width="narrow"
    :close-on-escape="!saving"
    :show-close-button="!saving"
    @close="close"
  >
    <div class="space-y-4">
      <p class="break-all text-sm font-medium">{{ account?.name }}</p>
      <Input
        id="batch-duplicate-count"
        v-model="count"
        type="number"
        :label="t('admin.accounts.batchDuplicateCount')"
        :disabled="saving"
        :error="validCount ? '' : t('admin.accounts.batchDuplicateCountError')"
        @enter="submit"
      />
      <p v-if="validCount" class="break-all text-sm text-gray-600 dark:text-gray-400">
        {{ t('admin.accounts.batchDuplicatePreview', { first: `${account?.name}-1`, last: `${account?.name}-${Number(count)}` }) }}
      </p>
      <p class="text-sm text-gray-600 dark:text-gray-400">{{ t('admin.accounts.batchDuplicateHelp') }}</p>
    </div>
    <template #footer>
      <button class="btn btn-secondary" :disabled="saving" @click="close">{{ t('common.cancel') }}</button>
      <button class="btn btn-primary" :disabled="saving || !validCount || !account" @click="submit">
        {{ t(saving ? 'common.saving' : 'admin.accounts.batchDuplicate') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Input from '@/components/common/Input.vue'
import type { Account } from '@/types'

const props = defineProps<{ show: boolean; account: Account | null }>()
const emit = defineEmits<{ close: []; duplicated: [] }>()
const { t } = useI18n()
const appStore = useAppStore()
const count = ref('10')
const saving = ref(false)
const validCount = computed(() => Number.isInteger(Number(count.value)) && Number(count.value) >= 1 && Number(count.value) <= 100)

watch(() => props.show, show => { if (show) count.value = '10' })

function close() {
  if (!saving.value) emit('close')
}

async function submit() {
  if (saving.value || !validCount.value || !props.account) return
  saving.value = true
  try {
    const accounts = await adminAPI.accounts.batchDuplicate(props.account.id, Number(count.value))
    appStore.showSuccess(t('admin.accounts.batchDuplicateSuccess', { count: accounts.length }))
    emit('duplicated')
    emit('close')
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('admin.accounts.duplicateFailed'))
  } finally {
    saving.value = false
  }
}
</script>
