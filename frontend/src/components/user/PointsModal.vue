<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import DailyCheckInCard from './DailyCheckInCard.vue'
import { fetchMyPoints, fetchMyPointsHistory, fetchAdminUserPoints, fetchAdminUserPointsHistory, type PointsFilter, type PointsHistory, type PointsSummary } from '@/api/points'
import { formatDateTime } from '@/utils/format'
import pointsIcon from '@/assets/icons/points-image-color.svg'
import '@/styles/key-dialog.css'

const props = defineProps<{ show: boolean; userId: number; admin?: boolean }>()
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
const pages = computed(() => Math.max(1, Math.ceil((history.value?.total || 0) / 20)))
const modalContent = ref<HTMLElement | null>(null)
const balanceTarget = ref<HTMLElement | null>(null)
const animatedBalance = ref<string | null>(null)
const balanceRoll = ref<{ digit: string; sequence: number[]; duration: number }[] | null>(null)
const rewardEffect = ref<{ multiplier: number; phase: 'burst' | 'fly' | 'coins' | 'impact'; x: number; y: number } | null>(null)
const coins = ref<{ amount: number; scatterX: number; scatterY: number; delay: number }[]>([])
const coinSource = ref({ x: 0, y: 0 })
const balanceText = computed(() => number(animatedBalance.value ?? summary.value?.balance))
const balanceColumns = computed(() => balanceRoll.value ?? Array.from(balanceText.value, digit => ({ digit, sequence: [], duration: 0 })))
let effectTimer: ReturnType<typeof setTimeout> | undefined
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
async function load(targetPage = 1, keepSummary = false) {
  const version = ++requestVersion
  page.value = targetPage
  loading.value = true
  error.value = false
  history.value = null
  try {
    const [current, records] = await Promise.all(props.admin
      ? [fetchAdminUserPoints(props.userId), fetchAdminUserPointsHistory(props.userId, targetPage, filter.value)]
      : [fetchMyPoints(), fetchMyPointsHistory(targetPage, filter.value)])
    if (version !== requestVersion) return
    summary.value = current
    history.value = records
    emit('loaded', current)
  } catch {
    if (version !== requestVersion) return
    error.value = true
    if (!keepSummary) summary.value = null
  } finally {
    if (version === requestVersion) loading.value = false
  }
}
watch(() => [props.show, props.userId, props.admin] as const, ([show]) => {
  requestVersion++
  stopRewardEffect()
  summary.value = null
  history.value = null
  page.value = 1
  filter.value = props.admin ? 'spend' : 'all'
  if (show && props.userId) void load()
}, { immediate: true })
onBeforeUnmount(() => { requestVersion++; stopRewardEffect() })

function stopRewardEffect() {
  clearTimeout(effectTimer)
  rewardEffect.value = null
  animatedBalance.value = null
  balanceRoll.value = null
  coins.value = []
}

function playRewardEffect(from: string, to: string, multiplier: number) {
  const start = BigInt(from)
  const increase = BigInt(to) - start
  if (increase <= 0n || window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
  animatedBalance.value = from
  rewardEffect.value = { multiplier, phase: multiplier > 1 ? 'burst' : 'coins', x: 0, y: 0 }
  const source = modalContent.value?.querySelector('.is-today .checkin-points-icon')?.getBoundingClientRect()
  coinSource.value = source ? { x: source.left + source.width / 2 - window.innerWidth / 2, y: source.top + source.height / 2 - window.innerHeight / 2 } : { x: 0, y: 0 }
  balanceTarget.value?.scrollIntoView?.({ block: 'nearest', behavior: 'instant' })
  const land = () => {
    if (!rewardEffect.value) return
    rewardEffect.value.phase = 'impact'
    const previous = from.padStart(to.length, '0')
    let place = 0
    balanceRoll.value = Array.from(number(to), digit => {
      if (!/\d/.test(digit)) return { digit, sequence: [], duration: 0 }
      const first = Number(previous[place])
      const steps = 10 + (Number(digit) - first + 10) % 10
      const duration = 850 + Math.round(place / Math.max(1, to.length - 1) * 200)
      place++
      return { digit, sequence: Array.from({ length: steps + 1 }, (_, index) => (first + index) % 10), duration }
    })
    effectTimer = setTimeout(stopRewardEffect, 1100)
  }
  effectTimer = setTimeout(() => {
    const rect = balanceTarget.value?.getBoundingClientRect()
    if (!rect || !rewardEffect.value) { stopRewardEffect(); return }
    rewardEffect.value = { multiplier, phase: multiplier > 1 ? 'fly' : 'coins', x: rect.left + rect.width / 2 - window.innerWidth / 2, y: rect.top + rect.height / 2 - window.innerHeight / 2 }
    if (multiplier > 1) effectTimer = setTimeout(land, 560)
    else {
      const reward = Number(increase)
      // Group large rewards into labeled coins to keep the animation light.
      const count = Math.min(reward, 60)
      coins.value = Array.from({ length: count }, (_, index) => ({
        amount: Math.floor(reward / count) + (index < reward % count ? 1 : 0),
        scatterX: Math.cos(index * 2.4) * (32 + index % 4 * 12),
        scatterY: -35 - index % 5 * 12,
        delay: index * 18
      }))
      effectTimer = setTimeout(land, 760 + (count - 1) * 18)
    }
  }, multiplier > 1 ? 820 : 220)
}

function onClaimed(points: PointsSummary, bonus?: { multiplier: number; reward: number }) {
  if (!props.show) return
  stopRewardEffect()
  summary.value = points
  emit('loaded', points)
  if (bonus) playRewardEffect((BigInt(points.balance) - BigInt(bonus.reward)).toString(), points.balance, bonus.multiplier)
  void load(1, true)
}

</script>

<template>
  <BaseDialog :show="show" :title="admin ? t('admin.users.pointsHistoryTitle', { id: userId }) : t('common.pointsDetails.title')" width="medium-wide" :close-on-click-outside="true" @close="emit('close')">
    <div ref="modalContent" class="key-dialog points-modal" :aria-busy="loading">
      <div class="points-summary">
        <div class="points-current">
          <span>{{ t('common.pointsDetails.current') }}</span>
          <strong :class="{ 'points-impact': rewardEffect?.phase === 'impact' }">
            <img :src="pointsIcon" alt="" />
            <span ref="balanceTarget" class="points-balance" role="status" :aria-label="number(summary?.balance)">
              <span v-for="(column, index) in balanceColumns" :key="balanceColumns.length - index" class="points-digit" :class="{ 'is-separator': !/\p{N}/u.test(column.digit) }" aria-hidden="true">
                <span v-if="column.sequence.length" class="points-reel" :style="{ '--reel-distance': (column.sequence.length - 1) * -1.25 + 'em', '--reel-duration': column.duration + 'ms' }">
                  <span v-for="(digit, step) in column.sequence" :key="step">{{ digit }}</span>
                </span>
                <span v-else>{{ column.digit }}</span>
              </span>
            </span>
          </strong>
        </div>
        <div><span>{{ t('common.pointsDetails.earned') }}</span><strong>{{ number(summary?.total_earned) }}</strong></div>
        <div><span>{{ t('common.pointsDetails.spent') }}</span><strong>{{ number(summary?.total_spent) }}</strong></div>
      </div>
      <DailyCheckInCard v-if="!admin" :show="show" :user-id="userId" @claimed="onClaimed" />
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
  <Teleport to="body">
    <div v-if="rewardEffect" class="checkin-reward-effect" :class="'is-' + rewardEffect.phase" :style="{ '--reward-x': rewardEffect.x + 'px', '--reward-y': rewardEffect.y + 'px' }" aria-hidden="true">
      <div v-if="rewardEffect.multiplier > 1" class="reward-burst">
        <i class="reward-ring" />
        <i v-for="ray in 16" :key="ray" class="reward-ray" :style="{ '--ray-angle': ray * 22.5 + 'deg', '--ray-length': (ray % 2 ? 160 : 220) + 'px' }" />
      </div>
      <div v-if="rewardEffect.multiplier > 1" class="reward-multiplier"><span>x</span>{{ rewardEffect.multiplier }}</div>
      <template v-if="rewardEffect.phase === 'coins'">
        <div v-for="(coin, index) in coins" :key="index" class="reward-coin" :data-amount="coin.amount" :style="{ '--coin-source-x': coinSource.x + 'px', '--coin-source-y': coinSource.y + 'px', '--coin-scatter-x': coin.scatterX + 'px', '--coin-scatter-y': coin.scatterY + 'px', '--coin-delay': coin.delay + 'ms' }">
          <img :src="pointsIcon" alt="" /><span v-if="coin.amount > 1">{{ number(String(coin.amount)) }}</span>
        </div>
      </template>
      <div v-if="rewardEffect.phase === 'impact'" class="reward-impact">
        <i /><i />
        <i v-for="spark in 12" :key="spark" class="reward-spark" :style="{ '--ray-angle': spark * 30 + 'deg' }" />
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.points-modal{--studio-muted:var(--key-form-muted);color:var(--key-form-ink);font-size:14px}
.points-summary{display:grid;grid-template-columns:1.3fr 1fr 1fr;gap:12px;padding:20px;border-radius:18px;background:var(--key-form-field);font-variant-numeric:tabular-nums}
.points-summary span{display:block;font-size:12px;color:var(--studio-muted,#8891a2)}.points-summary strong{display:flex;align-items:center;gap:8px;margin-top:8px;font-size:20px;font-weight:600;overflow-wrap:anywhere}.points-summary img{width:28px;height:28px}.points-current strong{font-size:28px}
.points-summary .points-balance{display:inline-flex;flex-wrap:wrap;font:inherit;color:inherit;line-height:1.25}
.points-summary .points-digit{position:relative;display:inline-block;width:.64em;height:1.25em;overflow:hidden;font:inherit;color:inherit;text-align:center}
.points-summary .points-digit.is-separator{width:.32em}.points-summary .points-digit>span{display:block;font:inherit;color:inherit}
.points-summary .points-reel{display:flex;flex-direction:column;will-change:transform;animation:points-reel-roll var(--reel-duration) cubic-bezier(.16,.7,.2,1) both}
.points-summary .points-reel>span{flex:0 0 1.25em;height:1.25em;font:inherit;line-height:1.25;color:inherit}
@keyframes points-reel-roll{from{transform:translateY(0)}to{transform:translateY(var(--reel-distance))}}
.points-impact{animation:points-smash .5s ease-out;transform-origin:center}
.checkin-reward-effect{position:fixed;inset:0;z-index:10000;pointer-events:none;overflow:hidden}
.checkin-reward-effect::before{content:'';position:absolute;inset:0;background:#120c20;animation:reward-dim 1.38s both}
.checkin-reward-effect.is-coins::before{display:none}
.reward-coin{position:absolute;top:50%;left:50%;width:28px;height:28px;filter:drop-shadow(0 2px 7px #ffbf4366);animation:reward-coin-flight .76s cubic-bezier(.3,.05,.65,1) var(--coin-delay) both;will-change:transform;pointer-events:none}.reward-coin img{display:block;width:100%;height:100%}.reward-coin>span{position:absolute;top:24px;left:50%;transform:translateX(-50%);padding:1px 4px;border-radius:4px;background:#6d3d13;color:#fff0b4;font-size:10px;font-weight:700;white-space:nowrap}
@keyframes reward-coin-flight{0%{transform:translate(calc(-50% + var(--coin-source-x)),calc(-50% + var(--coin-source-y))) scale(.3) rotateY(0);opacity:0}18%,30%{transform:translate(calc(-50% + var(--coin-source-x) + var(--coin-scatter-x)),calc(-50% + var(--coin-source-y) + var(--coin-scatter-y))) scale(1.1) rotateY(180deg);opacity:1}82%{opacity:1}100%{transform:translate(calc(-50% + var(--reward-x)),calc(-50% + var(--reward-y))) scale(.3) rotateY(720deg);opacity:0}}
.reward-multiplier{position:absolute;top:50%;left:50%;color:#ffda69;font-family:Impact,'Arial Black',sans-serif;font-size:clamp(100px,22vw,190px);font-style:italic;font-weight:900;line-height:1;letter-spacing:-.06em;white-space:nowrap;-webkit-text-stroke:2px #a45c16;paint-order:stroke fill;text-shadow:2px 4px 0 #bb771f,5px 9px 0 #6d3d13,0 0 38px #ffbe4680;transform:translate(-50%,-50%) rotate(-10deg) scale(1.05);animation:reward-pop .82s both}
.reward-multiplier>span{font-size:.72em;margin-right:.03em}.is-fly .reward-multiplier{animation:reward-fly .56s cubic-bezier(.6,-.15,.85,.55) both}.is-impact .reward-multiplier{visibility:hidden}
.reward-burst{position:absolute;left:50%;top:50%;width:0;height:0}.reward-ring{position:absolute;width:220px;height:220px;border:3px solid #ffd66b;border-radius:50%;box-shadow:0 0 24px #ffc65c80,inset 0 0 24px #ffc65c40;animation:reward-ring .75s ease-out both}
.reward-ray{position:absolute;left:-3px;top:-24px;width:6px;height:48px;border-radius:50%;background:#ffdd83;box-shadow:0 0 12px #ffce5d;animation:reward-ray .72s cubic-bezier(.12,.65,.2,1) both}
.is-fly .reward-burst,.is-impact .reward-burst{display:none}.is-impact::before{display:none}
.reward-impact{position:absolute;left:calc(50% + var(--reward-x));top:calc(50% + var(--reward-y))}.reward-impact>i:not(.reward-spark){position:absolute;left:-40px;top:-40px;width:80px;height:80px;border:3px solid #ffcd52;border-radius:50%;box-shadow:0 0 18px #ffbf43;animation:reward-hit .65s ease-out both}.reward-impact>i:nth-child(2){animation-delay:.08s}
.reward-spark{position:absolute;width:5px;height:16px;background:#ffd66b;border-radius:3px;animation:reward-spark .6s ease-out both}
@keyframes reward-pop{0%{transform:translate(-50%,-50%) rotate(-30deg) scale(.05);opacity:0}32%{transform:translate(-50%,-50%) rotate(-8deg) scale(1.35);opacity:1}50%{transform:translate(-50%,-50%) rotate(-13deg) scale(.94)}68%,100%{transform:translate(-50%,-50%) rotate(-10deg) scale(1.05)}}
@keyframes reward-fly{0%{transform:translate(-50%,-50%) rotate(-10deg) scale(1.05);opacity:1}22%{transform:translate(-50%,calc(-50% + 30px)) rotate(-18deg) scale(1.18);opacity:1}100%{transform:translate(calc(-50% + var(--reward-x)),calc(-50% + var(--reward-y))) rotate(12deg) scale(.12);opacity:.3}}
@keyframes reward-ring{0%{transform:translate(-50%,-50%) scale(.15);opacity:0}20%{opacity:.9}100%{transform:translate(-50%,-50%) scale(2.3);opacity:0}}
@keyframes reward-ray{0%{transform:rotate(var(--ray-angle)) translateY(-55px) scaleY(.2);opacity:0}18%{opacity:1}100%{transform:rotate(var(--ray-angle)) translateY(calc(-1 * var(--ray-length))) scaleY(.3);opacity:0}}
@keyframes reward-dim{0%,100%{opacity:0}18%,55%{opacity:.3}}
@keyframes reward-hit{0%{transform:scale(.25);opacity:1}100%{transform:scale(2.4);opacity:0}}
@keyframes reward-spark{0%{transform:rotate(var(--ray-angle)) translateY(-10px);opacity:1}100%{transform:rotate(var(--ray-angle)) translateY(-110px) scale(.2);opacity:0}}
@keyframes points-smash{0%{transform:scale(1);filter:drop-shadow(0 0 0 transparent)}22%{transform:translateY(5px) scale(1.2,.85);color:#f5b939;filter:drop-shadow(0 0 12px #ffcc65)}48%{transform:translateY(-4px) scale(.96,1.1);color:#f5b939}72%{transform:translateY(2px) scale(1.04,.97)}100%{transform:scale(1)}}
@media(prefers-reduced-motion:reduce){.checkin-reward-effect{display:none}.points-impact{animation:none}.points-summary .points-reel{animation:none;transform:translateY(var(--reel-distance))}}
.points-toolbar{display:flex;align-items:center;justify-content:space-between;gap:12px;margin:24px 0 8px}.points-filter{display:flex;align-items:center;gap:12px;min-width:0;font-weight:500}.points-filter-select{width:132px;font-weight:400}.points-filter-select :deep(.select-trigger){min-height:34px;padding:6px 10px;font-size:12px;background:var(--key-form-surface);color:var(--key-form-ink);border-color:var(--key-form-line)}
.points-text-button{padding:4px 2px;border:0;background:transparent;color:var(--tf-console-active-ink,#7c73e6);cursor:pointer;font:inherit}.points-text-button:hover:not(:disabled){text-decoration:underline;text-underline-offset:3px}.points-text-button:disabled{opacity:.45;cursor:default}.points-text-button:focus-visible{outline:2px solid #8796db;outline-offset:3px;border-radius:4px}
.points-state{min-height:180px;display:flex;align-items:center;justify-content:center;flex-direction:column;gap:12px;color:var(--studio-muted,#8891a2);text-align:center}
.points-records{max-height:360px;overflow:auto}.points-record{display:flex;justify-content:space-between;gap:20px;padding:16px 0;border-bottom:1px solid var(--key-form-line)}.points-record>div{min-width:0}.points-record strong{font-weight:500}.points-record p{margin:5px 0;font-size:13px;overflow-wrap:anywhere}.points-record time,.points-change small{display:block;margin-top:5px;font-size:12px;color:var(--studio-muted,#8891a2)}.points-change{text-align:right;flex-shrink:0;font-variant-numeric:tabular-nums}.is-earned{color:#16a36b}.is-spent{color:#e17b39}.points-pagination{display:flex;align-items:center;justify-content:center;gap:20px;margin-top:20px;font-size:13px}
@media(max-width:480px){.points-summary{padding:14px;gap:10px;grid-template-columns:1fr 1fr}.points-current{grid-column:1/-1}.points-record{gap:12px}.points-change{max-width:45%;overflow-wrap:anywhere}}
</style>

<style>
.modal-overlay:has(.points-modal){--points-dialog-gap:24px;align-items:flex-start;padding-block:var(--points-dialog-gap)}
.modal-overlay:has(.points-modal) .modal-content{max-height:calc(100vh - var(--points-dialog-gap) - var(--points-dialog-gap));max-height:calc(100dvh - var(--points-dialog-gap) - var(--points-dialog-gap));overflow:hidden}
.modal-overlay:has(.points-modal) .modal-body{min-height:0;scrollbar-gutter:stable;overflow-anchor:none}
@media(max-width:640px){.modal-overlay:has(.points-modal){--points-dialog-gap:12px}}
.select-dropdown-portal.points-filter-dropdown{background:var(--key-form-surface);border-color:var(--key-form-line);border-radius:10px;color:var(--key-form-ink)}
.points-filter-dropdown .select-option{padding:9px 12px;color:var(--key-form-ink);font-size:12px}
.points-filter-dropdown .select-option:hover,.points-filter-dropdown .select-option-focused{background:var(--key-form-field)}
.points-filter-dropdown .select-option-selected{background:var(--key-form-haze);color:var(--tf-console-active-ink,#536eff)}
.points-filter-dropdown .select-option svg{color:currentColor}
</style>
