<template>
  <div ref="containerRef" class="relative" :class="variant === 'menu' ? 'w-full' : ''">
    <button
      type="button"
      :class="variant === 'menu' ? 'dropdown-item w-full justify-between' : 'btn-ghost btn-icon relative'"
      :title="t('pushNotifications.subscription.title')"
      :aria-label="t('pushNotifications.subscription.title')"
      :aria-expanded="open"
      @click="open = !open"
    >
      <span v-if="variant === 'menu'" class="flex min-w-0 items-center gap-2">
        <Icon name="bell" size="sm" />
        <span>{{ t('pushNotifications.subscription.title') }}</span>
      </span>
      <Icon v-else name="bell" size="md" />
      <span
        v-if="variant === 'menu'"
        class="h-2 w-2 flex-shrink-0 rounded-full"
        :class="statusDotClass"
        :title="statusLabel"
        :aria-label="statusLabel"
        role="status"
      >
      </span>
      <span
        v-else
        class="absolute right-1 top-1 h-2 w-2 rounded-full border-2 border-white dark:border-dark-900"
        :class="state === 'enabled' ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-dark-500'"
      ></span>
    </button>

    <transition name="dropdown">
      <div
        v-if="open"
        class="absolute right-0 z-[60] mt-2 w-[min(22rem,calc(100vw-1rem))] rounded-lg border border-gray-200 bg-white p-4 shadow-xl dark:border-dark-600 dark:bg-dark-800"
      >
        <div class="flex items-start gap-3">
          <div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-md bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400">
            <Icon name="bell" size="md" />
          </div>
          <div class="min-w-0 flex-1">
            <p class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('pushNotifications.subscription.title') }}
            </p>
            <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-300">
              {{ description }}
            </p>
          </div>
        </div>

        <div class="mt-4 flex items-center justify-between border-t border-gray-100 pt-3 dark:border-dark-700">
          <span class="inline-flex items-center gap-1.5 text-xs font-medium" :class="statusClass">
            <span class="h-2 w-2 rounded-full" :class="statusDotClass"></span>
            {{ statusLabel }}
          </span>
          <button
            v-if="state !== 'unsupported' && state !== 'denied'"
            type="button"
            class="btn text-sm"
            :class="state === 'enabled' ? 'btn-secondary' : 'btn-primary'"
            :disabled="loading"
            @click="toggleSubscription"
          >
            {{ loading
              ? t('common.processing')
              : state === 'enabled'
                ? t('pushNotifications.subscription.disable')
                : t('pushNotifications.subscription.enable') }}
          </button>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import {
  disablePushNotifications,
  enablePushNotifications,
  getPushPermissionState,
  type PushPermissionState,
} from '@/services/pushNotifications'

const { t } = useI18n()
const appStore = useAppStore()
const { variant = 'icon' } = defineProps<{ variant?: 'icon' | 'menu' }>()
const containerRef = ref<HTMLElement | null>(null)
const open = ref(false)
const loading = ref(true)
const state = ref<PushPermissionState>('disabled')

const statusLabel = computed(() => t(`pushNotifications.subscription.status.${state.value}`))
const description = computed(() => t(`pushNotifications.subscription.description.${state.value}`))
const statusClass = computed(() => state.value === 'enabled'
  ? 'text-emerald-600 dark:text-emerald-400'
  : state.value === 'denied'
    ? 'text-red-600 dark:text-red-400'
    : 'text-gray-500 dark:text-dark-300')
const statusDotClass = computed(() => state.value === 'enabled'
  ? 'bg-emerald-500'
  : 'bg-red-500')

async function refreshState() {
  loading.value = true
  try {
    state.value = await getPushPermissionState(true)
  } catch {
    state.value = await getPushPermissionState(false).catch(() => 'disabled')
  } finally {
    loading.value = false
  }
}

async function toggleSubscription() {
  loading.value = true
  try {
    state.value = state.value === 'enabled'
      ? await disablePushNotifications()
      : await enablePushNotifications()
    if (state.value === 'enabled') appStore.showSuccess(t('pushNotifications.subscription.enabledMessage'))
    if (state.value === 'disabled') appStore.showSuccess(t('pushNotifications.subscription.disabledMessage'))
  } catch (error: any) {
    appStore.showError(error?.message || t('pushNotifications.subscription.failed'))
  } finally {
    loading.value = false
  }
}

function handleDocumentClick(event: MouseEvent) {
  if (containerRef.value && !containerRef.value.contains(event.target as Node)) open.value = false
}

function handleEscape(event: KeyboardEvent) {
  if (event.key === 'Escape') open.value = false
}

onMounted(() => {
  refreshState()
  document.addEventListener('click', handleDocumentClick)
  document.addEventListener('keydown', handleEscape)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleDocumentClick)
  document.removeEventListener('keydown', handleEscape)
})
</script>
