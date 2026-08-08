<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getModelPlazaGroups, getOfficialModelPrices, type LegacyModelPlazaGroup } from '@/api/modelPlaza'
import { extractApiErrorMessage } from '@/utils/apiError'
import { buildModelPlazaEntries, type ModelPlazaEntry } from './modelPlaza'

type Language = 'zh' | 'en'
type ViewMode = 'card' | 'list'

const props = withDefaults(defineProps<{ variant?: 'console' | 'home'; language?: Language }>(), { variant: 'console' })
const { locale } = useI18n()
const groups = ref<LegacyModelPlazaGroup[]>([])
const entries = ref<ModelPlazaEntry[]>([])
const selectedGroup = ref<number | 'all'>('all')
const query = ref('')
const loading = ref(true)
const error = ref('')
const updatedAt = ref<Date | null>(null)
const stale = ref(false)
const view = ref<ViewMode>(localStorage.getItem('llmfree-model-view') === 'list' ? 'list' : 'card')
const language = computed<Language>(() => props.language ?? (locale.value.startsWith('zh') ? 'zh' : 'en'))
const text = computed(() => language.value === 'zh' ? {
  source: '官方价格数据源', updated: '数据更新时间', connecting: '正在连接...', pending: '等待首次更新', cached: '（缓存）', placeholder: '搜索模型，例如 gpt-5、claude-sonnet', clear: '清空搜索', card: '卡片视图', list: '列表视图', all: '全部分组', lowest: '最低', models: '个可用模型', groups: '个分组', input: '输入', cache: '缓存', output: '输出', original: '原价', actual: '实际', model: '模型', rate: '费率倍数', group: '分组', unit: '单位', retry: '重新加载', failed: '暂时无法加载模型价格', empty: '没有符合条件的模型', emptyHint: '请更换分组或搜索关键词。', disclaimer: '本页官方原价仅供参考，实际价格按官方原价乘以当前分组费率倍数计算。价格会随上游模型厂商的官方定价实时变动，最终账单请以控制台和账单记录为准。',
} : {
  source: 'Official pricing source', updated: 'Last updated', connecting: 'Connecting...', pending: 'Waiting for first update', cached: ' (cached)', placeholder: 'Search models, e.g. gpt-5 or claude-sonnet', clear: 'Clear search', card: 'Card view', list: 'List view', all: 'All groups', lowest: 'LOWEST', models: 'available models', groups: 'groups', input: 'Input', cache: 'Cache', output: 'Output', original: 'List', actual: 'Actual', model: 'Model', rate: 'Rate', group: 'Group', unit: 'Unit', retry: 'Retry', failed: 'Model pricing is temporarily unavailable', empty: 'No matching models', emptyHint: 'Try another group or search term.', disclaimer: 'Official list prices are for reference, and actual prices are calculated using the current group rate. Prices update in real time with upstream model providers; final charges remain subject to console billing records.',
})

const filteredEntries = computed(() => {
  const term = query.value.trim().toLowerCase()
  return entries.value.filter((entry) => {
    const groupMatches = selectedGroup.value === 'all' || entry.groupId === selectedGroup.value
    const searchMatches = !term || entry.id.toLowerCase().includes(term) || entry.groupName.toLowerCase().includes(term) || providerLabel(entry.provider).toLowerCase().includes(term)
    return groupMatches && searchMatches
  })
})
const visibleGroupCount = computed(() => new Set(filteredEntries.value.map((entry) => entry.groupId)).size)
const lowestRateMultiplier = computed(() => groups.value.length > 0 ? Math.min(...groups.value.map((group) => group.rate_multiplier)) : null)
const updatedLabel = computed(() => {
  if (loading.value) return text.value.connecting
  if (!updatedAt.value) return text.value.pending
  return `${updatedAt.value.toLocaleString(language.value === 'zh' ? 'zh-CN' : 'en-US', { hour12: false })}${stale.value ? text.value.cached : ''}`
})

function providerLabel(provider: string) { return provider === 'anthropic' ? 'Claude' : 'OpenAI' }
function providerMark(provider: string) { return provider === 'anthropic' ? 'A' : 'OA' }
function formatPrice(value: number) { return Number(value).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }
function formatActualPrice(value: number, rateMultiplier: number) { return Number(value * rateMultiplier).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }
function formatRate(value: number) { return `${Number(value).toFixed(4).replace(/\.?0+$/, '')}x` }
function isLowestRate(group: LegacyModelPlazaGroup) { return lowestRateMultiplier.value !== null && Math.abs(group.rate_multiplier - lowestRateMultiplier.value) < Number.EPSILON }
function setView(nextView: ViewMode) { view.value = nextView; localStorage.setItem('llmfree-model-view', nextView) }

async function loadData() {
  loading.value = true
  error.value = ''
  try {
    const [groupData, priceData] = await Promise.all([getModelPlazaGroups(), getOfficialModelPrices()])
    groups.value = groupData
    entries.value = buildModelPlazaEntries(groupData, priceData.models)
    const parsed = priceData.updated_at ? new Date(priceData.updated_at) : null
    updatedAt.value = parsed && !Number.isNaN(parsed.getTime()) ? parsed : null
    stale.value = Boolean(priceData.stale)
  } catch (loadError) {
    error.value = extractApiErrorMessage(loadError, text.value.failed)
    groups.value = []
    entries.value = []
  } finally {
    loading.value = false
  }
}
onMounted(loadData)
</script>

<template>
  <section class="model-plaza" :class="`model-plaza--${variant}`">
    <div class="model-plaza__meta">
      <span class="model-plaza__source"><i></i>{{ text.source }}</span>
      <span class="model-plaza__updated">{{ text.updated }}<strong>{{ updatedLabel }}</strong></span>
    </div>
    <div class="model-plaza__toolbar">
      <div class="model-plaza__search-wrap">
        <svg class="model-plaza__search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/></svg>
        <input v-model="query" class="model-plaza__search" type="text" role="searchbox" inputmode="search" :placeholder="text.placeholder" autocomplete="off">
        <button v-if="query" class="model-plaza__clear" type="button" :title="text.clear" :aria-label="text.clear" @click="query = ''"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="m18 6-12 12M6 6l12 12"/></svg></button>
      </div>
      <div class="model-plaza__view-toggle">
        <button type="button" :class="{ active: view === 'card' }" :title="text.card" :aria-label="text.card" @click="setView('card')"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/></svg></button>
        <button type="button" :class="{ active: view === 'list' }" :title="text.list" :aria-label="text.list" @click="setView('list')"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M8 6h13M8 12h13M8 18h13"/><path d="M3.5 6h.01M3.5 12h.01M3.5 18h.01"/></svg></button>
      </div>
    </div>
    <div class="model-plaza__filters">
      <div class="model-plaza__groups">
        <button type="button" :class="{ active: selectedGroup === 'all' }" @click="selectedGroup = 'all'">{{ text.all }}</button>
        <button v-for="group in groups" :key="group.id" type="button" :class="{ active: selectedGroup === group.id, 'is-lowest-rate': isLowestRate(group) }" @click="selectedGroup = group.id"><span>{{ group.name }}</span><small v-if="isLowestRate(group)" class="model-plaza__lowest-badge" :title="text.lowest" :aria-label="text.lowest"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" aria-hidden="true"><path d="M12 4v15M6.5 13.5 12 19l5.5-5.5"/></svg></small></button>
      </div>
      <span class="model-plaza__count">{{ filteredEntries.length }} {{ text.models }} · {{ visibleGroupCount }} {{ text.groups }}</span>
    </div>
    <div v-if="loading" class="model-plaza__loading"><span></span></div>
    <div v-else-if="error" class="model-plaza__state"><strong>{{ text.failed }}</strong><span>{{ error }}</span><button type="button" @click="loadData">{{ text.retry }}</button></div>
    <div v-else-if="filteredEntries.length === 0" class="model-plaza__state"><strong>{{ text.empty }}</strong><span>{{ text.emptyHint }}</span></div>
    <div v-else-if="view === 'card'" class="model-plaza__grid">
      <article v-for="entry in filteredEntries" :key="`${entry.groupId}:${entry.id}`" class="model-plaza__card">
        <div class="model-plaza__card-head"><span class="model-plaza__provider-logo" :class="entry.provider">{{ providerMark(entry.provider) }}</span><span class="model-plaza__model-name"><strong :title="entry.id">{{ entry.id }}</strong><small>{{ providerLabel(entry.provider) }}</small></span><span class="model-plaza__rate" :title="text.rate">{{ formatRate(entry.rateMultiplier) }}</span></div>
        <div class="model-plaza__prices"><span><small>{{ text.input }}</small><span class="model-plaza__original-price"><i>{{ text.original }}</i><em>${{ formatPrice(entry.input) }}</em></span><span class="model-plaza__actual-price"><i>{{ text.actual }}</i><strong>${{ formatActualPrice(entry.input, entry.rateMultiplier) }}</strong></span></span><span><small>{{ text.cache }}</small><span class="model-plaza__original-price"><i>{{ text.original }}</i><em>${{ formatPrice(entry.cache) }}</em></span><span class="model-plaza__actual-price"><i>{{ text.actual }}</i><strong>${{ formatActualPrice(entry.cache, entry.rateMultiplier) }}</strong></span></span><span><small>{{ text.output }}</small><span class="model-plaza__original-price"><i>{{ text.original }}</i><em>${{ formatPrice(entry.output) }}</em></span><span class="model-plaza__actual-price"><i>{{ text.actual }}</i><strong>${{ formatActualPrice(entry.output, entry.rateMultiplier) }}</strong></span></span></div>
        <div class="model-plaza__card-foot"><strong :title="entry.groupName">{{ entry.groupName }}</strong><span>USD / 1M TOKENS</span></div>
      </article>
    </div>
    <div v-else class="model-plaza__table-wrap">
      <table class="model-plaza__table"><thead><tr><th>{{ text.model }}</th><th>{{ text.rate }}</th><th>{{ text.group }}</th><th>{{ text.input }}</th><th>{{ text.cache }}</th><th>{{ text.output }}</th><th>{{ text.unit }}</th></tr></thead><tbody><tr v-for="entry in filteredEntries" :key="`${entry.groupId}:${entry.id}`"><td><div class="model-plaza__table-model"><span class="model-plaza__provider-logo" :class="entry.provider">{{ providerMark(entry.provider) }}</span><span><code>{{ entry.id }}</code><small>{{ providerLabel(entry.provider) }}</small></span></div></td><td><span class="model-plaza__rate">{{ formatRate(entry.rateMultiplier) }}</span></td><td>{{ entry.groupName }}</td><td><span class="model-plaza__table-original">{{ text.original }} ${{ formatPrice(entry.input) }}</span><strong class="model-plaza__table-actual">{{ text.actual }} ${{ formatActualPrice(entry.input, entry.rateMultiplier) }}</strong></td><td><span class="model-plaza__table-original">{{ text.original }} ${{ formatPrice(entry.cache) }}</span><strong class="model-plaza__table-actual">{{ text.actual }} ${{ formatActualPrice(entry.cache, entry.rateMultiplier) }}</strong></td><td><span class="model-plaza__table-original">{{ text.original }} ${{ formatPrice(entry.output) }}</span><strong class="model-plaza__table-actual">{{ text.actual }} ${{ formatActualPrice(entry.output, entry.rateMultiplier) }}</strong></td><td>USD / 1M Token</td></tr></tbody></table>
    </div>
    <div class="model-plaza__disclaimer"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="9"/><path d="M12 10v6M12 7h.01"/></svg><span>{{ text.disclaimer }}</span></div>
  </section>
</template>

<style>
html.dark .model-plaza.model-plaza--console{--mp-bg:#1e293b;--mp-subtle:#0f172a;--mp-text:#f3f4f6;--mp-muted:#94a3b8;--mp-line:#334155;--mp-line-strong:#475569;--mp-accent:#2dd4bf;--mp-accent-soft:rgba(19,78,74,.55);--mp-green:#2dd4bf}
</style>

<style scoped>
.model-plaza{--mp-bg:#fff;--mp-subtle:#f9fafb;--mp-text:#111827;--mp-muted:#6b7280;--mp-line:#f3f4f6;--mp-line-strong:#e5e7eb;--mp-accent:#0d9488;--mp-accent-soft:#ccfbf1;--mp-green:#0d9488;color:var(--mp-text);font-family:Inter,ui-sans-serif,system-ui,"PingFang SC","Microsoft YaHei",sans-serif}.model-plaza--home{--mp-bg:var(--surface);--mp-subtle:var(--code);--mp-text:var(--ink);--mp-muted:var(--muted);--mp-line:var(--line);--mp-line-strong:var(--line-strong);--mp-accent:var(--forest);--mp-accent-soft:var(--forest-soft);--mp-green:var(--forest)}.model-plaza *{box-sizing:border-box}
.model-plaza__meta{display:flex;align-items:center;justify-content:space-between;gap:18px;margin-bottom:18px;color:var(--mp-muted);font-size:12px}.model-plaza__source{display:inline-flex;align-items:center;gap:8px;padding:7px 10px;border:1px solid var(--mp-line);border-radius:6px;background:var(--mp-bg)}.model-plaza__source i{width:7px;height:7px;border-radius:50%;background:var(--mp-green);box-shadow:0 0 0 3px color-mix(in srgb,var(--mp-green) 14%,transparent)}.model-plaza__updated{text-align:right}.model-plaza__updated strong{display:block;margin-top:3px;color:var(--mp-text);font-weight:650}
.model-plaza__toolbar{display:grid;grid-template-columns:minmax(260px,1fr) auto;align-items:center;gap:12px;margin-bottom:16px}.model-plaza__search-wrap{position:relative;min-width:0}.model-plaza__search-icon{position:absolute;left:14px;top:50%;width:18px;height:18px;color:var(--mp-muted);transform:translateY(-50%);pointer-events:none}.model-plaza__search{width:100%;height:44px;padding:0 44px 0 42px;border:1px solid var(--mp-line-strong);border-radius:7px;outline:none;color:var(--mp-text);background:var(--mp-bg);font-size:14px}.model-plaza__search:focus{border-color:var(--mp-accent);box-shadow:0 0 0 3px color-mix(in srgb,var(--mp-accent) 16%,transparent)}.model-plaza__search::placeholder{color:var(--mp-muted)}
.model-plaza__clear{position:absolute;right:7px;top:50%;display:flex;width:30px;height:30px;padding:0;align-items:center;justify-content:center;border:0;border-radius:5px;color:var(--mp-muted);background:transparent;line-height:0;transform:translateY(-50%);cursor:pointer}.model-plaza__clear:hover{color:var(--mp-text);background:var(--mp-subtle)}.model-plaza__clear svg{display:block;width:16px;height:16px;flex:none}.model-plaza__view-toggle{display:flex;gap:4px;padding:4px;border:1px solid var(--mp-line);border-radius:7px;background:var(--mp-bg)}.model-plaza__view-toggle button{display:grid;width:34px;height:34px;padding:0;place-items:center;border:0;border-radius:5px;color:var(--mp-muted);background:transparent;cursor:pointer}.model-plaza__view-toggle button:hover{color:var(--mp-text);background:var(--mp-subtle)}.model-plaza__view-toggle button.active{color:var(--mp-accent);background:var(--mp-accent-soft)}.model-plaza__view-toggle svg{width:18px;height:18px}
.model-plaza__filters{display:flex;align-items:center;justify-content:space-between;gap:18px;margin-bottom:20px}.model-plaza__groups{display:flex;min-width:0;gap:8px;padding:8px 6px 2px 0;overflow-x:auto;scrollbar-width:none}.model-plaza__groups::-webkit-scrollbar{display:none}.model-plaza__groups button{position:relative;min-height:34px;padding:0 13px;border:1px solid var(--mp-line);border-radius:6px;color:var(--mp-muted);background:var(--mp-bg);font-size:13px;white-space:nowrap;cursor:pointer}.model-plaza__groups button:hover{color:var(--mp-text);border-color:var(--mp-line-strong)}.model-plaza__groups button.active{color:var(--mp-text);border-color:var(--mp-accent);background:var(--mp-accent-soft);font-weight:650}.model-plaza__groups button.is-lowest-rate{padding-right:19px;border-color:color-mix(in srgb,var(--mp-green) 55%,var(--mp-line))}.model-plaza__lowest-badge{position:absolute;right:-6px;top:-9px;display:grid;width:19px;height:19px;place-items:center;border:2px solid var(--mp-bg);border-radius:50%;color:#fff;background:var(--mp-green);box-shadow:0 2px 6px color-mix(in srgb,var(--mp-green) 28%,transparent)}.model-plaza__lowest-badge svg{width:10px;height:10px}.model-plaza__count{flex:none;color:var(--mp-muted);font-size:12px;white-space:nowrap}
.model-plaza__loading{height:250px;overflow:hidden;border:1px solid var(--mp-line);border-radius:8px;background:var(--mp-bg)}.model-plaza__loading span{display:block;width:34%;height:2px;background:var(--mp-accent);animation:mp-loading 1s infinite ease-in-out}@keyframes mp-loading{from{transform:translateX(-110%)}to{transform:translateX(310%)}}.model-plaza__state{display:grid;min-height:250px;padding:30px;place-items:center;align-content:center;border:1px dashed var(--mp-line-strong);border-radius:8px;color:var(--mp-muted);background:var(--mp-bg);text-align:center}.model-plaza__state strong{color:var(--mp-text);font-size:14px}.model-plaza__state span{max-width:420px;margin-top:8px;font-size:12px;line-height:1.7}.model-plaza__state button{margin-top:16px;padding:8px 13px;border:1px solid var(--mp-line-strong);border-radius:6px;color:var(--mp-text);background:var(--mp-bg);cursor:pointer}
.model-plaza__grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px}.model-plaza__card{min-width:0;overflow:hidden;border:1px solid var(--mp-line);border-radius:8px;background:var(--mp-bg);box-shadow:0 1px 3px rgba(0,0,0,.04);transition:transform .16s,border-color .16s}.model-plaza__card:hover{transform:translateY(-2px);border-color:var(--mp-line-strong)}.model-plaza__card-head{display:flex;align-items:center;gap:12px;padding:18px 18px 16px;border-bottom:1px solid var(--mp-line)}.model-plaza__provider-logo{display:grid;width:38px;height:38px;flex:none;place-items:center;border:1px solid var(--mp-line);border-radius:7px;color:var(--mp-green);background:var(--mp-subtle);font-size:12px;font-weight:800}.model-plaza__provider-logo.anthropic{color:#b76035}.model-plaza__model-name{min-width:0;flex:1}.model-plaza__model-name strong{display:block;overflow:hidden;color:var(--mp-text);font:650 14px/1.4 ui-monospace,SFMono-Regular,Consolas,monospace;text-overflow:ellipsis;white-space:nowrap}.model-plaza__model-name small{display:block;margin-top:3px;color:var(--mp-muted);font-size:11px}.model-plaza__rate{flex:none;padding:5px 8px;border-radius:5px;color:var(--mp-green);background:color-mix(in srgb,var(--mp-green) 12%,transparent);font:750 12px/1 ui-monospace,SFMono-Regular,Consolas,monospace}
.model-plaza__prices{display:grid;grid-template-columns:repeat(3,1fr);padding:15px 10px 17px}.model-plaza__prices>span{min-width:0;padding:0 8px;border-right:1px solid var(--mp-line)}.model-plaza__prices>span:last-child{border:0}.model-plaza__prices>span>small{display:block;margin-bottom:9px;color:var(--mp-muted);font-size:11px}.model-plaza__original-price,.model-plaza__actual-price{display:flex;min-width:0;align-items:baseline;justify-content:space-between;gap:5px;white-space:nowrap}.model-plaza__actual-price{margin-top:6px}.model-plaza__original-price i,.model-plaza__actual-price i{color:var(--mp-muted);font-size:9px;font-style:normal}.model-plaza__original-price em{overflow:hidden;color:var(--mp-muted);font-size:11px;font-style:normal;text-overflow:ellipsis}.model-plaza__actual-price strong{overflow:hidden;color:var(--mp-green);font-size:14px;text-overflow:ellipsis}.model-plaza__card-foot{display:flex;justify-content:space-between;gap:12px;padding:10px 18px;color:var(--mp-muted);background:var(--mp-subtle);font-size:10px}.model-plaza__card-foot strong{overflow:hidden;color:var(--mp-text);font-size:11px;text-overflow:ellipsis;white-space:nowrap}
.model-plaza__table-wrap{overflow-x:auto;border:1px solid var(--mp-line);border-radius:8px;background:var(--mp-bg)}.model-plaza__table{width:100%;min-width:940px;border-collapse:collapse}.model-plaza__table th{padding:12px 16px;color:var(--mp-muted);background:var(--mp-subtle);font-size:11px;font-weight:650;text-align:left}.model-plaza__table td{padding:14px 16px;border-top:1px solid var(--mp-line);font-size:13px}.model-plaza__table-model{display:flex;align-items:center;gap:10px}.model-plaza__table-model .model-plaza__provider-logo{width:30px;height:30px;border-radius:6px;font-size:10px}.model-plaza__table-model code{display:block;color:var(--mp-text);font-size:12px}.model-plaza__table-model small{display:block;margin-top:3px;color:var(--mp-muted);font-size:10px}.model-plaza__table-original{display:block;color:var(--mp-muted);font-size:10px;white-space:nowrap}.model-plaza__table-actual{display:block;margin-top:5px;color:var(--mp-green);font-size:12px;white-space:nowrap}.model-plaza__disclaimer{display:flex;gap:10px;margin-top:22px;padding:13px 15px;border-left:3px solid var(--mp-line-strong);color:var(--mp-muted);background:var(--mp-subtle);font-size:11px;line-height:1.7}.model-plaza__disclaimer svg{width:16px;height:16px;flex:none;margin-top:1px}
@media(max-width:900px){.model-plaza__grid{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:640px){.model-plaza__meta{align-items:flex-start;flex-direction:column}.model-plaza__updated{text-align:left}.model-plaza__updated strong{display:inline;margin-left:6px}.model-plaza__filters{align-items:flex-start;flex-direction:column;gap:10px}.model-plaza__groups{width:100%}.model-plaza__grid{grid-template-columns:1fr}.model-plaza__count{white-space:normal}}
</style>
