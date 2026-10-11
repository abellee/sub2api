<template>
  <AppLayout>
    <div class="space-y-6" data-testid="iq-detection-page">
      <section
        class="flex flex-col gap-4 border-b border-gray-200 pb-5 dark:border-dark-700 sm:flex-row sm:items-end sm:justify-between"
      >
        <div class="min-w-0">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.iqDetection.title') }}
          </h2>
          <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.iqDetection.description') }}
          </p>
        </div>
        <button
          type="button"
          class="btn btn-secondary flex-shrink-0"
          :disabled="loading"
          :title="t('common.refresh')"
          @click="reload"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          <span class="sr-only">{{ t('common.refresh') }}</span>
        </button>
      </section>

      <div
        v-if="!health?.enabled"
        class="border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-100"
        data-testid="service-down"
      >
        {{ t('admin.iqDetection.serviceDown') }}
      </div>
      <div
        v-else
        class="flex flex-wrap items-center gap-x-6 gap-y-2 border border-gray-200 px-4 py-3 text-sm dark:border-dark-700"
        data-testid="service-up"
      >
        <span class="flex items-center gap-2 font-medium text-green-700 dark:text-green-400">
          <span class="h-2 w-2 rounded-full bg-green-500" />
          {{ t('admin.iqDetection.serviceUp') }}
        </span>
        <span v-if="health?.version" class="text-gray-500 dark:text-gray-400">
          {{ t('admin.iqDetection.version') }}: {{ health.version }}
        </span>
        <span class="text-gray-500 dark:text-gray-400">
          {{ t('admin.iqDetection.baseUrl') }}: {{ health?.base_url }}
        </span>
      </div>

      <div class="grid grid-cols-1 gap-4 lg:grid-cols-[280px,1fr]">
        <!-- 左侧：分组列表 -->
        <aside
          class="flex max-h-[560px] flex-col overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700"
          data-testid="group-list"
        >
          <div
            class="flex items-center justify-between border-b border-gray-200 px-4 py-3 text-sm font-medium text-gray-900 dark:border-dark-700 dark:text-white"
          >
            {{ t('admin.iqDetection.groups') }}
            <span class="text-xs font-normal text-gray-500 dark:text-gray-400">
              {{ groups.length }}
            </span>
          </div>
          <div v-if="groupsLoading" class="flex flex-1 items-center justify-center py-10 text-sm text-gray-500">
            {{ t('common.loading') }}
          </div>
          <div v-else-if="groups.length === 0" class="flex flex-1 items-center justify-center px-4 py-10 text-sm text-gray-500">
            {{ t('admin.iqDetection.noGroups') }}
          </div>
          <ul v-else class="flex-1 divide-y divide-gray-100 overflow-y-auto dark:divide-dark-700/60">
            <li v-for="group in groups" :key="group.id">
              <button
                type="button"
                class="flex w-full items-center justify-between gap-2 px-4 py-2.5 text-left text-sm transition-colors"
                :class="
                  group.id === selectedGroupId
                    ? 'bg-primary-50 font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
                    : 'text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-800'
                "
                :data-testid="`group-item-${group.id}`"
                @click="selectGroup(group.id)"
              >
                <span class="min-w-0 flex-1 truncate">
                  {{ group.name }}
                  <span
                    v-if="group.status === 'inactive'"
                    class="ml-1 align-middle text-xs text-gray-400"
                  >
                    {{ t('admin.iqDetection.inactive') }}
                  </span>
                </span>
                <span class="flex-shrink-0 text-xs text-gray-400">
                  {{ group.platform }}
                  <template v-if="typeof group.account_count === 'number'">
                    · {{ group.account_count }}
                  </template>
                </span>
              </button>
            </li>
          </ul>
        </aside>

        <!-- 右侧：分组背后的账号 -->
        <section
          class="flex max-h-[560px] flex-col overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700"
          data-testid="account-panel"
        >
          <div
            class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-200 px-4 py-3 dark:border-dark-700"
          >
            <div class="min-w-0 text-sm font-medium text-gray-900 dark:text-white">
              <template v-if="selectedGroup">
                {{ selectedGroup.name }}
                <span class="ml-2 text-xs font-normal text-gray-500 dark:text-gray-400">
                  {{ t('admin.iqDetection.selectedCount', { selected: selectedAccountIds.size, total: accounts.length }) }}
                </span>
              </template>
              <template v-else>{{ t('admin.iqDetection.accounts') }}</template>
            </div>
            <div v-if="selectedGroupId && accounts.length > 0" class="flex flex-shrink-0 items-center gap-2">
              <button type="button" class="btn btn-secondary btn-sm" :disabled="accountsLoading" @click="selectAllAccounts">
                {{ t('admin.iqDetection.selectAll') }}
              </button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="accountsLoading" @click="clearAccountSelection">
                {{ t('admin.iqDetection.clearAll') }}
              </button>
            </div>
          </div>

          <div v-if="!selectedGroupId" class="flex flex-1 flex-col items-center justify-center px-6 py-12 text-center">
            <Icon name="grid" size="xl" class="text-gray-400" />
            <p class="mt-3 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.iqDetection.selectGroupHint') }}
            </p>
          </div>

          <div v-else-if="accountsLoading" class="flex flex-1 items-center justify-center py-10 text-sm text-gray-500">
            {{ t('common.loading') }}
          </div>

          <div v-else-if="accounts.length === 0" class="flex flex-1 items-center justify-center px-6 py-12 text-sm text-gray-500">
            {{ t('admin.iqDetection.noAccounts') }}
          </div>

          <ul v-else class="flex-1 divide-y divide-gray-100 overflow-y-auto dark:divide-dark-700/60">
            <li v-for="account in accounts" :key="account.id">
              <label
                class="flex cursor-pointer items-center gap-3 px-4 py-2.5 text-sm transition-colors hover:bg-gray-50 dark:hover:bg-dark-800"
                :data-testid="`account-item-${account.id}`"
              >
                <input
                  type="checkbox"
                  class="h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                  :checked="selectedAccountIds.has(account.id)"
                  @change="toggleAccount(account.id)"
                />
                <span class="min-w-0 flex-1 truncate text-gray-700 dark:text-gray-300">
                  {{ account.name }}
                </span>
                <span class="flex-shrink-0 text-xs text-gray-400">
                  {{ account.platform }}
                  <template v-if="account.status">· {{ account.status }}</template>
                </span>
              </label>
            </li>
          </ul>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type IQDetectAgentHealth } from '@/api/admin'
import type { AdminGroup, AccountListItem } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const health = ref<IQDetectAgentHealth | null>(null)

const groups = ref<AdminGroup[]>([])
const groupsLoading = ref(false)

const selectedGroupId = ref<number | null>(null)
const accounts = ref<AccountListItem[]>([])
const accountsLoading = ref(false)

// 每个分组的账号勾选独立保存，切回时恢复；新分组默认全选。
const selectionByGroup = reactive(new Map<number, Set<number>>())

const selectedGroup = computed(() => groups.value.find((g) => g.id === selectedGroupId.value) || null)
const selectedAccountIds = computed<Set<number>>(
  () => selectionByGroup.get(selectedGroupId.value ?? -1) ?? new Set()
)

async function reload() {
  loading.value = true
  try {
    health.value = await adminAPI.iqDetection.getHealth()
  } catch {
    health.value = null
  } finally {
    loading.value = false
  }
  await loadGroups()
}

async function loadGroups() {
  groupsLoading.value = true
  try {
    const resp = await adminAPI.groups.list(1, 200)
    groups.value = resp?.items || []
  } catch (error) {
    groups.value = []
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    groupsLoading.value = false
  }
}

async function selectGroup(groupId: number) {
  if (selectedGroupId.value === groupId) return
  selectedGroupId.value = groupId
  await loadAccounts(groupId)
}

async function loadAccounts(groupId: number) {
  accountsLoading.value = true
  accounts.value = []
  try {
    const resp = await adminAPI.accounts.list(1, 200, { group: String(groupId) })
    // 用户可能快速切换分组，只接受仍是当前分组的响应。
    if (selectedGroupId.value !== groupId) return
    accounts.value = resp?.items || []
    if (!selectionByGroup.has(groupId)) {
      // 默认全部选中
      selectionByGroup.set(groupId, new Set(accounts.value.map((a) => a.id)))
    }
  } catch (error) {
    if (selectedGroupId.value === groupId) {
      appStore.showError(extractApiErrorMessage(error, t('common.error')))
    }
  } finally {
    if (selectedGroupId.value === groupId) {
      accountsLoading.value = false
    }
  }
}

function toggleAccount(accountId: number) {
  if (selectedGroupId.value == null) return
  const set = selectionByGroup.get(selectedGroupId.value) ?? new Set<number>()
  if (set.has(accountId)) {
    set.delete(accountId)
  } else {
    set.add(accountId)
  }
  selectionByGroup.set(selectedGroupId.value, set)
}

function selectAllAccounts() {
  if (selectedGroupId.value == null) return
  selectionByGroup.set(selectedGroupId.value, new Set(accounts.value.map((a) => a.id)))
}

function clearAccountSelection() {
  if (selectedGroupId.value == null) return
  selectionByGroup.set(selectedGroupId.value, new Set<number>())
}

onMounted(reload)
</script>
