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
            data-testid="open-model-settings"
            @click="openModelSettings"
          >
            <Icon name="cpu" size="sm" />
            {{ t('admin.iqDetection.detectionModelSettings') }}
          </button>
          <button
            type="button"
            class="btn btn-secondary"
            data-testid="open-account-manager"
            @click="openAccountManager"
          >
            <Icon name="users" size="sm" />
            {{ t('admin.iqDetection.accountManagement') }}
          </button>
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
          <div class="flex items-center gap-2">
            <button
              type="button"
              class="btn btn-primary btn-sm"
              :disabled="testRunning"
              data-testid="run-test"
              @click="runTest"
            >
              {{ t('admin.iqDetection.test') }}
            </button>
            <button type="button" class="btn btn-secondary btn-sm" @click="openSelector">
              {{ t('admin.iqDetection.editSelection') }}
            </button>
          </div>
        </div>
        <div v-if="selectedGroupEntries.length > 0" class="mt-3 grid gap-3 border-t border-gray-100 pt-3 dark:border-dark-700/60 lg:grid-cols-[240px,1fr]">
          <!-- 左侧：选中的分组竖向列表 -->
          <aside
            class="max-h-80 divide-y divide-gray-100 overflow-y-auto rounded-lg border border-gray-200 dark:divide-dark-700/60 dark:border-dark-700"
            data-testid="summary-group-list"
          >
            <button
              v-for="entry in selectedGroupEntries"
              :key="entry.id"
              type="button"
              class="flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-sm transition-colors"
              :class="
                entry.id === summaryGroupId
                  ? 'bg-primary-50 font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
                  : 'text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-800'
              "
              :data-testid="`summary-group-item-${entry.id}`"
              @click="focusSummaryGroup(entry.id)"
            >
              <span class="min-w-0 flex-1 truncate">{{ entry.name }}</span>
              <span class="flex-shrink-0 text-xs text-gray-400">{{ entry.count }}</span>
            </button>
          </aside>

          <!-- 右侧：该分组选中的账号卡片 -->
          <section class="min-w-0">
            <div v-if="summaryLoading" class="flex h-32 items-center justify-center text-sm text-gray-500">
              {{ t('common.loading') }}
            </div>
            <div
              v-else-if="!summaryGroup"
              class="flex h-32 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
            >
              {{ t('admin.iqDetection.selectGroupHint') }}
            </div>
            <div
              v-else-if="summaryAccounts.length === 0"
              class="flex h-32 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
            >
              {{ t('admin.iqDetection.noAccounts') }}
            </div>
            <div v-else class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3" data-testid="summary-account-cards">
              <div
                v-for="account in summaryAccounts"
                :key="account.id"
                class="min-w-0 rounded-lg border border-gray-200 p-3 dark:border-dark-700"
                :data-testid="`summary-card-${account.id}`"
              >
                <div class="flex items-center justify-between gap-2">
                  <span class="min-w-0 truncate text-sm font-medium text-gray-800 dark:text-gray-200">
                    {{ account.name }}
                  </span>
                  <span class="flex-shrink-0 text-xs text-gray-400">
                    {{ account.platform }}<template v-if="account.type"> · {{ account.type }}</template>
                  </span>
                </div>
                <div
                  v-if="accountBaseUrl(account)"
                  class="mt-1 truncate font-mono text-xs text-gray-400 dark:text-gray-500"
                  :title="accountBaseUrl(account)"
                >
                  {{ accountBaseUrl(account) }}
                </div>
                <div v-if="accountModels(account).length > 0" class="mt-2 flex flex-wrap gap-1">
                  <span
                    v-for="model in accountModels(account)"
                    :key="model"
                    class="rounded bg-gray-100 px-1.5 py-0.5 font-mono text-[10px] text-gray-600 dark:bg-dark-800 dark:text-gray-400"
                  >
                    {{ model }}
                  </span>
                </div>
              </div>
            </div>
          </section>
        </div>
        <p v-else class="mt-3 border-t border-gray-100 pt-3 text-xs text-gray-500 dark:border-dark-700/60 dark:text-gray-400">
          {{ t('admin.iqDetection.noSelection') }}
        </p>
      </div>

      <!-- 账号管理模态框：分页加载全部账号，逐个保存 API 地址与 API Key 到 iqdetectd -->
      <BaseDialog
        :show="accountManagerOpen"
        :title="t('admin.iqDetection.accountManagement')"
        width="wide"
        data-testid="account-manager-dialog"
        @close="accountManagerOpen = false"
      >
      <section data-testid="account-management">
        <div
          class="flex flex-wrap items-center justify-end gap-3 border-b border-gray-200 px-4 py-3 dark:border-dark-700"
        >
          <select
            v-model="mgGroupFilter"
            class="input h-8 w-56 py-0 text-xs"
            data-testid="management-group-filter"
            @change="onManagementGroupChange"
          >
            <option value="">{{ t('admin.iqDetection.allGroups') }}</option>
            <option v-for="group in groups" :key="group.id" :value="String(group.id)">
              {{ group.name }}
            </option>
          </select>
        </div>

        <div v-if="mgLoading" class="flex justify-center py-10 text-sm text-gray-500">
          {{ t('common.loading') }}
        </div>
        <template v-else>
          <div
            v-if="mgAccounts.length === 0"
            class="py-10 text-center text-sm text-gray-500"
            data-testid="management-empty"
          >
            {{ t('admin.iqDetection.noAccounts') }}
          </div>
          <ul v-else class="divide-y divide-gray-100 dark:divide-dark-700/60">
            <li
              v-for="account in mgAccounts"
              :key="account.id"
              class="flex flex-wrap items-center gap-3 px-4 py-3"
              :data-testid="`management-account-${account.id}`"
            >
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="truncate text-sm text-gray-800 dark:text-gray-200">
                    {{ account.name }}
                  </span>
                  <span class="text-xs text-gray-400">
                    {{ account.platform }}<template v-if="account.type"> · {{ account.type }}</template>
                  </span>
                  <span
                    v-if="savedCredentials[account.id]?.has_api_key"
                    class="rounded-full bg-green-50 px-2 py-0.5 text-xs text-green-700 dark:bg-green-900/30 dark:text-green-400"
                  >
                    {{ t('admin.iqDetection.savedBadge') }}
                  </span>
                </div>
                <div
                  v-if="managementApiUrl(account)"
                  class="truncate font-mono text-xs text-gray-400 dark:text-gray-500"
                  :title="managementApiUrl(account)"
                >
                  {{ managementApiUrl(account) }}
                </div>
              </div>
              <div class="flex items-center gap-2">
                <input
                  v-model="apiKeyInputs[account.id]"
                  type="password"
                  autocomplete="new-password"
                  class="input h-8 w-56 py-0 text-xs"
                  :placeholder="t('admin.iqDetection.apiKeyPlaceholder')"
                  :data-testid="`api-key-input-${account.id}`"
                />
                <button
                  type="button"
                  class="btn btn-primary btn-sm"
                  :disabled="mgSaving.has(account.id)"
                  :data-testid="`save-credential-${account.id}`"
                  @click="saveAccountCredential(account)"
                >
                  {{ t('admin.iqDetection.save') }}
                </button>
              </div>
            </li>
          </ul>
          <div
            class="flex items-center justify-between border-t border-gray-200 px-4 py-2 text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400"
          >
            <span>
              {{ t('admin.iqDetection.pageInfo', { page: mgPage, pages: mgPages, total: mgTotal }) }}
            </span>
            <div class="flex gap-2">
              <button type="button" class="btn btn-secondary btn-sm" :disabled="mgPage <= 1" @click="mgPage--; void loadManagementAccounts()">
                {{ t('admin.iqDetection.prevPage') }}
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="mgPage >= mgPages"
                @click="mgPage++; void loadManagementAccounts()"
              >
                {{ t('admin.iqDetection.nextPage') }}
              </button>
            </div>
          </div>
        </template>
      </section>
      </BaseDialog>

      <!-- 检测模型设置弹窗 -->
      <BaseDialog
        :show="modelSettingsOpen"
        :title="t('admin.iqDetection.detectionModelSettings')"
        width="normal"
        data-testid="model-settings-dialog"
        @close="modelSettingsOpen = false"
      >
        <div class="space-y-4">
          <p class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.iqDetection.modelsHint') }}
          </p>
          <div v-if="modelSettingsLoading" class="flex justify-center py-10 text-sm text-gray-500">
            {{ t('common.loading') }}
          </div>
          <template v-else>
            <div>
              <label class="input-label" for="detection-models-input">{{ t('admin.iqDetection.modelsLabel') }}</label>
              <textarea
                id="detection-models-input"
                v-model="modelSettingsText"
                class="input min-h-32 font-mono text-xs"
                rows="6"
                :placeholder="t('admin.iqDetection.modelsPlaceholder')"
                data-testid="detection-models-input"
              />
            </div>
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div>
                <label class="input-label" for="detection-effort">{{ t('admin.iqDetection.reasoningEffort') }}</label>
                <select
                  id="detection-effort"
                  v-model="reasoningEffort"
                  class="input"
                  data-testid="detection-effort-input"
                >
                  <option value="">{{ t('admin.iqDetection.effortDefault') }}</option>
                  <option value="low">{{ t('admin.iqDetection.effortLow') }}</option>
                  <option value="medium">{{ t('admin.iqDetection.effortMedium') }}</option>
                  <option value="high">{{ t('admin.iqDetection.effortHigh') }}</option>
                </select>
              </div>
              <div>
                <label class="input-label" for="detection-temperature">{{ t('admin.iqDetection.temperature') }}</label>
                <input
                  id="detection-temperature"
                  v-model="temperatureInput"
                  type="number"
                  min="0"
                  max="2"
                  step="0.1"
                  class="input"
                  :placeholder="t('admin.iqDetection.temperaturePlaceholder')"
                  data-testid="detection-temperature-input"
                />
              </div>
            </div>
            <div>
              <label class="input-label" for="pelican-test">{{ t('admin.iqDetection.pelicanTest') }}</label>
              <textarea
                id="pelican-test"
                v-model="pelicanPrompt"
                class="input min-h-24"
                rows="3"
                :placeholder="t('admin.iqDetection.customProbePlaceholder')"
                data-testid="pelican-test-input"
              />
            </div>
            <div>
              <label class="input-label" for="candy-test">{{ t('admin.iqDetection.candyTest') }}</label>
              <textarea
                id="candy-test"
                v-model="candyPrompt"
                class="input min-h-24"
                rows="3"
                :placeholder="t('admin.iqDetection.customProbePlaceholder')"
                data-testid="candy-test-input"
              />
            </div>
            <div>
              <label class="input-label" for="candy-answer">{{ t('admin.iqDetection.candyAnswer') }}</label>
              <input
                id="candy-answer"
                v-model="candyAnswer"
                type="text"
                class="input"
                :placeholder="t('admin.iqDetection.candyAnswerPlaceholder')"
                data-testid="candy-answer-input"
              />
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.iqDetection.candyAnswerHint') }}
              </p>
            </div>
            <div class="flex items-center justify-end gap-2">
              <button type="button" class="btn btn-secondary btn-sm" @click="modelSettingsOpen = false">
                {{ t('common.cancel') }}
              </button>
              <button
                type="button"
                class="btn btn-primary btn-sm"
                :disabled="modelSettingsSaving"
                data-testid="save-detection-models"
                @click="saveDetectionModels"
              >
                {{ t('admin.iqDetection.save') }}
              </button>
            </div>
          </template>
        </div>
      </BaseDialog>
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
              <button
                type="button"
                class="btn btn-primary btn-sm"
                :disabled="selectionSaving"
                data-testid="save-selection"
                @click="saveSelection"
              >
                {{ t('admin.iqDetection.save') }}
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
                class="flex cursor-pointer items-start gap-3 px-4 py-2.5 text-sm transition-colors hover:bg-gray-50 dark:hover:bg-dark-800"
                :data-testid="`account-item-${account.id}`"
              >
                <input
                  type="checkbox"
                  class="mt-0.5 h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                  :checked="selectedAccountIds.has(account.id)"
                  @change="toggleAccount(account.id)"
                />
                <span class="min-w-0 flex-1">
                  <span class="block truncate text-gray-700 dark:text-gray-300">
                    {{ account.name }}
                  </span>
                  <span
                    v-if="accountBaseUrl(account)"
                    class="block truncate font-mono text-xs text-gray-400 dark:text-gray-500"
                    :title="accountBaseUrl(account)"
                  >
                    {{ accountBaseUrl(account) }}
                  </span>
                  <span
                    v-if="accountModels(account).length > 0"
                    class="block truncate text-xs text-gray-400 dark:text-gray-500"
                    :title="accountModels(account).join(', ')"
                    :data-testid="`account-models-${account.id}`"
                  >
                    {{ accountModels(account).join(', ') }}
                  </span>
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
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type IQDetectAgentHealth, type SavedAccountCredential, type SelectionGroup } from '@/api/admin'
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
const accountManagerOpen = ref(false)

const groups = ref<AdminGroup[]>([])
const groupsLoading = ref(false)

const selectedGroupId = ref<number | null>(null)
const accountsLoading = ref(false)

// 聚焦分组的在途请求：切换分组前先取消，避免旧响应浪费和晚到覆盖。
let focusAbort: AbortController | null = null

function isRequestCanceled(error: unknown): boolean {
  return (error as { code?: string })?.code === 'ERR_CANCELED'
}

// 按模型厂商（platform）筛选分组；空串 = 全部。
const platformFilter = ref('')

// 自定义探针内容（鹈鹕测试 / 糖果测试），后续随检测请求一起提交。
const pelicanPrompt = ref('')
const candyPrompt = ref('')

// ---- 账号管理 ----
const mgAccounts = ref<AccountListItem[]>([])
const mgPage = ref(1)
const mgPageSize = 20
const mgTotal = ref(0)
const mgGroupFilter = ref('')
const mgLoading = ref(false)
const mgSaving = reactive(new Set<number>())
const apiKeyInputs = reactive<Record<number, string>>({})
const savedCredentials = reactive<Record<number, SavedAccountCredential>>({})

const mgPages = computed(() => Math.max(1, Math.ceil(mgTotal.value / mgPageSize)))

async function loadSavedCredentials() {
  try {
    const resp = await adminAPI.iqDetection.listAccountCredentials()
    for (const item of resp.items) {
      savedCredentials[item.account_id] = item
    }
  } catch {
    // sidecar 未启动时静默降级，账号列表仍可浏览
  }
}

async function loadManagementAccounts() {
  mgLoading.value = true
  try {
    const filters = mgGroupFilter.value ? { group: mgGroupFilter.value } : undefined
    const resp = await adminAPI.accounts.list(mgPage.value, mgPageSize, filters)
    mgAccounts.value = resp?.items || []
    mgTotal.value = resp?.total || 0
  } catch (error) {
    mgAccounts.value = []
    mgTotal.value = 0
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    mgLoading.value = false
  }
}

function onManagementGroupChange() {
  mgPage.value = 1
  void loadManagementAccounts()
}

// 保存优先使用 sidecar 已存的地址，其次取账号当前配置的 base_url。
function managementApiUrl(account: AccountListItem): string {
  return savedCredentials[account.id]?.api_url || accountBaseUrl(account)
}

async function saveAccountCredential(account: AccountListItem) {
  if (mgSaving.has(account.id)) return
  mgSaving.add(account.id)
  try {
    const saved = await adminAPI.iqDetection.saveAccountCredential({
      account_id: account.id,
      api_url: accountBaseUrl(account),
      api_key: (apiKeyInputs[account.id] || '').trim() || undefined
    })
    savedCredentials[account.id] = saved
    delete apiKeyInputs[account.id]
    appStore.showSuccess(t('admin.iqDetection.saved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    mgSaving.delete(account.id)
  }
}

// 分组账号列表与勾选状态按分组缓存；分组与账号默认都不选中。
const accountsByGroup = reactive(new Map<number, AccountListItem[]>())
const accountSelection = reactive(new Map<number, Set<number>>())
// 未加载账号的分组被"全选"时先记意图，加载后物化为具体 id 集合。
const groupAllIntent = reactive(new Set<number>())

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
    .map((entry) => {
      const set = accountSelection.get(entry.group.id)
      const count = set
        ? set.size
        : groupAllIntent.has(entry.group.id)
          ? entry.group.account_count ?? 0
          : 0
      return { id: entry.group.id, name: entry.group.name, count }
    })
)

const selectionSummary = computed(() => ({
  groups: selectedGroupEntries.value.length,
  accounts: selectedGroupEntries.value.reduce((sum, entry) => sum + entry.count, 0)
}))

// ---- 主页面选择概览：左侧分组列表 + 右侧账号卡片 ----
const summaryGroupId = ref<number | null>(null)
const summaryLoading = ref(false)

const summaryGroup = computed(
  () => selectedGroupEntries.value.find((entry) => entry.id === summaryGroupId.value) || null
)
const summaryAccounts = computed<AccountListItem[]>(() => {
  if (summaryGroupId.value == null) return []
  const all = accountsByGroup.get(summaryGroupId.value) ?? []
  const selected = accountSelection.get(summaryGroupId.value)
  if (!selected) return all
  return all.filter((a) => selected.has(a.id))
})

// 焦点分组被取消选择时自动落到第一个仍然选中的分组
watch(
  () => selectedGroupEntries.value.map((entry) => entry.id),
  (ids) => {
    if (!summaryGroupId.value || !ids.includes(summaryGroupId.value)) {
      summaryGroupId.value = ids[0] ?? null
    }
  }
)

async function focusSummaryGroup(groupId: number) {
  summaryGroupId.value = groupId
  if (accountsByGroup.has(groupId)) return
  summaryLoading.value = true
  try {
    await fetchAccounts(groupId)
  } catch (error) {
    if (summaryGroupId.value === groupId) {
      appStore.showError(extractApiErrorMessage(error, t('common.error')))
    }
  } finally {
    if (summaryGroupId.value === groupId) {
      summaryLoading.value = false
    }
  }
}

function openAccountManager() {
  accountManagerOpen.value = true
  if (mgAccounts.value.length === 0 && !mgLoading.value) {
    void loadSavedCredentials()
    void loadManagementAccounts()
  }
}

// ---- 检测模型设置 ----
const modelSettingsOpen = ref(false)
const modelSettingsLoading = ref(false)
const modelSettingsSaving = ref(false)
const modelSettingsText = ref('')
const reasoningEffort = ref('')
const temperatureInput = ref('')
const candyAnswer = ref('')

async function openModelSettings() {
  modelSettingsOpen.value = true
  modelSettingsLoading.value = true
  try {
    const resp = await adminAPI.iqDetection.getDetectionModels()
    modelSettingsText.value = resp.items.map((m) => m.name).join('\n')
    pelicanPrompt.value = resp.pelican_prompt
    candyPrompt.value = resp.candy_prompt
    candyAnswer.value = resp.candy_answer || ''
    reasoningEffort.value = resp.reasoning_effort || ''
    temperatureInput.value = resp.temperature == null ? '' : String(resp.temperature)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    modelSettingsLoading.value = false
  }
}

async function saveDetectionModels() {
  if (modelSettingsSaving.value) return
  modelSettingsSaving.value = true
  try {
    const names = modelSettingsText.value
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line !== '')
    const temperature = temperatureInput.value.trim() === '' ? null : Number(temperatureInput.value)
    await adminAPI.iqDetection.saveDetectionModels({
      names,
      pelican_prompt: pelicanPrompt.value,
      candy_prompt: candyPrompt.value,
      candy_answer: candyAnswer.value.trim(),
      reasoning_effort: reasoningEffort.value,
      temperature: temperature != null && Number.isFinite(temperature) ? temperature : null
    })
    appStore.showSuccess(t('admin.iqDetection.modelsSaved'))
    modelSettingsOpen.value = false
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    modelSettingsSaving.value = false
  }
}

// ---- 发起测试 ----
const testRunning = ref(false)

async function runTest() {
  if (testRunning.value) return
  testRunning.value = true
  try {
    const plan = await adminAPI.iqDetection.triggerTestRun()
    if (plan.triggered > 0) {
      appStore.showSuccess(t('admin.iqDetection.testTriggered', { n: plan.triggered }))
    } else {
      appStore.showError(t('admin.iqDetection.testNothing'))
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    testRunning.value = false
  }
}

// ---- 选择保存 / 回显 ----
const selectionSaving = ref(false)
const selectionSavedAt = ref<string | null>(null)

async function openSelector() {
  selectorOpen.value = true
  if (groups.value.length === 0 && !groupsLoading.value) {
    await loadGroups()
  }
  await loadSavedSelection()
}

async function loadSavedSelection() {
  try {
    const saved = await adminAPI.iqDetection.getSelection()
    if (!saved.updated_at) return // 从未保存过，保持默认不选中
    selectionSavedAt.value = saved.updated_at
    for (const savedGroup of saved.groups) {
      groupAllIntent.delete(savedGroup.id)
      accountSelection.set(savedGroup.id, new Set((savedGroup.accounts || []).map((a) => a.id)))
    }
  } catch {
    // sidecar 未启动时静默降级，保持默认
  }
}

async function saveSelection() {
  if (selectionSaving.value) return
  selectionSaving.value = true
  try {
    const payloadGroups: SelectionGroup[] = []
    for (const group of groups.value) {
      if (groupCheckState(group) === false) continue
      // 未加载过账号的分组先拉取，物化"全选"意图并落具体 id 与模型
      if (!accountsByGroup.has(group.id)) {
        await fetchAccounts(group.id)
      }
      const accounts = accountsByGroup.get(group.id) ?? []
      const selected = accountSelection.get(group.id) ?? new Set<number>()
      payloadGroups.push({
        id: group.id,
        name: group.name,
        rate_multiplier: group.rate_multiplier,
        accounts: accounts
          .filter((a) => selected.has(a.id))
          .map((a) => ({ id: a.id, name: a.name, models: accountModels(a) }))
      })
    }
    const saved = await adminAPI.iqDetection.saveSelection(payloadGroups)
    selectionSavedAt.value = saved.updated_at ?? null
    appStore.showSuccess(t('admin.iqDetection.selectionSaved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    selectionSaving.value = false
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

async function fetchAccounts(groupId: number, signal?: AbortSignal): Promise<AccountListItem[]> {
  const resp = await adminAPI.accounts.list(1, 200, { group: String(groupId) }, { signal })
  const items = resp?.items || []
  accountsByGroup.set(groupId, items)
  if (groupAllIntent.has(groupId)) {
    // "全选"意图落地为具体 id 集合
    accountSelection.set(groupId, new Set(items.map((a) => a.id)))
    groupAllIntent.delete(groupId)
  } else if (!accountSelection.has(groupId)) {
    // 默认不选中
    accountSelection.set(groupId, new Set<number>())
  }
  return items
}

async function focusGroup(groupId: number) {
  if (selectedGroupId.value === groupId) return
  selectedGroupId.value = groupId
  // 取消上一个未完成的账号列表请求（无论新分组是否命中缓存）。
  focusAbort?.abort()
  focusAbort = null
  if (accountsByGroup.has(groupId)) {
    // 上一个分组的加载可能残留 loading，命中缓存时直接就绪。
    accountsLoading.value = false
    return
  }
  const controller = new AbortController()
  focusAbort = controller
  accountsLoading.value = true
  try {
    await fetchAccounts(groupId, controller.signal)
  } catch (error) {
    if (selectedGroupId.value === groupId && !isRequestCanceled(error)) {
      appStore.showError(extractApiErrorMessage(error, t('common.error')))
    }
  } finally {
    // 被同组的新请求顶替时由新请求管理 loading；被其他分组顶替时 selectedGroupId 已变。
    if (selectedGroupId.value === groupId) {
      accountsLoading.value = false
      if (focusAbort === controller) focusAbort = null
    }
  }
}

// 分组 checkbox 三态：true = 账号全选；false = 全不选；'partial' = 部分选中。
// 从未操作过的分组默认全不选。
function groupCheckState(group: AdminGroup): boolean | 'partial' {
  if (groupAllIntent.has(group.id)) return true
  const set = accountSelection.get(group.id)
  if (!set || set.size === 0) return false
  const total = accountsByGroup.get(group.id)?.length ?? group.account_count ?? 0
  if (total > 0 && set.size >= total) return true
  return 'partial'
}

async function toggleGroupCheck(group: AdminGroup) {
  const turnOn = groupCheckState(group) !== true
  if (turnOn) {
    if (accountsByGroup.has(group.id)) {
      const ids = (accountsByGroup.get(group.id) ?? []).map((a) => a.id)
      accountSelection.set(group.id, new Set(ids))
    } else {
      // 未加载：记意图，账号加载后再物化
      accountSelection.delete(group.id)
      groupAllIntent.add(group.id)
    }
  } else {
    groupAllIntent.delete(group.id)
    accountSelection.set(group.id, new Set<number>())
  }
}

// 分组全选/清空只作用于当前筛选可见的分组。
function selectAllGroups() {
  for (const group of filteredGroups.value) {
    const cached = accountsByGroup.get(group.id)
    if (cached) {
      accountSelection.set(group.id, new Set(cached.map((a) => a.id)))
    } else {
      accountSelection.delete(group.id)
      groupAllIntent.add(group.id)
    }
  }
}

function clearAllGroups() {
  for (const group of filteredGroups.value) {
    groupAllIntent.delete(group.id)
    accountSelection.set(group.id, new Set<number>())
  }
}

// 账号的上游 API 地址（credentials.base_url 为非敏感字段，接口明文返回）。
function accountBaseUrl(account: AccountListItem): string {
  const raw = account.credentials?.base_url
  return typeof raw === 'string' ? raw.trim() : ''
}

// 账号可用的模型列表：取 credentials.model_mapping 的键（非敏感字段）。
function accountModels(account: AccountListItem): string[] {
  const mapping = account.credentials?.model_mapping
  if (mapping && typeof mapping === 'object' && !Array.isArray(mapping)) {
    return Object.keys(mapping).map(String).filter((m) => m.trim() !== '').sort()
  }
  return []
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
