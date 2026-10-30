<template>
  <div>
    <label :for="id" class="input-label">{{ t('creativeDocs.bindingLabel') }}</label>
    <Select :id="id" :model-value="modelValue" :options="options" :disabled="disabled"
      :aria-label="t('creativeDocs.bindingLabel')" @update:model-value="$emit('update:modelValue', String($event))" />
    <p class="input-hint">{{ t('creativeDocs.bindingHint') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import { documentCatalog } from '../../../public/creative-docs/catalog.js'

defineProps<{ id: string; modelValue: string; disabled?: boolean }>()
defineEmits<{ 'update:modelValue': [value: string] }>()
const { t } = useI18n()
const options = computed(() => [
  { value: '', label: t('creativeDocs.noDocument') },
  ...documentCatalog.map(doc => ({ value: doc.id, label: `${t(doc.kind === 'image' ? 'creativeDocs.images' : 'creativeDocs.videos')} / ${doc.label} · ${doc.id}` })),
])
</script>
