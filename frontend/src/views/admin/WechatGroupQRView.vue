<template>
  <AppLayout>
    <div class="grid grid-cols-1 gap-5 xl:grid-cols-[minmax(0,1fr)_22rem]">
      <section class="card p-5 sm:p-6">
        <div class="flex items-center justify-between gap-3 border-b border-gray-100 pb-4 dark:border-dark-700">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('admin.wechatGroupQR.upload.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">
              {{ t('admin.wechatGroupQR.upload.description') }}
            </p>
          </div>
          <Icon name="upload" size="lg" class="text-[#07C160]" />
        </div>

        <div class="mt-5">
          <label
            for="wechat-qr-file"
            class="flex min-h-40 cursor-pointer flex-col items-center justify-center rounded-md border border-dashed border-gray-300 bg-gray-50 px-5 py-8 text-center transition-colors hover:border-[#07C160] hover:bg-emerald-50/40 dark:border-dark-600 dark:bg-dark-900 dark:hover:border-[#07C160] dark:hover:bg-emerald-950/10"
          >
            <Icon name="upload" size="xl" class="text-gray-400" />
            <span class="mt-3 text-sm font-medium text-gray-800 dark:text-dark-100">
              {{ selectedFile?.name || t('admin.wechatGroupQR.upload.select') }}
            </span>
            <span class="mt-1 text-xs text-gray-500 dark:text-dark-300">
              {{ t('admin.wechatGroupQR.upload.limit') }}
            </span>
          </label>
          <input
            id="wechat-qr-file"
            ref="fileInput"
            type="file"
            accept="image/png,image/jpeg"
            class="sr-only"
            @change="handleFileChange"
          />
        </div>

        <div v-if="sourcePreviewURL" class="mt-5 border-t border-gray-100 pt-5 dark:border-dark-700">
          <p class="text-sm font-medium text-gray-700 dark:text-dark-200">
            {{ t('admin.wechatGroupQR.upload.sourcePreview') }}
          </p>
          <div class="mt-3 flex max-h-[28rem] justify-center overflow-hidden rounded-md bg-gray-100 p-3 dark:bg-dark-900">
            <img
              :src="sourcePreviewURL"
              :alt="t('admin.wechatGroupQR.upload.sourcePreview')"
              class="max-h-[26rem] max-w-full object-contain"
            />
          </div>
        </div>

        <div class="mt-5">
          <label class="input-label" for="wechat-qr-expires-at">
            {{ t('admin.wechatGroupQR.upload.expiresAt') }}
          </label>
          <input
            id="wechat-qr-expires-at"
            v-model="expiresAt"
            type="datetime-local"
            class="input"
            :min="minimumExpiry"
          />
          <p class="input-hint">{{ t('admin.wechatGroupQR.upload.expiresAtHint') }}</p>
        </div>

        <div class="mt-5 flex justify-end">
          <button
            type="button"
            class="btn btn-primary"
            :disabled="!selectedFile || saving"
            @click="upload"
          >
            <Icon
              :name="saving ? 'refresh' : 'upload'"
              size="sm"
              class="mr-1.5"
              :class="saving ? 'animate-spin' : ''"
            />
            {{ saving ? t('admin.wechatGroupQR.upload.saving') : t('admin.wechatGroupQR.upload.action') }}
          </button>
        </div>
      </section>

      <aside class="card self-start p-5">
        <div class="flex items-center justify-between gap-3">
          <p class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.wechatGroupQR.current.title') }}
          </p>
          <span class="h-2.5 w-2.5 rounded-full bg-[#07C160]"></span>
        </div>
        <div class="mt-4 rounded-md bg-gray-100 p-3 dark:bg-dark-900">
          <img
            v-if="current.image_url"
            :src="current.image_url"
            :alt="t('admin.wechatGroupQR.current.title')"
            class="mx-auto aspect-square w-full max-w-72 rounded-md bg-white object-contain"
          />
          <div v-else class="flex aspect-square items-center justify-center text-sm text-gray-400">
            {{ t('common.loading') }}
          </div>
        </div>
        <dl class="mt-4 space-y-3 text-sm">
          <div class="flex items-center justify-between gap-3">
            <dt class="text-gray-500 dark:text-dark-300">{{ t('admin.wechatGroupQR.current.source') }}</dt>
            <dd class="font-medium text-gray-800 dark:text-dark-100">
              {{
                current.custom
                  ? t('admin.wechatGroupQR.current.uploaded')
                  : t('admin.wechatGroupQR.current.builtin')
              }}
            </dd>
          </div>
          <div class="flex items-center justify-between gap-3">
            <dt class="text-gray-500 dark:text-dark-300">{{ t('admin.wechatGroupQR.current.expiresAt') }}</dt>
            <dd class="text-right font-medium text-gray-800 dark:text-dark-100">
              <span v-if="current.expires_at">{{ formatDateTime(current.expires_at) }}</span>
              <span v-else>{{ t('admin.wechatGroupQR.current.never') }}</span>
              <span v-if="current.expired" class="ml-1 text-red-600 dark:text-red-400">
                ({{ t('admin.wechatGroupQR.current.expired') }})
              </span>
            </dd>
          </div>
          <div class="flex items-center justify-between gap-3">
            <dt class="text-gray-500 dark:text-dark-300">{{ t('admin.wechatGroupQR.current.updatedAt') }}</dt>
            <dd class="text-right font-medium text-gray-800 dark:text-dark-100">
              {{ current.updated_at ? formatDateTime(current.updated_at) : t('common.notAvailable') }}
            </dd>
          </div>
        </dl>
      </aside>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import {
  getWechatGroupQR,
  uploadWechatGroupQR,
  WECHAT_GROUP_QR_UPDATED_EVENT,
  type WechatGroupQRInfo
} from '@/api/wechatGroupQR'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const fileInput = ref<HTMLInputElement | null>(null)
const selectedFile = ref<File | null>(null)
const sourcePreviewURL = ref('')
const expiresAt = ref('')
const saving = ref(false)
const current = reactive<WechatGroupQRInfo>({
  image_url: '/llmfree/wechat-group-qr.png',
  custom: false
})

const minimumExpiry = new Date(Date.now() + 60_000).toISOString().slice(0, 16)

function applyCurrent(info: WechatGroupQRInfo) {
  current.image_url = info.image_url
  current.updated_at = info.updated_at
  current.expires_at = info.expires_at
  current.custom = info.custom
  current.expired = info.expired
}

function clearSourcePreview() {
  if (sourcePreviewURL.value) URL.revokeObjectURL(sourcePreviewURL.value)
  sourcePreviewURL.value = ''
}

function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0] || null
  if (!file) return
  if (!['image/png', 'image/jpeg'].includes(file.type)) {
    selectedFile.value = null
    clearSourcePreview()
    appStore.showError(t('admin.wechatGroupQR.errors.type'))
    input.value = ''
    return
  }
  if (file.size > 12 * 1024 * 1024) {
    selectedFile.value = null
    clearSourcePreview()
    appStore.showError(t('admin.wechatGroupQR.errors.size'))
    input.value = ''
    return
  }
  clearSourcePreview()
  selectedFile.value = file
  sourcePreviewURL.value = URL.createObjectURL(file)
}

async function loadCurrent() {
  try {
    applyCurrent(await getWechatGroupQR())
  } catch (error) {
    console.error('Failed to load WeChat group QR code:', error)
    appStore.showError(t('admin.wechatGroupQR.errors.load'))
  }
}

async function upload() {
  if (!selectedFile.value || saving.value) return
  saving.value = true
  try {
    const expiryISO = expiresAt.value ? new Date(expiresAt.value).toISOString() : undefined
    const uploadedInfo = await uploadWechatGroupQR(selectedFile.value, expiryISO)
    // Keep the preview accurate while an older local preview backend is still running.
    const info: WechatGroupQRInfo = {
      ...uploadedInfo,
      expires_at: uploadedInfo.expires_at || expiryISO,
    }
    applyCurrent(info)
    window.dispatchEvent(new CustomEvent<WechatGroupQRInfo>(WECHAT_GROUP_QR_UPDATED_EVENT, { detail: info }))
    selectedFile.value = null
    expiresAt.value = ''
    clearSourcePreview()
    if (fileInput.value) {
      fileInput.value.value = ''
    }
    appStore.showSuccess(t('admin.wechatGroupQR.success'))
  } catch (error) {
    const message = (error as { message?: string })?.message
    appStore.showError(message || t('admin.wechatGroupQR.errors.upload'))
  } finally {
    saving.value = false
  }
}

onMounted(loadCurrent)
onBeforeUnmount(clearSourcePreview)
</script>
