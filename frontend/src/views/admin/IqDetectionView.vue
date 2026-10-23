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
        <div class="ml-auto flex items-center gap-2">
          <button
            type="button"
            class="btn btn-primary btn-sm"
            :disabled="testRunning"
            data-testid="run-all-tests"
            @click="runTest()"
          >
            <Icon name="refresh" size="sm" :class="testRunning ? 'animate-spin' : ''" />
            {{ t('admin.iqDetection.testAll') }}
          </button>
          <button
            type="button"
            class="btn btn-primary btn-sm"
            :disabled="testRunning || !canTestCurrentGroup"
            :title="t('admin.iqDetection.testCurrentGroupHint')"
            data-testid="run-test"
            @click="runCurrentGroupTest"
          >
            <Icon name="refresh" size="sm" :class="testRunning ? 'animate-spin' : ''" />
            {{ t('admin.iqDetection.test') }}
          </button>
        </div>
      </div>

      <!-- 测试结果：卡片列表；鹈鹕 HTML 按需拉取后渲染进无脚本沙箱 iframe -->
      <section
        v-if="health?.enabled"
        class="border border-gray-200 dark:border-dark-700"
        data-testid="test-results"
      >
        <div
          class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 px-4 py-3 dark:border-dark-700"
        >
          <h3 class="text-sm font-medium text-gray-900 dark:text-white">
            {{ t('admin.iqDetection.testResults') }}
            <span v-if="testResultsTotal > 0" class="ml-2 text-xs font-normal text-gray-400">
              {{ t('admin.iqDetection.testResultsTotal', { n: testResultsTotal }) }}
            </span>
          </h3>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="testResultsLoading"
            data-testid="refresh-test-results"
            @click="reloadTestResults()"
          >
            <Icon name="refresh" size="sm" :class="testResultsLoading ? 'animate-spin' : ''" />
            {{ t('common.refresh') }}
          </button>
        </div>

        <!-- 按选择的分组分 tab：组名 + 选中账号数 + 检测间隔 -->
        <div
          v-if="resultTabs.length > 0"
          class="flex flex-wrap gap-1 border-b border-gray-200 px-3 pt-1 dark:border-dark-700"
          data-testid="test-result-tabs"
        >
          <button
            v-for="tab in resultTabs"
            :key="tab.id"
            type="button"
            class="rounded-t-md border-b-2 px-3 py-2 text-sm transition-colors"
            :class="
              tab.id === activeResultGroupId
                ? 'border-primary-500 font-medium text-primary-700 dark:text-primary-300'
                : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'
            "
            :data-testid="`test-result-tab-${tab.id}`"
            @click="activeResultGroupId = tab.id"
          >
            {{ tab.label }}
            <span class="text-xs font-normal text-gray-400">
              {{ tab.accounts }}
              <template v-if="tab.intervalMinutes > 0">
                · {{ t('admin.iqDetection.intervalEvery', { n: tab.intervalMinutes }) }}
              </template>
            </span>
          </button>
        </div>

        <div
          v-if="testResultsLoading && testResults.length === 0"
          class="flex justify-center py-10 text-sm text-gray-500"
        >
          {{ t('common.loading') }}
        </div>
        <div
          v-else-if="testResults.length === 0"
          class="py-10 text-center text-sm text-gray-500"
          data-testid="test-results-empty"
        >
          {{
            resultTabs.length === 0
              ? t('admin.iqDetection.noSelection')
              : t('admin.iqDetection.testResultsEmpty')
          }}
        </div>
        <div v-else class="grid gap-4 p-4 md:grid-cols-2 xl:grid-cols-3" data-testid="test-results-grid">
          <article
            v-for="item in testResults"
            :key="item.id"
            class="flex flex-col gap-3 border border-gray-200 p-4 dark:border-dark-700"
            :data-testid="`test-result-card-${item.id}`"
          >
            <header class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <p class="truncate text-sm font-medium text-gray-900 dark:text-white">
                  {{ item.account_name || item.account_id }}
                </p>
                <p class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">
                  {{ item.model }} · {{ formatTestResultTime(item.created_at) }}
                </p>
              </div>
              <span
                class="flex flex-shrink-0 items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium"
                :class="statusBadgeClass(item.status)"
                :data-testid="`test-result-status-${item.id}`"
              >
                <Icon v-if="item.status === 'running'" name="refresh" size="sm" class="animate-spin" />
                {{ statusText(item.status) }}
              </span>
            </header>

            <p v-if="item.status === 'running'" class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.iqDetection.testRunningHint') }}
            </p>
            <template v-else>
              <!-- 糖果测试：结果 + 起止时间 + 人工复核 -->
              <div class="text-xs" :data-testid="`test-result-candy-section-${item.id}`">
                <div class="flex items-center justify-between gap-2">
                  <span class="font-medium text-gray-500 dark:text-gray-400">
                    {{ t('admin.iqDetection.candyTest') }}
                  </span>
                  <span class="text-gray-400">
                    {{ t('admin.iqDetection.startedAt') }} {{ formatTimeOfDay(item.candy_started_at) }}
                    <template v-if="item.candy_finished_at">
                      → {{ t('admin.iqDetection.finishedAt') }} {{ formatTimeOfDay(item.candy_finished_at) }}
                    </template>
                  </span>
                </div>
                <p class="mt-1 text-gray-600 dark:text-gray-300">
                  <span v-if="item.candy_error" class="text-red-600 dark:text-red-400">
                    {{ t('admin.iqDetection.testResultFailed') }}（{{ item.candy_error }}）
                  </span>
                  <template v-else>
                    {{ candyFinalAnswer(item.candy_answer) || '-' }}
                    <span v-if="item.candy_latency_ms" class="text-gray-400">
                      · {{ formatLatency(item.candy_latency_ms) }}
                    </span>
                  </template>
                </p>
                <div class="mt-1.5 flex items-center gap-1.5">
                  <span class="text-gray-400">{{ t('admin.iqDetection.reviewLabel') }}</span>
                  <button
                    type="button"
                    class="rounded px-1.5 py-0.5 transition-colors"
                    :class="reviewButtonClass(item.candy_review, 'approved')"
                    :disabled="reviewSaving.has(`${item.id}:candy`)"
                    :data-testid="`test-result-candy-approve-${item.id}`"
                    @click="setReview(item, 'candy', 'approved')"
                  >
                    {{ t('admin.iqDetection.reviewApprove') }}
                  </button>
                  <button
                    type="button"
                    class="rounded px-1.5 py-0.5 transition-colors"
                    :class="reviewButtonClass(item.candy_review, 'rejected')"
                    :disabled="reviewSaving.has(`${item.id}:candy`)"
                    :data-testid="`test-result-candy-reject-${item.id}`"
                    @click="setReview(item, 'candy', 'rejected')"
                  >
                    {{ t('admin.iqDetection.reviewReject') }}
                  </button>
                </div>
              </div>

              <!-- 鹈鹕测试：结果 + 起止时间 + 人工复核 + 渲染 -->
              <div class="text-xs" :data-testid="`test-result-pelican-section-${item.id}`">
                <div class="flex items-center justify-between gap-2">
                  <span class="font-medium text-gray-500 dark:text-gray-400">
                    {{ t('admin.iqDetection.pelicanTest') }}
                  </span>
                  <span class="text-gray-400">
                    {{ t('admin.iqDetection.startedAt') }} {{ formatTimeOfDay(item.pelican_started_at) }}
                    <template v-if="item.pelican_finished_at">
                      → {{ t('admin.iqDetection.finishedAt') }} {{ formatTimeOfDay(item.pelican_finished_at) }}
                    </template>
                  </span>
                </div>
                <p class="mt-1 text-gray-600 dark:text-gray-300">
                  <span v-if="item.pelican_error" class="text-red-600 dark:text-red-400">
                    {{ t('admin.iqDetection.testResultFailed') }}（{{ item.pelican_error }}）
                  </span>
                  <span v-else-if="item.pelican_html_len">
                    {{ t('admin.iqDetection.pelicanReady', { size: formatBytes(item.pelican_html_len) }) }}
                  </span>
                  <template v-else>-</template>
                </p>
                <div class="mt-1.5 flex items-center gap-1.5">
                  <span class="text-gray-400">{{ t('admin.iqDetection.reviewLabel') }}</span>
                  <button
                    type="button"
                    class="rounded px-1.5 py-0.5 transition-colors"
                    :class="reviewButtonClass(item.pelican_review, 'approved')"
                    :disabled="reviewSaving.has(`${item.id}:pelican`)"
                    :data-testid="`test-result-pelican-approve-${item.id}`"
                    @click="setReview(item, 'pelican', 'approved')"
                  >
                    {{ t('admin.iqDetection.reviewApprove') }}
                  </button>
                  <button
                    type="button"
                    class="rounded px-1.5 py-0.5 transition-colors"
                    :class="reviewButtonClass(item.pelican_review, 'rejected')"
                    :disabled="reviewSaving.has(`${item.id}:pelican`)"
                    :data-testid="`test-result-pelican-reject-${item.id}`"
                    @click="setReview(item, 'pelican', 'rejected')"
                  >
                    {{ t('admin.iqDetection.reviewReject') }}
                  </button>
                </div>
              </div>

              <button
                v-if="!item.pelican_error && item.pelican_html_len"
                type="button"
                class="btn btn-secondary btn-sm self-start"
                :disabled="pelicanLoading.has(item.id)"
                :data-testid="`test-result-toggle-pelican-${item.id}`"
                @click="openPelicanDialog(item)"
              >
                {{ t('admin.iqDetection.showPelican') }}
              </button>
            </template>
          </article>
        </div>
        <div
          v-if="testResults.length < testResultsTotal"
          class="border-t border-gray-100 px-4 py-3 text-center dark:border-dark-700/60"
        >
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="testResultsLoading"
            data-testid="test-results-load-more"
            @click="loadMoreTestResults"
          >
            {{ t('admin.iqDetection.loadMoreResults') }}
          </button>
        </div>
      </section>



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
            <div>
              <span class="input-label">{{ t('admin.iqDetection.quietWindows') }}</span>
              <div class="space-y-2">
                <div
                  v-for="(window, index) in quietWindows"
                  :key="index"
                  class="flex items-center gap-2"
                  :data-testid="`quiet-window-row-${index}`"
                >
                  <input
                    v-model="window.start"
                    type="time"
                    class="input h-8 w-28 py-0 text-xs"
                    :data-testid="`quiet-window-start-${index}`"
                  />
                  <span class="text-xs text-gray-400">→</span>
                  <input
                    v-model="window.end"
                    type="time"
                    class="input h-8 w-28 py-0 text-xs"
                    :data-testid="`quiet-window-end-${index}`"
                  />
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm px-2"
                    :title="t('admin.iqDetection.quietWindowRemove')"
                    :data-testid="`quiet-window-remove-${index}`"
                    @click="removeQuietWindow(index)"
                  >
                    <Icon name="trash" size="sm" />
                  </button>
                </div>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  data-testid="quiet-window-add"
                  @click="addQuietWindow"
                >
                  <Icon name="plus" size="sm" />
                  {{ t('admin.iqDetection.quietWindowAdd') }}
                </button>
              </div>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.iqDetection.quietWindowsHint') }}
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
            <div v-if="selectedGroupId" class="flex flex-wrap items-center gap-2">
              <label
                class="flex cursor-pointer items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400"
                :title="t('admin.iqDetection.visibleHint')"
              >
                <input
                  type="checkbox"
                  class="h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                  :checked="groupVisibility[selectedGroupId] !== false"
                  data-testid="group-visible-checkbox"
                  @change="groupVisibility[selectedGroupId] = ($event.target as HTMLInputElement).checked"
                />
                {{ t('admin.iqDetection.visible') }}
              </label>
              <label class="flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.iqDetection.testInterval') }}
                <input
                  type="number"
                  min="0"
                  step="5"
                  class="input h-8 w-20 py-0 text-xs"
                  v-model.number="groupIntervals[selectedGroupId]"
                  :placeholder="t('admin.iqDetection.intervalPlaceholder')"
                  data-testid="group-interval-input"
                />
                {{ t('admin.iqDetection.intervalUnit') }}
              </label>
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

    <!-- 鹈鹕渲染模态框：大视口沙箱 iframe 预览 -->
    <BaseDialog
      :show="pelicanDialogOpen"
      :title="pelicanDialogTitle"
      width="full"
      data-testid="pelican-render-dialog"
      @close="pelicanDialogOpen = false"
    >
      <div
        v-if="pelicanDialogItem && pelicanLoading.has(pelicanDialogItem.id)"
        class="flex h-[60vh] items-center justify-center text-sm text-gray-500"
      >
        {{ t('common.loading') }}
      </div>
      <iframe
        v-else-if="pelicanDialogItem"
        class="h-[70vh] w-full bg-white"
        sandbox=""
        data-testid="pelican-render-frame"
        :srcdoc="pelicanHtmlCache.get(pelicanDialogItem.id) || ''"
        title="pelican"
      />
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type IQDetectAgentHealth, type QuietWindow, type SelectionGroup, type TestResultItem, type TestResultProbe, type TestResultVerdict } from '@/api/admin'
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

// 分组账号列表与勾选状态按分组缓存；分组与账号默认都不选中。
const accountsByGroup = reactive(new Map<number, AccountListItem[]>())
const accountSelection = reactive(new Map<number, Set<number>>())
// 未加载账号的分组被"全选"时先记意图，加载后物化为具体 id 集合。
const groupAllIntent = reactive(new Set<number>())
// 每个分组的智商检测间隔（分钟），0 = 仅手动触发。
const groupIntervals = reactive<Record<number, number>>({})
const groupVisibility = reactive<Record<number, boolean>>({})

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

// ---- 检测模型设置 ----
const modelSettingsOpen = ref(false)
const modelSettingsLoading = ref(false)
const modelSettingsSaving = ref(false)
const modelSettingsText = ref('')
const reasoningEffort = ref('')
const temperatureInput = ref('')
const candyAnswer = ref('')
// 不检测时间段（北京时间），支持跨天（如 23:00-07:00）
const quietWindows = ref<QuietWindow[]>([])

function addQuietWindow() {
  quietWindows.value.push({ start: '', end: '' })
}

function removeQuietWindow(index: number) {
  quietWindows.value.splice(index, 1)
}

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
    quietWindows.value = (resp.quiet_windows || []).map((w) => ({ start: w.start, end: w.end }))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    modelSettingsLoading.value = false
  }
}

async function saveDetectionModels() {
  if (modelSettingsSaving.value) return
  // 只保留填写完整的行；半填的行直接报错提示
  const windows = quietWindows.value.filter((w) => w.start && w.end)
  if (quietWindows.value.some((w) => (!w.start && w.end) || (w.start && !w.end))) {
    appStore.showError(t('admin.iqDetection.quietWindowIncomplete'))
    return
  }
  modelSettingsSaving.value = true
  try {
    const names = modelSettingsText.value
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line !== '')
    // v-model 在 type="number" 输入框上会把值转成数字，先归一化为字符串再判空
    const temperatureRaw = String(temperatureInput.value ?? '').trim()
    const temperature = temperatureRaw === '' ? null : Number(temperatureRaw)
    await adminAPI.iqDetection.saveDetectionModels({
      names,
      pelican_prompt: pelicanPrompt.value,
      candy_prompt: candyPrompt.value,
      candy_answer: candyAnswer.value.trim(),
      reasoning_effort: reasoningEffort.value,
      temperature: temperature != null && Number.isFinite(temperature) ? temperature : null,
      quiet_windows: windows
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

async function runTest(groupId?: number) {
  if (testRunning.value) return
  testRunning.value = true
  try {
    const plan = groupId === undefined
      ? await adminAPI.iqDetection.triggerTestRun()
      : await adminAPI.iqDetection.triggerTestRun(groupId)
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

// ---- 测试结果 ----
const TEST_RESULTS_PAGE_SIZE = 12
const testResults = ref<TestResultItem[]>([])
const testResultsTotal = ref(0)
const testResultsLoading = ref(false)
const pelicanHtmlCache = reactive(new Map<number, string>())
const pelicanLoading = reactive(new Set<number>())

// 已保存的选择（用于结果 tab 与轮换展示）；activeResultGroupId=0 表示未选分组时展示全部
const savedSelectionGroups = ref<SelectionGroup[]>([])
const activeResultGroupId = ref(0)
const canTestCurrentGroup = computed(() =>
  activeResultGroupId.value > 0 && savedSelectionGroups.value.some(group => group.id === activeResultGroupId.value)
)

function runCurrentGroupTest() {
  if (!canTestCurrentGroup.value) return
  return runTest(activeResultGroupId.value)
}

const resultTabs = computed(() => {
  const tabs = savedSelectionGroups.value.map((group) => ({
    id: group.id,
    label: group.name,
    accounts: (group.accounts || []).length,
    intervalMinutes: group.interval_minutes || 0
  }))
  // 兜底：无法归属到任何已选分组的历史结果收进「未分组」
  if (tabs.length > 0) {
    tabs.push({
      id: 0,
      label: t('admin.iqDetection.ungroupedTab'),
      accounts: 0,
      intervalMinutes: 0
    })
  }
  return tabs
})

watch(activeResultGroupId, () => {
  void loadTestResults(0)
})

// 请求序号：分组 tab 切换/初始加载可能并发，晚到的过期响应直接丢弃
let testResultsRequestId = 0

async function loadTestResults(offset: number) {
  const requestId = ++testResultsRequestId
  testResultsLoading.value = true
  try {
    const resp = await adminAPI.iqDetection.listTestResults(
      TEST_RESULTS_PAGE_SIZE,
      offset,
      // 有 tab 时始终按 tab 过滤（0 = 未分组）；无 tab 时返回全部
      resultTabs.value.length > 0 ? activeResultGroupId.value : undefined
    )
    if (requestId !== testResultsRequestId) return
    testResults.value = offset === 0 ? resp.items : [...testResults.value, ...resp.items]
    testResultsTotal.value = resp.total
  } catch {
    // sidecar 未启动时静默降级（页面顶部已有服务未启动横幅）
    if (offset === 0 && requestId === testResultsRequestId) {
      testResults.value = []
      testResultsTotal.value = 0
    }
  } finally {
    if (requestId === testResultsRequestId) {
      testResultsLoading.value = false
    }
  }
  scheduleTestResultsPoll()
}

function reloadTestResults() {
  return loadTestResults(0)
}

function loadMoreTestResults() {
  return loadTestResults(testResults.value.length)
}

// 鹈鹕渲染模态框：HTML 按需拉取后缓存
const pelicanDialogItem = ref<TestResultItem | null>(null)
const pelicanDialogOpen = ref(false)

const pelicanDialogTitle = computed(() =>
  pelicanDialogItem.value
    ? t('admin.iqDetection.pelicanRenderTitle', {
        name: pelicanDialogItem.value.account_name || pelicanDialogItem.value.account_id
      })
    : t('admin.iqDetection.showPelican')
)

async function openPelicanDialog(item: TestResultItem) {
  pelicanDialogItem.value = item
  pelicanDialogOpen.value = true
  if (!pelicanHtmlCache.has(item.id)) {
    pelicanLoading.add(item.id)
    try {
      pelicanHtmlCache.set(item.id, await adminAPI.iqDetection.getTestResultPelican(item.id))
    } catch (error) {
      appStore.showError(extractApiErrorMessage(error, t('common.error')))
    } finally {
      pelicanLoading.delete(item.id)
    }
  }
}

function formatTestResultTime(value: string): string {
  const ts = Date.parse(value)
  return Number.isNaN(ts) ? value : new Date(ts).toLocaleString()
}

function formatLatency(ms: number): string {
  return ms >= 1000 ? `${(ms / 1000).toFixed(1)}s` : `${ms}ms`
}

function formatBytes(len?: number): string {
  if (!len) return '0B'
  if (len >= 1024 * 1024) return `${(len / 1024 / 1024).toFixed(1)}MB`
  if (len >= 1024) return `${Math.round(len / 1024)}KB`
  return `${len}B`
}

// 只展示模型的最终作答（判分依据），不显示整段回复内容
function candyFinalAnswer(answer?: string): string {
  if (!answer) return ''
  const match = answer.match(/<final_answer>([\s\S]*?)<\/final_answer>/)
  return match ? match[1].trim() : ''
}

// ---- 测试状态与人工复核 ----
let testResultsPollTimer: ReturnType<typeof setTimeout> | null = null

function scheduleTestResultsPoll() {
  if (testResultsPollTimer) clearTimeout(testResultsPollTimer)
  if (!testResults.value.some((item) => item.status === 'running')) return
  testResultsPollTimer = setTimeout(() => {
    testResultsPollTimer = null
    void loadTestResults(0)
  }, 5000)
}

function statusBadgeClass(status: string): string {
  if (status === 'running') return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-400'
  if (status === 'failed') return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-400'
  if (status === 'stopped') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-400'
  return 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
}

function statusText(status: string): string {
  if (status === 'running') return t('admin.iqDetection.statusRunning')
  if (status === 'failed') return t('admin.iqDetection.statusFailed')
  if (status === 'stopped') return t('admin.iqDetection.statusStopped')
  return t('admin.iqDetection.statusCompleted')
}

const reviewSaving = reactive(new Set<string>())

async function setReview(item: TestResultItem, probe: TestResultProbe, verdict: TestResultVerdict) {
  const key = `${item.id}:${probe}`
  if (reviewSaving.has(key)) return
  reviewSaving.add(key)
  try {
    await adminAPI.iqDetection.updateTestResultReview(item.id, probe, verdict)
    if (probe === 'candy') item.candy_review = verdict
    else item.pelican_review = verdict
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    reviewSaving.delete(key)
  }
}

function reviewButtonClass(current: string, verdict: TestResultVerdict): string {
  if (current === verdict) {
    return verdict === 'approved'
      ? 'bg-green-100 font-medium text-green-700 dark:bg-green-900/40 dark:text-green-400'
      : 'bg-red-100 font-medium text-red-700 dark:bg-red-900/40 dark:text-red-400'
  }
  return 'text-gray-400 hover:bg-gray-100 dark:hover:bg-dark-700'
}

function formatTimeOfDay(value?: string): string {
  if (!value) return '-'
  const ts = Date.parse(value)
  return Number.isNaN(ts) ? '-' : new Date(ts).toLocaleTimeString()
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
    savedSelectionGroups.value = saved.groups
    for (const savedGroup of saved.groups) {
      groupAllIntent.delete(savedGroup.id)
      accountSelection.set(savedGroup.id, new Set((savedGroup.accounts || []).map((a) => a.id)))
      groupIntervals[savedGroup.id] = savedGroup.interval_minutes || 0
      groupVisibility[savedGroup.id] = savedGroup.visible !== false
    }
    // 结果 tab 默认落在第一个分组
    if (
      saved.groups.length > 0 &&
      !saved.groups.some((group) => group.id === activeResultGroupId.value)
    ) {
      activeResultGroupId.value = saved.groups[0].id
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
      const interval = Number(groupIntervals[group.id]) || 0
      payloadGroups.push({
        id: group.id,
        name: group.name,
        rate_multiplier: group.rate_multiplier,
        interval_minutes: interval,
        visible: groupVisibility[group.id] !== false,
        accounts: accounts
          .filter((a) => selected.has(a.id))
          .map((a) => ({ id: a.id, name: a.name, models: accountModels(a) }))
      })
    }
    const saved = await adminAPI.iqDetection.saveSelection(payloadGroups)
    savedSelectionGroups.value = saved.groups
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
  // 每次进入页面回显已保存的分组与账号选择（结果 tab 依赖它，先于结果加载）
  await loadSavedSelection()
  if (health.value?.enabled) {
    void loadTestResults(0)
  }
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
onUnmounted(() => {
  if (testResultsPollTimer) clearTimeout(testResultsPollTimer)
})
</script>
