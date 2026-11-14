<template>
  <BaseDialog
    :show="show"
    :title="t('admin.groups.tags.title')"
    width="normal"
    :close-on-escape="!saving"
    :show-close-button="!saving"
    @close="close"
  >
    <form id="group-tags-form" class="space-y-4" @submit.prevent="save">
      <p class="break-all text-sm font-medium">{{ group?.name }}</p>
      <p class="text-sm text-gray-500">{{ t('admin.groups.tags.hint') }}</p>
      <div v-for="(tag, index) in draft" :key="index" data-test="tag-row" class="space-y-3 rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
        <div class="flex items-end gap-3">
          <div class="min-w-0 flex-1">
            <label :for="`group-tag-name-${index}`" class="input-label">{{ t('admin.groups.tags.name') }}</label>
            <input :id="`group-tag-name-${index}`" v-model="tag.name" data-test="tag-name" class="input" required maxlength="50" :disabled="saving" />
          </div>
          <button type="button" data-test="remove-tag" class="btn btn-secondary" :disabled="saving" @click="draft.splice(index, 1)">{{ t('common.delete') }}</button>
        </div>
        <div>
          <label :for="`group-tag-description-${index}`" class="input-label">{{ t('admin.groups.tags.description') }}</label>
          <input :id="`group-tag-description-${index}`" v-model="tag.description" data-test="tag-description" class="input" maxlength="200" :disabled="saving" />
        </div>
      </div>
      <button type="button" data-test="add-tag" class="btn btn-secondary" :disabled="saving || draft.length >= 20" @click="draft.push({ name: '', description: '' })">{{ t('admin.groups.tags.add') }}</button>
    </form>
    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="saving" @click="close">{{ t('common.cancel') }}</button>
      <button type="submit" form="group-tags-form" data-test="save-tags" class="btn btn-primary" :disabled="saving || !valid">{{ t(saving ? 'common.saving' : 'common.save') }}</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { GroupTag } from '@/api/groups'
import type { AdminGroup } from '@/types'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ show: boolean; group: AdminGroup | null; tags: GroupTag[] }>()
const emit = defineEmits<{ close: []; saved: [groupId: number, tags: GroupTag[]] }>()
const { t } = useI18n()
const appStore = useAppStore()
const saving = ref(false)
const draft = ref<GroupTag[]>([])
const valid = computed(() => draft.value.length <= 20 && draft.value.every(tag => {
  const nameLength = [...tag.name.trim()].length
  return nameLength > 0 && nameLength <= 50 && [...tag.description.trim()].length <= 200
}))

watch(() => props.show, show => {
  if (show) draft.value = props.tags.length ? props.tags.map(tag => ({ ...tag })) : [{ name: '', description: '' }]
}, { immediate: true })

function close() {
  if (!saving.value) emit('close')
}

async function save() {
  if (saving.value || !valid.value || !props.group) return
  const groupId = props.group.id
  const tags = draft.value.map(tag => ({ name: tag.name.trim(), description: tag.description.trim() }))
  saving.value = true
  try {
    await adminAPI.groupCategories.setGroupTags(groupId, tags)
    appStore.showSuccess(t('admin.groups.tags.saved'))
    emit('saved', groupId, tags)
    emit('close')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.groups.tags.saveFailed')))
  } finally {
    saving.value = false
  }
}
</script>
