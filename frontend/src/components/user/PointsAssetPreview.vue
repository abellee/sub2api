<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import type { AnimationItem } from 'lottie-web'
import { useI18n } from 'vue-i18n'
import { fetchPointsAnimation, pointsAssetURL } from '@/api/pointsShop'

const props = defineProps<{ asset: string; animation?: boolean }>()
const { t } = useI18n()
const container = ref<HTMLElement | null>(null)
const isLottie = ref(false)
const failed = ref(false)
let player: AnimationItem | undefined
let version = 0
watch(() => [props.asset, props.animation], async () => {
  const current = ++version
  player?.destroy(); player = undefined
  isLottie.value = false; failed.value = false
  if (!props.asset || !props.animation) return
  try {
    const data = await fetchPointsAnimation(props.asset)
    if (current !== version || !data || typeof data !== 'object') return
    isLottie.value = true
    await nextTick()
    const { default: lottie } = await import('lottie-web/build/player/lottie_light')
    if (current !== version || !container.value) return
    player = lottie.loadAnimation({ container: container.value, renderer: 'svg', loop: true, autoplay: !window.matchMedia('(prefers-reduced-motion: reduce)').matches, animationData: data })
    player.addEventListener('data_failed', () => { if (current === version) failed.value = true })
  } catch { if (current === version) failed.value = true }
}, { immediate: true })
onBeforeUnmount(() => { version++; player?.destroy() })
</script>

<template>
  <div class="points-asset-preview">
    <span v-if="failed" role="status" class="text-xs text-gray-500">{{ t('common.pointsCatalog.previewError') }}</span>
    <div v-else-if="isLottie" ref="container" class="points-animation" :aria-label="t('common.pointsCatalog.animation')" role="img"></div>
    <img v-else-if="asset" :src="pointsAssetURL(asset)" alt="" @error="failed = true" />
  </div>
</template>

<style scoped>
.points-asset-preview{display:flex;align-items:center;justify-content:center;width:100%;height:100%;min-width:0;overflow:hidden}.points-asset-preview img,.points-animation{width:100%;height:100%;object-fit:contain}.points-animation :deep(svg){max-width:100%;max-height:100%}
</style>
