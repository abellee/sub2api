<template>
  <AppLayout>
    <div class="space-y-6" data-testid="app-center-page">
      <section class="border-b border-gray-200 pb-5 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('appCenter.title') }}
        </h2>
        <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">
          {{ t('appCenter.description') }}
        </p>
      </section>

      <div
        v-if="loading"
        class="flex min-h-48 items-center justify-center text-sm text-gray-500"
      >
        {{ t('common.loading') }}
      </div>

      <div
        v-else-if="apps.length === 0"
        class="flex min-h-56 flex-col items-center justify-center border border-dashed border-gray-300 px-6 text-center dark:border-dark-600"
        data-testid="app-center-empty"
      >
        <Icon name="cube" size="xl" class="text-gray-400" />
        <p class="mt-3 font-medium text-gray-800 dark:text-gray-200">
          {{ t('appCenter.empty') }}
        </p>
        <p class="mt-1 max-w-lg text-sm text-gray-500 dark:text-gray-400">
          {{ t('appCenter.emptyHint') }}
        </p>
      </div>

      <div v-else class="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <CatalogAppCard
          v-for="app in apps"
          :key="app.id"
          :app="app"
          show-download-action
          @details="openDetail(app)"
        />
      </div>
    </div>

    <CatalogAppDetailsDialog :app="detailApp" show-download-action @close="closeDetail" />
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { listCatalogApps, type CatalogApp } from '@/api/appCatalog'
import AppLayout from '@/components/layout/AppLayout.vue'
import CatalogAppCard from '@/components/app-center/CatalogAppCard.vue'
import CatalogAppDetailsDialog from '@/components/app-center/CatalogAppDetailsDialog.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const loading = ref(true)
const apps = ref<CatalogApp[]>([])
const detailApp = ref<CatalogApp | null>(null)

function openDetail(app: CatalogApp): void {
  detailApp.value = app
}

function closeDetail(): void {
  detailApp.value = null
}

async function reload(): Promise<void> {
  loading.value = true
  try {
    const list = await listCatalogApps()
    apps.value = list.items || []
  } catch {
    apps.value = []
  } finally {
    loading.value = false
  }
}

onMounted(reload)
</script>
