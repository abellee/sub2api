<template>
  <div class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-600">
    <div class="flex items-center justify-between gap-3">
      <div>
        <label :id="`${idPrefix}-label`" class="input-label mb-0">{{ t('admin.groups.thinkingCompat.title') }}</label>
        <p class="input-hint">{{ t('admin.groups.thinkingCompat.hint') }}</p>
      </div>
      <Toggle :model-value="modelValue.enabled" :aria-labelledby="`${idPrefix}-label`" @update:model-value="update({ enabled: $event })" />
    </div>
    <div v-if="modelValue.enabled" class="space-y-3">
      <div class="flex flex-wrap gap-4">
        <label class="flex items-center gap-2 text-sm">
          <input type="radio" :name="`${idPrefix}-scope`" :checked="modelValue.all_models" @change="update({ all_models: true })" />
          {{ t('admin.groups.thinkingCompat.allModels') }}
        </label>
        <label class="flex items-center gap-2 text-sm">
          <input type="radio" :name="`${idPrefix}-scope`" :checked="!modelValue.all_models" @change="update({ all_models: false })" />
          {{ t('admin.groups.thinkingCompat.selectedModels') }}
        </label>
      </div>
      <div v-if="!modelValue.all_models">
        <label :for="`${idPrefix}-models`" class="input-label">{{ t('admin.groups.thinkingCompat.models') }}</label>
        <textarea :id="`${idPrefix}-models`" v-model="modelText" class="input font-mono" rows="3" :placeholder="'claude-sonnet-5-5\nclaude-opus-5-5'" :aria-invalid="invalid || undefined" :aria-describedby="invalid ? `${idPrefix}-error` : undefined" />
        <p class="input-hint">{{ t('admin.groups.thinkingCompat.modelsHint') }}</p>
        <p v-if="invalid" :id="`${idPrefix}-error`" class="text-sm text-red-600" role="alert">{{ t('admin.groups.thinkingCompat.selectAtLeastOne') }}</p>
      </div>
      <p class="input-hint">{{ t('admin.groups.thinkingCompat.limitHint') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import type { AnthropicThinkingCompat } from '@/types'

const props = defineProps<{ modelValue: AnthropicThinkingCompat; idPrefix: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: AnthropicThinkingCompat] }>()
const { t } = useI18n()
const showValidation = ref(false)
const invalid = computed(() => showValidation.value && props.modelValue.enabled && !props.modelValue.all_models && !(props.modelValue.models ?? []).length)
const update = (patch: Partial<AnthropicThinkingCompat>) => emit('update:modelValue', { ...props.modelValue, ...patch })
const parseModels = (text: string) => [...new Set(text.split(/[\s,，]+/).filter(Boolean))]
const modelText = ref((props.modelValue.models ?? []).join('\n'))
watch(modelText, text => update({ models: parseModels(text) }))
watch(() => props.modelValue.models, models => {
  if (JSON.stringify(parseModels(modelText.value)) !== JSON.stringify(models ?? [])) {
    modelText.value = (models ?? []).join('\n')
  }
}, { deep: true })
defineExpose({
  validate: () => { showValidation.value = true; return !invalid.value },
  resetValidation: () => { showValidation.value = false },
})
</script>
