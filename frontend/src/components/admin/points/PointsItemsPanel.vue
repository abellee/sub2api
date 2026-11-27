<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { fetchManagedItems, saveManagedItem, type ManagedItem, type AssetSlot } from '@/api/pointsShop'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PointsAssetUpload from './PointsAssetUpload.vue'
import PointsAssetPreview from '@/components/user/PointsAssetPreview.vue'
import '@/styles/key-dialog.css'

const { t } = useI18n()
const app = useAppStore()
const items = ref<ManagedItem[]>([])
const loading = ref(false), failed = ref(false), saving = ref(false), open = ref(false)
const revision = ref(0)
const busy = reactive<Record<string, boolean>>({})
const blank = (): ManagedItem => ({ id: 0, code: '', name: '', description: '', cover_asset: '', active_asset: '', expired_asset: '', animation_asset: '' })
const form = reactive(blank())
const effectKind = ref<'none' | 'grant_points' | 'grant_item'>('none')
const effectPoints = ref(10), effectItemID = ref(0), effectQuantity = ref(1)
let originalEffect = ''
const slots: { slot: AssetSlot; field: 'cover_asset' | 'active_asset' | 'expired_asset' | 'animation_asset'; label: string }[] = [
  { slot: 'cover', field: 'cover_asset', label: 'cover' }, { slot: 'active', field: 'active_asset', label: 'active' }, { slot: 'expired', field: 'expired_asset', label: 'expired' }, { slot: 'animation', field: 'animation_asset', label: 'animation' }
]
const uploading = computed(() => Object.values(busy).some(Boolean))
const valid = computed(() => /^[a-zA-Z0-9][a-zA-Z0-9_-]{0,79}$/.test(form.code) && !!form.name.trim() && slots.every(slot => !!form[slot.field]) &&
  (effectKind.value !== 'grant_points' || (Number.isSafeInteger(effectPoints.value) && effectPoints.value >= 1 && effectPoints.value <= 1_000_000_000_000)) &&
  (effectKind.value !== 'grant_item' || (effectItemID.value > 0 && Number.isSafeInteger(effectQuantity.value) && effectQuantity.value >= 1 && effectQuantity.value <= 1_000_000)))
async function load() {
  loading.value = true; failed.value = false
  try { items.value = await fetchManagedItems() } catch { failed.value = true } finally { loading.value = false }
}
function edit(item?: ManagedItem) {
  Object.assign(form, item ?? blank()); Object.keys(busy).forEach(key => delete busy[key]); revision.value++; open.value = true
  form.effect = item?.effect ? { ...item.effect } : undefined
  effectKind.value = item?.effect?.kind || 'none'; effectPoints.value = item?.effect?.points || 10
  effectItemID.value = item?.effect?.item_id || 0; effectQuantity.value = item?.effect?.quantity || 1
  originalEffect = JSON.stringify(effectValue())
}
function effectValue(): NonNullable<ManagedItem['effect']> {
  if (effectKind.value === 'grant_points') return { kind: 'grant_points', points: effectPoints.value }
  if (effectKind.value === 'grant_item') return { kind: 'grant_item', item_id: effectItemID.value, quantity: effectQuantity.value }
  return { kind: 'none' }
}
async function save() {
  if (!valid.value || uploading.value || saving.value) return
  saving.value = true
  try {
    const value = { ...form, name: form.name.trim(), description: form.description.trim() }
    if (originalEffect !== JSON.stringify(effectValue())) value.effect = effectValue()
    await saveManagedItem(value); open.value = false; app.showSuccess(t('common.saved')); await load()
  }
  catch (err) { app.showError(err instanceof Error ? err.message : t('common.error')) }
  finally { saving.value = false }
}
onMounted(load)
</script>

<template>
  <section class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-3"><div><h2 class="text-base font-semibold">{{ t('common.pointsCatalog.items') }}</h2><p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('common.pointsCatalog.itemHint') }}</p></div><button type="button" class="btn btn-primary" @click="edit()">{{ t('common.pointsCatalog.addItem') }}</button></div>
    <div v-if="loading" role="status">{{ t('common.loading') }}</div>
    <div v-else-if="failed" class="card space-y-3 p-5" role="alert"><p>{{ t('common.pointsCatalog.loadError') }}</p><button class="btn btn-secondary" type="button" @click="load">{{ t('common.tryAgain') }}</button></div>
    <p v-else-if="!items.length" class="card p-6 text-sm text-gray-500">{{ t('common.pointsCatalog.noItems') }}</p>
    <div v-else class="grid gap-4 sm:grid-cols-2">
      <article v-for="item in items" :key="item.id" class="card flex items-start gap-4 p-5"><div class="h-16 w-16 flex-shrink-0"><PointsAssetPreview :asset="item.cover_asset" /></div><div class="min-w-0 flex-1"><h3 class="break-words font-medium">{{ item.name }}</h3><p class="mt-1 break-all text-xs text-gray-500">{{ item.code }}</p><p class="mt-2 break-words text-sm text-gray-500 dark:text-gray-400">{{ item.description }}</p><button type="button" class="btn btn-secondary btn-sm mt-3" @click="edit(item)">{{ t('common.edit') }}</button></div></article>
    </div>
    <BaseDialog :show="open" :title="form.id ? t('common.pointsCatalog.editItem') : t('common.pointsCatalog.addItem')" width="medium-wide" @close="!saving && !uploading && (open = false)">
      <form :key="revision" class="key-dialog space-y-5" @submit.prevent="save">
        <div><label for="item-code" class="input-label">{{ t('common.pointsCatalog.code') }}</label><input id="item-code" v-model="form.code" class="input" maxlength="80" pattern="[a-zA-Z0-9][a-zA-Z0-9_-]{0,79}" required :disabled="!!form.id || saving" /><p class="mt-1 text-xs text-gray-500">{{ t('common.pointsCatalog.codeHint') }}</p></div>
        <div><label for="item-name" class="input-label">{{ t('common.name') }}</label><input id="item-name" v-model="form.name" class="input" maxlength="100" required :disabled="saving" /></div>
        <div><label for="item-description" class="input-label">{{ t('common.pointsCatalog.description') }}</label><textarea id="item-description" v-model="form.description" class="input" rows="2" maxlength="2000" :disabled="saving"></textarea></div>
        <div class="space-y-3 rounded-xl border border-gray-200 p-4 dark:border-dark-700">
          <div><label for="item-effect" class="input-label">{{ t('common.events.itemEffect') }}</label><select id="item-effect" v-model="effectKind" class="input" :disabled="saving"><option value="none">{{ t('common.events.noEffect') }}</option><option value="grant_points">{{ t('common.events.actions.grant_points') }}</option><option value="grant_item">{{ t('common.events.actions.grant_item') }}</option></select><p class="mt-1 text-xs text-gray-500">{{ t('common.events.effectHint') }}</p></div>
          <div v-if="effectKind === 'grant_points'"><label for="item-effect-points" class="input-label">{{ t('common.events.effectPoints') }}</label><input id="item-effect-points" v-model.number="effectPoints" class="input" type="number" min="1" max="1000000000000" step="1" required :disabled="saving" /></div>
          <div v-if="effectKind === 'grant_item'" class="grid gap-3 sm:grid-cols-2"><div><label for="item-effect-target" class="input-label">{{ t('common.events.effectItem') }}</label><select id="item-effect-target" v-model.number="effectItemID" class="input" required :disabled="saving"><option :value="0" disabled>{{ t('common.pointsCatalog.selectItem') }}</option><option v-for="target in items" :key="target.id" :value="target.id">{{ target.name }}</option></select></div><div><label for="item-effect-quantity" class="input-label">{{ t('common.events.effectQuantity') }}</label><input id="item-effect-quantity" v-model.number="effectQuantity" class="input" type="number" min="1" max="1000000" step="1" required :disabled="saving" /></div></div>
        </div>
        <div class="grid gap-4 sm:grid-cols-2"><PointsAssetUpload v-for="slot in slots" :key="slot.slot" v-model="form[slot.field]" :asset-slot="slot.slot" :label="t('common.pointsCatalog.' + slot.label)" :disabled="saving" @busy="busy[slot.slot] = $event" /></div>
        <div class="flex justify-end gap-3"><button class="btn btn-secondary" type="button" :disabled="saving || uploading" @click="open = false">{{ t('common.cancel') }}</button><button class="btn btn-primary" type="submit" :disabled="!valid || saving || uploading">{{ saving ? t('common.saving') : t('common.save') }}</button></div>
      </form>
    </BaseDialog>
  </section>
</template>
