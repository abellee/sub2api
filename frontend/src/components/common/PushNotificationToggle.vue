<template>
  <div :class="variant === 'menu' ? 'w-full' : ''">
    <button
      type="button"
      :class="variant === 'menu' ? 'dropdown-item w-full justify-between' : 'btn-ghost btn-icon relative'"
      :title="t('pushNotifications.subscription.title')"
      :aria-label="t('pushNotifications.subscription.title')"
      @click="openPreferences"
    >
      <span v-if="variant === 'menu'" class="flex min-w-0 items-center gap-2">
        <Icon name="bell" size="sm" />
        <span>{{ t('pushNotifications.subscription.title') }}</span>
      </span>
      <Icon v-else name="bell" size="md" />
      <span v-if="variant === 'menu'" class="h-2 w-2 flex-shrink-0 rounded-full" :class="statusDotClass" :title="statusLabel" :aria-label="statusLabel" role="status"></span>
      <span v-else class="absolute right-1 top-1 h-2 w-2 rounded-full border-2 border-white dark:border-dark-900" :class="state === 'enabled' ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-dark-500'"></span>
    </button>

    <BaseDialog
      :show="modalOpen"
      :title="t('pushNotifications.subscription.title')"
      width="wide"
      :close-on-click-outside="true"
      @close="modalOpen = false"
    >
      <div class="space-y-5">
        <div class="flex items-start gap-3 rounded-lg bg-gray-50 p-3 dark:bg-dark-700/60">
          <span class="mt-0.5 h-2.5 w-2.5 flex-shrink-0 rounded-full" :class="statusDotClass"></span>
          <div class="min-w-0 flex-1">
            <p class="text-sm font-medium text-gray-900 dark:text-white">{{ statusLabel }}</p>
            <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-300">{{ t(`pushNotifications.subscription.description.${state}`) }}</p>
          </div>
          <button
            v-if="state !== 'unsupported' && state !== 'denied'"
            type="button"
            class="btn btn-sm"
            :class="state === 'enabled' ? 'btn-secondary' : 'btn-primary'"
            :disabled="loading"
            @click="toggleSubscription"
          >
            {{ loading ? t('common.processing') : state === 'enabled' ? t('pushNotifications.subscription.disable') : t('pushNotifications.subscription.enable') }}
          </button>
        </div>

        <div>
          <div class="mb-2 flex items-center justify-between gap-3">
            <div>
              <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('pushNotifications.subscription.monitorsTitle') }}</h4>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-300">{{ t('pushNotifications.subscription.monitorsHint') }}</p>
            </div>
            <button v-if="monitors.length" type="button" class="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400" @click="toggleAll">
              {{ allSelected ? t('pushNotifications.subscription.clearAll') : t('pushNotifications.subscription.selectAll') }}
            </button>
          </div>

          <div v-if="loadingMonitors" class="rounded-lg border border-gray-200 p-4 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-dark-300">{{ t('common.loading') }}</div>
          <div v-else-if="monitorLoadError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300">{{ monitorLoadError }}</div>
          <div v-else-if="!monitors.length" class="rounded-lg border border-gray-200 p-4 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-dark-300">{{ t('pushNotifications.subscription.noMonitors') }}</div>
          <div v-else class="max-h-64 overflow-y-auto rounded-lg border border-gray-200 dark:border-dark-600">
            <label v-for="monitor in monitors" :key="monitor.id" class="flex cursor-pointer items-center gap-3 border-b border-gray-100 px-3 py-2.5 last:border-b-0 hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-700/50">
              <input v-model="selectedMonitorIDs" type="checkbox" :value="monitor.id" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500">
              <span class="min-w-0 flex-1">
                <span class="block truncate text-sm font-medium text-gray-900 dark:text-white">{{ monitor.name }}</span>
                <span class="mt-0.5 block truncate text-xs text-gray-500 dark:text-dark-300">{{ monitor.provider }}<span v-if="monitor.group_name"> · {{ monitor.group_name }}</span></span>
              </span>
              <span class="flex items-center gap-1.5 text-xs" :class="monitorStatusClass(monitor.primary_status)">
                <span class="h-2 w-2 rounded-full" :class="monitorStatusDot(monitor.primary_status)"></span>
                {{ monitorStatusLabel(monitor.primary_status) }}
              </span>
            </label>
          </div>
        </div>

        <label class="flex cursor-pointer items-start gap-3 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
          <input v-model="notifyOnlyOnChange" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500">
          <span>
            <span class="block text-sm font-semibold text-gray-900 dark:text-white">{{ t('pushNotifications.subscription.onlyOnChange.title') }}</span>
            <span class="mt-1 block text-xs leading-5 text-gray-500 dark:text-dark-300">{{ t('pushNotifications.subscription.onlyOnChange.hint') }}</span>
          </span>
        </label>

        <div>
          <div class="mb-2 flex items-center justify-between gap-3">
            <label class="block text-sm font-semibold text-gray-900 dark:text-white">{{ t('pushNotifications.subscription.muteTitle') }}</label>
            <label class="inline-flex cursor-pointer items-center gap-2 text-xs text-gray-600 dark:text-dark-300">
              <input v-model="muteDaily" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500">
              {{ t('pushNotifications.subscription.mute.daily') }}
            </label>
          </div>
          <p class="mb-2 text-xs text-gray-500 dark:text-dark-300">{{ t('pushNotifications.subscription.mute.hint') }}</p>
          <div class="grid gap-2 sm:grid-cols-2">
            <label class="text-xs text-gray-500 dark:text-dark-300">
              {{ t('pushNotifications.subscription.mute.start') }}
              <input v-model="muteStart" :type="muteDaily ? 'time' : 'datetime-local'" class="input mt-1 w-full" :min="muteDaily ? undefined : minimumMuteDate">
            </label>
            <label class="text-xs text-gray-500 dark:text-dark-300">
              {{ t('pushNotifications.subscription.mute.end') }}
              <input v-model="muteEnd" :type="muteDaily ? 'time' : 'datetime-local'" class="input mt-1 w-full" :min="muteDaily ? undefined : muteStart || minimumMuteDate">
            </label>
          </div>
          <button v-if="muteStart || muteEnd" type="button" class="mt-2 text-xs font-medium text-gray-500 hover:text-gray-700 dark:text-dark-300 dark:hover:text-white" @click="clearMuteRange">
            {{ t('pushNotifications.subscription.mute.clear') }}
          </button>
        </div>
      </div>

      <template #footer>
        <button type="button" class="btn btn-secondary" @click="modalOpen = false">{{ t('common.cancel') }}</button>
        <button type="button" class="btn btn-primary" :disabled="loading || loadingMonitors || saving" @click="savePreferences">
          {{ saving ? t('common.processing') : t('common.save') }}
        </button>
      </template>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { channelMonitorUserAPI, type MonitorStatus, type UserMonitorView } from '@/api/channelMonitor'
import { getPushSubscriptionPreferences, savePushSubscriptionPreferences } from '@/api/pushNotifications'
import { disablePushNotifications, enablePushNotifications, getCurrentPushSubscription, getPushPermissionState, type PushPermissionState } from '@/services/pushNotifications'

const { t } = useI18n()
const appStore = useAppStore()
const { variant = 'icon' } = defineProps<{ variant?: 'icon' | 'menu' }>()
const modalOpen = ref(false)
const loading = ref(false)
const saving = ref(false)
const loadingMonitors = ref(false)
const monitorLoadError = ref('')
const state = ref<PushPermissionState>('disabled')
const monitors = ref<UserMonitorView[]>([])
const selectedMonitorIDs = ref<number[]>([])
const muteStart = ref('')
const muteEnd = ref('')
const muteDaily = ref(false)
const notifyOnlyOnChange = ref(true)

const statusLabel = computed(() => t(`pushNotifications.subscription.status.${state.value}`))
const statusDotClass = computed(() => state.value === 'enabled' ? 'bg-emerald-500' : 'bg-red-500')
const allSelected = computed(() => monitors.value.length > 0 && selectedMonitorIDs.value.length === monitors.value.length)
const minimumMuteDate = computed(() => toLocalDateTime(new Date()))

async function refreshState() {
  try {
    state.value = await getPushPermissionState(true)
  } catch {
    state.value = await getPushPermissionState(false).catch(() => 'disabled')
  }
}

function monitorStatusLabel(status: MonitorStatus | '') {
  if (!status) return t('pushNotifications.subscription.status.unknown')
  return t(`pushNotifications.subscription.monitorStatus.${status}`)
}
function monitorStatusClass(status: MonitorStatus | '') {
  return status === 'operational' ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'
}
function monitorStatusDot(status: MonitorStatus | '') {
  return status === 'operational' ? 'bg-emerald-500' : status === 'failed' || status === 'error' ? 'bg-red-500' : 'bg-amber-500'
}
function toggleAll() {
  selectedMonitorIDs.value = allSelected.value ? [] : monitors.value.map((monitor) => monitor.id)
}
function toLocalDateTime(date: Date) {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}
function clearMuteRange() {
  muteStart.value = ''
  muteEnd.value = ''
  muteDaily.value = false
}
function fromStoredMute(preferences: { muteStart: string | null; muteEnd: string | null; muteDaily: boolean; muteUntil: string | null }) {
  if (preferences.muteStart && preferences.muteEnd) {
    muteStart.value = preferences.muteDaily ? preferences.muteStart : toLocalDateTime(new Date(preferences.muteStart))
    muteEnd.value = preferences.muteDaily ? preferences.muteEnd : toLocalDateTime(new Date(preferences.muteEnd))
    muteDaily.value = preferences.muteDaily
  } else if (preferences.muteUntil && new Date(preferences.muteUntil).getTime() > Date.now()) {
    muteStart.value = toLocalDateTime(new Date())
    muteEnd.value = toLocalDateTime(new Date(preferences.muteUntil))
    muteDaily.value = false
  } else {
    clearMuteRange()
  }
}
async function loadPreferences() {
  const subscription = await getCurrentPushSubscription()
  const endpoint = subscription?.toJSON().endpoint
  if (!endpoint) return
  try {
    const preferences = await getPushSubscriptionPreferences(endpoint)
    selectedMonitorIDs.value = preferences.monitorIDs === null ? monitors.value.map((monitor) => monitor.id) : preferences.monitorIDs.filter((id) => monitors.value.some((monitor) => monitor.id === id))
    notifyOnlyOnChange.value = preferences.notifyOnlyOnChange !== false
    fromStoredMute(preferences)
  } catch (error: any) {
    // A newly registered subscription has no preferences yet. The notifier
    // reports that state as 404, which should keep the default selections
    // rather than make the monitor list appear to have failed.
    const status = error?.status ?? error?.response?.status
    const message = String(error?.message || '').toLowerCase()
    const isMissingPreferences = status === 404 || message.includes('subscription not found')
    if (!isMissingPreferences) throw error
    selectedMonitorIDs.value = monitors.value.map((monitor) => monitor.id)
    notifyOnlyOnChange.value = true
  }
}
async function openPreferences() {
  modalOpen.value = true
  loading.value = true
  loadingMonitors.value = true
  monitorLoadError.value = ''
  try {
    state.value = await getPushPermissionState(true)
  } catch {
    state.value = await getPushPermissionState(false).catch(() => 'disabled')
  }
  try {
    const response = await channelMonitorUserAPI.list()
    monitors.value = response.items || []
    selectedMonitorIDs.value = monitors.value.map((monitor) => monitor.id)
    if (state.value === 'enabled') await loadPreferences()
  } catch (error: any) {
    monitorLoadError.value = error?.message || t('pushNotifications.subscription.monitorsLoadFailed')
  } finally {
    loadingMonitors.value = false
    loading.value = false
  }
}
async function toggleSubscription() {
  loading.value = true
  try {
    state.value = state.value === 'enabled' ? await disablePushNotifications() : await enablePushNotifications()
    if (state.value === 'enabled') {
      selectedMonitorIDs.value = monitors.value.map((monitor) => monitor.id)
      appStore.showSuccess(t('pushNotifications.subscription.enabledMessage'))
    } else if (state.value === 'disabled') appStore.showSuccess(t('pushNotifications.subscription.disabledMessage'))
  } catch (error: any) {
    appStore.showError(error?.message || t('pushNotifications.subscription.failed'))
  } finally {
    loading.value = false
  }
}
function muteRangeValue() {
  if (!muteStart.value && !muteEnd.value) return { muteStart: null, muteEnd: null, muteDaily: false, muteTimezone: null, muteUntil: null }
  if (!muteStart.value || !muteEnd.value) throw new Error(t('pushNotifications.subscription.mute.invalidRange'))
  if (muteDaily.value) {
    if (muteStart.value === muteEnd.value) throw new Error(t('pushNotifications.subscription.mute.invalidRange'))
    return {
      muteStart: muteStart.value,
      muteEnd: muteEnd.value,
      muteDaily: true,
      muteTimezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
      muteUntil: null,
    }
  }
  const start = new Date(muteStart.value)
  const end = new Date(muteEnd.value)
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime()) || end <= start) throw new Error(t('pushNotifications.subscription.mute.invalidRange'))
  return { muteStart: start.toISOString(), muteEnd: end.toISOString(), muteDaily: false, muteTimezone: null, muteUntil: null }
}
async function savePreferences() {
  saving.value = true
  try {
    if (state.value !== 'enabled') throw new Error(t('pushNotifications.subscription.enableRequired'))
    const subscription = await getCurrentPushSubscription()
    const endpoint = subscription?.toJSON().endpoint
    if (!endpoint) throw new Error(t('pushNotifications.subscription.failed'))
    await savePushSubscriptionPreferences(endpoint, { monitorIDs: [...new Set(selectedMonitorIDs.value)], notifyOnlyOnChange: notifyOnlyOnChange.value, ...muteRangeValue() })
    appStore.showSuccess(t('pushNotifications.subscription.saved'))
    modalOpen.value = false
  } catch (error: any) {
    appStore.showError(error?.message || t('pushNotifications.subscription.failed'))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void refreshState()
})
</script>
