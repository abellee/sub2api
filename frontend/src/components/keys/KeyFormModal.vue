<template>
  <BaseDialog :show="show" :title="title" width="medium-wide" @close="emit('close')">
    <form id="key-form" class="space-y-5" @submit.prevent="emit('submit')">
      <!-- 第一行：名称 -->
      <div>
        <label class="input-label">{{ t('keys.nameLabel') }}</label>
        <input
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
        <div class="relative shrink-0">
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
                      @click="selectGroup(option)"
                      @keydown.enter.prevent="selectGroup(option)"
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
                      <!-- 底部：上限倍率(左) | 模型价格入口(右) -->
                      <div class="group-row-footer">
                        <span
                          v-if="capFor(option.value) != null"
                          class="group-cap-tag"
                          :title="t('keys.capRateHint')"
                        >
                          {{ t('keys.capRateTag', { rate: formatCapRate(capFor(option.value)!) }) }}
                        </span>
                        <span v-else />
                        <button
                          type="button"
                          class="group-pricing-link ml-auto"
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
        />
      </div>

      <template v-if="showMore">
        <div class="more-settings-divider" role="separator" :aria-label="t('keys.moreSettings')">
          <span class="more-settings-divider-text">{{ t('keys.moreSettings') }}</span>
        </div>

        <!-- Custom Key Section（仅创建模式） -->
          <div v-if="!isEdit" class="space-y-3">
            <div class="flex items-center justify-between">
              <label class="input-label mb-0">{{ t('keys.customKeyLabel') }}</label>
              <button
                type="button"
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
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <label class="input-label mb-0">{{ t('keys.ipRestriction') }}</label>
              <button
                type="button"
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
          <div class="space-y-3">
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
                <div class="flex-1 rounded-lg bg-gray-100 px-3 py-2 dark:bg-dark-700">
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
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <label class="input-label mb-0">{{ t('keys.rateLimitSection') }}</label>
              <button
                type="button"
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
                    <div class="flex-1 rounded-lg bg-gray-100 px-3 py-2 dark:bg-dark-700 text-sm">
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
                    <div class="flex-1 rounded-lg bg-gray-100 px-3 py-2 dark:bg-dark-700 text-sm">
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
                    <div class="flex-1 rounded-lg bg-gray-100 px-3 py-2 dark:bg-dark-700 text-sm">
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
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <label class="input-label mb-0">{{ t('keys.expiration') }}</label>
              <button
                type="button"
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
                  :class="[
                    'rounded-lg px-3 py-1.5 text-sm transition-colors',
                    formData.expiration_preset === days
                      ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400'
                      : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600'
                  ]"
                >
                  {{ t('keys.expiresInDays', { days }) }}
                </button>
                <button
                  type="button"
                  @click="formData.expiration_preset = 'custom'"
                  :class="[
                    'rounded-lg px-3 py-1.5 text-sm transition-colors',
                    formData.expiration_preset === 'custom'
                      ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400'
                      : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600'
                  ]"
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
        </template>

        <!-- 底部「更多设置」切换按钮：text 样式，填满窗口宽度 -->
        <button
          type="button"
          class="more-settings-toggle"
          :aria-expanded="showMore"
          @click="showMore = !showMore"
        >
          <Icon :name="showMore ? 'chevronUp' : 'chevronDown'" size="sm" />
          <span>{{ t('keys.moreSettings') }}</span>
        </button>
    </form>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button @click="emit('close')" type="button" class="btn btn-secondary">
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
import type { GroupCategory, GroupRecommendation } from '@/api/groups'
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

const UNCATEGORIZED_TAB = '__uncategorized__'
const RECOMMENDED_TAB = '__recommended__'

// 更多设置折叠区（仅创建模式展示切换按钮）
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
/* 底部「更多设置」切换按钮：text 样式，填满宽度 */
.more-settings-toggle {
  @apply flex w-full items-center justify-center gap-1.5 rounded-xl py-2 text-sm font-medium text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-gray-200;
}

/* 分组左右布局：左侧分类 tab，右侧分组 list */
.group-layout {
  @apply flex min-h-0 overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-800;
  height: min(460px, 56vh);
}

.group-layout-tabs {
  @apply flex w-36 shrink-0 flex-col gap-0.5 overflow-y-auto border-r border-gray-200 bg-gray-50/60 p-2 dark:border-dark-700 dark:bg-dark-900/40 sm:w-44;
  scrollbar-width: thin;
}

.group-layout-tab {
  @apply flex shrink-0 items-center gap-1.5 rounded-lg px-2.5 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-gray-100;
}

.group-layout-tab-active {
  @apply bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300;
}

.group-layout-tab-recommended {
  @apply text-amber-700 dark:text-amber-300;
}

.group-layout-tab-recommended.group-layout-tab-active {
  @apply bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300;
}

.group-layout-main {
  @apply flex min-w-0 flex-1 flex-col gap-2 p-3;
}

.group-layout-list {
  @apply min-h-0 flex-1 overflow-y-auto pr-1;
  scrollbar-width: thin;
  overscroll-behavior: contain;
  scroll-behavior: smooth;
}

.group-row {
  @apply block min-h-[5.5rem] w-full rounded-lg border border-gray-100 bg-white px-3 py-2.5 text-left transition-colors hover:border-primary-300 hover:bg-primary-50/60 dark:border-dark-700 dark:bg-dark-800 dark:hover:border-primary-500 dark:hover:bg-primary-900/20;
}

.group-row-selected {
  @apply bg-primary-50/80 ring-1 ring-inset ring-primary-500/30 dark:bg-primary-900/30;
}

/* 条目底部：上限倍率(左) | 时间格子(中) | 模型价格入口(右) */
.group-row-footer {
  @apply mt-2 flex items-center gap-2 border-t border-gray-100 pt-1.5 dark:border-dark-700/60;
}

.group-cap-tag {
  @apply inline-flex flex-none items-center whitespace-nowrap rounded-full bg-purple-100 px-2 py-0.5 text-[10px] font-medium text-purple-700 dark:bg-purple-900/30 dark:text-purple-300;
}

.group-pricing-link {
  @apply inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs font-medium text-gray-400 transition-colors hover:text-primary-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 dark:text-dark-400 dark:hover:text-primary-400;
}

.recommendation-note {
  @apply mt-2 w-full rounded-lg bg-primary-50/80 px-2.5 py-2 text-left dark:bg-primary-900/20;
}

.recommendation-note-label {
  @apply text-[11px] font-semibold tracking-wide text-amber-800 dark:text-amber-200;
}

.recommendation-note-stars {
  @apply flex shrink-0 items-center gap-px;
}

.recommendation-note-star {
  @apply text-[13px] leading-none;
}

.recommendation-note-text {
  @apply mt-1.5 w-full overflow-hidden text-left text-xs leading-relaxed text-amber-900/80 dark:text-amber-100/80 line-clamp-1;
}

/* 更多设置折叠区分隔线 */
.more-settings-divider {
  @apply flex items-center gap-3;
}

.more-settings-divider::before,
.more-settings-divider::after {
  content: '';
  @apply h-px flex-1 bg-gray-200 dark:bg-dark-700;
}

.more-settings-divider-text {
  @apply text-xs font-medium uppercase tracking-wide text-gray-400 dark:text-gray-500;
}
</style>
