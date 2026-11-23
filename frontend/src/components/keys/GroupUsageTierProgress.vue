<template>
  <div v-for="progress in entries" :key="progress.source_group_id" class="mt-2 space-y-2 rounded-lg bg-primary-50/60 p-3 text-xs text-gray-700 dark:bg-primary-900/10 dark:text-gray-300" data-test="tier-progress">
    <details @click.stop @keydown.stop>
      <summary class="cursor-pointer font-medium">{{ t('usageTiers.title') }} · {{ progress.source_group_name }}</summary>
      <p class="mt-2 leading-relaxed">{{ t('usageTiers.userHint') }}</p>
      <p class="mt-1 leading-relaxed">{{ t('usageTiers.modelScope') }}: {{ progress.models?.length ? progress.models.join(', ') : t('usageTiers.allModels') }}</p>
      <p v-for="(tier, index) in progress.tiers" :key="index" class="mt-1 leading-relaxed">
        {{ t('usageTiers.rule', { stage: index + 1, tokens: count(tier.daily_tokens), days: tier.consecutive_days, rate: tier.rate_multiplier, missed: tier.max_missed_days }) }}
      </p>
      <p class="mt-1 leading-relaxed">{{ t('usageTiers.priorityHint') }}</p>
    </details>
    <p v-if="progress.excluded" data-test="tier-excluded">{{ t('usageTiers.excluded') }}</p>
    <template v-else>
      <div v-if="progress.stage > 0" class="flex items-center gap-3" data-test="tier-maintenance">
        <svg viewBox="0 0 40 40" class="h-12 w-12 shrink-0 -rotate-90" role="progressbar" :aria-label="t('usageTiers.maintenance')" :aria-valuenow="maintenancePercent(progress)" :aria-valuemin="0" :aria-valuemax="100">
          <circle cx="20" cy="20" r="16" fill="none" stroke="currentColor" stroke-width="4" class="text-gray-200 dark:text-gray-700" />
          <circle cx="20" cy="20" r="16" fill="none" stroke="currentColor" stroke-width="4" stroke-linecap="round" class="text-primary-500" :stroke-dasharray="`${maintenancePercent(progress)} 100`" pathLength="100" />
        </svg>
        <div class="space-y-1">
          <p class="font-semibold">{{ t('usageTiers.active', { stage: progress.stage, rate: progress.rate_multiplier }) }}</p>
          <p>{{ t('usageTiers.today', { tokens: count(progress.today_tokens), target: count(progress.tiers[progress.stage - 1]!.daily_tokens) }) }}</p>
          <p>{{ progress.today_qualified ? t('usageTiers.maintained') : t('usageTiers.remaining', { days: progress.tiers[progress.stage - 1]!.max_missed_days - progress.missed }) }}</p>
        </div>
      </div>
      <div v-if="progress.stage < progress.tiers.length" class="space-y-1" data-test="tier-next">
        <p>{{ t('usageTiers.next', { stage: progress.stage + 1, rate: progress.tiers[progress.stage]!.rate_multiplier, days: progress.streak, required: progress.tiers[progress.stage]!.consecutive_days }) }}</p>
        <div class="h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700" role="progressbar" :aria-label="t('usageTiers.nextProgress')" :aria-valuenow="nextPercent(progress)" :aria-valuemin="0" :aria-valuemax="100">
          <div class="h-full rounded-full bg-primary-500 transition-all" :style="{ width: `${nextPercent(progress)}%` }" />
        </div>
        <p v-if="progress.promoted_today">{{ t('usageTiers.nextTomorrow') }}</p>
        <p v-else>{{ t('usageTiers.today', { tokens: count(progress.today_tokens), target: count(progress.tiers[progress.stage]!.daily_tokens) }) }}</p>
      </div>
      <p v-else class="font-medium" data-test="tier-locked">{{ t('usageTiers.locked') }}</p>
    </template>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { GroupUsageTierProgress } from '@/types/usageTiers'

defineProps<{ entries: GroupUsageTierProgress[] }>()
const { t, locale } = useI18n()
const count = (value: number) => value.toLocaleString(locale.value)
function maintenancePercent(p: GroupUsageTierProgress) {
  return Math.min(100, Math.max(0, p.today_tokens / p.tiers[p.stage - 1]!.daily_tokens * 100))
}
function nextPercent(p: GroupUsageTierProgress) {
  if (p.promoted_today) return 0
  const tier = p.tiers[p.stage]!
  const partialDay = p.today_tokens < tier.daily_tokens ? Math.max(0, p.today_tokens / tier.daily_tokens) : 0
  return Math.min(100, (p.streak + partialDay) / tier.consecutive_days * 100)
}
</script>
