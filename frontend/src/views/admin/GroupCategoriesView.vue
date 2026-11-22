<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.groups.categories.description') }}
          </p>
          <div class="flex items-center gap-2">
            <button
              @click="loadCategories"
              :disabled="loading"
              class="btn btn-secondary"
              :title="t('common.refresh')"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button @click="openCreate" class="btn btn-primary">
              <Icon name="plus" size="md" class="mr-1" />
              {{ t('admin.groups.categories.create') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <div v-if="loading" class="space-y-2">
          <div v-for="i in 3" :key="i" class="h-16 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-700" />
        </div>
        <EmptyState
          v-else-if="!categories.length"
          :title="t('admin.groups.categories.empty')"
          :description="t('admin.groups.categories.createFirst')"
          :action-text="t('admin.groups.categories.create')"
          @action="openCreate"
        />
        <div v-else class="space-y-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.groups.categories.sortOrderHint') }}
          </p>
          <VueDraggable
            v-model="categories"
            :animation="200"
            handle=".category-drag-handle"
            class="space-y-2"
            @end="saveSortOrder"
          >
            <div
              v-for="category in categories"
              :key="category.id"
              class="flex items-center gap-3 rounded-lg border border-gray-200 bg-white p-3 dark:border-dark-600 dark:bg-dark-800"
            >
              <button
                type="button"
                class="category-drag-handle cursor-grab rounded-md p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 active:cursor-grabbing dark:hover:bg-dark-700"
                :title="t('admin.groups.categories.sortOrder')"
              >
                <Icon name="menu" size="md" />
              </button>
              <div class="min-w-0 flex-1">
                <div class="font-medium text-gray-900 dark:text-white">{{ category.name }}</div>
                <div class="truncate text-sm text-gray-500 dark:text-gray-400">
                  {{ category.description || '—' }}
                </div>
              </div>
              <div class="flex items-center gap-1">
                <button
                  @click="openEdit(category)"
                  class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700"
                >
                  <Icon name="edit" size="sm" />
                  <span class="text-xs">{{ t('common.edit') }}</span>
                </button>
                <button
                  @click="confirmDelete(category)"
                  class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
                >
                  <Icon name="trash" size="sm" />
                  <span class="text-xs">{{ t('common.delete') }}</span>
                </button>
              </div>
            </div>
          </VueDraggable>
        </div>
      </template>
    </TablePageLayout>

    <BaseDialog
      :show="showForm"
      :title="editing ? t('admin.groups.categories.edit') : t('admin.groups.categories.create')"
      width="narrow"
      @close="showForm = false"
    >
      <form id="group-category-form" class="space-y-4" @submit.prevent="saveCategory">
        <div>
          <label class="input-label">{{ t('admin.groups.categories.name') }}</label>
          <input
            v-model="form.name"
            type="text"
            required
            maxlength="80"
            class="input"
            :placeholder="t('admin.groups.categories.namePlaceholder')"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.groups.categories.descriptionLabel') }}</label>
          <textarea
            v-model="form.description"
            rows="3"
            maxlength="200"
            class="input"
            :placeholder="t('admin.groups.categories.descriptionPlaceholder')"
          ></textarea>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="showForm = false">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" form="group-category-form" class="btn btn-primary" :disabled="saving">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="!!deleting"
      :title="t('common.delete')"
      :message="t('admin.groups.categories.deleteConfirm', { name: deleting?.name || '' })"
      danger
      @confirm="doDelete"
      @cancel="deleting = null"
    />

  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { GroupCategory } from '@/api/admin/groupCategories'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { VueDraggable } from 'vue-draggable-plus'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const categories = ref<GroupCategory[]>([])
const showForm = ref(false)
const editing = ref<GroupCategory | null>(null)
const deleting = ref<GroupCategory | null>(null)
const form = ref({ name: '', description: '' })
const sortSaving = ref(false)

const loadCategories = async () => {
  loading.value = true
  try {
    const snap = await adminAPI.groupCategories.list()
    categories.value = [...snap.categories].sort((a, b) => (b.sort_order ?? 0) - (a.sort_order ?? 0))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.groups.categories.failedToLoad')))
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  editing.value = null
  form.value = { name: '', description: '' }
  showForm.value = true
}

const openEdit = (row: GroupCategory) => {
  editing.value = row
  form.value = { name: row.name, description: row.description || '' }
  showForm.value = true
}

const saveSortOrder = async () => {
  if (sortSaving.value || categories.value.length < 2) return
  sortSaving.value = true
  try {
    await adminAPI.groupCategories.reorder(categories.value.map((item) => item.id))
    appStore.showSuccess(t('admin.groups.categories.sortOrderUpdated'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.groups.categories.failedToUpdateSortOrder')))
    await loadCategories()
  } finally {
    sortSaving.value = false
  }
}

const saveCategory = async () => {
  const name = form.value.name.trim()
  if (!name) return
  saving.value = true
  try {
    const description = form.value.description.trim()
    if (editing.value) {
      await adminAPI.groupCategories.update(editing.value.id, {
        name,
        description
      })
      appStore.showSuccess(t('admin.groups.categories.updated'))
    } else {
      await adminAPI.groupCategories.create({
        name,
        description
      })
      appStore.showSuccess(t('admin.groups.categories.created'))
    }
    showForm.value = false
    await loadCategories()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.groups.categories.failedToSave')))
  } finally {
    saving.value = false
  }
}

const confirmDelete = (row: GroupCategory) => {
  deleting.value = row
}

const doDelete = async () => {
  if (!deleting.value) return
  try {
    await adminAPI.groupCategories.remove(deleting.value.id)
    appStore.showSuccess(t('admin.groups.categories.deleted'))
    deleting.value = null
    await loadCategories()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.groups.categories.failedToDelete')))
  }
}

onMounted(loadCategories)
</script>
