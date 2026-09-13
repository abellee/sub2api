<template>
  <AppLayout>
    <div class="space-y-6" data-testid="app-catalog-page">
      <section
        class="flex flex-col gap-4 border-b border-gray-200 pb-5 dark:border-dark-700 sm:flex-row sm:items-end sm:justify-between"
      >
        <div class="min-w-0">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.appCatalog.title') }}
          </h2>
          <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.appCatalog.description') }}
          </p>
        </div>
        <div class="flex flex-shrink-0 items-center gap-2">
          <button
            type="button"
            class="btn btn-primary"
            :disabled="!serviceEnabled || loading"
            data-testid="add-app-button"
            @click="openAddDialog"
          >
            <Icon name="plus" size="sm" />
            {{ t('admin.appCatalog.add') }}
          </button>
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="loading"
            :title="t('common.refresh')"
            @click="reload"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            <span class="sr-only">{{ t('common.refresh') }}</span>
          </button>
        </div>
      </section>

      <div
        v-if="!serviceEnabled"
        class="border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-100"
        data-testid="service-down"
      >
        {{ t('admin.appCatalog.serviceDown') }}
      </div>

      <div
        v-if="loading"
        class="flex min-h-48 items-center justify-center text-sm text-gray-500"
      >
        {{ t('common.loading') }}
      </div>

      <div
        v-else-if="serviceEnabled && apps.length === 0"
        class="flex min-h-56 flex-col items-center justify-center border border-dashed border-gray-300 px-6 text-center dark:border-dark-600"
        data-testid="empty-state"
      >
        <Icon name="cube" size="xl" class="text-gray-400" />
        <p class="mt-3 font-medium text-gray-800 dark:text-gray-200">
          {{ t('admin.appCatalog.empty') }}
        </p>
        <p class="mt-1 max-w-lg text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.appCatalog.emptyHint') }}
        </p>
        <button
          type="button"
          class="btn btn-primary mt-4"
          data-testid="empty-add-app-button"
          @click="openAddDialog"
        >
          <Icon name="plus" size="sm" />
          {{ t('admin.appCatalog.add') }}
        </button>
      </div>

      <div v-else-if="serviceEnabled" class="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <CatalogAppCard
          v-for="app in apps"
          :key="app.id"
          :app="app"
          @details="openDetail(app)"
        >
          <template #actions>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="formBusy"
              data-testid="edit-app-button"
              @click="editApp(app)"
            >
              {{ t('admin.appCatalog.edit') }}
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="deletingId === app.id"
              @click="removeApp(app)"
            >
              {{ t('admin.appCatalog.delete') }}
            </button>
          </template>
        </CatalogAppCard>
      </div>
    </div>

    <BaseDialog
      :show="formOpen"
      :title="editingId ? t('admin.appCatalog.editTitle') : t('admin.appCatalog.add')"
      width="wide"
      data-testid="add-app-dialog"
      @close="closeFormDialog"
    >
      <form id="add-app-form" class="grid grid-cols-1 gap-4 lg:grid-cols-2" data-testid="add-app-form" @submit.prevent="submit">
        <div>
          <label class="input-label" for="app-official-url">{{ t('admin.appCatalog.officialUrl') }}</label>
          <input
            id="app-official-url"
            v-model="form.officialUrl"
            type="url"
            class="input"
            placeholder="https://example.com"
            :disabled="formBusy"
          />
        </div>
        <div>
          <label class="input-label" for="app-github-url">{{ t('admin.appCatalog.githubUrl') }}</label>
          <input
            id="app-github-url"
            v-model="form.githubUrl"
            type="url"
            class="input"
            placeholder="https://github.com/owner/repo"
            :disabled="formBusy"
          />
        </div>
        <div>
          <label class="input-label" for="app-name">{{ t('admin.appCatalog.appName') }}</label>
          <input
            id="app-name"
            v-model="form.name"
            type="text"
            class="input"
            :disabled="formBusy"
            data-testid="app-name-input"
          />
        </div>
        <div>
          <label class="input-label" for="app-version">{{ t('admin.appCatalog.version') }}</label>
          <input
            id="app-version"
            v-model="form.version"
            type="text"
            class="input"
            :disabled="formBusy"
          />
        </div>
        <div>
          <label class="input-label" for="app-icon-url">{{ t('admin.appCatalog.iconUrl') }}</label>
          <div class="flex items-center gap-3">
            <input
              id="app-icon-url"
              v-model="form.iconUrl"
              type="url"
              class="input"
              :disabled="formBusy"
              data-testid="app-icon-input"
            />
            <img
              v-if="form.iconUrl"
              :src="form.iconUrl"
              alt=""
              class="h-10 w-10 flex-shrink-0 rounded-md bg-white object-contain"
              referrerpolicy="no-referrer"
            />
          </div>
        </div>
        <div>
          <label class="input-label" for="app-download-url">{{ t('admin.appCatalog.downloadPageUrl') }}</label>
          <input
            id="app-download-url"
            v-model="form.downloadPageUrl"
            type="url"
            class="input"
            :disabled="formBusy"
          />
        </div>
        <div class="lg:col-span-2">
          <label class="input-label" for="app-intro">{{ t('admin.appCatalog.intro') }}</label>
          <textarea
            id="app-intro"
            v-model="form.intro"
            class="input min-h-24"
            rows="3"
            :disabled="formBusy"
            data-testid="app-intro-input"
          />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.appCatalog.introHint') }}
          </p>
        </div>
        <div class="lg:col-span-2">
          <label class="input-label" for="app-screenshots">{{ t('admin.appCatalog.screenshots') }}</label>
          <textarea
            id="app-screenshots"
            v-model="form.screenshotsText"
            class="input min-h-24 font-mono text-xs"
            rows="4"
            :disabled="formBusy"
            data-testid="app-screenshots-input"
          />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.appCatalog.screenshotsHint') }}
          </p>
        </div>
        <div class="lg:col-span-2">
          <label class="input-label" for="app-changelog">{{ t('admin.appCatalog.changelog') }}</label>
          <textarea
            id="app-changelog"
            v-model="form.changelog"
            class="input min-h-32 font-mono text-xs"
            rows="8"
            :disabled="formBusy"
            data-testid="app-changelog-input"
          />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.appCatalog.changelogHint') }}
          </p>
        </div>
        <p class="text-xs text-gray-500 dark:text-gray-400 lg:col-span-2">
          {{ t('admin.appCatalog.urlHint') }}
        </p>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="formBusy" @click="closeFormDialog">
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="formBusy"
          data-testid="fetch-fill-button"
          @click="fetchFill"
        >
          {{ fetching ? t('admin.appCatalog.fetching') : t('admin.appCatalog.fetchFill') }}
        </button>
        <button
          type="submit"
          form="add-app-form"
          class="btn btn-primary"
          :disabled="formBusy"
          data-testid="save-app-button"
        >
          {{ submitting ? t('common.submitting') : t('admin.appCatalog.submit') }}
        </button>
      </template>
    </BaseDialog>

    <CatalogAppDetailsDialog
      :app="detailApp"
      :show-edit="!formBusy"
      @close="closeDetail"
      @edit="editFromDetail"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type CatalogApp, type CatalogAppDraft } from '@/api/admin'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import CatalogAppCard from '@/components/app-center/CatalogAppCard.vue'
import CatalogAppDetailsDialog from '@/components/app-center/CatalogAppDetailsDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const submitting = ref(false)
const fetching = ref(false)
const deletingId = ref<number | null>(null)
const editingId = ref<number | null>(null)
const formOpen = ref(false)
const serviceEnabled = ref(true)
const apps = ref<CatalogApp[]>([])
const detailApp = ref<CatalogApp | null>(null)
const form = reactive({
  officialUrl: '',
  githubUrl: '',
  name: '',
  iconUrl: '',
  intro: '',
  downloadPageUrl: '',
  screenshotsText: '',
  changelog: '',
  version: '',
})

const formBusy = computed(() => !serviceEnabled.value || submitting.value || fetching.value)

function openDetail(app: CatalogApp): void {
  detailApp.value = app
}

function closeDetail(): void {
  detailApp.value = null
}

function editFromDetail(): void {
  const app = detailApp.value
  if (!app) return
  closeDetail()
  editApp(app)
}

function parseScreenshotLines(text: string): string[] {
  return text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
}

function resetForm(): void {
  editingId.value = null
  form.officialUrl = ''
  form.githubUrl = ''
  form.name = ''
  form.iconUrl = ''
  form.intro = ''
  form.downloadPageUrl = ''
  form.screenshotsText = ''
  form.changelog = ''
  form.version = ''
}

function openAddDialog(): void {
  resetForm()
  formOpen.value = true
}

function closeFormDialog(): void {
  formOpen.value = false
  resetForm()
}

function buildPayload(): CatalogAppDraft {
  return {
    official_url: form.officialUrl.trim(),
    github_url: form.githubUrl.trim(),
    name: form.name.trim(),
    icon_url: form.iconUrl.trim(),
    description: form.intro.trim(),
    download_page_url: form.downloadPageUrl.trim(),
    screenshots: parseScreenshotLines(form.screenshotsText),
    changelog: form.changelog,
    version: form.version.trim(),
  }
}

function applyFetchResult(result: CatalogAppDraft): void {
  if (result.official_url && !form.officialUrl.trim()) form.officialUrl = result.official_url
  if (result.github_url && !form.githubUrl.trim()) form.githubUrl = result.github_url
  if (result.name) form.name = result.name
  if (result.icon_url) form.iconUrl = result.icon_url
  if (result.description) form.intro = result.description
  if (result.download_page_url) form.downloadPageUrl = result.download_page_url
  if (result.screenshots?.length) form.screenshotsText = result.screenshots.join('\n')
  if (result.changelog) form.changelog = result.changelog
  if (result.version) form.version = result.version
}

function editApp(app: CatalogApp): void {
  editingId.value = app.id
  form.officialUrl = app.official_url || ''
  form.githubUrl = app.github_url || ''
  form.name = app.name || ''
  form.iconUrl = app.icon_url || ''
  form.intro = app.description || ''
  form.downloadPageUrl = app.download_page_url || ''
  form.screenshotsText = (app.screenshots || []).join('\n')
  form.changelog = app.changelog || ''
  form.version = app.version || ''
  formOpen.value = true
}

async function reload(): Promise<void> {
  loading.value = true
  try {
    const health = await adminAPI.appCatalog.getHealth()
    serviceEnabled.value = Boolean(health.enabled)
    if (!health.enabled) {
      apps.value = []
      return
    }
    const list = await adminAPI.appCatalog.list()
    apps.value = list.items || []
  } catch (error: unknown) {
    serviceEnabled.value = false
    apps.value = []
    appStore.showError(extractApiErrorMessage(error, t('admin.appCatalog.serviceDown')))
  } finally {
    loading.value = false
  }
}

async function fetchFill(): Promise<void> {
  const official = form.officialUrl.trim()
  const github = form.githubUrl.trim()
  if (!official && !github) {
    appStore.showError(t('admin.appCatalog.requiredUrl'))
    return
  }
  fetching.value = true
  try {
    const result = await adminAPI.appCatalog.fetchPreview({
      official_url: official,
      github_url: github,
    })
    applyFetchResult(result)
    appStore.showSuccess(t('admin.appCatalog.fetchFilled'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    fetching.value = false
  }
}

async function submit(): Promise<void> {
  const payload = buildPayload()
  if (!payload.official_url && !payload.github_url) {
    appStore.showError(t('admin.appCatalog.requiredUrl'))
    return
  }
  submitting.value = true
  try {
    if (editingId.value) {
      await adminAPI.appCatalog.update(editingId.value, payload)
      appStore.showSuccess(t('admin.appCatalog.updated'))
    } else {
      await adminAPI.appCatalog.create(payload)
      appStore.showSuccess(t('admin.appCatalog.added'))
    }
    closeFormDialog()
    await reload()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    submitting.value = false
  }
}

async function removeApp(app: CatalogApp): Promise<void> {
  if (!window.confirm(t('admin.appCatalog.deleteConfirm'))) return
  deletingId.value = app.id
  try {
    await adminAPI.appCatalog.remove(app.id)
    if (editingId.value === app.id) closeFormDialog()
    if (detailApp.value?.id === app.id) closeDetail()
    appStore.showSuccess(t('common.deleted'))
    await reload()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    deletingId.value = null
  }
}

onMounted(reload)
</script>
