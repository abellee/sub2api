<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="max-w-3xl text-sm text-gray-500 dark:text-gray-400">{{ t('admin.accountMatching.description') }}</p>
          <div class="flex gap-2">
            <button class="btn btn-secondary" :disabled="loading" @click="load">{{ t('common.refresh') }}</button>
            <button class="btn btn-primary" :disabled="loading || !optionsReady" @click="openForm()"><Icon name="plus" size="md" class="mr-1" />{{ t('common.add') }}</button>
          </div>
        </div>
      </template>
      <template #table>
        <p v-if="loading" class="p-8 text-center text-gray-500">{{ t('common.loading') }}</p>
        <p v-else-if="loadError" role="alert" class="p-8 text-center text-red-600">{{ loadError }}</p>
        <EmptyState v-else-if="!rules.length" :title="t('admin.accountMatching.empty')" :action-text="t('common.add')" @action="openForm()" />
        <div v-else class="overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead class="border-b border-gray-200 text-gray-500 dark:border-dark-600"><tr>
              <th class="p-4">{{ t('admin.accountMatching.user') }}</th><th class="p-4">{{ t('admin.accountMatching.groups') }}</th><th class="p-4">{{ t('admin.accountMatching.accounts') }}</th><th class="p-4">{{ t('common.actions') }}</th>
            </tr></thead>
            <tbody><tr v-for="rule in rules" :key="rule.id" class="border-b border-gray-100 align-top dark:border-dark-700">
              <td class="p-4"><span class="break-all font-medium">{{ rule.email }}</span><p class="mt-1 text-xs text-gray-400">#{{ rule.user_id }}</p></td>
              <td class="p-4"><div class="flex flex-wrap gap-1"><span v-for="id in rule.group_ids" :key="id" class="rounded bg-gray-100 px-2 py-1 dark:bg-dark-700">{{ groupName(id) }}</span></div></td>
              <td class="p-4"><div class="flex flex-wrap gap-1"><span v-for="id in rule.account_ids" :key="id" class="rounded bg-primary-50 px-2 py-1 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300">{{ accountName(id) }}</span></div></td>
              <td class="p-4"><div class="flex gap-2"><button class="btn btn-secondary btn-sm" @click="openForm(rule)">{{ t('common.edit') }}</button><button class="btn btn-danger btn-sm" @click="deleting = rule">{{ t('common.delete') }}</button></div></td>
            </tr></tbody>
          </table>
        </div>
      </template>
    </TablePageLayout>

    <BaseDialog :show="showForm" :title="t(editing ? 'admin.accountMatching.edit' : 'admin.accountMatching.add')" width="wide" :show-close-button="!saving" :close-on-escape="!saving" @close="closeForm">
      <form id="account-matching-form" class="space-y-5" @submit.prevent="save">
        <div>
          <label for="matching-email" class="input-label">{{ t('admin.accountMatching.user') }}</label>
          <input id="matching-email" v-model="email" class="input" type="search" autocomplete="off" :disabled="!!editing || saving" :placeholder="t('admin.accountMatching.emailPlaceholder')" />
          <p v-if="editing" class="mt-1 text-xs text-gray-500">{{ t('admin.accountMatching.ownerFixed') }}</p>
          <template v-else>
            <p v-if="searching" class="mt-2 text-sm text-gray-500">{{ t('common.loading') }}</p>
            <p v-else-if="searchError" role="alert" class="mt-2 text-sm text-red-600">{{ searchError }}</p>
            <p v-else-if="email.trim() && !users.length" class="mt-2 text-sm text-gray-500">{{ t('admin.accountMatching.noResults') }}</p>
            <fieldset v-if="users.length" class="mt-2 max-h-40 overflow-y-auto rounded-lg border p-2 dark:border-dark-600" :disabled="saving">
              <legend class="sr-only">{{ t('admin.accountMatching.selectUser') }}</legend>
              <label v-for="user in users" :key="user.id" class="flex cursor-pointer items-center gap-2 rounded p-2 hover:bg-gray-50 dark:hover:bg-dark-700">
                <input v-model="form.user_id" type="radio" name="matching-user" :value="user.id" class="text-primary-600" /><span class="break-all">{{ user.email }}</span><span class="text-xs text-gray-400">#{{ user.id }}</span>
              </label>
            </fieldset>
          </template>
        </div>
        <fieldset :disabled="saving" class="space-y-2">
          <legend class="input-label">{{ t('admin.accountMatching.groups') }} <span class="font-normal text-gray-500">{{ t('admin.accountMatching.selected', { count: form.group_ids.length }) }}</span></legend>
          <input v-model="groupSearch" class="input" :aria-label="t('admin.accountMatching.searchGroups')" :placeholder="t('admin.accountMatching.searchGroups')" />
          <div class="max-h-44 overflow-y-auto rounded-lg border p-2 dark:border-dark-600">
            <label v-for="group in visibleGroups" :key="group.id" class="flex cursor-pointer items-center gap-2 rounded p-2 hover:bg-gray-50 dark:hover:bg-dark-700">
              <input v-model="form.group_ids" type="checkbox" :value="group.id" class="rounded text-primary-600" /><span>{{ group.name }}</span><span class="text-xs text-gray-400">#{{ group.id }} · {{ group.platform }}</span>
            </label>
            <p v-if="!visibleGroups.length" class="p-2 text-sm text-gray-500">{{ t('admin.accountMatching.noResults') }}</p>
            <label v-for="id in missingGroupIDs" :key="id" class="flex items-center gap-2 p-2 text-red-600"><input v-model="form.group_ids" type="checkbox" :value="id" />{{ groupName(id) }}</label>
          </div>
        </fieldset>
        <fieldset :disabled="saving" class="space-y-2">
          <legend class="input-label">{{ t('admin.accountMatching.accounts') }} <span class="font-normal text-gray-500">{{ t('admin.accountMatching.selected', { count: form.account_ids.length }) }}</span></legend>
          <div class="flex flex-wrap gap-2">
            <select v-model="platform" class="input w-auto" :aria-label="t('admin.accountMatching.platform')"><option value="">{{ t('admin.accountMatching.allPlatforms') }}</option><option v-for="option in CONCRETE_PLATFORM_OPTIONS" :key="option.value" :value="option.value">{{ option.label }}</option></select>
            <input v-model="accountSearch" class="input min-w-40 flex-1" :aria-label="t('admin.accountMatching.searchAccounts')" :placeholder="t('admin.accountMatching.searchAccounts')" />
          </div>
          <div class="max-h-60 overflow-y-auto rounded-lg border p-2 dark:border-dark-600">
            <label v-for="account in visibleAccounts" :key="account.id" class="flex cursor-pointer items-center gap-2 rounded p-2 hover:bg-gray-50 dark:hover:bg-dark-700">
              <input v-model="form.account_ids" type="checkbox" :value="account.id" class="rounded text-primary-600" /><span class="break-all">{{ account.name }}</span><span class="text-xs text-gray-400">#{{ account.id }} · {{ account.platform }}</span>
            </label>
            <p v-if="!visibleAccounts.length" class="p-2 text-sm text-gray-500">{{ t('admin.accountMatching.noResults') }}</p>
            <label v-for="id in missingAccountIDs" :key="id" class="flex items-center gap-2 p-2 text-red-600"><input v-model="form.account_ids" type="checkbox" :value="id" />{{ accountName(id) }}</label>
          </div>
          <p class="text-xs text-gray-500">{{ t('admin.accountMatching.selectionHint') }}</p>
        </fieldset>
        <p v-if="formError" role="alert" class="text-sm text-red-600">{{ formError }}</p>
      </form>
      <template #footer><div class="flex justify-end gap-3"><button class="btn btn-secondary" :disabled="saving" @click="closeForm">{{ t('common.cancel') }}</button><button class="btn btn-primary" type="submit" form="account-matching-form" :disabled="saving || !canSave">{{ t(saving ? 'common.saving' : 'common.save') }}</button></div></template>
    </BaseDialog>
    <ConfirmDialog :show="!!deleting" :title="t('common.delete')" :message="t('admin.accountMatching.deleteConfirm', { email: deleting?.email || '' })" :confirm-text="t(deletingBusy ? 'common.processing' : 'common.confirm')" danger @confirm="remove" @cancel="!deletingBusy && (deleting = null)" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { accountMatchingAPI, type AccountMatchingInput, type AccountMatchingRule, type AccountMatchingUser } from '@/api/admin/accountMatching'
import accountsAPI from '@/api/admin/accounts'
import groupsAPI from '@/api/admin/groups'
import type { AccountListItem, AdminGroup } from '@/types'
import { CONCRETE_PLATFORM_OPTIONS } from '@/constants/platforms'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const app = useAppStore()
const rules = ref<AccountMatchingRule[]>([])
const groups = ref<AdminGroup[]>([])
const accounts = ref<AccountListItem[]>([])
const loading = ref(false)
const optionsReady = ref(false)
const loadError = ref('')
const showForm = ref(false)
const saving = ref(false)
const editing = ref<AccountMatchingRule | null>(null)
const deleting = ref<AccountMatchingRule | null>(null)
const deletingBusy = ref(false)
const form = ref<AccountMatchingInput>({ user_id: 0, group_ids: [], account_ids: [] })
const formError = ref('')
const email = ref('')
const users = ref<AccountMatchingUser[]>([])
const searching = ref(false)
const searchError = ref('')
const groupSearch = ref('')
const accountSearch = ref('')
const platform = ref('')
let searchTimer: ReturnType<typeof setTimeout> | undefined
let searchController: AbortController | undefined

const groupMap = computed(() => new Map(groups.value.map(group => [group.id, group])))
const accountMap = computed(() => new Map(accounts.value.map(account => [account.id, account])))
const groupName = (id: number) => groupMap.value.get(id)?.name || t('admin.accountMatching.missingGroup', { id })
const accountName = (id: number) => accountMap.value.get(id)?.name || t('admin.accountMatching.missingAccount', { id })
const matches = (id: number, name: string, query: string) => `${id} ${name}`.toLowerCase().includes(query.trim().toLowerCase())
const visibleGroups = computed(() => groups.value.filter(g => matches(g.id, g.name, groupSearch.value)))
const visibleAccounts = computed(() => accounts.value.filter(a => (!platform.value || a.platform === platform.value) && matches(a.id, a.name, accountSearch.value)))
const missingGroupIDs = computed(() => form.value.group_ids.filter(id => !groupMap.value.has(id)))
const missingAccountIDs = computed(() => form.value.account_ids.filter(id => !accountMap.value.has(id)))
const canSave = computed(() => form.value.user_id > 0 && form.value.group_ids.length > 0 && form.value.account_ids.length > 0 && !missingGroupIDs.value.length && !missingAccountIDs.value.length)

async function loadAccounts() {
  const result: AccountListItem[] = []
  for (let page = 1; ; page++) {
    const data = await accountsAPI.list(page, 100, { lite: 'true', sort_by: 'id', sort_order: 'asc' })
    result.push(...data.items)
    if (page >= data.pages || data.items.length === 0) break
  }
  return result
}
async function load() {
  loading.value = true
  loadError.value = ''
  try {
    const [ruleData, options] = await Promise.allSettled([accountMatchingAPI.list(), Promise.all([groupsAPI.getAllIncludingInactive(), loadAccounts()])])
    if (options.status === 'fulfilled') {
      groups.value = options.value[0]
      accounts.value = options.value[1]
      optionsReady.value = true
    } else { optionsReady.value = false }
    if (ruleData.status === 'fulfilled') rules.value = ruleData.value
    else throw ruleData.reason
    if (options.status === 'rejected') throw options.reason
  } catch (error) {
    loadError.value = extractApiErrorMessage(error, t('admin.accountMatching.loadFailed'))
  } finally { loading.value = false }
}
function cancelSearch() { clearTimeout(searchTimer); searchController?.abort(); searching.value = false }
function openForm(rule?: AccountMatchingRule) {
  cancelSearch()
  editing.value = rule || null
  form.value = { user_id: rule?.user_id || 0, group_ids: [...(rule?.group_ids || [])], account_ids: [...(rule?.account_ids || [])] }
  email.value = rule?.email || ''
  users.value = []
  groupSearch.value = accountSearch.value = platform.value = formError.value = searchError.value = ''
  showForm.value = true
}
function closeForm() { if (!saving.value) { showForm.value = false; cancelSearch() } }
watch(email, value => {
  cancelSearch()
  if (editing.value || !showForm.value) return
  form.value.user_id = 0
  users.value = []
  searchError.value = ''
  if (!value.trim()) return
  searching.value = true
  const controller = new AbortController()
  searchController = controller
  searchTimer = setTimeout(async () => {
    try {
      const found = await accountMatchingAPI.searchUsers(value.trim(), controller.signal)
      if (!controller.signal.aborted) users.value = found
    } catch (error) {
      if (!controller.signal.aborted) searchError.value = extractApiErrorMessage(error, t('admin.accountMatching.loadFailed'))
    } finally { if (!controller.signal.aborted) searching.value = false }
  }, 300)
})
async function save() {
  if (saving.value) return
  if (!canSave.value) { formError.value = t('admin.accountMatching.required'); return }
  saving.value = true
  formError.value = ''
  try {
    const rule = await accountMatchingAPI.save(form.value, editing.value?.id)
    rules.value = [rule, ...rules.value.filter(item => item.id !== rule.id)]
    showForm.value = false
    app.showSuccess(t('common.saved'))
  } catch (error) { formError.value = extractApiErrorMessage(error, t('admin.accountMatching.saveFailed')) }
  finally { saving.value = false }
}
async function remove() {
  if (!deleting.value || deletingBusy.value) return
  const id = deleting.value.id
  deletingBusy.value = true
  try {
    await accountMatchingAPI.delete(id)
    rules.value = rules.value.filter(item => item.id !== id)
    deleting.value = null
    app.showSuccess(t('common.deleted'))
  } catch (error) { app.showError(extractApiErrorMessage(error, t('admin.accountMatching.deleteFailed'))) }
  finally { deletingBusy.value = false }
}
onMounted(load)
onBeforeUnmount(cancelSearch)
</script>
