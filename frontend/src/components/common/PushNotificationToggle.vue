<template>
  <div class="dropdown-item w-full justify-between" data-tour="push-notification-toggle" @click="onRowClick">
    <span class="flex min-w-0 items-center gap-2">
      <Icon name="bell" size="sm" class="flex-shrink-0" />
      <span class="min-w-0">
        <span class="block truncate">{{ t('pushNotifications.subscription.title') }}</span>
        <span class="block truncate text-xs text-gray-500 dark:text-dark-400">{{ statusLabel }}</span>
      </span>
    </span>
    <Toggle
      v-if="ready && canToggle"
      class="flex-shrink-0"
      :class="loading ? 'pointer-events-none opacity-60' : ''"
      :model-value="state === 'enabled'"
      @click.stop
      @update:model-value="onToggle"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import {
  disablePushNotifications,
  enablePushNotifications,
  getPushPermissionState,
  type PushPermissionState,
} from '@/services/pushNotifications'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(true)
const ready = ref(false)
const state = ref<PushPermissionState>('disabled')

const statusLabel = computed(() => t(`pushNotifications.subscription.status.${state.value}`))
const canToggle = computed(() => state.value === 'enabled' || state.value === 'disabled')

async function refreshState() {
  loading.value = true
  try {
    state.value = await getPushPermissionState(true)
  } catch {
    state.value = await getPushPermissionState(false).catch(() => 'disabled')
  } finally {
    loading.value = false
    ready.value = true
  }
}

async function onToggle(enabled: boolean) {
  if (loading.value || !canToggle.value) return
  loading.value = true
  try {
    state.value = enabled ? await enablePushNotifications() : await disablePushNotifications()
    if (state.value === 'enabled') appStore.showSuccess(t('pushNotifications.subscription.enabledMessage'))
    else if (state.value === 'disabled') appStore.showSuccess(t('pushNotifications.subscription.disabledMessage'))
    else if (state.value === 'denied') appStore.showError(t('pushNotifications.subscription.description.denied'))
  } catch (error: any) {
    appStore.showError(error?.message || t('pushNotifications.subscription.failed'))
    state.value = await getPushPermissionState(false).catch(() => state.value)
  } finally {
    loading.value = false
  }
}

function onRowClick() {
  if (!ready.value || !canToggle.value || loading.value) return
  void onToggle(state.value !== 'enabled')
}

onMounted(() => {
  void refreshState()
})
</script>
