<template>
  <article
    class="card overflow-hidden border border-gray-200 dark:border-dark-700"
    data-testid="app-card"
  >
    <div class="flex items-start justify-between gap-3 border-b border-gray-100 p-5 dark:border-dark-700">
      <div class="flex min-w-0 items-start gap-3">
        <img
          v-if="app.icon_url"
          :src="app.icon_url"
          alt=""
          class="h-12 w-12 flex-shrink-0 rounded-lg bg-white object-contain"
          referrerpolicy="no-referrer"
        />
        <div
          v-else
          class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-lg bg-gray-100 text-sm font-semibold text-gray-500 dark:bg-dark-700"
        >
          {{ (app.name || '?').slice(0, 1).toUpperCase() }}
        </div>
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h3 class="truncate text-base font-semibold text-gray-900 dark:text-white">
              {{ app.name || t('appCenter.empty') }}
            </h3>
            <span v-if="app.version" class="font-mono text-xs text-gray-500">
              {{ t('admin.appCatalog.version') }} {{ app.version }}
            </span>
          </div>
          <div
            v-if="app.description"
            class="app-catalog-intro markdown-body mt-2 line-clamp-4 text-sm text-gray-600 dark:text-gray-300"
            data-testid="app-intro-preview"
            v-html="renderCatalogMarkdown(app.description)"
          />
          <div
            v-if="app.official_url || app.github_url || (!showDownloadAction && app.download_page_url)"
            class="mt-3 flex flex-wrap gap-3 text-xs"
          >
            <a
              v-if="app.official_url"
              :href="app.official_url"
              target="_blank"
              rel="noopener noreferrer"
              class="text-primary-600 hover:underline dark:text-primary-400"
            >
              {{ t('admin.appCatalog.official') }}
            </a>
            <a
              v-if="app.github_url"
              :href="app.github_url"
              target="_blank"
              rel="noopener noreferrer"
              class="text-primary-600 hover:underline dark:text-primary-400"
            >
              {{ t('admin.appCatalog.github') }}
            </a>
            <a
              v-if="!showDownloadAction && app.download_page_url"
              :href="app.download_page_url"
              target="_blank"
              rel="noopener noreferrer"
              class="text-primary-600 hover:underline dark:text-primary-400"
            >
              {{ t('admin.appCatalog.download') }}
            </a>
          </div>
        </div>
      </div>
      <div class="flex flex-shrink-0 flex-wrap justify-end gap-2">
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          data-testid="view-app-button"
          @click="$emit('details')"
        >
          {{ t('admin.appCatalog.details') }}
        </button>
        <slot name="actions" />
      </div>
    </div>

    <div class="flex flex-wrap items-center justify-between gap-2 px-5 py-3 text-xs text-gray-500">
      <span data-testid="app-updated-at">
        {{ t('admin.appCatalog.lastUpdated') }}:
        {{ app.updated_at ? formatDateTime(app.updated_at) : t('common.notAvailable') }}
      </span>
      <a
        v-if="showDownloadAction && downloadUrl"
        :href="downloadUrl"
        target="_blank"
        rel="noopener noreferrer"
        class="btn btn-primary btn-sm"
        data-testid="download-page-button"
      >
        {{ t('appCenter.openDownloadPage') }}
      </a>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CatalogApp } from '@/api/appCatalog'
import { formatDateTime } from '@/utils/format'
import { renderCatalogMarkdown } from './catalogMarkdown'
import '@/styles/announcement-markdown.css'

const props = withDefaults(defineProps<{
  app: CatalogApp
  showDownloadAction?: boolean
}>(), {
  showDownloadAction: false,
})

defineEmits<{
  details: []
}>()

const { t } = useI18n()
const downloadUrl = computed(() => props.app.download_page_url?.trim() || '')
</script>

<style scoped>
.app-catalog-intro :deep(h1),
.app-catalog-intro :deep(h2),
.app-catalog-intro :deep(h3),
.app-catalog-intro :deep(h4) {
  @apply mb-1 mt-0 border-0 pb-0 text-sm font-semibold;
}

.app-catalog-intro :deep(p),
.app-catalog-intro :deep(ul),
.app-catalog-intro :deep(ol) {
  @apply mb-1;
}

.app-catalog-intro :deep(ul),
.app-catalog-intro :deep(ol) {
  @apply ml-4 space-y-0;
}

.app-catalog-intro :deep(li) {
  @apply pl-0;
}
</style>
