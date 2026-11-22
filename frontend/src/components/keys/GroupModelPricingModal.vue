<template>
  <BaseDialog :show="visible" :title="pricingTitle" width="extra-wide" @close="emit('close')">
    <div
      v-if="loading"
      class="py-12 text-center text-sm text-gray-400 dark:text-dark-400"
    >
      {{ t('common.loading') }}
    </div>
    <PlazaGroupSection
      v-else-if="pricingGroup"
      :group="pricingGroup"
      scrollable
    />
    <div
      v-else
      class="py-12 text-center text-sm text-gray-400 dark:text-dark-400"
    >
      {{ t('keys.modelPricingEmpty') }}
    </div>
    <template #footer>
      <div class="flex justify-end">
        <button type="button" class="btn btn-secondary" @click="emit('close')">
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import BaseDialog from '@/components/common/BaseDialog.vue'
import PlazaGroupSection from '@/components/modelPlaza/PlazaGroupSection.vue'
import { modelPlazaAPI, type ModelPlazaGroup } from '@/api/modelPlaza'

/**
 * 分组模型价格弹窗:复用模型广场分组卡片(价格表)。
 * plaza 数据按进程缓存,多处打开只请求一次;失败后下次重新请求。
 */
const props = defineProps<{
  visible: boolean
  groupId: number | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()

const loading = ref(false)
const pricingGroup = ref<ModelPlazaGroup | null>(null)
let plazaGroupsPromise: Promise<ModelPlazaGroup[]> | null = null

const pricingTitle = computed(() =>
  pricingGroup.value
    ? `${pricingGroup.value.name} · ${t('keys.modelPricing')}`
    : t('keys.modelPricing')
)

function loadPlazaGroups(): Promise<ModelPlazaGroup[]> {
  plazaGroupsPromise ??= modelPlazaAPI
    .getModelPlaza()
    .then((response) => response.groups ?? [])
    .catch((error) => {
      // 失败后清空缓存,下次打开重新请求
      plazaGroupsPromise = null
      throw error
    })
  return plazaGroupsPromise
}

watch(
  () => [props.visible, props.groupId] as const,
  async ([visible]) => {
    if (!visible || props.groupId == null) return
    loading.value = true
    pricingGroup.value = null
    try {
      const groups = await loadPlazaGroups()
      pricingGroup.value = groups.find((group) => group.id === props.groupId) ?? null
    } catch {
      pricingGroup.value = null
    } finally {
      loading.value = false
    }
  },
  { immediate: true }
)
</script>
