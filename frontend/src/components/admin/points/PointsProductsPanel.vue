<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { fetchManagedItems, fetchShopProducts, saveShopProduct, setShopProductStatus, type ManagedItem, type ShopProduct } from '@/api/pointsShop'
import Select from '@/components/common/Select.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PointsAssetUpload from './PointsAssetUpload.vue'
import PointsAssetPreview from '@/components/user/PointsAssetPreview.vue'
import pointsIcon from '@/assets/icons/points-image-color.svg'
import '@/styles/key-dialog.css'

const { t, locale } = useI18n()
const app = useAppStore()
const products = ref<ShopProduct[]>([]), items = ref<ManagedItem[]>([])
const loading = ref(false), failed = ref(false), saving = ref(false), uploading = ref(false), open = ref(false)
const changing = ref<number | null>(null)
const revision = ref(0)
const blank = (): ShopProduct => ({ id: 0, kind: 'item', item_id: null, icon_asset: '', name: '', description: '', price: '', status: 'draft', sort_order: 0 })
const form = reactive(blank())
const kindOptions = computed(() => [{ value: 'item', label: t('common.pointsCatalog.itemProduct') }, { value: 'custom', label: t('common.pointsCatalog.customProduct') }])
const itemOptions = computed(() => items.value.filter(item => item.cover_asset && item.active_asset && item.expired_asset && item.animation_asset).map(item => ({ value: item.id, label: item.name })))
const statusOptions = computed(() => [{ value: 'draft', label: t('common.pointsCatalog.unpublished') }, { value: 'published', label: t('common.pointsCatalog.published') }])
const selectedItem = computed(() => items.value.find(item => item.id === form.item_id))
const valid = computed(() => /^\d+$/.test(form.price) && BigInt(form.price) > 0n && BigInt(form.price) <= 1000000000000n && Number.isInteger(form.sort_order) && Math.abs(form.sort_order) <= 1000000 && (form.kind === 'item' ? !!selectedItem.value : !!form.icon_asset && !!form.name.trim() && !!form.description.trim()))
const format = (value: string) => BigInt(value).toLocaleString(locale.value)
watch(() => form.kind, () => { form.item_id = null; form.icon_asset = ''; form.name = ''; form.description = '' })
async function load() {
  loading.value = true; failed.value = false
  try { [products.value, items.value] = await Promise.all([fetchShopProducts(true), fetchManagedItems()]) }
  catch { failed.value = true } finally { loading.value = false }
}
async function edit(product?: ShopProduct) {
  // Finish the type change before assigning the fields of an existing product.
  const value = product ?? blank()
  form.kind = value.kind
  await nextTick()
  Object.assign(form, value); uploading.value = false; revision.value++; open.value = true
}
async function save() {
  if (!valid.value || saving.value || uploading.value) return
  saving.value = true
  try {
    const payload = { ...form, price: BigInt(form.price).toString(), name: form.name.trim(), description: form.description.trim() }
    if (payload.kind === 'item') { payload.icon_asset = ''; payload.name = ''; payload.description = '' } else payload.item_id = null
    await saveShopProduct(payload); open.value = false; app.showSuccess(t('common.saved')); await load()
  } catch (err) { app.showError(err instanceof Error ? err.message : t('common.error')) }
  finally { saving.value = false }
}
async function toggle(product: ShopProduct) {
  if (changing.value !== null) return
  changing.value = product.id
  try { await setShopProductStatus(product.id, product.status === 'published' ? 'draft' : 'published'); await load(); app.showSuccess(t('common.saved')) }
  catch (err) { app.showError(err instanceof Error ? err.message : t('common.error')) }
  finally { changing.value = null }
}
onMounted(load)
</script>

<template>
  <section class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-3"><div><h2 class="text-base font-semibold">{{ t('common.pointsCatalog.products') }}</h2><p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('common.pointsCatalog.productHint') }}</p></div><button type="button" class="btn btn-primary" :disabled="loading || failed" @click="edit()">{{ t('common.pointsCatalog.addProduct') }}</button></div>
    <div v-if="loading" role="status">{{ t('common.loading') }}</div>
    <div v-else-if="failed" class="card space-y-3 p-5" role="alert"><p>{{ t('common.pointsCatalog.loadError') }}</p><button type="button" class="btn btn-secondary" @click="load">{{ t('common.tryAgain') }}</button></div>
    <p v-else-if="!products.length" class="card p-6 text-sm text-gray-500">{{ t('common.pointsCatalog.noProducts') }}</p>
    <div v-else class="grid gap-4 sm:grid-cols-2">
      <article v-for="product in products" :key="product.id" class="card flex gap-4 p-5"><div class="h-16 w-16 flex-shrink-0"><PointsAssetPreview :asset="product.icon_asset" /></div><div class="min-w-0 flex-1"><div class="flex flex-wrap items-center gap-2"><h3 class="break-words font-medium">{{ product.name }}</h3><span class="text-xs" :class="product.status === 'published' ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-500'">{{ product.status === 'published' ? t('common.pointsCatalog.published') : t('common.pointsCatalog.unpublished') }}</span></div><p class="mt-2 break-words text-sm text-gray-500 dark:text-gray-400">{{ product.description }}</p><p class="mt-3 flex items-center gap-1.5 text-sm font-medium"><img :src="pointsIcon" alt="" class="h-5 w-5" />{{ format(product.price) }}</p><div class="mt-3 flex flex-wrap gap-2"><button type="button" class="btn btn-secondary btn-sm" :disabled="changing !== null" @click="edit(product)">{{ t('common.edit') }}</button><button type="button" class="btn btn-secondary btn-sm" :disabled="changing !== null" @click="toggle(product)">{{ changing === product.id ? t('common.saving') : product.status === 'published' ? t('common.pointsCatalog.unpublish') : t('common.pointsCatalog.publish') }}</button></div></div></article>
    </div>
    <BaseDialog :show="open" :title="form.id ? t('common.pointsCatalog.editProduct') : t('common.pointsCatalog.addProduct')" width="medium-wide" @close="!saving && !uploading && (open = false)">
      <form :key="revision" class="key-dialog space-y-5" @submit.prevent="save">
        <div><label for="product-kind" class="input-label">{{ t('common.pointsCatalog.productKind') }}</label><Select id="product-kind" v-model="form.kind" :options="kindOptions" :disabled="saving || uploading" /></div>
        <template v-if="form.kind === 'item'"><div><label for="product-item" class="input-label">{{ t('common.pointsCatalog.selectItem') }}</label><Select id="product-item" v-model="form.item_id" :options="itemOptions" :disabled="saving" /><p class="mt-1 text-xs text-gray-500">{{ t('common.pointsCatalog.itemProductHint') }}</p></div><div v-if="selectedItem" class="flex gap-4 rounded-xl bg-gray-50 p-4 dark:bg-dark-800"><div class="h-16 w-16 flex-shrink-0"><PointsAssetPreview :asset="selectedItem.cover_asset" /></div><div class="min-w-0"><p>{{ selectedItem.name }}</p><p class="mt-1 break-words text-sm text-gray-500">{{ selectedItem.description }}</p></div></div></template>
        <template v-else><PointsAssetUpload v-model="form.icon_asset" asset-slot="icon" :label="t('common.pointsCatalog.icon')" :disabled="saving" @busy="uploading = $event" /><div><label for="product-name" class="input-label">{{ t('common.name') }}</label><input id="product-name" v-model="form.name" class="input" maxlength="100" required :disabled="saving" /></div><div><label for="product-description" class="input-label">{{ t('common.pointsCatalog.description') }}</label><textarea id="product-description" v-model="form.description" class="input" rows="3" maxlength="2000" required :disabled="saving"></textarea></div></template>
        <div class="grid gap-4 sm:grid-cols-2"><div><label for="product-price" class="input-label">{{ t('common.pointsCatalog.price') }}</label><input id="product-price" v-model="form.price" type="text" inputmode="numeric" pattern="[0-9]+" maxlength="13" class="input" required :disabled="saving" /></div><div><label for="product-sort" class="input-label">{{ t('common.pointsCatalog.sort') }}</label><input id="product-sort" v-model.number="form.sort_order" type="number" min="-1000000" max="1000000" step="1" class="input" required :disabled="saving" /></div></div>
        <div><label for="product-status" class="input-label">{{ t('common.status') }}</label><Select id="product-status" v-model="form.status" :options="statusOptions" :disabled="saving" /></div>
        <div class="flex justify-end gap-3"><button class="btn btn-secondary" type="button" :disabled="saving || uploading" @click="open = false">{{ t('common.cancel') }}</button><button class="btn btn-primary" type="submit" :disabled="!valid || saving || uploading">{{ saving ? t('common.saving') : t('common.save') }}</button></div>
      </form>
    </BaseDialog>
  </section>
</template>
