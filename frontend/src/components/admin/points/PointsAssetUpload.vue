<script setup lang="ts">
import { computed, onBeforeUnmount, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { uploadPointsAsset, type AssetSlot } from '@/api/pointsShop'
import PointsAssetPreview from '@/components/user/PointsAssetPreview.vue'

const props = defineProps<{ modelValue: string; assetSlot: AssetSlot; label: string; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string]; busy: [value: boolean] }>()
const { t } = useI18n()
const id = useId()
const uploading = ref(false)
const error = ref('')
const filename = ref('')
let version = 0
const accept = computed(() => props.assetSlot === 'animation' ? '.svg,.json,.lottie' : props.assetSlot === 'icon' ? '.svg,.png,.jpg,.jpeg' : '.svg')
async function change(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  input.value = ''
  const current = ++version
  error.value = ''
  const extension = '.' + file.name.split('.').pop()?.toLowerCase()
  const limit = extension === '.svg' ? 2 * 1024 * 1024 : 8 * 1024 * 1024
  if (!accept.value.split(',').includes(extension) || file.size > limit || file.size === 0) { error.value = t('common.pointsCatalog.invalidFile'); return }
  uploading.value = true; emit('busy', true)
  try {
    const result = await uploadPointsAsset(file, props.assetSlot)
    if (current !== version) return
    filename.value = file.name
    emit('update:modelValue', result.id)
  } catch (err) { if (current === version) error.value = err instanceof Error ? err.message : t('common.pointsCatalog.uploadError') }
  finally { if (current === version) { uploading.value = false; emit('busy', false) } }
}
onBeforeUnmount(() => { version++ })
</script>

<template>
  <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
    <label :for="id" class="input-label">{{ label }} <span aria-hidden="true">*</span></label>
    <div class="mb-3 h-20 w-20 rounded-lg bg-gray-50 p-2 dark:bg-dark-800"><PointsAssetPreview v-if="modelValue" :asset="modelValue" :animation="assetSlot === 'animation'" /></div>
    <input :id="id" type="file" :accept="accept" :disabled="disabled || uploading" class="block w-full text-xs file:mr-2 file:rounded-lg file:border-0 file:px-3 file:py-2" @change="change" />
    <p class="mt-2 break-words text-xs text-gray-500 dark:text-gray-400">{{ uploading ? t('common.submitting') : filename || (assetSlot === 'animation' ? t('common.pointsCatalog.animationHint') : assetSlot === 'icon' ? t('common.pointsCatalog.iconHint') : t('common.pointsCatalog.svgHint')) }}</p>
    <p v-if="error" class="mt-2 text-xs text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
  </div>
</template>
