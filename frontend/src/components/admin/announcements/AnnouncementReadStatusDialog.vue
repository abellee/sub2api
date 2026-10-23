<template>
  <BaseDialog
    :show="show"
    :title="t('admin.announcements.readStatus')"
    width="extra-wide"
    @close="handleClose"
  >
    <div class="space-y-4">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex-1">
          <input
            v-model="search"
            type="text"
            class="input"
            :placeholder="t('admin.announcements.searchUsers')"
            @input="handleSearch"
          />
        </div>
        <Select
          v-model="readStatus"
          :options="readStatusOptions"
          :aria-label="t('admin.announcements.readStatus')"
          class="w-full sm:w-36"
          @change="handleReadStatusChange"
        />
        <button @click="load" :disabled="loading" class="btn btn-secondary" :title="t('common.refresh')">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <DataTable
        :columns="columns"
        :data="items"
        :loading="loading"
        :server-side-sort="true"
        default-sort-key="email"
        default-sort-order="asc"
        @sort="handleSort"
      >
        <template #cell-email="{ value }">
          <span class="font-medium text-gray-900 dark:text-white">{{ value }}</span>
        </template>

        <template #cell-balance="{ value }">
          <span class="font-medium text-gray-900 dark:text-white">${{ Number(value ?? 0).toFixed(2) }}</span>
        </template>

        <template #cell-eligible="{ value }">
          <span :class="['badge', value ? 'badge-success' : 'badge-gray']">
            {{ value ? t('admin.announcements.eligible') : t('common.no') }}
          </span>
        </template>

        <template #cell-read_at="{ value }">
          <span class="text-sm text-gray-500 dark:text-dark-400">
            {{ value ? formatDateTime(value) : t('admin.announcements.unread') }}
          </span>
        </template>
      </DataTable>

      <div class="flex flex-col gap-2 border-t border-gray-200 pt-3 dark:border-dark-700 lg:flex-row lg:items-center lg:gap-4">
        <p class="shrink-0 text-sm text-gray-700 dark:text-gray-300" aria-live="polite">
          {{ t('admin.announcements.readCount', { count: readCount }) }}
        </p>
        <Pagination
          v-if="pagination.total > 0"
          class="flex-1 border-t-0 px-0 py-0 sm:px-0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end">
        <button type="button" class="btn btn-secondary" @click="handleClose">{{ t('common.close') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { formatDateTime } from '@/utils/format'
import type { AnnouncementUserReadStatus } from '@/types'
import type { Column } from '@/components/common/types'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'

import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'

const { t } = useI18n()
const appStore = useAppStore()

const props = defineProps<{
  show: boolean
  announcementId: number | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const loading = ref(false)
const search = ref('')
const readStatus = ref<'all' | 'read'>('all')
const readCount = ref(0)
const readStatusOptions = computed(() => [
  { value: 'all', label: t('common.all') },
  { value: 'read', label: t('admin.announcements.read') }
])

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const sortState = reactive({
  sort_by: 'email',
  sort_order: 'asc' as 'asc' | 'desc'
})

const items = ref<AnnouncementUserReadStatus[]>([])

const columns = computed<Column[]>(() => [
  { key: 'email', label: t('common.email'), sortable: true },
  { key: 'username', label: t('admin.users.columns.username'), sortable: true },
  { key: 'balance', label: t('common.balance'), sortable: true },
  { key: 'eligible', label: t('admin.announcements.eligible') },
  { key: 'read_at', label: t('admin.announcements.readAt') }
])

let currentController: AbortController | null = null
let searchDebounceTimer: number | null = null

function resetDialogState() {
  loading.value = false
  search.value = ''
  readStatus.value = 'all'
  readCount.value = 0
  items.value = []
  pagination.page = 1
  pagination.total = 0
  pagination.pages = 0
  sortState.sort_by = 'email'
  sortState.sort_order = 'asc'
}

function cancelPendingLoad(resetState = false) {
  if (searchDebounceTimer) {
    window.clearTimeout(searchDebounceTimer)
    searchDebounceTimer = null
  }
  currentController?.abort()
  currentController = null
  if (resetState) {
    resetDialogState()
  }
}

async function load() {
  if (!props.show || !props.announcementId) return

  currentController?.abort()
  const requestController = new AbortController()
  currentController = requestController
  const { signal } = requestController

  try {
    loading.value = true
    const res = await adminAPI.announcements.getReadStatus(
      props.announcementId,
      pagination.page,
      pagination.page_size,
      {
        search: search.value,
        read_status: readStatus.value,
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order
      },
      { signal }
    )

    if (signal.aborted || currentController !== requestController) return

    items.value = res.items
    readCount.value = res.read_count
    pagination.total = res.total
    pagination.pages = res.pages
    pagination.page = res.page
    pagination.page_size = res.page_size
  } catch (error: any) {
    if (
      signal.aborted ||
      currentController !== requestController ||
      error?.name === 'AbortError' ||
      error?.code === 'ERR_CANCELED'
    ) {
      return
    }
    console.error('Failed to load read status:', error)
    appStore.showError(error.response?.data?.detail || t('admin.announcements.failedToLoadReadStatus'))
  } finally {
    if (currentController === requestController) {
      loading.value = false
      currentController = null
    }
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  load()
}

function handleReadStatusChange() {
  cancelPendingLoad()
  pagination.page = 1
  load()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  load()
}

function handleSort(key: string, order: 'asc' | 'desc') {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  load()
}

function handleSearch() {
  if (searchDebounceTimer) window.clearTimeout(searchDebounceTimer)
  searchDebounceTimer = window.setTimeout(() => {
    pagination.page = 1
    load()
  }, 300)
}

function handleClose() {
  cancelPendingLoad(true)
  emit('close')
}

watch(
  () => props.show,
  (v) => {
    if (!v) {
      cancelPendingLoad(true)
      return
    }
    pagination.page = 1
    load()
  }
)

watch(
  () => props.announcementId,
  () => {
    if (!props.show) return
    pagination.page = 1
    load()
  }
)

onUnmounted(() => {
  cancelPendingLoad()
})
</script>
