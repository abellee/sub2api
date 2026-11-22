<template>
  <section
    class="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6"
  >
    <div v-if="loading" class="text-sm text-gray-400">
      <span class="animate-pulse">{{ t('common.loading') }}</span>
    </div>

    <template v-else>
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="tabs inline-flex w-full max-w-xl flex-wrap" role="radiogroup" :aria-label="t('admin.channelMonitor.visibility.title')">
          <button
            type="button"
            role="radio"
            class="tab flex-1 sm:flex-none"
            :class="visibility === 'all' ? 'tab-active' : ''"
            :aria-checked="visibility === 'all'"
            @click="visibility = 'all'"
          >
            {{ t('admin.channelMonitor.visibility.all') }}
          </button>
          <button
            type="button"
            role="radio"
            class="tab flex-1 sm:flex-none"
            :class="visibility === 'selected' ? 'tab-active' : ''"
            :aria-checked="visibility === 'selected'"
            @click="visibility = 'selected'"
          >
            {{ t('admin.channelMonitor.visibility.selected') }}
          </button>
        </div>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="saving || !dirty"
          @click="save"
        >
          <Icon name="check" size="sm" />
          {{ t('admin.channelMonitor.visibility.save') }}
        </button>
      </div>
      <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
        {{
          visibility === 'selected'
            ? t('admin.channelMonitor.visibility.selectedHint')
            : t('admin.channelMonitor.visibility.allHint')
        }}
      </p>

      <div v-if="visibility === 'selected'" class="mt-4 space-y-2">
        <ChannelMonitorUserSelector v-model="visibleUserIds" />
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{
            visibleUserIds.length > 0
              ? t('admin.channelMonitor.visibility.selectedCount', { count: visibleUserIds.length })
              : t('admin.channelMonitor.visibility.emptySelected')
          }}
        </p>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import ChannelMonitorUserSelector from './ChannelMonitorUserSelector.vue'

type VisibilityMode = 'all' | 'selected'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(true)
const saving = ref(false)
const visibility = ref<VisibilityMode>('selected')
const visibleUserIds = ref<number[]>([])
const original = ref('')

const snapshot = computed(() =>
  JSON.stringify({
    visibility: visibility.value,
    ids: [...visibleUserIds.value].sort((a, b) => a - b),
  }),
)
const dirty = computed(() => snapshot.value !== original.value)

function normalizeVisibility(value: string | undefined): VisibilityMode {
  return value === 'all' ? 'all' : 'selected'
}

function normalizeIds(ids: number[] | undefined): number[] {
  return Array.from(new Set((ids || []).filter((id) => Number.isInteger(id) && id > 0))).sort(
    (a, b) => a - b,
  )
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const settings = await adminAPI.settings.getSettings()
    visibility.value = normalizeVisibility(settings.channel_monitor_visibility)
    visibleUserIds.value = normalizeIds(settings.channel_monitor_visible_user_ids)
    original.value = snapshot.value
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channelMonitor.visibility.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function save(): Promise<void> {
  if (saving.value || !dirty.value) return
  saving.value = true
  const intendedVisibility = visibility.value
  const intendedIds = normalizeIds(visibleUserIds.value)
  try {
    await adminAPI.settings.updateSettings({
      channel_monitor_visibility: intendedVisibility,
      channel_monitor_visible_user_ids: intendedIds,
    })
    const saved = await adminAPI.settings.getSettings()
    visibility.value = normalizeVisibility(saved.channel_monitor_visibility)
    visibleUserIds.value = normalizeIds(saved.channel_monitor_visible_user_ids)
    original.value = snapshot.value
    await appStore.fetchPublicSettings(true)
    const persisted =
      visibility.value === intendedVisibility &&
      JSON.stringify(visibleUserIds.value) === JSON.stringify(intendedIds)
    if (!persisted) {
      appStore.showError(t('admin.channelMonitor.visibility.saveNotPersisted'))
      return
    }
    appStore.showSuccess(t('admin.channelMonitor.visibility.saveSuccess'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channelMonitor.visibility.saveFailed')))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void load()
})
</script>
