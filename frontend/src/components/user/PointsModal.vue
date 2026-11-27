<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import DailyCheckInCard from './DailyCheckInCard.vue'
import { fetchMyPoints, fetchMyPointsHistory, type PointsFilter, type PointsHistory, type PointsSummary } from '@/api/points'
import { formatDateTime } from '@/utils/format'
import pointsIcon from '@/assets/icons/points-image-color.svg'
import '@/styles/key-dialog.css'

const props = defineProps<{ show: boolean; userId: number }>()
const emit = defineEmits<{ close: []; loaded: [summary: PointsSummary] }>()
const { t, locale } = useI18n()
const summary = ref<PointsSummary | null>(null)
const history = ref<PointsHistory | null>(null)
const filter = ref<PointsFilter>('all')
const filterOptions = computed(() => [
  { value: 'all', label: t('common.all') },
  { value: 'spend', label: t('common.pointsDetails.spend') },
  { value: 'grant', label: t('common.pointsDetails.grant') }
])
const page = ref(1)
const loading = ref(false)
const error = ref(false)
const pages = computed(() => Math.max(1, Math.ceil((history.value?.total || 0) / 10)))
let requestVersion = 0

function number(value?: string) {
  return value == null ? '—' : BigInt(value).toLocaleString(locale.value)
}
function kindLabel(kind: string) {
  switch (kind) {
    case 'grant': return t('common.pointsDetails.grant')
    case 'spend': return t('common.pointsDetails.spend')
    case 'refund': return t('common.pointsDetails.refund')
    default: return t('common.pointsDetails.adjust')
  }
}
async function load(targetPage = 1) {
  const version = ++requestVersion
  page.value = targetPage
  loading.value = true
  error.value = false
  history.value = null
  try {
    const [current, records] = await Promise.all([fetchMyPoints(), fetchMyPointsHistory(targetPage, filter.value)])
    if (version !== requestVersion) return
    summary.value = current
    history.value = records
    emit('loaded', current)
  } catch {
    if (version !== requestVersion) return
    error.value = true
    summary.value = null
  } finally {
    if (version === requestVersion) loading.value = false
  }
}
watch(() => [props.show, props.userId] as const, ([show]) => {
  requestVersion++
  summary.value = null
  history.value = null
  page.value = 1
  filter.value = 'all'
  if (show && props.userId) void load()
}, { immediate: true })
onBeforeUnmount(() => { requestVersion++ })

function onClaimed(points: PointsSummary) {
  summary.value = points
  emit('loaded', points)
  void load()
}
</script>

<template>
  <BaseDialog :show="show" :title="t('common.pointsDetails.title')" width="medium-wide" :close-on-click-outside="true" @close="emit('close')">
    <div class="key-dialog points-modal" :aria-busy="loading">
      <div class="points-summary">
        <div class="points-current"><span>{{ t('common.pointsDetails.current') }}</span><strong><img :src="pointsIcon" alt="" />{{ number(summary?.balance) }}</strong></div>
        <div><span>{{ t('common.pointsDetails.earned') }}</span><strong>{{ number(summary?.total_earned) }}</strong></div>
        <div><span>{{ t('common.pointsDetails.spent') }}</span><strong>{{ number(summary?.total_spent) }}</strong></div>
      </div>
      <DailyCheckInCard :show="show" :user-id="userId" @claimed="onClaimed" />
      <div class="points-toolbar">
        <div class="points-filter"><span>{{ t('common.pointsDetails.records') }}</span><Select v-model="filter" class="points-filter-select" dropdown-class="points-filter-dropdown" :aria-label="t('common.pointsDetails.records')" :options="filterOptions" :searchable="false" :disabled="loading" @change="load()" /></div>
        <button type="button" class="points-text-button" :disabled="loading" @click="load(page)">{{ t('common.refresh') }}</button>
      </div>
      <div v-if="loading" class="points-state" role="status">{{ t('common.loading') }}</div>
      <div v-else-if="error" class="points-state" role="alert"><p>{{ t('common.pointsDetails.loadError') }}</p><button type="button" class="points-text-button" @click="load(page)">{{ t('common.tryAgain') }}</button></div>
      <div v-else-if="!history?.items.length" class="points-state">{{ t('common.pointsDetails.empty') }}</div>
      <div v-else class="points-records">
        <div v-for="record in history.items" :key="record.id" class="points-record">
          <div><strong>{{ kindLabel(record.kind) }}</strong><p v-if="record.note">{{ record.note }}</p><time :datetime="record.created_at">{{ formatDateTime(record.created_at) }}</time></div>
          <div class="points-change"><strong :class="record.delta.startsWith('-') ? 'is-spent' : 'is-earned'">{{ record.delta.startsWith('-') ? '' : '+' }}{{ number(record.delta) }}</strong><small>{{ t('common.pointsDetails.balanceAfter') }} {{ number(record.balance_after) }}</small></div>
        </div>
      </div>
      <nav v-if="history && pages > 1 && !loading && !error" class="points-pagination" :aria-label="t('common.pointsDetails.records')"><button type="button" class="points-text-button" :disabled="page <= 1" @click="load(page - 1)">{{ t('pagination.previous') }}</button><span>{{ page }} / {{ pages }}</span><button type="button" class="points-text-button" :disabled="page >= pages" @click="load(page + 1)">{{ t('pagination.next') }}</button></nav>
    </div>
  </BaseDialog>
</template>

<style scoped>
.points-modal{--studio-muted:var(--key-form-muted);color:var(--key-form-ink);font-size:14px}
.points-summary{display:grid;grid-template-columns:1.3fr 1fr 1fr;gap:12px;padding:20px;border-radius:18px;background:var(--key-form-field);font-variant-numeric:tabular-nums}
.points-summary span{display:block;font-size:12px;color:var(--studio-muted,#8891a2)}.points-summary strong{display:flex;align-items:center;gap:8px;margin-top:8px;font-size:20px;font-weight:600;overflow-wrap:anywhere}.points-summary img{width:28px;height:28px}.points-current strong{font-size:28px}
.points-toolbar{display:flex;align-items:center;justify-content:space-between;gap:12px;margin:24px 0 8px}.points-filter{display:flex;align-items:center;gap:12px;min-width:0;font-weight:500}.points-filter-select{width:132px;font-weight:400}.points-filter-select :deep(.select-trigger){min-height:34px;padding:6px 10px;font-size:12px;background:var(--key-form-surface);color:var(--key-form-ink);border-color:var(--key-form-line)}
.points-text-button{padding:4px 2px;border:0;background:transparent;color:var(--tf-console-active-ink,#7c73e6);cursor:pointer;font:inherit}.points-text-button:hover:not(:disabled){text-decoration:underline;text-underline-offset:3px}.points-text-button:disabled{opacity:.45;cursor:default}.points-text-button:focus-visible{outline:2px solid #8796db;outline-offset:3px;border-radius:4px}
.points-state{min-height:180px;display:flex;align-items:center;justify-content:center;flex-direction:column;gap:12px;color:var(--studio-muted,#8891a2);text-align:center}
.points-records{max-height:360px;overflow:auto}.points-record{display:flex;justify-content:space-between;gap:20px;padding:16px 0;border-bottom:1px solid var(--key-form-line)}.points-record>div{min-width:0}.points-record strong{font-weight:500}.points-record p{margin:5px 0;font-size:13px;overflow-wrap:anywhere}.points-record time,.points-change small{display:block;margin-top:5px;font-size:12px;color:var(--studio-muted,#8891a2)}.points-change{text-align:right;flex-shrink:0;font-variant-numeric:tabular-nums}.is-earned{color:#16a36b}.is-spent{color:#e17b39}.points-pagination{display:flex;align-items:center;justify-content:center;gap:20px;margin-top:20px;font-size:13px}
@media(max-width:480px){.points-summary{padding:14px;gap:10px;grid-template-columns:1fr 1fr}.points-current{grid-column:1/-1}.points-record{gap:12px}.points-change{max-width:45%;overflow-wrap:anywhere}}
</style>

<style>
.select-dropdown-portal.points-filter-dropdown{background:var(--key-form-surface);border-color:var(--key-form-line);border-radius:10px;color:var(--key-form-ink)}
.points-filter-dropdown .select-option{padding:9px 12px;color:var(--key-form-ink);font-size:12px}
.points-filter-dropdown .select-option:hover,.points-filter-dropdown .select-option-focused{background:var(--key-form-field)}
.points-filter-dropdown .select-option-selected{background:var(--key-form-haze);color:var(--tf-console-active-ink,#536eff)}
.points-filter-dropdown .select-option svg{color:currentColor}
</style>
