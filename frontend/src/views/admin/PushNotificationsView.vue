<template>
  <AppLayout>
    <div class="space-y-5">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-300">{{ t('admin.pushNotifications.stats.subscriptions') }}</p>
          <p class="mt-2 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ overview.activeSubscriptions }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-300">{{ t('admin.pushNotifications.stats.messages') }}</p>
          <p class="mt-2 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ overview.messageCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-300">{{ t('admin.pushNotifications.stats.lastSent') }}</p>
          <p class="mt-2 text-sm font-semibold text-gray-900 dark:text-white">
            {{ overview.lastSentAt ? formatDateTime(overview.lastSentAt) : t('common.notAvailable') }}
          </p>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-5 xl:grid-cols-[minmax(0,1fr)_22rem]">
        <form class="card p-5 sm:p-6" @submit.prevent="openConfirmation">
          <div class="flex items-center justify-between gap-3 border-b border-gray-100 pb-4 dark:border-dark-700">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.pushNotifications.compose.title') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">{{ t('admin.pushNotifications.compose.description') }}</p>
            </div>
            <Icon name="bell" size="lg" class="text-primary-500" />
          </div>

          <div class="mt-5 space-y-4">
            <div>
              <label class="input-label" for="push-title">{{ t('admin.pushNotifications.form.title') }}</label>
              <input id="push-title" v-model.trim="form.title" class="input" maxlength="100" required />
              <p class="input-hint text-right">{{ form.title.length }}/100</p>
            </div>
            <div>
              <label class="input-label" for="push-body">{{ t('admin.pushNotifications.form.body') }}</label>
              <textarea id="push-body" v-model.trim="form.body" class="input min-h-32 resize-y" maxlength="500" required></textarea>
              <p class="input-hint text-right">{{ form.body.length }}/500</p>
            </div>
            <div>
              <label class="input-label" for="push-url">{{ t('admin.pushNotifications.form.url') }}</label>
              <input id="push-url" v-model.trim="form.url" class="input" maxlength="2048" placeholder="/model-plaza" />
              <p class="input-hint">{{ t('admin.pushNotifications.form.urlHint') }}</p>
            </div>
            <div>
              <label class="input-label" for="push-image">{{ t('admin.pushNotifications.form.image') }}</label>
              <input
                id="push-image"
                v-model.trim="form.image"
                class="input"
                :class="imageError ? 'border-red-400 focus:border-red-500 focus:ring-red-500' : ''"
                maxlength="2048"
                placeholder="https://example.com/notification.jpg"
              />
              <p v-if="imageError" class="input-hint text-red-600 dark:text-red-400">{{ imageError }}</p>
              <p v-else class="input-hint">{{ t('admin.pushNotifications.form.imageHint') }}</p>
            </div>
          </div>

          <div class="mt-5 flex justify-end">
            <button type="submit" class="btn btn-primary" :disabled="sending || !form.title || !form.body || !!imageError">
              <Icon name="bell" size="sm" class="mr-1.5" />
              {{ sending ? t('admin.pushNotifications.sending') : t('admin.pushNotifications.send') }}
            </button>
          </div>
        </form>

        <div class="card self-start p-5">
          <p class="text-xs font-semibold uppercase text-gray-400 dark:text-dark-400">{{ t('admin.pushNotifications.preview') }}</p>
          <div class="mt-4 rounded-md border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-900">
            <div class="flex items-start gap-3">
              <div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-md bg-primary-600 text-white">
                <Icon name="bell" size="sm" />
              </div>
              <div class="min-w-0">
                <p class="break-words text-sm font-semibold text-gray-900 dark:text-white">
                  {{ form.title || t('admin.pushNotifications.previewTitle') }}
                </p>
                <p class="mt-1 whitespace-pre-wrap break-words text-sm leading-5 text-gray-600 dark:text-dark-200">
                  {{ form.body || t('admin.pushNotifications.previewBody') }}
                </p>
                <p class="mt-2 text-xs text-gray-400">Sub2API · {{ t('common.now') }}</p>
              </div>
            </div>
            <img
              v-if="showImagePreview"
              :src="form.image"
              :alt="form.title || t('admin.pushNotifications.previewTitle')"
              class="mt-4 aspect-[2/1] max-h-44 w-full rounded object-cover"
              referrerpolicy="no-referrer"
              @error="imagePreviewFailed = true"
            />
            <p v-else-if="form.image && imagePreviewFailed" class="mt-4 text-xs text-red-600 dark:text-red-400">
              {{ t('admin.pushNotifications.form.imageLoadFailed') }}
            </p>
          </div>
          <p class="mt-3 text-xs leading-5 text-gray-500 dark:text-dark-300">
            {{ t('admin.pushNotifications.previewHint') }}
          </p>
        </div>
      </div>

      <div class="card overflow-hidden">
        <div class="flex items-center justify-between border-b border-gray-200 px-5 py-4 dark:border-dark-700">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.pushNotifications.history') }}</h2>
          <div class="flex items-center gap-2">
            <button
              type="button"
              class="btn btn-ghost btn-icon text-red-600 hover:bg-red-50 disabled:opacity-50 dark:text-red-400 dark:hover:bg-red-900/20"
              :title="t('admin.pushNotifications.clear.action')"
              :aria-label="t('admin.pushNotifications.clear.action')"
              :disabled="loading || clearingMessages || !messages.length"
              @click="clearConfirmation = true"
            >
              <Icon
                :name="clearingMessages ? 'refresh' : 'trash'"
                size="sm"
                :class="clearingMessages ? 'animate-spin' : ''"
              />
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-icon"
              :title="t('common.refresh')"
              :disabled="loading"
              @click="loadData"
            >
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-5 py-3 text-left text-xs font-medium text-gray-500">{{ t('admin.pushNotifications.columns.message') }}</th>
                <th class="px-5 py-3 text-left text-xs font-medium text-gray-500">{{ t('admin.pushNotifications.columns.delivery') }}</th>
                <th class="px-5 py-3 text-left text-xs font-medium text-gray-500">{{ t('admin.pushNotifications.columns.sentAt') }}</th>
                <th class="w-16 px-5 py-3 text-right text-xs font-medium text-gray-500">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-800">
              <tr v-for="message in messages" :key="message.id">
                <td class="max-w-xl px-5 py-4">
                  <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ message.title }}</p>
                  <p class="mt-1 truncate text-xs text-gray-500 dark:text-dark-300">{{ message.body }}</p>
                </td>
                <td class="whitespace-nowrap px-5 py-4 text-sm tabular-nums text-gray-600 dark:text-dark-200">
                  <span class="text-emerald-600 dark:text-emerald-400">{{ message.delivered }}</span>
                  <span class="mx-1 text-gray-300">/</span>
                  <span :class="message.failed ? 'text-red-600 dark:text-red-400' : 'text-gray-500'">{{ message.failed }}</span>
                </td>
                <td class="whitespace-nowrap px-5 py-4 text-sm text-gray-500 dark:text-dark-300">{{ formatDateTime(message.createdAt) }}</td>
                <td class="whitespace-nowrap px-5 py-4 text-right">
                  <button
                    type="button"
                    class="btn btn-ghost btn-icon text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
                    :title="t('admin.pushNotifications.delete.action')"
                    :aria-label="t('admin.pushNotifications.delete.action')"
                    :disabled="deletingMessageId === message.id"
                    @click="deletingMessage = message"
                  >
                    <Icon :name="deletingMessageId === message.id ? 'refresh' : 'trash'" size="sm" :class="deletingMessageId === message.id ? 'animate-spin' : ''" />
                  </button>
                </td>
              </tr>
              <tr v-if="!messages.length && !loading">
                <td colspan="4" class="px-5 py-10 text-center text-sm text-gray-500">{{ t('admin.pushNotifications.empty') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <ConfirmDialog
      :show="showConfirm"
      :title="t('admin.pushNotifications.confirmTitle')"
      :message="t('admin.pushNotifications.confirmMessage', { count: overview.activeSubscriptions })"
      :confirm-text="t('admin.pushNotifications.send')"
      @confirm="sendNotification"
      @cancel="showConfirm = false"
    />
    <ConfirmDialog
      :show="!!deletingMessage"
      :title="t('admin.pushNotifications.delete.title')"
      :message="t('admin.pushNotifications.delete.message', { title: deletingMessage?.title || '' })"
      :confirm-text="deletingMessageId ? t('common.processing') : t('common.delete')"
      :danger="true"
      @confirm="confirmDeleteMessage"
      @cancel="deletingMessage = null"
    />
    <ConfirmDialog
      :show="clearConfirmation"
      :title="t('admin.pushNotifications.clear.title')"
      :message="t('admin.pushNotifications.clear.message', { count: messages.length })"
      :confirm-text="clearingMessages ? t('common.processing') : t('admin.pushNotifications.clear.action')"
      :danger="true"
      @confirm="confirmClearMessages"
      @cancel="clearConfirmation = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import {
  broadcastPush,
  clearPushMessages,
  deletePushMessage,
  getPushOverview,
  listPushMessages,
  type PushMessage,
  type PushOverview,
} from '@/api/pushNotifications'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const sending = ref(false)
const showConfirm = ref(false)
const deletingMessage = ref<PushMessage | null>(null)
const deletingMessageId = ref('')
const clearConfirmation = ref(false)
const clearingMessages = ref(false)
const imagePreviewFailed = ref(false)
const overview = reactive<PushOverview>({ activeSubscriptions: 0, messageCount: 0, lastSentAt: null })
const messages = ref<PushMessage[]>([])
const form = reactive({ title: '', body: '', url: '/', image: '' })

const imageError = computed(() => {
  if (!form.image) return ''
  try {
    const parsed = new URL(form.image)
    if (parsed.protocol === 'http:' || parsed.protocol === 'https:') return ''
  } catch {
    // Use the same message for malformed and unsupported URLs.
  }
  return t('admin.pushNotifications.form.imageInvalid')
})
const showImagePreview = computed(() => !!form.image && !imageError.value && !imagePreviewFailed.value)

watch(() => form.image, () => {
  imagePreviewFailed.value = false
})

function openConfirmation() {
  if (imageError.value) return
  showConfirm.value = true
}

async function loadData() {
  loading.value = true
  try {
    const [summary, history] = await Promise.all([getPushOverview(), listPushMessages()])
    Object.assign(overview, summary)
    messages.value = history
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.pushNotifications.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function sendNotification() {
  showConfirm.value = false
  sending.value = true
  try {
    const message = await broadcastPush({
      title: form.title,
      body: form.body,
      url: form.url || '/',
      image: form.image,
    })
    messages.value.unshift(message)
    overview.messageCount += 1
    overview.lastSentAt = message.createdAt
    form.title = ''
    form.body = ''
    form.url = '/'
    form.image = ''
    appStore.showSuccess(t('admin.pushNotifications.sentResult', {
      delivered: message.delivered,
      failed: message.failed,
    }))
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.pushNotifications.sendFailed'))
  } finally {
    sending.value = false
  }
}

async function confirmDeleteMessage() {
  if (!deletingMessage.value || deletingMessageId.value) return
  const message = deletingMessage.value
  deletingMessageId.value = message.id
  try {
    const result = await deletePushMessage(message.id)
    messages.value = messages.value.filter((item) => item.id !== message.id)
    Object.assign(overview, result.overview)
    deletingMessage.value = null
    appStore.showSuccess(t('admin.pushNotifications.delete.success'))
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.pushNotifications.delete.failed'))
  } finally {
    deletingMessageId.value = ''
  }
}

async function confirmClearMessages() {
  if (clearingMessages.value || !messages.value.length) return
  clearingMessages.value = true
  try {
    const result = await clearPushMessages()
    messages.value = []
    Object.assign(overview, result.overview)
    clearConfirmation.value = false
    appStore.showSuccess(t('admin.pushNotifications.clear.success'))
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.pushNotifications.clear.failed'))
  } finally {
    clearingMessages.value = false
  }
}

onMounted(loadData)
</script>
