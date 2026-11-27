<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores'
import { fetchPointsSettings, savePointsSettings } from '@/api/points'
import { refreshDailyCheckIn } from '@/composables/useDailyCheckIn'
import PointsProductsPanel from '@/components/admin/points/PointsProductsPanel.vue'
import Select from '@/components/common/Select.vue'

const { t, locale } = useI18n()
const appStore = useAppStore()
const rewards = ref<number[]>([])
const multipliers = ref<number[]>([])
const multiplierOptions = computed(() => Array.from({ length: 10 }, (_, index) => ({ value: index + 1, label: t(`common.checkIn.multiplierLabels.${index + 1}`) })))
const loaded = ref(false)
const loading = ref(false)
const saving = ref(false)
const loadError = ref(false)
const activeTab = ref<'checkin' | 'products'>('checkin')
const tabs = computed(() => [
  { value: 'checkin' as const, label: t('common.checkIn.title') },
  { value: 'products' as const, label: t('common.pointsCatalog.products') }
])
const valid = computed(() => rewards.value.length === 7 && rewards.value.every(value => Number.isInteger(value) && value >= 1 && value <= 2000000) && multipliers.value.length === 7 && multipliers.value.every(value => Number.isInteger(value) && value >= 1 && value <= 10))
const cycleTotal = computed(() => rewards.value.reduce((total, value, index) => total + value * multipliers.value[index], 0))
const format = (value: number) => value.toLocaleString(locale.value)
async function load() {
  loading.value = true
  loadError.value = false
  try {
    const settings = await fetchPointsSettings()
    rewards.value = [...settings.daily_checkin_rewards]
    multipliers.value = [...(settings.daily_checkin_multipliers ?? Array<number>(7).fill(1))]
    if (!valid.value) throw new Error('Invalid check-in rewards')
    loaded.value = true
  }
  catch { loadError.value = true; loaded.value = false }
  finally { loading.value = false }
}
async function save() {
  if (!valid.value || !loaded.value || saving.value) return
  saving.value = true
  try {
    await savePointsSettings({ daily_checkin_rewards: [...rewards.value], daily_checkin_multipliers: [...multipliers.value] })
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
        <div><div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"><div v-for="(_, index) in rewards" :key="index"><label :for="`daily-checkin-points-${index + 1}`" class="input-label">{{ t('common.checkIn.day', { day: index + 1 }) }}</label><div class="flex items-center gap-2"><input :id="`daily-checkin-points-${index + 1}`" v-model.number="rewards[index]" type="number" min="1" max="2000000" step="1" required class="input min-w-0 flex-1" :disabled="saving" /><Select :id="`daily-checkin-multiplier-${index + 1}`" v-model="multipliers[index]" :options="multiplierOptions" :aria-label="t('common.pointsAdmin.multiplierLabel', { day: index + 1 })" :searchable="false" :disabled="saving" class="w-28 flex-shrink-0" /></div><p v-if="valid" class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t('common.pointsAdmin.dailyTotal', { points: format(rewards[index] * multipliers[index]) }) }}</p></div></div><p class="mt-3 text-xs text-gray-500 dark:text-gray-400">{{ t('common.pointsAdmin.range') }}</p></div>
        <div v-if="valid" class="rounded-xl bg-gray-50 p-4 text-sm dark:bg-dark-800">{{ t('common.pointsAdmin.cycleTotal', { points: format(cycleTotal) }) }}</div>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('common.pointsAdmin.effectiveHint') }}</p>
        <button type="submit" class="btn btn-primary" :disabled="saving || !valid">{{ saving ? t('common.saving') : t('common.save') }}</button>
      </form>
      </section>
    </div>
  </AppLayout>
</template>
