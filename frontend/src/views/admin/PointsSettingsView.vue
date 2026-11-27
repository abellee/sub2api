<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores'
import { fetchPointsSettings, savePointsSettings } from '@/api/points'
import { refreshDailyCheckIn } from '@/composables/useDailyCheckIn'
import PointsProductsPanel from '@/components/admin/points/PointsProductsPanel.vue'

const { t, locale } = useI18n()
const appStore = useAppStore()
const reward = ref(10)
const loaded = ref(false)
const loading = ref(false)
const saving = ref(false)
const loadError = ref(false)
const activeTab = ref<'checkin' | 'products'>('checkin')
const tabs = computed(() => [
  { value: 'checkin' as const, label: t('common.checkIn.title') },
  { value: 'products' as const, label: t('common.pointsCatalog.products') }
])
const valid = computed(() => Number.isInteger(reward.value) && reward.value >= 1 && reward.value <= 1000000)
const format = (value: number) => value.toLocaleString(locale.value)
async function load() {
  loading.value = true
  loadError.value = false
  try { reward.value = (await fetchPointsSettings()).daily_checkin_points; loaded.value = true }
  catch { loadError.value = true; loaded.value = false }
  finally { loading.value = false }
}
async function save() {
  if (!valid.value || !loaded.value || saving.value) return
  saving.value = true
  try {
    await savePointsSettings({ daily_checkin_points: reward.value })
    void refreshDailyCheckIn(true)
    appStore.showSuccess(t('common.saved'))
  } catch { appStore.showError(t('common.pointsAdmin.saveError')) }
  finally { saving.value = false }
}
onMounted(load)
</script>

<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-4xl space-y-6">
      <header><h1 class="text-xl font-semibold text-gray-900 dark:text-gray-100">{{ t('nav.pointsAdmin') }}</h1><p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('common.pointsAdmin.description') }}</p></header>
      <nav class="flex flex-wrap gap-2" :aria-label="t('nav.pointsAdmin')"><button v-for="tab in tabs" :key="tab.value" type="button" class="btn btn-sm" :class="activeTab === tab.value ? 'btn-primary' : 'btn-secondary'" :aria-pressed="activeTab === tab.value" @click="activeTab = tab.value">{{ tab.label }}</button></nav>
      <PointsProductsPanel v-if="activeTab === 'products'" />
      <section v-else>
      <div v-if="loading" role="status" class="card p-6">{{ t('common.loading') }}</div>
      <div v-else-if="loadError" role="alert" class="card space-y-4 p-6"><p>{{ t('common.pointsAdmin.loadError') }}</p><button type="button" class="btn btn-secondary" @click="load">{{ t('common.tryAgain') }}</button></div>
      <form v-else-if="loaded" class="card space-y-6 p-6" @submit.prevent="save">
        <div><h2 class="text-base font-medium">{{ t('common.checkIn.title') }}</h2><p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('common.checkIn.rules') }}</p></div>
        <div><label for="daily-checkin-points" class="input-label">{{ t('common.pointsAdmin.baseReward') }}</label><input id="daily-checkin-points" v-model.number="reward" type="number" min="1" max="1000000" step="1" required class="input max-w-xs" :disabled="saving" /><p class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t('common.pointsAdmin.range') }}</p></div>
        <div v-if="valid" class="rounded-xl bg-gray-50 p-4 text-sm dark:bg-dark-800"><p>{{ t('common.pointsAdmin.normalReward', { points: format(reward) }) }}</p><p class="mt-2">{{ t('common.pointsAdmin.doubleReward', { points: format(reward * 2) }) }}</p><p class="mt-2 text-gray-500 dark:text-gray-400">{{ t('common.pointsAdmin.cycleTotal', { points: format(reward * 9) }) }}</p></div>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('common.pointsAdmin.effectiveHint') }}</p>
        <button type="submit" class="btn btn-primary" :disabled="saving || !valid">{{ saving ? t('common.saving') : t('common.save') }}</button>
      </form>
      </section>
    </div>
  </AppLayout>
</template>
