<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { MediaFile } from '@/api/mediaLibrary'

const props = defineProps<{ file: MediaFile; now: number; disabled: boolean }>()
defineEmits<{ preview: []; delete: [] }>()
const { t } = useI18n()
const root = ref<HTMLElement>()
const near = ref(false)
const failed = ref(false)
let observer: IntersectionObserver | undefined
let resizeObserver: ResizeObserver | undefined
const rows = ref(10)
const video = computed(() => props.file.content_type.startsWith('video/'))
const remaining = computed(() => props.file.expires_at ? Math.max(0, Math.ceil((Date.parse(props.file.expires_at) - props.now) / 1000)) : null)
const countdown = computed(() => {
  const seconds = remaining.value
  if (seconds === null) return t('mediaLibrary.noExpiry')
  if (!seconds) return t('mediaLibrary.expired')
  const days = Math.floor(seconds / 86400)
  const time = [Math.floor(seconds % 86400 / 3600), Math.floor(seconds % 3600 / 60), seconds % 60].map(v => String(v).padStart(2, '0')).join(':')
  return t('mediaLibrary.remaining', { time: days ? `${t('mediaLibrary.days', { count: days })} ${time}` : time })
})
const previewURL = computed(() => video.value ? `${props.file.download_url}#t=0.1` : props.file.download_url)
onMounted(() => {
  observer = new IntersectionObserver(entries => { if (entries.some(entry => entry.isIntersecting)) { near.value = true; observer?.disconnect() } }, { rootMargin: '200px' })
  if (root.value) observer.observe(root.value)
  resizeObserver = new ResizeObserver(entries => { rows.value = Math.max(1, Math.ceil((entries[0].contentRect.height + 22) / 28)) })
  if (root.value) resizeObserver.observe(root.value)
})
onUnmounted(() => { observer?.disconnect(); resizeObserver?.disconnect() })
</script>

<template>
  <article ref="root" class="media-card" :data-media-id="file.id" :style="{ gridRowEnd: `span ${rows}` }">
    <button class="card-preview" :disabled="disabled" :aria-label="t('mediaLibrary.openPreview', { name: file.model || t(video ? 'mediaLibrary.video' : 'mediaLibrary.image') })" @click="$emit('preview')">
      <div v-if="failed" class="card-fallback"><Icon name="exclamationCircle" size="lg" /><span>{{ t('mediaLibrary.previewError') }}</span></div>
      <video v-else-if="video" :src="near ? previewURL : undefined" muted playsinline preload="metadata" @error="failed = true" />
      <img v-else :src="file.download_url" :alt="file.model || t('mediaLibrary.image')" loading="lazy" decoding="async" referrerpolicy="no-referrer" @error="failed = true" />
      <span class="card-kind"><Icon :name="video ? 'play' : 'grid'" size="xs" />{{ t(video ? 'mediaLibrary.video' : 'mediaLibrary.image') }}</span>
      <span v-if="video && !failed" class="card-play"><Icon name="play" size="lg" /></span>
    </button>
    <div class="card-info">
      <div class="card-model" :title="file.model">{{ file.model || t('mediaLibrary.generatedFile') }}</div>
      <div class="card-bottom"><span class="card-countdown" :class="{ urgent: remaining !== null && remaining < 3600 }" :title="file.expires_at ? new Date(file.expires_at).toLocaleString() : ''"><Icon name="clock" size="xs" />{{ countdown }}</span><button class="card-delete" :disabled="disabled" :aria-label="t('mediaLibrary.deleteTitle')" @click="$emit('delete')"><Icon name="trash" size="sm" /></button></div>
    </div>
  </article>
</template>

<style scoped>
.media-card { align-self: start; background: #fff; border: 1px solid #e5eaf1; border-radius: 16px; overflow: hidden; box-shadow: 0 3px 12px #182a4205; }
.card-preview { display: block; position: relative; width: 100%; min-height: 140px; background: #e8ecf2; overflow: hidden; }
.card-preview img, .card-preview video { width: 100%; height: auto; min-height: 140px; object-fit: cover; transition: transform .25s; }
.card-preview video { aspect-ratio: 4/3; pointer-events: none; }
.card-preview:hover img { transform: scale(1.025); }
.card-kind { position: absolute; top: 12px; left: 12px; display: flex; align-items: center; gap: 5px; color: white; background: #111b2e88; backdrop-filter: blur(8px); font-size: 10px; padding: 4px 8px; border-radius: 6px; }
.card-play { position: absolute; inset: 0; margin: auto; display: grid; place-items: center; width: 48px; height: 48px; border: 1px solid #ffffff80; background: #111b2e66; color: #fff; border-radius: 50%; }
.card-info { padding: 14px 14px 10px; }
.card-model { white-space: nowrap; text-overflow: ellipsis; overflow: hidden; font-size: 13px; font-weight: 600; color: #334155; }
.card-bottom { display: flex; align-items: center; justify-content: space-between; gap: 4px; margin-top: 6px; }
.card-countdown { display: flex; align-items: center; gap: 5px; font-size: 11px; font-variant-numeric: tabular-nums; color: #8490a2; }
.card-countdown.urgent { color: #d78235; }
.card-delete { padding: 8px; color: #8c98aa; border-radius: 8px; }
.card-delete:hover { color: #dc3545; background: #fff0f1; }
.card-delete:disabled { opacity: .4; cursor: not-allowed; }
.card-fallback { display: grid; place-content: center; justify-items: center; gap: 10px; min-height: 180px; color: #8290a3; font-size: 12px; }
:global(.dark .media-card) { background: #1b2433; border-color: #334155; }
:global(.dark .card-model) { color: #e2e8f0; }
:global(.dark .card-preview) { background: #111827; }
</style>
