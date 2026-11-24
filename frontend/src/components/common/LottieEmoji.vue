<template>
  <span class="relative inline-block shrink-0" aria-hidden="true">
    <span v-if="!ready" class="absolute inset-0 flex items-center justify-center text-[0.8em]">{{ emoji }}</span>
    <span ref="container" class="block h-full w-full" />
  </span>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { AnimationItem } from 'lottie-web'
import type { AnnouncementEmojiName } from './announcementReactions'

const props = defineProps<{ name: AnnouncementEmojiName; emoji: string; playing?: boolean }>()
const container = ref<HTMLElement>()
const ready = ref(false)
let animation: AnimationItem | undefined
let disposed = false
const reducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

onMounted(async () => {
  try {
    const { default: lottie } = await import('lottie-web/build/player/lottie_light')
    if (disposed || !container.value) return
    animation = lottie.loadAnimation({ container: container.value, renderer: 'svg', loop: false,
      autoplay: false, path: `/emoji-reactions/${props.name}.json`, rendererSettings: { progressiveLoad: true } })
    animation.addEventListener('DOMLoaded', () => {
      if (disposed) return
      ready.value = true
      if (props.playing && !reducedMotion) animation?.goToAndPlay(0, true)
    })
  } catch {
    // Keep the Unicode fallback if animation loading fails.
  }
})
watch(() => props.playing, playing => {
  if (playing && !reducedMotion) animation?.goToAndPlay(0, true)
})
onBeforeUnmount(() => { disposed = true; animation?.destroy() })
</script>
