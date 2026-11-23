<template>
  <BaseDialog :show="show" :title="`${t('usageTiers.title')} · ${group?.name ?? ''}`" width="wide" :close-on-escape="!saving" :show-close-button="!saving" @close="close">
    <form id="group-usage-tiers" class="space-y-4" @submit.prevent="save">
      <p class="text-sm text-gray-500">{{ t('usageTiers.adminHint') }}</p>
      <p class="text-sm text-gray-500">{{ t('usageTiers.priorityHint') }}</p>
      <p v-if="loading" role="status">{{ t('common.loading') }}</p>
      <fieldset :disabled="loading || saving || !loaded" class="space-y-4">
        <label class="flex items-center gap-2 text-sm"><input v-model="draft.enabled" type="checkbox" />{{ t('usageTiers.enabled') }}</label>
        <div v-for="(tier, index) in draft.tiers" :key="index" class="space-y-3 rounded-lg bg-gray-50 p-3 dark:bg-dark-800" data-test="tier-row">
          <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <div><label :for="`tier-cost-${index}`" class="input-label">{{ t('usageTiers.dailyCost') }}</label><input :id="`tier-cost-${index}`" v-model.number="tier.daily_cost" type="number" min="0.0000000001" step="any" required class="input" /></div>
            <div><label :for="`tier-days-${index}`" class="input-label">{{ t('usageTiers.days') }}</label><input :id="`tier-days-${index}`" v-model.number="tier.consecutive_days" type="number" min="1" max="365" step="1" required class="input" /></div>
            <div><label :for="`tier-missed-${index}`" class="input-label">{{ t('usageTiers.missed') }}</label><input :id="`tier-missed-${index}`" v-model.number="tier.max_missed_days" type="number" min="1" max="365" step="1" required class="input" /></div>
            <div><label :for="`tier-rate-${index}`" class="input-label">{{ t('usageTiers.rate') }}</label><input :id="`tier-rate-${index}`" v-model.number="tier.rate_multiplier" type="number" min="0" max="100" step="any" required class="input" /></div>
          </div>
          <button type="button" class="btn btn-secondary text-xs" @click="draft.tiers.splice(index, 1)">{{ t('common.delete') }}</button>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="draft.tiers.length >= 20" @click="addTier">{{ t('usageTiers.add') }}</button>
        <div>
          <label class="input-label">{{ t('usageTiers.linked') }}</label>
          <div class="grid max-h-48 grid-cols-1 gap-2 overflow-auto rounded-lg border p-3 sm:grid-cols-2 dark:border-dark-600">
            <label v-for="item in linkOptions" :key="item.id" class="flex items-center gap-2 text-sm"><input v-model="draft.linked_group_ids" type="checkbox" :value="item.id" />{{ item.name }}</label>
          </div>
        </div>
        <div>
          <label for="tier-excluded-users" class="input-label">{{ t('usageTiers.excludedUsers') }}</label>
          <textarea id="tier-excluded-users" v-model="excludedText" class="input" rows="3" :placeholder="t('usageTiers.excludedHint')" />
        </div>
      </fieldset>
    </form>
    <template #footer>
      <button class="btn btn-secondary" :disabled="saving" @click="close">{{ t('common.cancel') }}</button>
      <button type="submit" form="group-usage-tiers" class="btn btn-primary" :disabled="loading || saving || !loaded">{{ t(saving ? 'common.saving' : 'common.save') }}</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import { getUsageTiers, saveUsageTiers } from '@/api/admin/groups'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { AdminGroup } from '@/types'
import type { GroupUsageTierConfig } from '@/types/usageTiers'

const props = defineProps<{ show: boolean; group: AdminGroup | null }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const app = useAppStore()
const emptyConfig = (): GroupUsageTierConfig => ({ enabled: false, tiers: [], linked_group_ids: [], excluded_user_ids: [] })
const draft = ref(emptyConfig())
const groups = ref<AdminGroup[]>([])
const excludedText = ref('')
const loading = ref(false)
const loaded = ref(false)
const saving = ref(false)
let version = 0
const linkOptions = computed(() => groups.value.filter(group => group.id !== props.group?.id))

watch(() => [props.show, props.group?.id] as const, async ([show, id]) => {
  const request = ++version
  loaded.value = false
  if (!show || !id) return
  loading.value = true
  draft.value = emptyConfig()
  excludedText.value = ''
  try {
    const [config, options] = await Promise.all([getUsageTiers(id), adminAPI.groups.getAllIncludingInactive()])
    if (request !== version) return
    draft.value = { ...config, tiers: config.tiers ?? [], linked_group_ids: config.linked_group_ids ?? [], excluded_user_ids: config.excluded_user_ids ?? [] }
    excludedText.value = draft.value.excluded_user_ids.join(', ')
    groups.value = options
    loaded.value = true
  } catch (error) {
    if (request === version) app.showError(extractApiErrorMessage(error, t('usageTiers.loadFailed')))
  } finally {
    if (request === version) loading.value = false
  }
}, { immediate: true })

function addTier() {
  const previous = draft.value.tiers.at(-1)
  draft.value.tiers.push({ daily_cost: previous?.daily_cost ?? 1, consecutive_days: 1, max_missed_days: 3, rate_multiplier: Number(((previous?.rate_multiplier ?? props.group?.rate_multiplier ?? 1) * 0.9).toFixed(6)) })
}

function close() { if (!saving.value) emit('close') }

async function save() {
  if (!loaded.value || saving.value || !props.group) return
  const tokens = excludedText.value.trim() ? excludedText.value.trim().split(/[,，\s]+/) : []
  const ids = tokens.map(Number)
  const tiers = draft.value.tiers
  const invalid = ids.some(id => !Number.isSafeInteger(id) || id <= 0) || ids.length > 1000 || tiers.length > 20 || (draft.value.enabled && !tiers.length) || tiers.some((tier, index) =>
    !Number.isFinite(tier.daily_cost) || tier.daily_cost <= 0 || !Number.isFinite(tier.rate_multiplier) || tier.rate_multiplier < 0 || tier.rate_multiplier > 100 ||
    !Number.isInteger(tier.consecutive_days) || tier.consecutive_days < 1 || tier.consecutive_days > 365 || !Number.isInteger(tier.max_missed_days) || tier.max_missed_days < 1 || tier.max_missed_days > 365 ||
    (index > 0 && (tier.daily_cost < tiers[index - 1]!.daily_cost || tier.rate_multiplier >= tiers[index - 1]!.rate_multiplier)))
  if (invalid) { app.showError(t('usageTiers.invalid')); return }
  saving.value = true
  const id = props.group.id
  try {
    await saveUsageTiers(id, { ...draft.value, excluded_user_ids: [...new Set(ids)] })
    app.showSuccess(t('usageTiers.saved'))
    emit('close')
  } catch (error) {
    app.showError(extractApiErrorMessage(error, t('usageTiers.saveFailed')))
  } finally { saving.value = false }
}
</script>
