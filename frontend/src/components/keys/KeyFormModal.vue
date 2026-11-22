<template>
  <BaseDialog :show="show" :title="title" width="medium-wide" @close="emit('close')">
    <form id="key-form" class="key-form space-y-5" @submit.prevent="emit('submit')">
      <!-- 第一行：名称 -->
      <div>
        <label for="key-form-name" class="input-label">{{ t('keys.nameLabel') }}</label>
        <input
          id="key-form-name"
          v-model="formData.name"
          type="text"
          required
          class="input"
          :placeholder="t('keys.namePlaceholder')"
          data-tour="key-form-name"
          @input="nameEditedByUser = true"
        />
      </div>

      <!-- 分组选择：创建/编辑共用，左侧分类 tab，右侧分组 list -->
      <div class="space-y-2">
        <label class="input-label">{{ t('keys.groupLabel') }}</label>
        <!-- 全局搜索：置于分组区域最上方，过滤所有分类下的分组 -->
        <div class="key-group-search relative shrink-0">
          <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
          <input
            v-model="groupSearch"
            type="search"
            class="input pl-9"
            :placeholder="t('keys.searchGroup')"
            :aria-label="t('keys.searchGroup')"
          />
        </div>
        <div
          data-tour="key-form-group"
          class="group-layout"
          role="tablist"
          :aria-label="t('keys.selectGroup')"
        >
          <!-- 左侧：分类 tab -->
          <nav class="group-layout-tabs" :aria-label="t('keys.selectGroup')">
            <button
              v-for="tab in groupTabs"
              :key="tab.id"
              type="button"
              role="tab"
              class="group-layout-tab"
              :class="{
                'group-layout-tab-active': activeCategoryTab === tab.id,
                'group-layout-tab-recommended': tab.recommended
              }"
              :aria-selected="activeCategoryTab === tab.id"
              :title="tab.description || tab.name"
              @click="activeCategoryTab = tab.id"
            >
              <Icon v-if="tab.recommended" name="trophy" size="sm" class="text-amber-500" />
              <span class="min-w-0 flex-1 truncate text-left">{{ tab.name }}</span>
              <span class="shrink-0 text-[11px] font-normal text-gray-400 dark:text-gray-500">{{ tab.options.length }}</span>
            </button>
          </nav>

          <!-- 右侧：分组 list -->
          <div class="group-layout-main">
            <div ref="groupListRef" class="group-layout-list">
              <div v-if="activeBrandSections.length" class="space-y-4">
                <section
                  v-for="section in activeBrandSections"
                  :key="section.platform"
                  class="space-y-1.5"
                >
                  <div class="flex items-center gap-2">
                    <PlatformIcon :platform="section.platform" size="sm" class="text-gray-500 dark:text-gray-300" />
                    <h4 class="text-sm font-semibold text-gray-800 dark:text-gray-100">{{ section.label }}</h4>
                    <span class="text-xs text-gray-400 dark:text-gray-500">{{ section.options.length }}</span>
                  </div>
                  <div class="space-y-2">
                    <div
                      v-for="option in section.options"
                      :key="option.value"
                      role="button"
                      tabindex="0"
                      class="group-row"
                      :data-group-id="option.value"
                      :class="{ 'group-row-selected': formData.group_id === option.value }"
                      :aria-pressed="formData.group_id === option.value"
                      @click="selectGroup(option)"
                      @keydown.enter.prevent="selectGroup(option)"
                      @keydown.space.prevent="selectGroup(option)"
                    >
                      <GroupOptionItem
                        class="w-full"
                        :name="option.label"
                        :platform="option.platform"
                        :subscription-type="option.subscriptionType"
                        :rate-multiplier="option.rate"
                        :user-rate-multiplier="option.userRate"
                        :peak-rate-enabled="option.peakRateEnabled"
                        :peak-start="option.peakStart"
                        :peak-end="option.peakEnd"
                        :peak-rate-multiplier="option.peakRateMultiplier"
                        :description="option.description"
                        :selected="formData.group_id === option.value"
                        :show-rate-label="false"
                      />
                      <div
                        v-if="option.recommendationReason || option.recommendationRating != null"
                        class="recommendation-note"
                      >
                        <div class="flex w-full items-center justify-between gap-3">
                          <span class="recommendation-note-label">
                            {{ t('keys.recommendationReason') }}
                          </span>
                          <span
                            v-if="option.recommendationRating != null"
                            class="recommendation-note-stars"
                            :aria-label="t('keys.recommendationReason')"
                          >
                            <span
                              v-for="index in 5"
                              :key="index"
                              class="recommendation-note-star"
                            >{{ recommendationStarEmoji(option.recommendationRating, index) }}</span>
                          </span>
                        </div>
                        <p
                          v-if="option.recommendationReason"
                          class="recommendation-note-text"
                          :title="option.recommendationReason"
                        >
                          {{ option.recommendationReason }}
                        </p>
                      </div>
                      <!-- 底部：上限倍率与使用限制(左) | 模型价格入口(右) -->
                      <div class="group-row-footer">
                        <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
                          <span
                            v-if="capFor(option.value) != null"
                            class="group-cap-tag"
                            :title="t('keys.capRateHint')"
                          >
                            {{ t('keys.capRateTag', { rate: formatCapRate(capFor(option.value)!) }) }}
                          </span>
                          <span
                            v-for="(tag, index) in groupTags?.[String(option.value)] ?? []"
                            :key="index"
                            class="group-custom-tag"
                            data-test="group-custom-tag"
                            :title="tag.description || undefined"
                          >{{ tag.name }}</span>
                          <div v-if="option.requiredProtocol || option.blockedClients?.length" class="contents" data-test="group-policy-tags">
                            <span v-if="option.requiredProtocol" class="inline-flex flex-none cursor-help items-center whitespace-nowrap rounded-full bg-blue-50 px-2 py-0.5 text-[10px] font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-300" :title="t('keys.requiredProtocolTagHint', { protocol: protocolNames[option.requiredProtocol] })">
                              {{ t('keys.requiredProtocolTag', { protocol: protocolNames[option.requiredProtocol] }) }}
                            </span>
                            <span v-for="client in option.blockedClients" :key="client" class="inline-flex flex-none cursor-help items-center whitespace-nowrap rounded-full bg-amber-50 px-2 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-300" :title="t('keys.blockedClientTagHint', { client })">
                              {{ t('keys.blockedClientTag', { client }) }}
                            </span>
                          </div>
                        </div>
                        <button
                          type="button"
                          class="group-pricing-link ml-auto shrink-0"
                          @click.stop="openModelPricing(option.value)"
                        >
                          <Icon name="chart" size="xs" />
                          <span>{{ t('keys.modelPricing') }}</span>
                        </button>
                      </div>
                    </div>
                  </div>
                </section>
              </div>
              <div
                v-else
                class="rounded-xl border border-dashed border-gray-300 px-4 py-8 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
              >
                {{ t('keys.noGroupFound') }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 编辑模式：状态（编辑状态专属，直接展示） -->
      <div v-if="isEdit" class="space-y-3">
        <label class="input-label">{{ t('keys.statusLabel') }}</label>
        <Select
          v-model="formData.status"
          :options="statusOptions"
          :placeholder="t('keys.selectStatus')"
          :aria-label="t('keys.statusLabel')"
        >
          <template #option="{ option, selected }">
            <span class="key-status-option">{{ option.label }}</span>
            <Icon v-if="selected" name="check" size="sm" />
          </template>
        </Select>
      </div>

      <div v-if="showMore" id="key-more-settings" class="key-more-settings">
        <div class="more-settings-divider" role="separator" :aria-label="t('keys.moreSettings')">
          <span class="more-settings-divider-text">{{ t('keys.moreSettings') }}</span>
        </div>

        <!-- Custom Key Section（仅创建模式） -->
          <div v-if="!isEdit" class="key-setting space-y-3">
            <div class="flex items-center justify-between">
              <label class="input-label mb-0">{{ t('keys.customKeyLabel') }}</label>
              <button
                type="button"
                class="key-setting-switch"
                role="switch"
                :aria-checked="formData.use_custom_key"
                :aria-label="t('keys.customKeyLabel')"
                @click="formData.use_custom_key = !formData.use_custom_key"
                :class="[
                  'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                  formData.use_custom_key ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
                ]"
              >
                <span
                  :class="[
                    'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                    formData.use_custom_key ? 'translate-x-4' : 'translate-x-0'
                  ]"
                />
              </button>
            </div>
            <div v-if="formData.use_custom_key">
              <input
                v-model="formData.custom_key"
                type="text"
                class="input font-mono"
                :placeholder="t('keys.customKeyPlaceholder')"
                :class="{ 'border-red-500 dark:border-red-500': customKeyError }"
              />
              <p v-if="customKeyError" class="mt-1 text-sm text-red-500">{{ customKeyError }}</p>
              <p v-else class="input-hint">{{ t('keys.customKeyHint') }}</p>
            </div>
          </div>

          <!-- IP Restriction Section -->
          <div class="key-setting space-y-3">
            <div class="flex items-center justify-between">
              <label class="input-label mb-0">{{ t('keys.ipRestriction') }}</label>
              <button
                type="button"
                class="key-setting-switch"
                role="switch"
                :aria-checked="formData.enable_ip_restriction"
                :aria-label="t('keys.ipRestriction')"
                @click="formData.enable_ip_restriction = !formData.enable_ip_restriction"
                :class="[
                  'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                  formData.enable_ip_restriction ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
                ]"
              >
                <span
                  :class="[
                    'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                    formData.enable_ip_restriction ? 'translate-x-4' : 'translate-x-0'
                  ]"
                />
              </button>
            </div>

            <div v-if="formData.enable_ip_restriction" class="space-y-4 pt-2">
              <div>
                <label class="input-label">{{ t('keys.ipWhitelist') }}</label>
                <textarea
                  v-model="formData.ip_whitelist"
                  rows="3"
                  class="input font-mono text-sm"
                  :placeholder="t('keys.ipWhitelistPlaceholder')"
                />
                <p class="input-hint">{{ t('keys.ipWhitelistHint') }}</p>
              </div>

              <div>
                <label class="input-label">{{ t('keys.ipBlacklist') }}</label>
                <textarea
                  v-model="formData.ip_blacklist"
                  rows="3"
                  class="input font-mono text-sm"
                  :placeholder="t('keys.ipBlacklistPlaceholder')"
                />
                <p class="input-hint">{{ t('keys.ipBlacklistHint') }}</p>
              </div>
            </div>
          </div>

          <!-- Quota Limit Section -->
          <div class="key-setting space-y-3">
            <label class="input-label">{{ t('keys.quotaLimit') }}</label>
            <div class="relative">
              <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
              <input
                v-model.number="formData.quota"
                type="number"
                step="0.01"
                min="0"
                class="input pl-7"
                :placeholder="t('keys.quotaAmountPlaceholder')"
              />
            </div>
            <p class="input-hint -mt-2">{{ t('keys.quotaAmountHint') }}</p>

            <!-- Quota used display (edit mode) -->
            <div v-if="selectedKey && selectedKey.quota > 0">
              <label class="input-label">{{ t('keys.quotaUsed') }}</label>
              <div class="flex items-center gap-2">
                <div class="key-usage-summary flex-1 rounded-lg px-3 py-2">
                  <span class="font-medium text-gray-900 dark:text-white">
                    ${{ selectedKey.quota_used?.toFixed(4) || '0.0000' }}
                  </span>
                  <span class="mx-2 text-gray-400">/</span>
                  <span class="text-gray-500 dark:text-gray-400">
                    ${{ selectedKey.quota?.toFixed(2) || '0.00' }}
                  </span>
                </div>
                <button
                  type="button"
                  @click="emit('reset-quota')"
                  class="btn btn-secondary text-sm"
                  :title="t('keys.resetQuotaUsed')"
                >
                  {{ t('keys.reset') }}
                </button>
              </div>
            </div>
          </div>

          <!-- Rate Limit Section -->
          <div class="key-setting space-y-3">
            <div class="flex items-center justify-between">
              <label class="input-label mb-0">{{ t('keys.rateLimitSection') }}</label>
              <button
                type="button"
                class="key-setting-switch"
                role="switch"
                :aria-checked="formData.enable_rate_limit"
                :aria-label="t('keys.rateLimitSection')"
                @click="formData.enable_rate_limit = !formData.enable_rate_limit"
                :class="[
                  'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                  formData.enable_rate_limit ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
                ]"
              >
                <span
                  :class="[
                    'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                    formData.enable_rate_limit ? 'translate-x-4' : 'translate-x-0'
                  ]"
                />
              </button>
            </div>

            <div v-if="formData.enable_rate_limit" class="space-y-4 pt-2">
              <p class="input-hint -mt-2">{{ t('keys.rateLimitHint') }}</p>
              <!-- 5-Hour Limit -->
              <div>
                <label class="input-label">{{ t('keys.rateLimit5h') }}</label>
                <div class="relative">
                  <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
                  <input
                    v-model.number="formData.rate_limit_5h"
                    type="number"
                    step="0.01"
                    min="0"
                    class="input pl-7"
                    :placeholder="'0'"
                  />
                </div>
                <!-- Usage info (edit mode) -->
                <div v-if="selectedKey && selectedKey.rate_limit_5h > 0" class="mt-2">
                  <div class="flex items-center gap-2">
                    <div class="key-usage-summary flex-1 rounded-lg px-3 py-2 text-sm">
                      <span :class="[
                        'font-medium',
                        selectedKey.usage_5h >= selectedKey.rate_limit_5h ? 'text-red-500' :
                        selectedKey.usage_5h >= selectedKey.rate_limit_5h * 0.8 ? 'text-yellow-500' :
                        'text-gray-900 dark:text-white'
                      ]">
                        ${{ selectedKey.usage_5h?.toFixed(4) || '0.0000' }}
                      </span>
                      <span class="mx-2 text-gray-400">/</span>
                      <span class="text-gray-500 dark:text-gray-400">
                        ${{ selectedKey.rate_limit_5h?.toFixed(2) || '0.00' }}
                      </span>
                    </div>
                  </div>
                  <div class="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                    <div
                      :class="[
                        'h-full rounded-full transition-all',
                        selectedKey.usage_5h >= selectedKey.rate_limit_5h ? 'bg-red-500' :
                        selectedKey.usage_5h >= selectedKey.rate_limit_5h * 0.8 ? 'bg-yellow-500' :
                        'bg-green-500'
                      ]"
                      :style="{ width: Math.min((selectedKey.usage_5h / selectedKey.rate_limit_5h) * 100, 100) + '%' }"
                    />
                  </div>
                </div>
              </div>

              <!-- Daily Limit -->
              <div>
                <label class="input-label">{{ t('keys.rateLimit1d') }}</label>
                <div class="relative">
                  <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
                  <input
                    v-model.number="formData.rate_limit_1d"
                    type="number"
                    step="0.01"
                    min="0"
                    class="input pl-7"
                    :placeholder="'0'"
                  />
                </div>
                <!-- Usage info (edit mode) -->
                <div v-if="selectedKey && selectedKey.rate_limit_1d > 0" class="mt-2">
                  <div class="flex items-center gap-2">
                    <div class="key-usage-summary flex-1 rounded-lg px-3 py-2 text-sm">
                      <span :class="[
                        'font-medium',
                        selectedKey.usage_1d >= selectedKey.rate_limit_1d ? 'text-red-500' :
                        selectedKey.usage_1d >= selectedKey.rate_limit_1d * 0.8 ? 'text-yellow-500' :
                        'text-gray-900 dark:text-white'
                      ]">
                        ${{ selectedKey.usage_1d?.toFixed(4) || '0.0000' }}
                      </span>
                      <span class="mx-2 text-gray-400">/</span>
                      <span class="text-gray-500 dark:text-gray-400">
                        ${{ selectedKey.rate_limit_1d?.toFixed(2) || '0.00' }}
                      </span>
                    </div>
                  </div>
                  <div class="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                    <div
                      :class="[
                        'h-full rounded-full transition-all',
                        selectedKey.usage_1d >= selectedKey.rate_limit_1d ? 'bg-red-500' :
                        selectedKey.usage_1d >= selectedKey.rate_limit_1d * 0.8 ? 'bg-yellow-500' :
                        'bg-green-500'
                      ]"
                      :style="{ width: Math.min((selectedKey.usage_1d / selectedKey.rate_limit_1d) * 100, 100) + '%' }"
                    />
                  </div>
                </div>
              </div>

              <!-- 7-Day Limit -->
              <div>
                <label class="input-label">{{ t('keys.rateLimit7d') }}</label>
                <div class="relative">
                  <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
                  <input
                    v-model.number="formData.rate_limit_7d"
                    type="number"
                    step="0.01"
                    min="0"
                    class="input pl-7"
                    :placeholder="'0'"
                  />
                </div>
                <!-- Usage info (edit mode) -->
                <div v-if="selectedKey && selectedKey.rate_limit_7d > 0" class="mt-2">
                  <div class="flex items-center gap-2">
                    <div class="key-usage-summary flex-1 rounded-lg px-3 py-2 text-sm">
                      <span :class="[
                        'font-medium',
                        selectedKey.usage_7d >= selectedKey.rate_limit_7d ? 'text-red-500' :
                        selectedKey.usage_7d >= selectedKey.rate_limit_7d * 0.8 ? 'text-yellow-500' :
                        'text-gray-900 dark:text-white'
                      ]">
                        ${{ selectedKey.usage_7d?.toFixed(4) || '0.0000' }}
                      </span>
                      <span class="mx-2 text-gray-400">/</span>
                      <span class="text-gray-500 dark:text-gray-400">
                        ${{ selectedKey.rate_limit_7d?.toFixed(2) || '0.00' }}
                      </span>
                    </div>
                  </div>
                  <div class="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                    <div
                      :class="[
                        'h-full rounded-full transition-all',
                        selectedKey.usage_7d >= selectedKey.rate_limit_7d ? 'bg-red-500' :
                        selectedKey.usage_7d >= selectedKey.rate_limit_7d * 0.8 ? 'bg-yellow-500' :
                        'bg-green-500'
                      ]"
                      :style="{ width: Math.min((selectedKey.usage_7d / selectedKey.rate_limit_7d) * 100, 100) + '%' }"
                    />
                  </div>
                </div>
              </div>

              <!-- Reset Rate Limit button (edit mode) -->
              <div v-if="selectedKey && (selectedKey.rate_limit_5h > 0 || selectedKey.rate_limit_1d > 0 || selectedKey.rate_limit_7d > 0)">
                <button
                  type="button"
                  @click="emit('reset-rate-limit')"
                  class="btn btn-secondary text-sm"
                >
                  {{ t('keys.resetRateLimitUsage') }}
                </button>
              </div>
            </div>
          </div>

          <!-- Expiration Section -->
          <div class="key-setting space-y-3">
            <div class="flex items-center justify-between">
              <label class="input-label mb-0">{{ t('keys.expiration') }}</label>
              <button
                type="button"
                class="key-setting-switch"
                role="switch"
                :aria-checked="formData.enable_expiration"
                :aria-label="t('keys.expiration')"
                @click="formData.enable_expiration = !formData.enable_expiration"
                :class="[
                  'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                  formData.enable_expiration ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
                ]"
              >
                <span
                  :class="[
                    'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                    formData.enable_expiration ? 'translate-x-4' : 'translate-x-0'
                  ]"
                />
              </button>
            </div>

            <div v-if="formData.enable_expiration" class="space-y-4 pt-2">
              <!-- Quick select buttons -->
              <div class="flex flex-wrap gap-2">
                <button
                  v-for="days in ['7', '30', '90']"
                  :key="days"
                  type="button"
                  @click="setExpirationDays(parseInt(days))"
                  class="key-expiration-preset"
                  :aria-pressed="formData.expiration_preset === days"
                >
                  {{ t('keys.expiresInDays', { days }) }}
                </button>
                <button
                  type="button"
                  @click="formData.expiration_preset = 'custom'"
                  class="key-expiration-preset"
                  :aria-pressed="formData.expiration_preset === 'custom'"
                >
                  {{ t('keys.customDate') }}
                </button>
              </div>

              <!-- Date picker -->
              <div>
                <label class="input-label">{{ t('keys.expirationDate') }}</label>
                <input
                  v-model="formData.expiration_date"
                  type="datetime-local"
                  class="input"
                />
                <p class="input-hint">{{ t('keys.expirationDateHint') }}</p>
              </div>

              <!-- Current expiration display (edit mode) -->
              <div v-if="selectedKey?.expires_at" class="text-sm">
                <span class="text-gray-500 dark:text-gray-400">{{ t('keys.currentExpiration') }}: </span>
                <span class="font-medium text-gray-900 dark:text-white">
                  {{ formatDateTime(selectedKey.expires_at) }}
                </span>
              </div>
            </div>
          </div>
        </div>

        <!-- 底部「更多设置」切换按钮：text 样式，填满窗口宽度 -->
        <button
          type="button"
          class="more-settings-toggle"
          :aria-expanded="showMore"
          aria-controls="key-more-settings"
          @click="showMore = !showMore"
        >
          <Icon :name="showMore ? 'chevronUp' : 'chevronDown'" size="sm" />
          <span>{{ t('keys.moreSettings') }}</span>
        </button>
    </form>
    <template #footer>
      <div class="key-form-actions">
        <button @click="emit('close')" type="button" class="btn key-form-cancel">
          {{ t('common.cancel') }}
        </button>
        <button
          form="key-form"
          type="submit"
          :disabled="submitting"
          class="btn btn-primary"
          data-tour="key-form-submit"
        >
          <svg
            v-if="submitting"
            class="-ml-1 mr-2 h-4 w-4 animate-spin"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            ></circle>
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            ></path>
          </svg>
          <Icon v-else :name="isEdit ? 'check' : 'plus'" size="sm" />
          {{ submitting ? t('keys.saving') : (isEdit ? t('common.save') : t('common.create')) }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <!-- 模型价格弹窗：复用模型广场的分组卡片（价格表） -->
  <GroupModelPricingModal
    :visible="showPricingModal"
    :group-id="pricingGroupId"
    @close="showPricingModal = false"
  />
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { ApiKey, SubscriptionType, GroupPlatform } from '@/types'
import { GROUP_PLATFORM_OPTIONS } from '@/constants/platforms'
import type { GroupCategory, GroupRecommendation, GroupTag } from '@/api/groups'
import GroupModelPricingModal from '@/components/keys/GroupModelPricingModal.vue'
import { formatDateTime } from '@/utils/format'

export interface KeyFormData {
  name: string
  group_id: number | null
  status: 'active' | 'inactive'
  use_custom_key: boolean
  custom_key: string
  enable_ip_restriction: boolean
  ip_whitelist: string
  ip_blacklist: string
  enable_quota: boolean
  quota: number | null
  enable_rate_limit: boolean
  rate_limit_5h: number | null
  rate_limit_1d: number | null
  rate_limit_7d: number | null
  enable_expiration: boolean
  expiration_preset: '7' | '30' | '90' | 'custom'
  expiration_date: string
}

export type KeyGroupOption = {
  value: number
  label: string
  description: string | null
  rate: number
  userRate: number | null
  peakRateEnabled: boolean
  peakStart: string
  peakEnd: string
  peakRateMultiplier: number
  subscriptionType: SubscriptionType
  platform: GroupPlatform
  requiredProtocol?: string
  blockedClients?: string[]
  recommendationReason?: string
  recommendationRating?: number
}

interface Props {
  show: boolean
  isEdit: boolean
  title: string
  selectedKey: ApiKey | null
  groupOptions: KeyGroupOption[]
  groupCategories: GroupCategory[]
  groupCategoryAssignments: Record<string, string>
  groupRecommendations: GroupRecommendation[]
  /** 分组上限倍率:分组 ID(字符串)→ 倍率;来自分组分类存储。 */
  groupCaps: Record<string, number>
  groupTags?: Record<string, GroupTag[]>
  submitting: boolean
}

interface Emits {
  (e: 'close'): void
  (e: 'submit'): void
  (e: 'reset-quota'): void
  (e: 'reset-rate-limit'): void
}

const props = defineProps<Props>()
const formData = defineModel<KeyFormData>('formData', { required: true })
const emit = defineEmits<Emits>()

const { t } = useI18n()
const protocolNames: Record<string, string> = { anthropic: 'Anthropic', openai: 'OpenAI', google: 'Google' }

const UNCATEGORIZED_TAB = '__uncategorized__'
const RECOMMENDED_TAB = '__recommended__'

// 创建和编辑共用更多设置折叠区
const showMore = ref(false)

// 名称是否由用户手动输入过；未输入过则选中分组时自动用分组名填充
const nameEditedByUser = ref(false)

// 分组选择内部状态
const groupSearch = ref('')
const activeCategoryTab = ref('')
const groupListRef = ref<HTMLElement | null>(null)
const pendingScrollToSelected = ref(false)

const statusOptions = computed(() => [
  { value: 'active', label: t('common.active') },
  { value: 'inactive', label: t('common.inactive') }
])

// 自定义Key验证
const customKeyError = computed(() => {
  if (!formData.value.use_custom_key || !formData.value.custom_key) {
    return ''
  }
  const key = formData.value.custom_key
  if (key.length < 16) {
    return t('keys.customKeyTooShort')
  }
  // 检查字符：只允许字母、数字、下划线、连字符
  if (!/^[a-zA-Z0-9_-]+$/.test(key)) {
    return t('keys.customKeyInvalidChars')
  }
  return ''
})

const recommendedGroupOptions = computed<KeyGroupOption[]>(() => {
  const optionsByID = new Map(props.groupOptions.map((option) => [option.value, option]))
  const options: KeyGroupOption[] = []
  for (const recommendation of props.groupRecommendations) {
    const option = optionsByID.get(recommendation.group_id)
    if (option) {
      options.push({
        ...option,
        recommendationReason: recommendation.reason,
        recommendationRating: recommendation.rating
      })
    }
  }
  return options
})

const sortGroupOptions = (options: KeyGroupOption[]) =>
  [...options].sort((a, b) => {
    const rateA = a.userRate ?? a.rate
    const rateB = b.userRate ?? b.rate
    return rateA - rateB || a.label.localeCompare(b.label)
  })

const groupTabs = computed(() => {
  const query = groupSearch.value.trim().toLowerCase()
  const filtered = props.groupOptions.filter((option) => {
    if (!query) return true
    return option.label.toLowerCase().includes(query) ||
      Boolean(option.description && option.description.toLowerCase().includes(query))
  })

  const categoryById = new Map(props.groupCategories.map((category) => [category.id, category]))
  const groupsByCategory = new Map<string, KeyGroupOption[]>()
  const uncategorized: KeyGroupOption[] = []

  for (const option of filtered) {
    const categoryId = props.groupCategoryAssignments[String(option.value)]
    if (categoryId && categoryById.has(categoryId)) {
      const list = groupsByCategory.get(categoryId) ?? []
      list.push(option)
      groupsByCategory.set(categoryId, list)
    } else {
      uncategorized.push(option)
    }
  }

  const toTab = (category: GroupCategory) => ({
    id: category.id,
    name: category.name,
    description: category.description || '',
    recommended: false,
    options: sortGroupOptions(groupsByCategory.get(category.id) ?? [])
  })

  const tabs: Array<{
    id: string
    name: string
    description: string
    recommended: boolean
    options: KeyGroupOption[]
  }> = []

  if (recommendedGroupOptions.value.length) {
    tabs.push({
      id: RECOMMENDED_TAB,
      name: t('keys.recommendedGroups'),
      description: '',
      recommended: true,
      options: sortGroupOptions(recommendedGroupOptions.value.filter((option) => {
        if (!query) return true
        return option.label.toLowerCase().includes(query) ||
          Boolean(option.description && option.description.toLowerCase().includes(query)) ||
          Boolean(option.recommendationReason && option.recommendationReason.toLowerCase().includes(query))
      }))
    })
  }

  if (!props.groupCategories.length && !tabs.length) {
    return [{
      id: 'all',
      name: t('keys.allGroups'),
      description: '',
      recommended: false,
      options: sortGroupOptions(filtered)
    }]
  }

  tabs.push(
    ...[...props.groupCategories]
      .sort((a, b) => (b.sort_order ?? 0) - (a.sort_order ?? 0))
      .map(toTab)
  )

  if (uncategorized.length) {
    tabs.push({
      id: UNCATEGORIZED_TAB,
      name: t('keys.uncategorizedGroups'),
      description: '',
      recommended: false,
      options: sortGroupOptions(uncategorized)
    })
  }

  return tabs
})

const defaultCategoryTabId = computed(() => {
  const groupId = formData.value.group_id
  if (groupId != null) {
    const categoryId = props.groupCategoryAssignments[String(groupId)]
    if (categoryId && groupTabs.value.some((tab) => tab.id === categoryId)) {
      return categoryId
    }
    const otherTab = groupTabs.value.find((tab) => tab.id === UNCATEGORIZED_TAB)
    if (otherTab?.options.some((option) => option.value === groupId)) {
      return UNCATEGORIZED_TAB
    }
    const recommendedTab = groupTabs.value.find((tab) => tab.recommended)
    if (recommendedTab?.options.some((option) => option.value === groupId)) {
      return recommendedTab.id
    }
  }
  const recommended = groupTabs.value.find((tab) => tab.recommended)
  return recommended?.id || groupTabs.value[0]?.id || ''
})

const activeCategoryGroups = computed(() => {
  const current = groupTabs.value.find((tab) => tab.id === activeCategoryTab.value)
  return current?.options ?? []
})

const activeBrandSections = computed(() => {
  const groupsByPlatform = new Map<GroupPlatform, KeyGroupOption[]>()
  for (const option of activeCategoryGroups.value) {
    const list = groupsByPlatform.get(option.platform) ?? []
    list.push(option)
    groupsByPlatform.set(option.platform, list)
  }
  return GROUP_PLATFORM_OPTIONS
    .filter((platform) => groupsByPlatform.has(platform.value))
    .map((platform) => ({
      platform: platform.value,
      label: platform.label,
      options: sortGroupOptions(groupsByPlatform.get(platform.value) ?? [])
    }))
})

watch(
  [groupTabs, () => props.show],
  () => {
    if (!props.show) return
    if (!groupTabs.value.some((tab) => tab.id === activeCategoryTab.value)) {
      activeCategoryTab.value = defaultCategoryTabId.value
    }
  }
)

// 切换分类 tab 时,重置右侧分组列表的滚动位置
watch(activeCategoryTab, () => {
  groupListRef.value?.scrollTo({ top: 0 })
})

const scrollSelectedGroupIntoView = () => {
  const groupId = formData.value.group_id
  const container = groupListRef.value
  if (groupId == null || !container) return false
  const el = container.querySelector<HTMLElement>(`[data-group-id="${groupId}"]`)
  if (!el) return false
  const containerRect = container.getBoundingClientRect()
  const elRect = el.getBoundingClientRect()
  const nextTop = container.scrollTop + (elRect.top - containerRect.top) - (container.clientHeight - elRect.height) / 2
  container.scrollTo({ top: Math.max(0, nextTop), behavior: 'smooth' })
  return true
}

watch(
  () => props.show,
  async (isOpen) => {
    if (!isOpen) return
    showMore.value = false
    // 编辑模式：名称属于该密钥，不随分组选择自动填充
    nameEditedByUser.value = props.isEdit
    groupSearch.value = ''
    activeCategoryTab.value = defaultCategoryTabId.value
    pendingScrollToSelected.value = formData.value.group_id != null
    await nextTick()
    requestAnimationFrame(() => {
      if (scrollSelectedGroupIntoView()) {
        pendingScrollToSelected.value = false
      }
    })
  }
)

watch(
  activeBrandSections,
  async () => {
    if (!props.show || !pendingScrollToSelected.value) return
    await nextTick()
    requestAnimationFrame(() => {
      if (scrollSelectedGroupIntoView()) {
        pendingScrollToSelected.value = false
      }
    })
  }
)

const selectGroup = (option: KeyGroupOption) => {
  formData.value.group_id = option.value
  // 名称留空或非用户手动输入时，跟随所选分组名
  if (!formData.value.name.trim() || !nameEditedByUser.value) {
    formData.value.name = option.label
  }
}

// 模型价格弹窗（共用组件 GroupModelPricingModal，plaza 数据进程内缓存）
const showPricingModal = ref(false)
const pricingGroupId = ref<number | null>(null)

const openModelPricing = (groupId: number) => {
  pricingGroupId.value = groupId
  showPricingModal.value = true
}

// 分组上限倍率
const capFor = (groupId: number): number | null => {
  const cap = props.groupCaps[String(groupId)]
  return typeof cap === 'number' && Number.isFinite(cap) ? cap : null
}

const formatCapRate = (value: number): string => String(Math.round(value * 1000) / 1000)

// Helper to format date for datetime-local input
const formatDateTimeLocal = (isoDate: string): string => {
  const date = new Date(isoDate)
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

// Set expiration date based on quick select days
const setExpirationDays = (days: number) => {
  formData.value.expiration_preset = days.toString() as '7' | '30' | '90'
  const expDate = new Date()
  expDate.setDate(expDate.getDate() + days)
  formData.value.expiration_date = formatDateTimeLocal(expDate.toISOString())
}

const recommendationStarEmoji = (rating: number | undefined, index: number) => {
  const value = Number(rating)
  if (Number.isNaN(value) || value < index - 0.5) return '☆'
  return '⭐'
}
</script>

<style scoped>
/* Teleported dialogs need their own palette; keep these rules local to this form. */
:global(.modal-overlay:has(.key-form)),
:global(.select-dropdown-portal:has(.key-status-option)) {
  --key-form-surface: #fff;
  --key-form-field: #f7f8fc;
  --key-form-ink: #20273e;
  --key-form-muted: #69748d;
  --key-form-line: #dce2f0;
  --key-form-haze: #536de512;
}
:global(.dark .modal-overlay:has(.key-form)),
:global(.dark .select-dropdown-portal:has(.key-status-option)) {
  --key-form-surface: #1b1e32;
  --key-form-field: #15192b;
  --key-form-ink: #e9edfa;
  --key-form-muted: #a0abc4;
  --key-form-line: #353d58;
  --key-form-haze: #6f84ed16;
  color-scheme: dark;
}
:global(.modal-overlay:has(.key-form)) { background: #080e2399; backdrop-filter: blur(8px); }
:global(.modal-overlay:has(.key-form) .modal-content) {
  background: var(--key-form-surface);
  color: var(--key-form-ink);
  border: 1px solid var(--key-form-line);
  border-radius: 22px;
  box-shadow: 0 24px 80px #080e2333;
}
:global(.modal-overlay:has(.key-form) .modal-header) { padding: 24px 28px 16px; border: 0; }
:global(.modal-overlay:has(.key-form) .modal-title) { color: var(--key-form-ink); font-size: 20px; font-weight: 600; letter-spacing: -.5px; }
:global(.modal-overlay:has(.key-form) .modal-header button) { color: var(--key-form-muted); border-radius: 8px; }
:global(.modal-overlay:has(.key-form) .modal-header button:hover) { color: var(--key-form-ink); background: var(--key-form-haze); }
:global(.modal-overlay:has(.key-form) .modal-body) { padding: 8px 28px 20px; scrollbar-width: thin; }
:global(.modal-overlay:has(.key-form) .modal-footer) { padding: 18px 28px; border-color: var(--key-form-line); }
.key-form { min-width: 0; color: var(--key-form-ink); }
.key-form .input-label { color: var(--key-form-ink); font-size: 13px; font-weight: 500; }
.key-form .input-hint { color: var(--key-form-muted); font-size: 12px; line-height: 1.7; }
.key-form .input,
.key-form :deep(.select-trigger) {
  min-height: 42px;
  border: 1px solid transparent;
  border-radius: 10px;
  background: var(--key-form-field);
  box-shadow: none;
  color: var(--key-form-ink);
  font-size: 13px;
  --tw-ring-shadow: 0 0 #0000;
}
.key-form .input::placeholder { color: var(--key-form-muted); opacity: .7; }
.key-form .input:hover,
.key-form :deep(.select-trigger:hover) { border-color: var(--key-form-line); }
.key-form .input:focus,
.key-form :deep(.select-trigger:focus-visible),
.key-form :deep(.select-trigger-open) { border-color: #8297f5; box-shadow: 0 0 0 3px #8297f51a; outline: none; }
.key-form .input.border-red-500 { border-color: #ef4444; }
.key-form textarea { resize: vertical; }
.key-group-search .input { padding-left: 36px; }
.group-layout { display: grid; grid-template-columns: 148px minmax(0, 1fr); gap: 18px; height: min(440px, 50vh); min-height: 220px; }
.group-layout-tabs { display: flex; flex-direction: column; gap: 4px; min-width: 0; overflow-y: auto; padding: 2px; scrollbar-width: thin; }
.group-layout-tab { display: flex; flex-shrink: 0; align-items: center; gap: 8px; padding: 10px 12px; border-radius: 9px; color: var(--key-form-muted); font-size: 13px; text-align: left; transition: background-color .15s, color .15s; }
.group-layout-tab:hover { background: var(--key-form-haze); color: var(--key-form-ink); }
.group-layout-tab-active { background: var(--key-form-haze); color: var(--tf-console-active-ink); font-weight: 600; }
.group-layout-tab :deep(.text-gray-400) { color: var(--key-form-muted); }
.group-layout-main { display: flex; min-width: 0; min-height: 0; }
.group-layout-list { min-width: 0; flex: 1; overflow-y: auto; padding: 2px 5px 2px 2px; scrollbar-width: thin; overscroll-behavior: contain; scroll-behavior: smooth; }
.group-row { display: flex; flex-direction: column; gap: 0; min-width: 0; width: 100%; padding: 14px 14px 0; border: 1px solid transparent; border-radius: 14px; background: var(--key-form-field); text-align: left; cursor: pointer; transition: border-color .15s, background-color .15s; }
.group-row:hover { border-color: var(--key-form-line); background: var(--key-form-haze); }
.group-row-selected { border-color: #8297f5; background: var(--key-form-haze); box-shadow: 0 0 0 1px #8297f533; }
.group-row-footer { display: flex; align-items: center; gap: 8px; margin-top: 12px; padding: 10px 0; border-top: 1px solid var(--key-form-line); }
.group-cap-tag { display: inline-flex; flex: none; align-items: center; padding: 2px 7px; border-radius: 6px; background: var(--key-form-haze); color: var(--tf-console-active-ink); font-size: 10px; font-weight: 500; white-space: nowrap; }
.group-pricing-link { display: inline-flex; align-items: center; gap: 4px; border-radius: 6px; padding: 4px 6px; color: var(--key-form-muted); font-size: 12px; font-weight: 500; white-space: nowrap; }
.group-pricing-link:hover { color: var(--tf-console-active-ink); background: var(--key-form-haze); }
.group-custom-tag { display: inline-flex; flex: none; align-items: center; padding: 2px 7px; border-radius: 6px; background: var(--key-form-haze); color: var(--key-form-muted); font-size: 10px; font-weight: 500; white-space: nowrap; cursor: help; }
.recommendation-note { margin-top: 10px; padding: 10px 12px; border-radius: 9px; background: var(--key-form-haze); }
.recommendation-note-label { color: var(--key-form-muted); font-size: 11px; font-weight: 500; }
.recommendation-note-stars { display: flex; flex-shrink: 0; align-items: center; gap: 1px; }
.recommendation-note-star { font-size: 12px; line-height: 1; }
.recommendation-note-text { display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 1; overflow: hidden; margin-top: 6px; color: var(--key-form-ink); font-size: 12px; line-height: 1.7; }
.more-settings-toggle { display: flex; width: 100%; align-items: center; justify-content: center; gap: 6px; padding: 10px; border-radius: 9px; background: var(--key-form-field); color: var(--key-form-muted); font-size: 13px; transition: background-color .15s, color .15s; }
.more-settings-toggle:hover { color: var(--tf-console-active-ink); background: var(--key-form-haze); }
.more-settings-divider { display: flex; align-items: center; gap: 12px; margin: 4px 0; }
.more-settings-divider::before,.more-settings-divider::after { content: ''; height: 1px; flex: 1; background: var(--key-form-line); }
.more-settings-divider-text { color: var(--key-form-muted); font-size: 12px; font-weight: 500; }
.key-more-settings { display: flex; flex-direction: column; gap: 16px; }
.key-setting { padding: 18px; border-radius: 14px; background: var(--key-form-field); }
.key-setting .input { background: var(--key-form-surface); }
.key-setting-switch { outline: none; }
.key-setting-switch[aria-checked=true] { background: #536eff; }
.key-setting-switch[aria-checked=false] { background: var(--key-form-line); }
.key-usage-summary { min-width: 0; background: var(--key-form-haze); }
.key-expiration-preset { padding: 7px 12px; border-radius: 8px; background: var(--key-form-surface); color: var(--key-form-muted); font-size: 12px; transition: background-color .15s, color .15s; }
.key-expiration-preset:hover,.key-expiration-preset[aria-pressed=true] { background: var(--key-form-haze); color: var(--tf-console-active-ink); }
.key-form-actions { display: flex; justify-content: flex-end; gap: 10px; }
.key-form-actions .btn { min-width: 88px; border-radius: 10px; font-size: 13px; }
.key-form-cancel { background: var(--key-form-field); color: var(--key-form-muted); box-shadow: none; }
.key-form-cancel:hover { background: var(--key-form-haze); color: var(--key-form-ink); }
.key-form .btn-secondary { border-color: var(--key-form-line); background: var(--key-form-surface); color: var(--key-form-ink); box-shadow: none; }
.key-form button:focus-visible,.group-row:focus-visible,.key-form-cancel:focus-visible { outline: 2px solid #8297f5; outline-offset: 3px; }
.key-status-option { flex: 1; }
:global(.select-dropdown-portal:has(.key-status-option)) { padding: 6px; border: 1px solid var(--key-form-line); border-radius: 14px; background: var(--key-form-surface); color: var(--key-form-ink); box-shadow: 0 16px 40px -8px #080e2333; }
:global(.select-dropdown-portal:has(.key-status-option) .select-options) { padding: 0; }
:global(.select-dropdown-portal:has(.key-status-option) .select-option) { padding: 10px 12px; border-radius: 9px; color: var(--key-form-muted); font-size: 13px; }
:global(.select-dropdown-portal:has(.key-status-option) .select-option:hover),
:global(.select-dropdown-portal:has(.key-status-option) .select-option-selected),
:global(.select-dropdown-portal:has(.key-status-option) .select-option-focused) { background: var(--key-form-haze); color: var(--tf-console-active-ink); }
@media(max-width:640px) {
  :global(.modal-overlay:has(.key-form) .modal-content) { border-radius: 18px; }
  :global(.modal-overlay:has(.key-form) .modal-header) { padding: 20px 18px 14px; }
  :global(.modal-overlay:has(.key-form) .modal-body) { padding: 6px 18px 18px; }
  :global(.modal-overlay:has(.key-form) .modal-footer) { padding: 14px 18px; }
  .group-layout { grid-template-columns: minmax(0, 1fr); grid-template-rows: auto minmax(0, 1fr); gap: 14px; height: min(460px, 56vh); }
  .group-layout-tabs { flex-direction: row; overflow-x: auto; padding: 2px 2px 6px; }
  .group-layout-tab { padding: 8px 10px; white-space: nowrap; }
  .group-layout-tab>span { flex: none; }
  .group-row { padding: 12px 12px 0; }
  .group-row-footer { flex-wrap: wrap; }
  .key-setting { padding: 14px; }
  .key-form-actions { width: 100%; }
  .key-form-actions .btn { flex: 1; }
}
</style>
