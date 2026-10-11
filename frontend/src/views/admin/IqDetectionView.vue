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
        <div class="flex flex-shrink-0 items-center gap-2">
          <button
            type="button"
            class="btn btn-secondary"
            :title="t('admin.iqDetection.selectScope')"
            data-testid="open-selector"
            @click="openSelector"
          >
            <Icon name="cog" size="sm" />
            <span class="sr-only">{{ t('admin.iqDetection.selectScope') }}</span>
          </button>
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="loading"
            :title="t('common.refresh')"
            @click="reload"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            <span class="sr-only">{{ t('common.refresh') }}</span>
          </button>
        </div>
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

      <!-- 当前选择概览：分组名 + 各组选中账号数 -->
      <div
        class="border border-gray-200 px-4 py-3 text-sm dark:border-dark-700"
        data-testid="selection-summary"
      >
        <div class="flex flex-wrap items-center justify-between gap-3">
          <span class="text-gray-700 dark:text-gray-300">
            {{
              t('admin.iqDetection.selectedSummary', {
                groups: selectionSummary.groups,
                accounts: selectionSummary.accounts
              })
            }}
          </span>
          <button type="button" class="btn btn-secondary btn-sm" @click="openSelector">
            {{ t('admin.iqDetection.editSelection') }}
          </button>
        </div>
        <div
          v-if="selectedGroupEntries.length > 0"
          class="mt-3 flex flex-wrap gap-2 border-t border-gray-100 pt-3 dark:border-dark-700/60"
          data-testid="selected-group-chips"
        >
          <span
            v-for="entry in selectedGroupEntries"
            :key="entry.id"
            class="inline-flex items-center gap-1.5 rounded-full bg-gray-100 px-3 py-1 text-xs text-gray-700 dark:bg-dark-800 dark:text-gray-300"
            :data-testid="`selected-group-chip-${entry.id}`"
          >
            <span class="max-w-48 truncate font-medium">{{ entry.name }}</span>
            <span class="text-gray-500 dark:text-gray-400">· {{ entry.count }}</span>
          </span>
        </div>
        <p v-else class="mt-3 border-t border-gray-100 pt-3 text-xs text-gray-500 dark:border-dark-700/60 dark:text-gray-400">
          {{ t('admin.iqDetection.noSelection') }}
        </p>
      </div>
    </div>

    <!-- 分组/账号选择弹窗 -->
    <BaseDialog
      :show="selectorOpen"
      :title="t('admin.iqDetection.selectorTitle')"
      width="wide"
      data-testid="selector-dialog"
      @close="selectorOpen = false"
    >
      <div class="grid grid-cols-1 gap-4 lg:grid-cols-[280px,1fr]">
        <!-- 左侧：分组列表（checkbox = 该分组账号全选开关，点击行 = 聚焦查看） -->
        <aside
          class="flex max-h-[50vh] flex-col overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700"
          data-testid="group-list"
        >
          <div
            class="flex items-center justify-between border-b border-gray-200 px-4 py-2.5 text-sm font-medium text-gray-900 dark:border-dark-700 dark:text-white"
          >
            {{ t('admin.iqDetection.groups') }}
            <span class="text-xs font-normal text-gray-500 dark:text-gray-400">
              {{ filteredGroups.length }}
            </span>
          </div>
          <div class="flex items-center gap-2 border-b border-gray-200 px-3 py-2 dark:border-dark-700">
            <select
              v-model="platformFilter"
              class="input h-8 min-w-0 flex-1 py-0 text-xs"
              data-testid="group-platform-filter"
            >
              <option value="">{{ t('admin.iqDetection.allPlatforms') }}</option>
              <option v-for="platform in platformOptions" :key="platform" :value="platform">
                {{ platform }}
              </option>
            </select>
            <button
              type="button"
              class="btn btn-secondary btn-sm flex-shrink-0"
              :title="t('admin.iqDetection.selectAll')"
              @click="selectAllGroups"
            >
              {{ t('admin.iqDetection.selectAll') }}
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-sm flex-shrink-0"
              :title="t('admin.iqDetection.clearAll')"
              @click="clearAllGroups"
            >
              {{ t('admin.iqDetection.clearAll') }}
            </button>
          </div>
          <div v-if="groupsLoading" class="flex flex-1 items-center justify-center py-10 text-sm text-gray-500">
            {{ t('common.loading') }}
          </div>
          <div v-else-if="filteredGroups.length === 0" class="flex flex-1 items-center justify-center px-4 py-10 text-sm text-gray-500">
            {{ t('admin.iqDetection.noGroups') }}
          </div>
          <ul v-else class="flex-1 divide-y divide-gray-100 overflow-y-auto dark:divide-dark-700/60">
            <li v-for="group in filteredGroups" :key="group.id">
              <div
                class="flex w-full cursor-pointer items-center gap-2 px-3 py-2.5 text-left text-sm transition-colors"
                :class="
                  group.id === selectedGroupId
                    ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
                    : 'text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-800'
                "
                :data-testid="`group-item-${group.id}`"
                @click="focusGroup(group.id)"
              >
                <input
                  type="checkbox"
                  class="h-4 w-4 flex-shrink-0 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                  :checked="groupCheckState(group) === true"
                  :indeterminate="groupCheckState(group) === 'partial'"
                  :data-testid="`group-checkbox-${group.id}`"
                  @click.stop
                  @change="toggleGroupCheck(group)"
                />
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
              </div>
            </li>
          </ul>
        </aside>

        <!-- 右侧：分组背后的账号 -->
        <section
          class="flex max-h-[50vh] flex-col overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700"
          data-testid="account-panel"
        >
          <div
            class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-200 px-4 py-2.5 dark:border-dark-700"
          >
            <div class="min-w-0 text-sm font-medium text-gray-900 dark:text-white">
              <template v-if="selectedGroup">
                {{ selectedGroup.name }}
                <span class="ml-2 text-xs font-normal text-gray-500 dark:text-gray-400">
                  {{ t('admin.iqDetection.selectedCount', { selected: selectedAccountIds.size, total: focusedAccounts.length }) }}
                </span>
              </template>
              <template v-else>{{ t('admin.iqDetection.accounts') }}</template>
            </div>
            <div v-if="selectedGroupId && focusedAccounts.length > 0" class="flex flex-shrink-0 items-center gap-2">
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

          <div v-else-if="focusedAccounts.length === 0" class="flex flex-1 items-center justify-center px-6 py-12 text-sm text-gray-500">
            {{ t('admin.iqDetection.noAccounts') }}
          </div>

          <ul v-else class="flex-1 divide-y divide-gray-100 overflow-y-auto dark:divide-dark-700/60">
            <li v-for="account in focusedAccounts" :key="account.id">
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
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type IQDetectAgentHealth } from '@/api/admin'
import type { AdminGroup, AccountListItem } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const health = ref<IQDetectAgentHealth | null>(null)

const selectorOpen = ref(false)

const groups = ref<AdminGroup[]>([])
const groupsLoading = ref(false)

const selectedGroupId = ref<number | null>(null)
const accountsLoading = ref(false)

// 按模型厂商（platform）筛选分组；空串 = 全部。
const platformFilter = ref('')

// 分组账号列表与勾选状态按分组缓存；新拉取的分组默认全部选中。
const accountsByGroup = reactive(new Map<number, AccountListItem[]>())
const accountSelection = reactive(new Map<number, Set<number>>())

const selectedGroup = computed(() => groups.value.find((g) => g.id === selectedGroupId.value) || null)
const platformOptions = computed(() =>
  Array.from(new Set(groups.value.map((g) => g.platform))).sort()
)
const filteredGroups = computed(() =>
  platformFilter.value ? groups.value.filter((g) => g.platform === platformFilter.value) : groups.value
)
const focusedAccounts = computed<AccountListItem[]>(() =>
  selectedGroupId.value == null ? [] : accountsByGroup.get(selectedGroupId.value) ?? []
)
const selectedAccountIds = computed<Set<number>>(
  () => accountSelection.get(selectedGroupId.value ?? -1) ?? new Set()
)

// 主页面概览：每个勾选中的分组 + 该组选中账号数（未加载的分组按 account_count 估算）。
const selectedGroupEntries = computed(() =>
  groups.value
    .map((group) => ({ group, state: groupCheckState(group) }))
    .filter((entry) => entry.state !== false)
    .map((entry) => ({
      id: entry.group.id,
      name: entry.group.name,
      count: accountSelection.get(entry.group.id)?.size ?? entry.group.account_count ?? 0
    }))
)

const selectionSummary = computed(() => ({
  groups: selectedGroupEntries.value.length,
  accounts: selectedGroupEntries.value.reduce((sum, entry) => sum + entry.count, 0)
}))

function openSelector() {
  selectorOpen.value = true
  if (groups.value.length === 0 && !groupsLoading.value) {
    void loadGroups()
  }
}

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

async function fetchAccounts(groupId: number): Promise<AccountListItem[]> {
  const resp = await adminAPI.accounts.list(1, 200, { group: String(groupId) })
  const items = resp?.items || []
  accountsByGroup.set(groupId, items)
  if (!accountSelection.has(groupId)) {
    // 默认全部选中
    accountSelection.set(groupId, new Set(items.map((a) => a.id)))
  }
  return items
}

async function focusGroup(groupId: number) {
  if (selectedGroupId.value === groupId) return
  selectedGroupId.value = groupId
  if (accountsByGroup.has(groupId)) return
  accountsLoading.value = true
  try {
    await fetchAccounts(groupId)
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

// 分组 checkbox 三态：true = 账号全选；false = 全不选；'partial' = 部分选中。
// 从未加载过的分组默认视为全选。
function groupCheckState(group: AdminGroup): boolean | 'partial' {
  const set = accountSelection.get(group.id)
  if (!set) return true
  const total = accountsByGroup.get(group.id)?.length ?? group.account_count ?? 0
  if (set.size === 0) return false
  if (total > 0 && set.size >= total) return true
  return 'partial'
}

async function toggleGroupCheck(group: AdminGroup) {
  const turnOn = groupCheckState(group) !== true
  if (!accountsByGroup.has(group.id)) {
    try {
      await fetchAccounts(group.id)
    } catch (error) {
      appStore.showError(extractApiErrorMessage(error, t('common.error')))
      return
    }
  }
  const ids = (accountsByGroup.get(group.id) ?? []).map((a) => a.id)
  accountSelection.set(group.id, turnOn ? new Set(ids) : new Set<number>())
}

// 分组全选/清空只作用于当前筛选可见的分组；未加载的分组用「删除记录 = 默认全选」表达。
function selectAllGroups() {
  for (const group of filteredGroups.value) {
    const cached = accountsByGroup.get(group.id)
    if (cached) {
      accountSelection.set(group.id, new Set(cached.map((a) => a.id)))
    } else {
      accountSelection.delete(group.id)
    }
  }
}

function clearAllGroups() {
  for (const group of filteredGroups.value) {
    accountSelection.set(group.id, new Set<number>())
  }
}

function toggleAccount(accountId: number) {
  if (selectedGroupId.value == null) return
  const set = accountSelection.get(selectedGroupId.value) ?? new Set<number>()
  if (set.has(accountId)) {
    set.delete(accountId)
  } else {
    set.add(accountId)
  }
  accountSelection.set(selectedGroupId.value, set)
}

function selectAllAccounts() {
  if (selectedGroupId.value == null) return
  accountSelection.set(selectedGroupId.value, new Set(focusedAccounts.value.map((a) => a.id)))
}

function clearAccountSelection() {
  if (selectedGroupId.value == null) return
  accountSelection.set(selectedGroupId.value, new Set<number>())
}

onMounted(reload)
</script>
