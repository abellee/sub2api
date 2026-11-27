<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useDailyCheckIn } from '@/composables/useDailyCheckIn'
import type { PointsSummary } from '@/api/points'
import pointsIcon from '@/assets/icons/points-image-color.svg'
import { refreshUserPoints } from '@/composables/useUserPoints'

const props = defineProps<{ show: boolean; userId: number }>()
const emit = defineEmits<{ claimed: [points: PointsSummary] }>()
const { t, locale } = useI18n()
const { status, loading, claiming, error, refresh, claim } = useDailyCheckIn()
const claimError = ref(false)
const earned = ref<number | null>(null)
const rewards = computed(() => status.value?.rewards ?? Array<number | null>(7).fill(null))
let version = 0
const number = (value: number) => value.toLocaleString(locale.value)
const subtitle = computed(() => status.value?.claimed
  ? t('common.checkIn.streakDone', { days: status.value.completed_days })
  : t('common.checkIn.streak', { days: status.value?.completed_days ?? 0 }))
watch(() => [props.show, props.userId] as const, ([show]) => {
  version++
  claimError.value = false
  earned.value = null
  if (show) void refresh(true)
}, { immediate: true })
async function submit() {
  const current = version
  claimError.value = false
  try {
    const result = await claim()
    if (!result || current !== version) return
    earned.value = result.granted ? result.status.today_reward : null
    emit('claimed', result.points)
  } catch {
    if (current === version) {
      claimError.value = true
      // If the response was lost after a grant, read the server before allowing a retry.
      const confirmed = await refresh(true)
      if (confirmed?.claimed && current === version) {
        const points = await refreshUserPoints(true)
        if (points && current === version) emit('claimed', points)
      }
    }
  }
}
</script>

<template>
  <section class="checkin-card" :aria-label="t('common.checkIn.title')" :aria-busy="loading || claiming">
    <header class="checkin-heading"><div><h4><Icon name="calendar" size="sm" />{{ t('common.checkIn.title') }}</h4><p>{{ status ? subtitle : t('common.checkIn.doubleHint') }}</p></div><span class="checkin-cycle">{{ t('common.checkIn.cycle') }}</span></header>
      <div class="checkin-days" :aria-label="t('common.checkIn.progress')">
        <div v-for="(reward, index) in rewards" :key="index" class="checkin-day" :class="{ 'is-today': index + 1 === status?.day, 'is-complete': index < (status?.completed_days ?? 0), 'is-double': index >= 5 }">
          <span v-if="index >= 5" class="checkin-double">{{ t('common.checkIn.double') }}</span><span class="checkin-day-label">{{ t('common.checkIn.day', { day: index + 1 }) }}</span><img class="checkin-points-icon" :src="pointsIcon" alt="" /><strong>{{ reward === null ? '—' : '+' + number(reward) }}</strong>
          <span class="checkin-day-state"><Icon v-if="index < (status?.completed_days ?? 0)" name="check" size="xs" />{{ !status ? '—' : index < status.completed_days ? t('common.checkIn.received') : index + 1 === status.day ? t('common.checkIn.today') : t('common.checkIn.locked') }}</span>
        </div>
      </div>
      <div v-if="status" class="checkin-claim-row"><div class="checkin-description" role="status" aria-live="polite"><strong>{{ earned !== null ? t('common.checkIn.success', { points: number(earned) }) : status.claimed ? t('common.checkIn.credited') : status.day >= 6 ? t('common.checkIn.doubleToday') : t('common.checkIn.encouragement') }}</strong><span>{{ status.claimed && status.day === 7 ? t('common.checkIn.newCycle') : status.day >= 5 && status.day < 7 ? t('common.checkIn.doubleTomorrow') : t('common.checkIn.doubleHint') }}</span></div><button type="button" class="checkin-claim" :disabled="status.claimed || claiming || loading" @click="submit"><Icon :name="status.claimed ? 'check' : 'gift'" size="sm" />{{ claiming ? t('common.submitting') : status.claimed ? t('common.checkIn.claimed') : t('common.checkIn.claim', { points: number(status.today_reward) }) }}</button></div>
    <div v-if="!status" class="checkin-claim-row"><div class="checkin-description" :role="error ? 'alert' : 'status'"><strong>{{ loading ? t('common.loading') : t('common.checkIn.loadError') }}</strong><button v-if="error" type="button" class="checkin-retry" @click="refresh(true)">{{ t('common.tryAgain') }}</button></div><button type="button" class="checkin-claim" disabled><Icon name="gift" size="sm" />{{ loading ? t('common.loading') : t('common.checkIn.unavailable') }}</button></div>
    <p v-if="claimError" class="checkin-error" role="alert">{{ status?.claimed ? t('common.checkIn.credited') : t('common.checkIn.claimError') }}</p>
    <div class="checkin-rule"><Icon name="clock" size="xs" /><span>{{ t('common.checkIn.rules') }}</span></div>
  </section>
</template>

<style scoped>
.checkin-points-icon{width:22px;height:22px;object-fit:contain}
.checkin-day strong{max-width:100%;text-align:center}
.checkin-card{--checkin-accent:#536eff;--checkin-ink:#4359dc;--checkin-haze:#f0f3ff;--checkin-bonus-bg:#fff5e6;--checkin-bonus:#935819;--checkin-positive:#238268;margin-top:22px;padding:20px;border:1px solid var(--key-form-line);border-radius:16px;background:var(--key-form-surface)}
.checkin-retry{align-self:flex-start;padding:0;border:0;background:transparent;color:var(--checkin-ink);font:inherit;font-size:12px;cursor:pointer}.checkin-retry:hover{text-decoration:underline;text-underline-offset:3px}
:global(.dark .checkin-card){--checkin-accent:#9aabff;--checkin-ink:#c1ccff;--checkin-haze:#252d4b;--checkin-bonus-bg:#352b22;--checkin-bonus:#f0c48b;--checkin-positive:#8bd5b7}
.checkin-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:12px;margin-bottom:24px}.checkin-heading h4{display:flex;align-items:center;gap:7px;margin:0;font-size:15px;font-weight:600}.checkin-heading h4 svg{color:var(--checkin-accent)}.checkin-heading p{margin:5px 0 0;font-size:12px;color:var(--key-form-muted)}.checkin-cycle{padding:3px 8px;border-radius:6px;background:var(--checkin-haze);color:var(--checkin-ink);font-size:11px;white-space:nowrap}
.checkin-days{display:grid;grid-template-columns:repeat(7,minmax(0,1fr));gap:7px}.checkin-day{position:relative;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:8px;min-width:0;padding:16px 3px 13px;border:1px solid transparent;border-radius:11px;background:var(--key-form-field);text-align:center}.checkin-day-label,.checkin-day-state{color:var(--key-form-muted);font-size:11px;white-space:nowrap}.checkin-day strong{font-size:21px;font-weight:600;letter-spacing:-.6px;font-variant-numeric:tabular-nums;overflow-wrap:anywhere}.checkin-day-state{display:flex;align-items:center;justify-content:center;gap:2px}.checkin-day-state svg{width:11px;height:11px}.checkin-day.is-complete strong{color:var(--key-form-muted)}.checkin-day.is-complete .checkin-day-state{color:var(--checkin-positive)}.checkin-day.is-double{background:var(--key-form-field)}.is-double strong{color:var(--checkin-bonus)}.checkin-double{position:absolute;top:-10px;padding:1px 8px;border-radius:5px;background:var(--checkin-bonus);color:var(--checkin-bonus-bg);font-size:11px;white-space:nowrap}.checkin-day.is-today{border-color:var(--checkin-accent);background:var(--checkin-haze)}.is-today strong,.is-today .checkin-day-state{color:var(--checkin-ink)}
.checkin-claim-row{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-top:20px}.checkin-description{display:flex;flex-direction:column;gap:4px;min-width:0}.checkin-description strong{font-size:12px;font-weight:500}.checkin-description span{font-size:11px;color:var(--key-form-muted)}.checkin-claim{display:flex;align-items:center;justify-content:center;gap:7px;flex-shrink:0;min-height:44px;padding:12px 16px;border:0;border-radius:10px;background:var(--checkin-accent);color:#fff;font:inherit;font-size:12px;font-weight:500;cursor:pointer}.checkin-claim:hover:not(:disabled){filter:brightness(1.08)}.checkin-claim:disabled{background:var(--key-form-field);color:var(--checkin-positive);cursor:default}.checkin-claim:disabled:not(:has(svg)){opacity:.6}:global(.dark .checkin-claim:not(:disabled)){color:#161b33}.checkin-rule{display:flex;align-items:flex-start;gap:6px;margin-top:18px;padding-top:14px;border-top:1px solid var(--key-form-line);color:var(--key-form-muted);font-size:11px}.checkin-rule svg{flex-shrink:0;margin-top:2px}.checkin-state{display:flex;align-items:center;justify-content:center;flex-wrap:wrap;gap:12px;min-height:120px;color:var(--key-form-muted);font-size:12px}.checkin-state button{border:0;background:transparent;color:var(--checkin-ink);cursor:pointer;font:inherit}.checkin-error{margin:12px 0 0;color:var(--key-form-muted);font-size:12px}
@media(max-width:580px){.checkin-card{padding:16px 12px}.checkin-days{grid-template-columns:repeat(4,minmax(0,1fr));gap:16px 7px}.checkin-day:last-child{grid-column:span 2}.checkin-claim-row{flex-direction:column;align-items:stretch;gap:12px}.checkin-claim{width:100%}.checkin-heading{gap:6px}.checkin-cycle{padding:3px 6px}}
@media(min-width:581px){.checkin-day strong{font-size:clamp(14px,2.6vw,21px)}}
</style>
