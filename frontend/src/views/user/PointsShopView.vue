<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import StudioLayout from '@/components/user/dashboard/studio/DashboardStudioLayout.vue'
import PointsAssetPreview from '@/components/user/PointsAssetPreview.vue'
import { fetchShopProducts, type ShopProduct } from '@/api/pointsShop'
import { useUserPoints } from '@/composables/useUserPoints'
import pointsIcon from '@/assets/icons/points-image-color.svg'

const { t, locale } = useI18n()
const { balance } = useUserPoints()
const products = ref<ShopProduct[]>([])
const loading = ref(false), failed = ref(false)
let version = 0
async function load() {
  const current = ++version
  loading.value = true; failed.value = false
  try { const list = await fetchShopProducts(); if (current === version) products.value = list }
  catch { if (current === version) failed.value = true }
  finally { if (current === version) loading.value = false }
}
function onVisible() { if (document.visibilityState === 'visible') void load() }
onMounted(() => { void load(); document.addEventListener('visibilitychange', onVisible) })
onBeforeUnmount(() => { version++; document.removeEventListener('visibilitychange', onVisible) })
</script>

<template>
  <StudioLayout>
    <div class="mx-auto w-full max-w-6xl space-y-6 p-5 sm:p-8">
      <div class="flex flex-wrap items-center justify-between gap-3"><p class="flex items-center gap-2 text-sm"><img :src="pointsIcon" alt="" class="h-6 w-6" />{{ t('common.pointsDetails.current') }}：{{ balance }}</p><button class="btn btn-secondary btn-sm" type="button" :disabled="loading" @click="load">{{ t('common.refresh') }}</button></div>
      <p v-if="loading" role="status">{{ t('common.loading') }}</p>
      <div v-else-if="failed" class="card space-y-3 p-6" role="alert"><p>{{ t('common.pointsCatalog.loadError') }}</p><button class="btn btn-secondary" type="button" @click="load">{{ t('common.tryAgain') }}</button></div>
      <p v-else-if="!products.length" class="card p-8 text-center text-sm text-gray-500">{{ t('common.pointsCatalog.shopEmpty') }}</p>
      <div v-else class="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
        <article v-for="product in products" :key="product.id" class="card flex flex-col p-6"><div class="mb-5 h-32"><PointsAssetPreview :asset="product.icon_asset" /></div><h2 class="break-words text-base font-semibold">{{ product.name }}</h2><p class="mt-2 flex-1 whitespace-pre-wrap break-words text-sm text-gray-500 dark:text-gray-400">{{ product.description }}</p><p class="mt-5 flex items-center gap-2 font-medium"><img :src="pointsIcon" alt="" class="h-5 w-5" />{{ BigInt(product.price).toLocaleString(locale) }} {{ t('common.points') }}</p></article>
      </div>
    </div>
  </StudioLayout>
</template>
