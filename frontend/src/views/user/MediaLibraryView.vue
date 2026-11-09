<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import { useI18n } from 'vue-i18n'
import StudioLayout from '@/components/user/dashboard/studio/DashboardStudioLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import MediaLibraryCard from '@/components/media/MediaLibraryCard.vue'
import MediaLightbox from '@/components/media/MediaLightbox.vue'
import { mediaLibraryAPI, type MediaFile } from '@/api/mediaLibrary'

const { t } = useI18n()
const files = ref<MediaFile[]>([])
const loading = ref(false)
const loadError = ref(false)
const nextCursor = ref('')
const hasMore = ref(true)
const sentinel = ref<HTMLElement>()
const filter = ref('all')
const now = ref(Date.now())
const selectedID = ref<string | null>(null)
const confirmation = ref<MediaFile[] | null>(null)
const clearing = ref(false)
const deleting = ref(false)
const progressOpen = ref(false)
const total = ref(0)
const processed = ref(0)
const deleted = ref(0)
const failedIDs = ref<string[]>([])
const actionError = ref(false)
const collecting = ref(false)
const clearLoadError = ref(false)
const controller = new AbortController()
let timer: ReturnType<typeof setInterval>
let observer: IntersectionObserver | undefined
const visible = computed(() => files.value.filter(file => filter.value === 'all' || file.content_type.startsWith(`${filter.value}/`)))
const selected = computed(() => visible.value.find(file => file.id === selectedID.value))
const selectedIndex = computed(() => visible.value.findIndex(file => file.id === selectedID.value))
const percent = computed(() => total.value ? Math.round(processed.value / total.value * 100) : deleting.value || clearLoadError.value ? 0 : 100)

async function loadMore() {
  if (loading.value || deleting.value || !hasMore.value) return
  loading.value = true
  loadError.value = false
  try {
    const page = await mediaLibraryAPI.list(nextCursor.value, controller.signal, filter.value)
    const existing = new Set(files.value.map(file => file.id))
    files.value.push(...page.items.filter(file => !existing.has(file.id)))
    nextCursor.value = page.next_cursor
    hasMore.value = !!page.next_cursor
  } catch {
    if (!controller.signal.aborted) loadError.value = true
  } finally { loading.value = false }
}

function refresh(kind = filter.value) {
  if (loading.value || deleting.value) return
  filter.value = kind
  files.value = []
  selectedID.value = null
  nextCursor.value = ''
  hasMore.value = true
  void loadMore()
}

function askDelete(items: MediaFile[], all = false) {
  if (deleting.value || loading.value) return
  clearing.value = all
  actionError.value = false
  confirmation.value = [...items]
}

async function remove(ids: string[], showProgress: boolean, all = false) {
  if (deleting.value || (!ids.length && !all)) return
  confirmation.value = null
  selectedID.value = null
  deleting.value = true
  progressOpen.value = showProgress
  total.value = ids.length
  processed.value = deleted.value = 0
  failedIDs.value = []
  actionError.value = false
  clearLoadError.value = false
  let allFiles: MediaFile[] = []
  try {
    if (all) {
      collecting.value = true
      try {
        let after = ''
        do {
          const page = await mediaLibraryAPI.list(after, controller.signal)
          allFiles.push(...page.items)
          after = page.next_cursor
        } while (after)
        allFiles = [...new Map(allFiles.map(file => [file.id, file])).values()]
        ids = allFiles.map(file => file.id)
        total.value = ids.length
      } catch {
        clearLoadError.value = true
        return
      } finally { collecting.value = false }
    }
    for (let offset = 0; offset < ids.length; offset += 10) {
      const batch = ids.slice(offset, offset + 10)
      try {
        const result = await mediaLibraryAPI.delete(batch)
        const removed = new Set(result.deleted_ids.filter(id => batch.includes(id)))
        files.value = files.value.filter(file => !removed.has(file.id))
        deleted.value += removed.size
        failedIDs.value.push(...batch.filter(id => !removed.has(id)))
      } catch { failedIDs.value.push(...batch) }
      processed.value += batch.length
    }
    actionError.value = failedIDs.value.length > 0
    if (all) {
      const failed = new Set(failedIDs.value)
      files.value = allFiles.filter(file => failed.has(file.id))
      hasMore.value = false
      nextCursor.value = ''
    }
  } finally { deleting.value = false }
}

function confirmDelete() {
  if (confirmation.value) void remove(confirmation.value.map(file => file.id), clearing.value, clearing.value)
}

function movePreview(direction: number) {
  const file = visible.value[selectedIndex.value + direction]
  if (file) selectedID.value = file.id
}

onBeforeRouteLeave(() => !deleting.value)
watch([sentinel, loading, hasMore, loadError, deleting, confirmation, progressOpen, selectedID], () => {
  observer?.disconnect()
  if (sentinel.value && !loading.value && hasMore.value && !loadError.value && !deleting.value && !confirmation.value && !progressOpen.value && !selectedID.value) observer?.observe(sentinel.value)
}, { flush: 'post' })
onMounted(() => {
  observer = new IntersectionObserver(entries => { if (entries.some(entry => entry.isIntersecting)) void loadMore() }, { rootMargin: '400px' })
  refresh()
  timer = setInterval(() => { now.value = Date.now() }, 1000)
})
onUnmounted(() => { controller.abort(); observer?.disconnect(); clearInterval(timer) })
</script>

<template>
  <StudioLayout>
    <section class="media-library">
      <header class="library-heading">
        <p class="library-description">{{ t('mediaLibrary.description') }}</p>
        <div class="library-actions">
          <button class="btn btn-secondary" :disabled="loading || deleting" :aria-label="t('mediaLibrary.refresh')" @click="refresh()"><Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" /></button>
          <button data-clear class="btn library-clear" :disabled="loading || deleting || (loadError && !files.length) || (!files.length && !hasMore && filter === 'all')" @click="askDelete(files, true)"><Icon name="trash" size="sm" />{{ t('mediaLibrary.clearAll') }}</button>
        </div>
      </header>

      <div class="library-toolbar">
        <div class="library-tabs" role="group" :aria-label="t('mediaLibrary.filter')">
          <button v-for="kind in ['all', 'image', 'video']" :key="kind" :aria-pressed="filter === kind" :disabled="loading || deleting" @click="refresh(kind)">{{ t(`mediaLibrary.${kind}`) }}</button>
        </div>
        <span class="library-hint"><Icon name="clock" size="sm" />{{ t('mediaLibrary.retentionHint') }}</span>
      </div>

      <div v-if="loadError && !files.length" class="library-state" role="alert"><Icon name="exclamationCircle" size="xl" /><p>{{ t('mediaLibrary.loadError') }}</p><button class="btn btn-secondary" @click="loadMore">{{ t('common.tryAgain') }}</button></div>
      <div v-else-if="loading && !files.length" class="library-state" role="status"><Icon name="refresh" size="xl" class="animate-spin" /><p>{{ t('common.loading') }}</p></div>
      <template v-else>
        <p v-if="actionError && !progressOpen" class="library-error" role="alert">{{ t('mediaLibrary.deleteError') }}</p>
        <div v-if="visible.length" class="library-masonry">
          <MediaLibraryCard v-for="file in visible" :key="file.id" :file="file" :now="now" :disabled="deleting || loading" @preview="selectedID = file.id" @delete="askDelete([file])" />
        </div>
        <div v-else-if="!hasMore" class="library-state"><Icon name="grid" size="xl" /><h3>{{ t('mediaLibrary.empty') }}</h3><p>{{ t('mediaLibrary.emptyHint') }}</p></div>
      </template>
      <div v-show="hasMore" ref="sentinel" class="library-load-more">
        <span v-if="loading" role="status">{{ t('common.loading') }}</span>
        <div v-else-if="loadError && files.length" role="alert"><p>{{ t('mediaLibrary.loadError') }}</p><button class="btn btn-secondary" @click="loadMore">{{ t('common.tryAgain') }}</button></div>
        <button v-else-if="!loadError" class="btn btn-secondary" :disabled="deleting" @click="loadMore">{{ t('mediaLibrary.loadMore') }}</button>
      </div>
    </section>

    <BaseDialog :show="!!confirmation" :title="t(clearing ? 'mediaLibrary.clearTitle' : 'mediaLibrary.deleteTitle')" width="narrow" @close="confirmation = null">
      <p class="text-sm text-gray-600 dark:text-gray-300">{{ t(clearing ? 'mediaLibrary.clearConfirm' : 'mediaLibrary.deleteConfirm', { count: confirmation?.length || 0 }) }}</p>
      <template #footer><div class="flex justify-end gap-3"><button class="btn btn-secondary" @click="confirmation = null">{{ t('common.cancel') }}</button><button data-confirm class="btn library-clear" @click="confirmDelete">{{ t('mediaLibrary.deleteNow') }}</button></div></template>
    </BaseDialog>

    <BaseDialog :show="progressOpen" :title="t(clearLoadError ? 'mediaLibrary.clearIncomplete' : deleting ? 'mediaLibrary.clearing' : 'mediaLibrary.clearFinished')" width="narrow" :close-on-escape="!deleting" :show-close-button="!deleting" @close="progressOpen = false">
      <div class="library-progress" role="status" aria-live="polite" tabindex="0">
        <div class="flex justify-between"><span>{{ collecting ? t('mediaLibrary.collecting') : t('mediaLibrary.progress', { done: processed, total }) }}</span><strong>{{ percent }}%</strong></div>
        <div class="library-progress-track" role="progressbar" :aria-label="t('mediaLibrary.clearing')" :aria-valuenow="processed" :aria-valuemax="Math.max(total, 1)" aria-valuemin="0"><div :style="{ width: `${percent}%` }" /></div>
        <p>{{ t('mediaLibrary.deletedCount', { count: deleted }) }}</p>
        <p v-if="deleting">{{ t('mediaLibrary.batchHint') }}</p>
        <p v-else-if="clearLoadError" class="library-error">{{ t('mediaLibrary.clearLoadError') }}</p>
        <p v-else-if="failedIDs.length" class="library-error">{{ t('mediaLibrary.failedCount', { count: failedIDs.length }) }}</p>
      </div>
      <template #footer><div class="flex justify-end gap-3"><button v-if="!deleting && (failedIDs.length || clearLoadError)" class="btn library-clear" @click="remove([...failedIDs], true, clearLoadError)">{{ t('common.tryAgain') }}</button><button class="btn btn-secondary" :disabled="deleting" @click="progressOpen = false">{{ t('common.close') }}</button></div></template>
    </BaseDialog>

    <MediaLightbox v-if="selected" :file="selected" :index="selectedIndex" :total="visible.length" @close="selectedID = null" @move="movePreview" />
  </StudioLayout>
</template>

<style scoped>
.media-library { max-width: 1600px; margin: auto; padding: 32px clamp(16px, 3vw, 40px); color: #192338; }
.library-heading { display: flex; align-items: center; justify-content: space-between; gap: 24px; flex-wrap: wrap; }
.library-description { font-size: 14px; color: #64748b; }
.library-actions { display: flex; gap: 10px; margin-left: auto; }
.library-clear { background: #dc3545; color: white; border: 1px solid transparent; }
.library-clear:hover { background: #be2535; }
.library-clear:disabled { opacity: .4; cursor: not-allowed; }
.library-toolbar { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 16px; margin: 32px 0 24px; }
.library-tabs { display: flex; gap: 6px; padding: 4px; border-radius: 12px; background: #e9edf3; }
.library-tabs button { padding: 8px 14px; border-radius: 9px; font-size: 13px; color: #64748b; display: flex; align-items: center; gap: 8px; }
.library-tabs button[aria-pressed=true] { background: white; color: #17233a; box-shadow: 0 1px 4px #182a4212; }
.library-tabs span { font-size: 11px; opacity: .65; }
.library-hint { display: flex; align-items: center; gap: 6px; color: #8490a2; font-size: 12px; }
.library-masonry { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); grid-auto-rows: 8px; gap: 20px; }
.library-load-more { display: flex; justify-content: center; padding: 24px 0; text-align: center; color: #8490a2; font-size: 13px; }
.library-state { min-height: 350px; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 16px; color: #8490a2; text-align: center; }
.library-state h3 { color: #475569; font-size: 18px; }
.library-error { color: #d53648; margin: 12px 0; font-size: 13px; }
.library-progress { display: grid; gap: 16px; font-size: 14px; }
.library-progress-track { height: 8px; border-radius: 10px; background: #e2e8f0; overflow: hidden; }
.library-progress-track > div { height: 100%; background: #dc3545; transition: width .2s; }
:global(.dark .media-library) { color: #e2e8f0; }
:global(.dark .library-tabs) { background: #1b2433; }
:global(.dark .library-tabs button[aria-pressed=true]) { background: #344156; color: #f1f5f9; }
@media (min-width: 1800px) { .library-masonry { grid-template-columns: repeat(5, minmax(0, 1fr)); } }
@media (max-width: 1200px) { .library-masonry { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
@media (max-width: 850px) { .library-masonry { grid-template-columns: repeat(2, minmax(0, 1fr)); column-gap: 14px; } }
@media (max-width: 480px) { .library-masonry { grid-template-columns: minmax(0, 1fr); } .library-hint { font-size: 11px; } }
</style>
