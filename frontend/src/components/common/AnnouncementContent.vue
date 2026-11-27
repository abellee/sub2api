<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import MediaLightbox from '@/components/media/MediaLightbox.vue'
import '@/styles/announcement-markdown.css'

const props = defineProps<{ content: string }>()
const { t } = useI18n()
const body = ref<HTMLElement>()
const images = ref<{ id: string; model: string; download_url: string; content_type: string }[]>([])
const imageIndex = ref<number | null>(null)
const currentImage = computed(() => imageIndex.value === null ? null : images.value[imageIndex.value])
const renderedContent = computed(() => DOMPurify.sanitize(marked.parse(props.content || '', { breaks: true, gfm: true }) as string))

watch([renderedContent, body], () => {
  imageIndex.value = null
  body.value?.querySelectorAll('img').forEach(image => {
    image.tabIndex = 0
    image.setAttribute('role', 'button')
    image.setAttribute('aria-label', `${t('mediaLibrary.preview')}: ${image.alt || t('mediaLibrary.image')}`)
  })
}, { immediate: true, flush: 'post' })

function openImage(event: MouseEvent | KeyboardEvent) {
  if (!(event.target instanceof HTMLImageElement) || !event.target.getAttribute('src')) return
  event.preventDefault()
  event.target.focus()
  const elements = [...(body.value?.querySelectorAll('img[src]') || [])]
  images.value = elements.map((element, index) => {
    const image = element as HTMLImageElement
    return { id: `${index}:${image.src}`, model: image.alt, download_url: image.src, content_type: 'image/' }
  })
  imageIndex.value = elements.indexOf(event.target)
}

function moveImage(direction: number) {
  if (imageIndex.value !== null) imageIndex.value = Math.max(0, Math.min(images.value.length - 1, imageIndex.value + direction))
}
</script>

<template>
  <div
    ref="body"
    class="markdown-body prose prose-sm max-w-none dark:prose-invert"
    @click="openImage"
    @keydown.enter="openImage"
    @keydown.space="openImage"
    v-html="renderedContent"
  ></div>
  <MediaLightbox
    v-if="currentImage"
    :file="currentImage"
    :index="imageIndex ?? 0"
    :total="images.length"
    @close="imageIndex = null"
    @move="moveImage"
  />
</template>
