<template>
  <BaseDialog
    :show="Boolean(app)"
    :title="detailTitle"
    width="wide"
    data-testid="app-detail-dialog"
    @close="$emit('close')"
  >
    <div v-if="app" class="space-y-5" data-testid="app-detail-body">
      <div v-if="detailLogo || detailScreenshots.length" class="space-y-3">
        <button
          v-if="detailLogo"
          type="button"
          class="flex w-full cursor-zoom-in items-center justify-center overflow-hidden rounded-xl bg-gray-50 p-6 dark:bg-dark-800"
          data-testid="detail-logo"
          @click="openLightbox(detailGallery, 0)"
        >
          <img
            :src="detailLogo"
            :alt="t('admin.appCatalog.logo')"
            class="max-h-40 max-w-full object-contain"
            referrerpolicy="no-referrer"
          />
        </button>
        <div v-if="detailScreenshots.length" class="flex gap-2 overflow-x-auto">
          <button
            v-for="(shot, index) in detailScreenshots"
            :key="shot"
            type="button"
            class="block h-24 w-40 flex-shrink-0 cursor-zoom-in overflow-hidden rounded-md bg-gray-100 dark:bg-dark-800"
            data-testid="app-screenshot-thumb"
            @click="openLightbox(detailGallery, detailScreenshotOffset + index)"
          >
            <img :src="shot" alt="" class="h-full w-full object-cover" referrerpolicy="no-referrer" />
          </button>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ app.name || t('appCenter.empty') }}
        </h3>
        <span v-if="app.version" class="font-mono text-xs text-gray-500">
          {{ t('admin.appCatalog.version') }} {{ app.version }}
        </span>
      </div>

      <div
        v-if="app.official_url || app.github_url || (!showDownloadAction && app.download_page_url)"
        class="flex flex-wrap gap-3 text-sm"
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

      <div
        v-if="app.description"
        class="markdown-body break-words text-sm"
        data-testid="app-detail-intro"
        v-html="renderCatalogMarkdown(app.description)"
      />

      <section v-if="app.changelog">
        <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.appCatalog.changelog') }}
        </h4>
        <div
          class="markdown-body mt-2 max-h-[min(32vh,20rem)] overflow-auto break-words text-sm"
          data-testid="changelog-dialog-body"
          v-html="renderCatalogMarkdown(app.changelog)"
        />
      </section>

      <p class="text-xs text-gray-500">
        {{ t('admin.appCatalog.lastUpdated') }}:
        {{ app.updated_at ? formatDateTime(app.updated_at) : t('common.notAvailable') }}
      </p>
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary" @click="$emit('close')">
        {{ t('common.close') }}
      </button>
      <a
        v-if="showDownloadAction && downloadUrl"
        :href="downloadUrl"
        target="_blank"
        rel="noopener noreferrer"
        class="btn btn-primary"
        data-testid="download-page-button"
      >
        {{ t('appCenter.openDownloadPage') }}
      </a>
      <button
        v-if="showEdit"
        type="button"
        class="btn btn-primary"
        data-testid="detail-edit-button"
        @click="$emit('edit')"
      >
        {{ t('admin.appCatalog.edit') }}
      </button>
    </template>
  </BaseDialog>

  <Teleport to="body">
    <Transition name="fade">
      <div
        v-if="lightbox"
        class="fixed inset-0 z-[100] flex items-center justify-center bg-black/80 p-4"
        data-testid="screenshot-lightbox"
        role="dialog"
        aria-modal="true"
        @click.self="closeLightbox"
      >
        <button
          type="button"
          class="absolute right-4 top-4 rounded-full bg-black/50 p-2 text-white transition-colors hover:bg-black/70"
          :aria-label="t('common.close')"
          @click="closeLightbox"
        >
          <Icon name="x" size="lg" :stroke-width="2" />
        </button>
        <button
          v-if="lightbox.shots.length > 1"
          type="button"
          class="absolute left-4 top-1/2 -translate-y-1/2 rounded-full bg-black/50 p-2 text-white transition-colors hover:bg-black/70"
          :aria-label="t('admin.appCatalog.screenshotPrev')"
          data-testid="lightbox-prev"
          @click="lightboxPrev"
        >
          <Icon name="chevronLeft" size="lg" :stroke-width="2" />
        </button>
        <img
          :src="lightbox.shots[lightbox.index]"
          :alt="t('admin.appCatalog.screenshotLightboxAlt')"
          class="max-h-[90vh] max-w-[90vw] rounded-lg object-contain shadow-2xl"
          data-testid="lightbox-image"
          referrerpolicy="no-referrer"
        />
        <button
          v-if="lightbox.shots.length > 1"
          type="button"
          class="absolute right-4 top-1/2 -translate-y-1/2 rounded-full bg-black/50 p-2 text-white transition-colors hover:bg-black/70"
          :aria-label="t('admin.appCatalog.screenshotNext')"
          data-testid="lightbox-next"
          @click="lightboxNext"
        >
          <Icon name="chevronRight" size="lg" :stroke-width="2" />
        </button>
        <p
          v-if="lightbox.shots.length > 1"
          class="absolute bottom-4 rounded-full bg-black/50 px-3 py-1 text-xs text-white"
        >
          {{ lightbox.index + 1 }} / {{ lightbox.shots.length }}
        </p>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CatalogApp } from '@/api/appCatalog'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'
import { renderCatalogMarkdown } from './catalogMarkdown'
import '@/styles/announcement-markdown.css'

const props = withDefaults(defineProps<{
  app: CatalogApp | null
  showEdit?: boolean
  showDownloadAction?: boolean
}>(), {
  showEdit: false,
  showDownloadAction: false,
})

defineEmits<{
  close: []
  edit: []
}>()

const { t } = useI18n()
const lightbox = ref<{ shots: string[]; index: number } | null>(null)
const downloadUrl = computed(() => props.app?.download_page_url?.trim() || '')

const detailTitle = computed(() => {
  const current = props.app
  if (!current) return t('admin.appCatalog.details')
  return current.name || t('admin.appCatalog.details')
})

const detailGallery = computed(() => {
  const current = props.app
  if (!current) return []
  const images: string[] = []
  const logo = current.icon_url?.trim()
  if (logo) images.push(logo)
  for (const shot of current.screenshots || []) {
    const url = shot.trim()
    if (url && !images.includes(url)) images.push(url)
  }
  return images
})

const detailLogo = computed(() => props.app?.icon_url?.trim() || '')

const detailScreenshots = computed(() => {
  const current = props.app
  if (!current) return []
  const logo = detailLogo.value
  return (current.screenshots || []).filter((shot) => shot.trim() && shot.trim() !== logo)
})

const detailScreenshotOffset = computed(() => (detailLogo.value ? 1 : 0))

function openLightbox(shots: string[], index: number): void {
  lightbox.value = { shots: [...shots], index }
}

function closeLightbox(): void {
  lightbox.value = null
}

function lightboxPrev(): void {
  const current = lightbox.value
  if (!current?.shots.length) return
  current.index = (current.index + current.shots.length - 1) % current.shots.length
}

function lightboxNext(): void {
  const current = lightbox.value
  if (!current?.shots.length) return
  current.index = (current.index + 1) % current.shots.length
}

function onLightboxKey(event: KeyboardEvent): void {
  if (!lightbox.value) return
  if (event.key === 'Escape') {
    closeLightbox()
    return
  }
  if (event.key === 'ArrowLeft') {
    lightboxPrev()
    return
  }
  if (event.key === 'ArrowRight') {
    lightboxNext()
  }
}

watch(lightbox, (value) => {
  if (value) {
    document.body.style.overflow = 'hidden'
    window.addEventListener('keydown', onLightboxKey)
    return
  }
  document.body.style.overflow = ''
  window.removeEventListener('keydown', onLightboxKey)
})

watch(() => props.app, (value) => {
  if (!value) closeLightbox()
})

onUnmounted(() => {
  window.removeEventListener('keydown', onLightboxKey)
  document.body.style.overflow = ''
})
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
