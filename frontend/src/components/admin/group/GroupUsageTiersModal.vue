<template>
  <BaseDialog :show="show" :title="`${t('usageTiers.title')} · ${group?.name ?? ''}`" width="wide" :close-on-escape="!saving" :show-close-button="!saving" @close="close">
    <form id="group-usage-tiers" class="space-y-4" @submit.prevent="save">
      <p class="text-sm text-gray-500">{{ t('usageTiers.adminHint') }}</p>
      <p class="text-sm text-gray-500">{{ t('usageTiers.priorityHint') }}</p>
      <p v-if="loading" role="status">{{ t('common.loading') }}</p>
      <fieldset :disabled="loading || saving || !loaded" class="space-y-4">
        <label class="flex items-center gap-2 text-sm"><input v-model="draft.enabled" type="checkbox" />{{ t('usageTiers.enabled') }}</label>
        <div>
          <label for="tier-mode" class="input-label">{{ t('usageTiers.mode') }}</label>
          <select id="tier-mode" v-model="draft.mode" class="input">
            <option value="tokens">{{ t('usageTiers.tokenMode') }}</option>
            <option value="cost">{{ t('usageTiers.costMode') }}</option>
          </select>
        </div>
        <div>
          <label for="tier-model-scope" class="input-label">{{ t('usageTiers.modelScope') }}</label>
          <select id="tier-model-scope" v-model="modelScope" class="input">
            <option value="all">{{ t('usageTiers.allModels') }}</option>
            <option value="selected">{{ t('usageTiers.selectedModels') }}</option>
          </select>
          <template v-if="modelScope === 'selected'">
            <input v-model="modelSearch" class="input mt-2" :aria-label="t('usageTiers.searchModels')" :placeholder="t('usageTiers.searchModels')" />
            <div class="mt-2 grid max-h-48 gap-2 overflow-y-auto sm:grid-cols-2">
              <label v-for="model in filteredModels" :key="model" class="flex items-center gap-2 break-all text-sm"><input v-model="draft.models" type="checkbox" :value="model" />{{ model }}</label>
            </div>
          </template>
        </div>
        <div v-for="(tier, index) in draft.tiers" :key="index" class="space-y-3 rounded-lg bg-gray-50 p-3 dark:bg-dark-800" data-test="tier-row">
          <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <div v-if="draft.mode === 'cost'"><label :for="`tier-cost-${index}`" class="input-label">{{ t('usageTiers.dailyCost') }}</label><input :id="`tier-cost-${index}`" v-model.number="tier.daily_cost" type="number" min="0.0000000001" step="any" required class="input" /></div>
            <div v-else><label :for="`tier-tokens-${index}`" class="input-label">{{ t('usageTiers.dailyTokens') }}</label><input :id="`tier-tokens-${index}`" v-model.number="tier.daily_tokens" type="number" min="1" max="9007199254740991" step="1" required class="input" /></div>
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
          <div class="mb-2 flex flex-wrap gap-2">
            <span v-for="id in draft.excluded_user_ids" :key="id" class="inline-flex max-w-full items-center gap-1 rounded-full bg-primary-50 px-3 py-1 text-xs text-primary-700 dark:bg-primary-900/20 dark:text-primary-300" data-test="excluded-user-tag">
              <span class="break-all">{{ userLabel(selectedUsers[id]) }}</span>
              <button type="button" :aria-label="`${t('common.delete')} ${userLabel(selectedUsers[id])}`" @click="removeUser(id)"><Icon name="x" size="xs" /></button>
            </span>
          </div>
          <input id="tier-excluded-users" v-model="userSearch" type="search" class="input" autocomplete="off" :placeholder="t('usageTiers.excludedHint')" @input="scheduleUserSearch" />
          <p v-if="searching" class="mt-2 text-xs" role="status">{{ t('common.loading') }}</p>
          <p v-else-if="searchFailed" class="mt-2 text-xs text-red-500" role="alert">{{ t('usageTiers.searchFailed') }}</p>
          <div v-else-if="userResults.length" class="mt-2 max-h-48 space-y-1 overflow-y-auto rounded-lg border p-2 dark:border-dark-600">
            <button v-for="user in userResults" :key="user.id" type="button" class="block w-full rounded px-2 py-1 text-left text-sm hover:bg-gray-100 dark:hover:bg-dark-700" data-test="excluded-user-result" @click="addUser(user)">{{ userLabel(user) }}</button>
          </div>
          <p v-else-if="searched" class="mt-2 text-xs text-gray-500">{{ t('usageTiers.noUsers') }}</p>
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
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import { getModelAllowlistCandidates, getUsageTiers, saveUsageTiers } from '@/api/admin/groups'
import usersAPI from '@/api/admin/users'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { AdminGroup } from '@/types'
import type { GroupUsageTierConfig } from '@/types/usageTiers'

const props = defineProps<{ show: boolean; group: AdminGroup | null }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const app = useAppStore()
const emptyConfig = (): GroupUsageTierConfig => ({ mode: 'tokens', enabled: false, tiers: [], models: [], linked_group_ids: [], excluded_user_ids: [] })
const draft = ref(emptyConfig())
const groups = ref<AdminGroup[]>([])
type SelectedUser = { id: number; username?: string; email: string }
const selectedUsers = ref<Record<number, SelectedUser>>({})
const userSearch = ref(''), userResults = ref<SelectedUser[]>([])
const searching = ref(false), searched = ref(false), searchFailed = ref(false)
const modelScope = ref<'all' | 'selected'>('all'), modelSearch = ref(''), models = ref<string[]>([])
const filteredModels = computed(() => [...new Set([...models.value, ...draft.value.models])].filter(model => model.toLowerCase().includes(modelSearch.value.toLowerCase())))
const userLabel = (user?: SelectedUser) => user ? (user.username ? `${user.username} · ${user.email}` : user.email) : t('usageTiers.unavailableUser')
let searchTimer: ReturnType<typeof setTimeout> | undefined
let searchController: AbortController | undefined
function clearUserSearch() {
  clearTimeout(searchTimer); searchController?.abort(); userResults.value = []; searching.value = false; searched.value = false; searchFailed.value = false
}
function scheduleUserSearch() {
  clearUserSearch()
  const query = userSearch.value.trim(), request = version
  if (!query) return
  const controller = new AbortController(); searchController = controller
  searching.value = true
  searchTimer = setTimeout(async () => {
    try {
      const data = await usersAPI.list(1, 30, { search: query, include_subscriptions: false }, { signal: controller.signal })
      if (request === version && !controller.signal.aborted) { userResults.value = data.items.filter(user => !draft.value.excluded_user_ids.includes(user.id)); searched.value = true }
    } catch { if (request === version && !controller.signal.aborted) searchFailed.value = true }
    finally { if (request === version && !controller.signal.aborted) searching.value = false }
  }, 300)
}
function addUser(user: SelectedUser) {
  if (!draft.value.excluded_user_ids.includes(user.id) && draft.value.excluded_user_ids.length < 1000) {
    draft.value.excluded_user_ids.push(user.id); selectedUsers.value[user.id] = user
  }
  userSearch.value = ''; clearUserSearch()
}
function removeUser(id: number) { draft.value.excluded_user_ids = draft.value.excluded_user_ids.filter(value => value !== id) }
const loading = ref(false)
const loaded = ref(false)
const saving = ref(false)
let version = 0
const linkOptions = computed(() => groups.value.filter(group => group.id !== props.group?.id))

watch(() => [props.show, props.group?.id] as const, async ([show, id]) => {
  const request = ++version
  clearUserSearch()
  loaded.value = false
  if (!show || !id) return
  loading.value = true
  draft.value = emptyConfig()
  selectedUsers.value = {}; userSearch.value = ''; modelSearch.value = ''; models.value = []
  try {
    const [config, options, modelOptions] = await Promise.all([getUsageTiers(id), adminAPI.groups.getAllIncludingInactive(), getModelAllowlistCandidates(id, props.group?.platform)])
    if (request !== version) return
    draft.value = { ...config, mode: config.mode ?? (config.tiers?.some(tier => tier.daily_cost && !tier.daily_tokens) ? 'cost' : 'tokens'), tiers: config.tiers ?? [], models: config.models ?? [], linked_group_ids: config.linked_group_ids ?? [], excluded_user_ids: config.excluded_user_ids ?? [] }
    modelScope.value = draft.value.models.length ? 'selected' : 'all'
    models.value = props.group?.model_allowlist?.enabled ? props.group.model_allowlist.models : modelOptions
    groups.value = options
    loaded.value = true
    // Resolve saved selections in small batches; unavailable users remain removable.
    const ids = [...draft.value.excluded_user_ids]
    for (let offset = 0; offset < ids.length && request === version; offset += 10) {
      await Promise.all(ids.slice(offset, offset + 10).map(async userId => {
        try { const user = await usersAPI.getById(userId); if (request === version) selectedUsers.value[userId] = user } catch { /* Keep the saved exclusion until explicitly removed. */ }
      }))
    }
  } catch (error) {
    if (request === version) app.showError(extractApiErrorMessage(error, t('usageTiers.loadFailed')))
  } finally {
    if (request === version) loading.value = false
  }
}, { immediate: true })

function addTier() {
  const previous = draft.value.tiers.at(-1)
  draft.value.tiers.push({ daily_tokens: previous?.daily_tokens || 1000000, daily_cost: previous?.daily_cost || 1, consecutive_days: 1, max_missed_days: 3, rate_multiplier: Number(((previous?.rate_multiplier ?? props.group?.rate_multiplier ?? 1) * 0.9).toFixed(6)) })
}

function close() { if (!saving.value) emit('close') }

async function save() {
  if (!loaded.value || saving.value || !props.group) return
  const ids = draft.value.excluded_user_ids
  const tiers = draft.value.tiers
  const costMode = draft.value.mode === 'cost'
  const target = (tier: typeof tiers[number]) => costMode ? tier.daily_cost ?? 0 : tier.daily_tokens
  const invalid = (modelScope.value === 'selected' && !draft.value.models.length) || draft.value.models.length > 200 || ids.some(id => !Number.isSafeInteger(id) || id <= 0) || ids.length > 1000 || tiers.length > 20 || (draft.value.enabled && !tiers.length) || tiers.some((tier, index) =>
    !Number.isFinite(target(tier)) || target(tier) <= 0 || (!costMode && !Number.isSafeInteger(target(tier))) || !Number.isFinite(tier.rate_multiplier) || tier.rate_multiplier < 0 || tier.rate_multiplier > 100 ||
    !Number.isInteger(tier.consecutive_days) || tier.consecutive_days < 1 || tier.consecutive_days > 365 || !Number.isInteger(tier.max_missed_days) || tier.max_missed_days < 1 || tier.max_missed_days > 365 ||
    (index > 0 && (target(tier) < target(tiers[index - 1]!) || tier.rate_multiplier >= tiers[index - 1]!.rate_multiplier)))
  if (invalid) { app.showError(t('usageTiers.invalid')); return }
  saving.value = true
  const id = props.group.id
  try {
    await saveUsageTiers(id, { ...draft.value, tiers: tiers.map(tier => ({ ...tier, daily_tokens: costMode ? 0 : tier.daily_tokens, daily_cost: costMode ? tier.daily_cost : 0 })), models: modelScope.value === 'all' ? [] : [...draft.value.models].sort(), excluded_user_ids: [...new Set(ids)] })
    app.showSuccess(t('usageTiers.saved'))
    emit('close')
  } catch (error) {
    app.showError(extractApiErrorMessage(error, t('usageTiers.saveFailed')))
  } finally { saving.value = false }
}
onBeforeUnmount(() => { version++; clearUserSearch() })
</script>
