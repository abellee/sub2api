<template>
  <section class="space-y-5">
    <p class="text-sm text-gray-500 dark:text-dark-300">{{ t(`${prefix}.description`) }}</p>
    <p v-if="loading" role="status">{{ t('common.loading') }}</p>
    <button v-else-if="!loaded" class="btn btn-secondary" type="button" @click="load">{{ t(`${prefix}.reload`) }}</button>
    <div v-else class="grid gap-5 xl:grid-cols-2">
      <form class="card space-y-4 p-5" @submit.prevent="save">
        <h2 class="font-semibold">{{ t(`${prefix}.settings`) }}</h2>
        <fieldset class="space-y-4" :disabled="saving || sending">
          <label class="flex items-center gap-2 text-sm">
            <input id="group-enabled" v-model="config.enabled" type="checkbox" />{{ t(`${prefix}.enabled`) }}
          </label>
          <div>
            <label class="input-label" for="group-base-url">{{ t(`${prefix}.baseURL`) }}</label>
            <input id="group-base-url" v-model.trim="config.baseURL" class="input" type="url" :required="config.enabled" placeholder="http://astrbot:6185" />
            <p class="input-hint">{{ t(`${prefix}.baseURLHint`) }}</p>
          </div>
          <div>
            <label class="input-label" for="group-api-key">{{ t(`${prefix}.apiKey`) }}</label>
            <input id="group-api-key" v-model.trim="apiKey" class="input" type="password" autocomplete="new-password" maxlength="4096" :required="config.enabled && !config.keyConfigured" />
            <p class="input-hint">{{ t(`${prefix}.${config.keyConfigured ? 'keyStored' : 'keyHint'}`) }}</p>
          </div>
          <div>
            <label class="input-label" for="group-platform">{{ t(`${prefix}.platformID`) }}</label>
            <input id="group-platform" v-model.trim="config.platformID" class="input" required pattern="[\w-]{1,80}" />
          </div>
          <div>
            <label class="input-label" for="group-ids">{{ t(`${prefix}.groups`) }}</label>
            <textarea id="group-ids" v-model="groupText" class="input min-h-24" :required="config.enabled" placeholder="764163343"></textarea>
            <p class="input-hint">{{ t(`${prefix}.groupsHint`) }}</p>
          </div>
          <div>
            <label class="input-label" for="group-config-id">{{ t(`${prefix}.configID`) }}</label>
            <input id="group-config-id" v-model.trim="config.configID" class="input" required pattern="[\w-]{1,80}" />
            <p class="input-hint">{{ t(`${prefix}.configIDHint`) }}</p>
          </div>
          <button class="btn btn-primary" type="submit">{{ saving ? t('common.loading') : t('common.save') }}</button>
        </fieldset>
      </form>
      <form class="card space-y-4 p-5" @submit.prevent="openConfirmation">
        <h2 class="font-semibold">{{ t(`${prefix}.compose`) }}</h2>
        <p class="input-hint">{{ t(`${prefix}.composeHint`) }}</p>
        <p class="text-sm">{{ t(`${prefix}.recipients`, { groups: saved?.groupIDs.join('、') || '—' }) }}</p>
        <p v-if="dirty" class="input-hint">{{ t(`${prefix}.saveFirst`) }}</p>
        <fieldset class="space-y-4" :disabled="sending">
          <div>
            <label class="input-label" for="group-title">{{ t('admin.pushNotifications.form.title') }}</label>
            <input id="group-title" v-model.trim="message.title" class="input" maxlength="100" required />
          </div>
          <div>
            <label class="input-label" for="group-body">{{ t('admin.pushNotifications.form.body') }}</label>
            <textarea id="group-body" v-model.trim="message.body" class="input min-h-32" maxlength="500" required></textarea>
          </div>
          <button class="btn btn-primary" type="submit" :disabled="!canSend">{{ sending ? t('admin.pushNotifications.sending') : t(`${prefix}.send`) }}</button>
        </fieldset>
      </form>
    </div>
    <div v-if="loaded" class="card space-y-4 p-5">
      <div class="flex items-center justify-between gap-3">
        <h2 class="font-semibold">{{ t('admin.pushNotifications.history') }}</h2>
        <button type="button" class="btn btn-secondary" :disabled="refreshing" @click="refreshHistory">{{ t(`${prefix}.reload`) }}</button>
      </div>
      <p class="input-hint">{{ t(`${prefix}.uncertainHint`) }}</p>
      <p v-if="!records.length" class="text-sm text-gray-500">{{ t('admin.pushNotifications.empty') }}</p>
      <ul v-else class="divide-y divide-gray-100 dark:divide-dark-700">
        <li v-for="record in records" :key="record.key" class="space-y-1 py-3 text-sm">
          <div class="flex flex-wrap justify-between gap-2">
            <span>{{ record.title }} · {{ record.groupID }}</span>
            <span :class="record.status === 'sent' ? 'text-green-600' : 'text-amber-600'">{{ t(`${prefix}.status.${record.status}`) }}</span>
          </div>
          <p class="whitespace-pre-wrap text-gray-500">{{ record.body }}</p>
          <p class="text-xs text-gray-400">{{ formatDateTime(record.updatedAt) }}</p>
        </li>
      </ul>
    </div>
    <ConfirmDialog :show="showConfirm" :title="t(`${prefix}.confirmTitle`)" :message="t(`${prefix}.confirm`, { groups: saved?.groupIDs.join('、') })" @cancel="showConfirm = false" @confirm="send">
      <p class="whitespace-pre-wrap break-words text-sm">{{ message.title }}<br />{{ message.body }}</p>
    </ConfirmDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import { getGroupNotificationConfig, saveGroupNotificationConfig, sendGroupNotification, listGroupNotifications, type GroupNotificationConfig, type GroupNotificationRecord } from '@/api/pushNotifications'

const prefix = 'admin.pushNotifications.group'
const { t } = useI18n()
const appStore = useAppStore()
const config = reactive<GroupNotificationConfig>({ enabled: false, baseURL: '', platformID: 'napcat', groupIDs: [], configID: 'default', keyConfigured: false })
const saved = ref<GroupNotificationConfig | null>(null)
const apiKey = ref('')
const groupText = ref('')
const loading = ref(false)
const loaded = ref(false)
const saving = ref(false)
const sending = ref(false)
const refreshing = ref(false)
const showConfirm = ref(false)
const records = ref<GroupNotificationRecord[]>([])
const message = reactive({ title: '', body: '' })
const requestID = ref(crypto.randomUUID())
const groupIDs = computed(() => [...new Set(groupText.value.split(/[\s,，;；]+/).filter(Boolean))])
const dirty = computed(() => Boolean(apiKey.value) || JSON.stringify({ ...config, groupIDs: groupIDs.value }) !== JSON.stringify(saved.value))
const canSend = computed(() => loaded.value && saved.value?.enabled && !dirty.value && !saving.value && !sending.value && Boolean(message.title && message.body))
watch(message, () => { requestID.value = crypto.randomUUID() })

function applyConfig(value: GroupNotificationConfig) {
  Object.assign(config, value)
  saved.value = { ...value, groupIDs: [...value.groupIDs] }
  groupText.value = value.groupIDs.join('\n')
  apiKey.value = ''
}

async function load() {
  loading.value = true
  try {
    applyConfig(await getGroupNotificationConfig())
    loaded.value = true
    await refreshHistory()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.pushNotifications.loadFailed'))
  } finally { loading.value = false }
}

async function refreshHistory() {
  refreshing.value = true
  try { records.value = await listGroupNotifications() }
  catch (error: any) { appStore.showError(error?.message || t('admin.pushNotifications.loadFailed')) }
  finally { refreshing.value = false }
}

async function save() {
  if (saving.value || sending.value) return
  if (groupIDs.value.length > 20 || groupIDs.value.some(id => !/^\d{5,12}$/.test(id))) {
    appStore.showError(t(`${prefix}.invalidGroups`))
    return
  }
  saving.value = true
  try {
    applyConfig(await saveGroupNotificationConfig({ ...config, groupIDs: groupIDs.value, ...(apiKey.value ? { apiKey: apiKey.value } : {}) }))
    appStore.showSuccess(t(`${prefix}.saved`))
  } catch (error: any) { appStore.showError(error?.message || t(`${prefix}.saveFailed`)) }
  finally { saving.value = false }
}

function openConfirmation() {
  if (canSend.value) showConfirm.value = true
}

async function send() {
  if (!canSend.value || !showConfirm.value) return
  showConfirm.value = false
  sending.value = true
  try {
    const result = await sendGroupNotification({ ...message, requestID: requestID.value })
    appStore.showSuccess(t('admin.pushNotifications.sentResult', result.group))
    message.title = ''
    message.body = ''
  } catch (error: any) { appStore.showError(error?.message || t('admin.pushNotifications.sendFailed')) }
  finally {
    sending.value = false
    await refreshHistory()
  }
}

onMounted(load)
</script>
