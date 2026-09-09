<template>
  <BaseDialog
    :show="show"
    :title="t('admin.groups.setCategoryTitle')"
    width="narrow"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <p class="text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.groups.setCategoryHint', { name: group?.name || '' }) }}
      </p>
      <Select
        v-model="selectedId"
        :options="options"
        :placeholder="t('admin.groups.setCategoryPlaceholder')"
        :clearable="true"
        :disabled="loading"
      />
    </div>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="emit('close')">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="saving || loading" @click="save">
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { GroupCategory } from '@/api/admin/groupCategories'
import type { AdminGroup } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{
  show: boolean
  group: AdminGroup | null
}>()

const emit = defineEmits<{
  close: []
  success: []
}>()

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const saving = ref(false)
const categories = ref<GroupCategory[]>([])
const selectedId = ref<string | null>('')

const options = computed(() => [
  { value: '', label: t('admin.groups.setCategoryNone') },
  ...categories.value.map((cat) => {
    const note = cat.description ? ` — ${cat.description}` : ''
    return { value: cat.id, label: `${cat.name}${note}` }
  })
])

const load = async () => {
  if (!props.group) return
  loading.value = true
  try {
    const snap = await adminAPI.groupCategories.list()
    categories.value = [...snap.categories].sort((a, b) => (b.sort_order ?? 0) - (a.sort_order ?? 0))
    selectedId.value = snap.assignments[String(props.group.id)] || ''
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.groups.categories.failedToLoad')))
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.show, props.group?.id],
  ([show]) => {
    if (show) void load()
  }
)

const save = async () => {
  if (!props.group) return
  saving.value = true
  try {
    await adminAPI.groupCategories.assign(props.group.id, selectedId.value || null)
    appStore.showSuccess(t('admin.groups.setCategorySuccess'))
    emit('success')
    emit('close')
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.groups.setCategoryFailed')))
  } finally {
    saving.value = false
  }
}
</script>
