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
        v-if="loading"
        class="flex min-h-48 items-center justify-center text-sm text-gray-500"
      >
        {{ t('common.loading') }}
      </div>

      <template v-else>
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

        <div
          class="flex flex-col items-center justify-center border border-dashed border-gray-300 px-6 py-12 text-center dark:border-dark-600"
        >
          <Icon name="lightbulb" size="xl" class="text-gray-400" />
          <p class="mt-3 font-medium text-gray-800 dark:text-gray-200">
            {{ t('admin.iqDetection.comingSoon') }}
          </p>
          <p class="mt-1 max-w-lg text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.iqDetection.comingSoonHint') }}
          </p>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type IQDetectAgentHealth } from '@/api/admin'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const health = ref<IQDetectAgentHealth | null>(null)

async function reload() {
  loading.value = true
  try {
    health.value = await adminAPI.iqDetection.getHealth()
  } catch (error) {
    health.value = null
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    loading.value = false
  }
}

onMounted(reload)
</script>
