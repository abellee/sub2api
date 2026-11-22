<template>
  <AppLayout>
    <div class="space-y-6" data-testid="menu-management-page">
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.menuManagement.title') }}</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.menuManagement.description') }}</p>
        </div>
        <div class="flex items-center gap-2">
          <span v-if="dirty" class="text-xs text-amber-600 dark:text-amber-400">{{ t('admin.menuManagement.unsaved') }}</span>
          <button class="btn btn-secondary" :disabled="loading || saving" @click="load">
            {{ dirty ? t('admin.menuManagement.discard') : t('common.refresh') }}
          </button>
          <button class="btn btn-primary" :disabled="!loaded || !dirty || saving" data-testid="save-menu" @click="save">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </header>

      <div v-if="loading" class="py-16 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
      <div v-else-if="!loaded" class="rounded-xl border border-red-200 bg-red-50 p-5 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-300" role="alert">
        {{ t('admin.menuManagement.loadFailed') }}
      </div>
      <div v-else class="grid items-start gap-6 xl:grid-cols-[340px,minmax(0,1fr)]">
        <section class="rounded-xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
          <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('admin.menuManagement.headings') }}</h3>
          <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('admin.menuManagement.headingsHint') }}</p>
          <form class="mt-4 flex gap-2" @submit.prevent="addSection">
            <input v-model="newTitle" class="input min-w-0 flex-1" maxlength="80" :aria-label="t('admin.menuManagement.newHeading')" :placeholder="t('admin.menuManagement.newHeading')" data-testid="new-heading" />
            <button type="submit" class="btn btn-primary shrink-0" :disabled="!newTitle.trim() || sections.length >= 32 || saving" data-testid="add-heading">
              {{ t('common.add') }}
            </button>
          </form>
          <div class="mt-4 space-y-3">
            <div v-for="(section, index) in sections" :key="section.id" class="rounded-lg border border-gray-200 p-3 dark:border-dark-600" :data-testid="`heading-${section.id}`">
              <input v-model="section.title" class="input" maxlength="80" :aria-label="t('admin.menuManagement.headingTitle')" :disabled="saving" />
              <div class="mt-2 flex items-center justify-between gap-2">
                <span class="text-xs text-gray-400">{{ t('admin.menuManagement.itemCount', { n: section.items.length }) }}</span>
                <div class="flex gap-1">
                  <button class="btn btn-secondary btn-sm" :title="t('admin.menuManagement.moveUp')" :aria-label="t('admin.menuManagement.moveUp')" :disabled="index === 0 || saving" @click="moveSection(index, -1)"><Icon name="arrowUp" size="sm" /></button>
                  <button class="btn btn-secondary btn-sm" :title="t('admin.menuManagement.moveDown')" :aria-label="t('admin.menuManagement.moveDown')" :disabled="index === sections.length - 1 || saving" @click="moveSection(index, 1)"><Icon name="arrowDown" size="sm" /></button>
                  <button class="btn btn-secondary btn-sm text-red-600 dark:text-red-400" :disabled="saving" :data-testid="`delete-heading-${section.id}`" @click="sections.splice(index, 1)">{{ t('common.delete') }}</button>
                </div>
              </div>
            </div>
            <p v-if="!sections.length" class="rounded-lg bg-gray-50 p-4 text-sm leading-6 text-gray-500 dark:bg-dark-900/50 dark:text-gray-400">{{ t('admin.menuManagement.empty') }}</p>
          </div>
        </section>

        <section class="min-w-0 overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
          <div class="border-b border-gray-200 p-5 dark:border-dark-700">
            <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('admin.menuManagement.menuItems') }}</h3>
            <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('admin.menuManagement.itemsHint') }}</p>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full text-left text-sm">
              <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900/50 dark:text-gray-400">
                <tr>
                  <th class="px-5 py-3 font-medium">{{ t('admin.menuManagement.menuItem') }}</th>
                  <th class="px-5 py-3 font-medium">{{ t('admin.menuManagement.visible') }}</th>
                  <th class="px-5 py-3 font-medium">{{ t('admin.menuManagement.underHeading') }}</th>
                  <th class="w-24 px-5 py-3 text-right font-medium">{{ t('admin.menuManagement.order') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="item in catalog" :key="item.path">
                  <td class="px-5 py-3 font-medium text-gray-900 dark:text-white">
                    {{ item.label }}
                    <span v-if="item.custom" class="mt-0.5 block text-xs font-normal text-gray-400">{{ t('admin.menuManagement.customPage') }}</span>
                  </td>
                  <td class="px-5 py-3">
                    <Toggle :model-value="!hiddenItems.includes(item.path)" :disabled="saving" :aria-label="t('admin.menuManagement.visibleLabel', { name: item.label })" :data-testid="`visible-${item.path}`" @update:model-value="setItemVisible(item.path, $event)" />
                  </td>
                  <td class="px-5 py-3">
                    <select class="input min-w-36" :value="sectionFor(item.path)?.id ?? ''" :disabled="saving" :aria-label="t('admin.menuManagement.assignLabel', { name: item.label })" :data-testid="`assign-${item.path}`" @change="assignUserMenuItem(sections, item.path, ($event.target as HTMLSelectElement).value)">
                      <option value="">{{ t('nav.myAccount') }} · {{ t('admin.menuManagement.defaultHeading') }}</option>
                      <option v-for="section in sections" :key="section.id" :value="section.id">{{ section.title }}</option>
                    </select>
                  </td>
                  <td class="px-5 py-3">
                    <div class="flex justify-end gap-1">
                      <button class="btn btn-secondary btn-sm" :title="t('admin.menuManagement.moveUp')" :aria-label="t('admin.menuManagement.moveUp')" :disabled="!canMoveItem(item.path, -1) || saving" @click="moveItem(item.path, -1)"><Icon name="arrowUp" size="sm" /></button>
                      <button class="btn btn-secondary btn-sm" :title="t('admin.menuManagement.moveDown')" :aria-label="t('admin.menuManagement.moveDown')" :disabled="!canMoveItem(item.path, 1) || saving" @click="moveItem(item.path, 1)"><Icon name="arrowDown" size="sm" /></button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="border-t border-gray-200 p-5 dark:border-dark-700">
            <h4 class="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-400">{{ t('admin.menuManagement.preview') }}</h4>
            <div class="flex flex-wrap gap-3">
              <div v-for="section in preview" :key="section.id" class="min-w-40 rounded-lg bg-gray-50 px-4 py-3 dark:bg-dark-900/50">
                <div class="sidebar-section-title mb-2 px-0">{{ section.title }}</div>
                <div v-for="item in section.items" :key="item.path" class="py-1 text-sm text-gray-700 dark:text-gray-300">{{ item.label }}</div>
              </div>
            </div>
          </div>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { getSettings } from '@/api/admin/settings'
import type { UserMenuSection } from '@/api/userMenu'
import type { CustomMenuItem } from '@/types'
import { useAppStore } from '@/stores/app'
import { useUserMenuStore } from '@/stores/userMenu'
import { assignUserMenuItem, groupUserMenu, USER_MENU_ITEMS } from '@/utils/userMenu'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const menuStore = useUserMenuStore()
const sections = ref<UserMenuSection[]>([])
const hiddenItems = ref<string[]>([])
const customItems = ref<CustomMenuItem[]>([])
const newTitle = ref('')
const loading = ref(false)
const loaded = ref(false)
const saving = ref(false)
const original = ref('')
const draft = computed(() => ({ sections: sections.value, hidden_items: hiddenItems.value }))
const dirty = computed(() => loaded.value && JSON.stringify(draft.value) !== original.value)

const catalog = computed(() => {
  const items: { path: string; label: string; custom: boolean }[] = USER_MENU_ITEMS.map(item => ({
    path: item.path, label: t(item.labelKey), custom: false,
  }))
  for (const item of [...customItems.value].filter(item => item.visibility === 'user').sort((a, b) => a.sort_order - b.sort_order)) {
    items.push({ path: `/custom/${item.id}`, label: item.label, custom: true })
  }
  // Retain removed custom pages in the editor so their stale assignments can be cleared.
  for (const path of [...sections.value.flatMap(section => section.items), ...hiddenItems.value]) {
    if (!items.some(item => item.path === path)) items.push({ path, label: path, custom: true })
  }
  return items
})
const preview = computed(() => groupUserMenu(catalog.value, sections.value, t('nav.myAccount'), hiddenItems.value))

function setItemVisible(path: string, visible: boolean) {
  hiddenItems.value = hiddenItems.value.filter(item => item !== path)
  if (!visible) hiddenItems.value.push(path)
}

async function load() {
  if (loading.value || saving.value) return
  loading.value = true
  loaded.value = false
  try {
    const [config, settings] = await Promise.all([menuStore.load(true, true), getSettings()])
    sections.value = config.sections.map(section => ({ ...section, items: [...section.items] }))
    hiddenItems.value = [...(config.hidden_items ?? [])]
    customItems.value = settings.custom_menu_items ?? []
    original.value = JSON.stringify(draft.value)
    loaded.value = true
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.menuManagement.loadFailed')))
  } finally {
    loading.value = false
  }
}

function addSection() {
  const title = newTitle.value.trim()
  if (!title || sections.value.length >= 32 || saving.value) return
  if (sections.value.some(section => section.title.trim() === title)) {
    appStore.showError(t('admin.menuManagement.invalidTitles'))
    return
  }
  sections.value.push({ id: crypto.randomUUID(), title, items: [] })
  newTitle.value = ''
}

function moveSection(index: number, direction: number) {
  const target = index + direction
  if (target < 0 || target >= sections.value.length) return
  const [section] = sections.value.splice(index, 1)
  sections.value.splice(target, 0, section)
}

function sectionFor(path: string) {
  return sections.value.find(section => section.items.includes(path))
}

function canMoveItem(path: string, direction: number) {
  const section = sectionFor(path)
  if (!section) return false
  const target = section.items.indexOf(path) + direction
  return target >= 0 && target < section.items.length
}

function moveItem(path: string, direction: number) {
  const section = sectionFor(path)
  if (!section || !canMoveItem(path, direction)) return
  const index = section.items.indexOf(path)
  section.items.splice(index, 1)
  section.items.splice(index + direction, 0, path)
}

async function save() {
  if (!loaded.value || !dirty.value || saving.value) return
  const titles = sections.value.map(section => section.title.trim())
  if (titles.some(title => !title || title.length > 80) || new Set(titles).size !== titles.length) {
    appStore.showError(t('admin.menuManagement.invalidTitles'))
    return
  }
  saving.value = true
  try {
    const config = await menuStore.save({ sections: sections.value.map(section => ({ ...section, title: section.title.trim(), items: [...section.items] })), hidden_items: [...hiddenItems.value] })
    sections.value = config.sections.map(section => ({ ...section, items: [...section.items] }))
    hiddenItems.value = [...(config.hidden_items ?? [])]
    original.value = JSON.stringify(draft.value)
    appStore.showSuccess(t('admin.menuManagement.saved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
