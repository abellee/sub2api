<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { modelPlazaAPI, type ModelPlazaGroup } from '@/api/modelPlaza'
import { extractApiErrorMessage } from '@/utils/apiError'
import ModelIcon from '@/components/common/ModelIcon.vue'
import Select from '@/components/common/Select.vue'
import { formatDecimal } from '@/utils/pricing'
import { buildModelPlazaEntries, formatActualModelPrice, modelPlazaEntryKind, modelPlazaProviderForGroup, type ModelPlazaEntry } from './modelPlaza'

type Language = 'zh' | 'en'
type ViewMode = 'card' | 'list'

const props = withDefaults(defineProps<{ variant?: 'console' | 'home'; language?: Language }>(), { variant: 'console' })
const { locale } = useI18n()
const groups = ref<ModelPlazaGroup[]>([])
const entries = ref<ModelPlazaEntry[]>([])
const selectedProvider = ref<string>('all')
const selectedGroup = ref<number | 'all'>('all')
const query = ref('')
const loading = ref(true)
const error = ref('')
const updatedAt = ref<Date | null>(null)
const stale = ref(false)
const view = ref<ViewMode>(localStorage.getItem('llmfree-model-view') === 'list' ? 'list' : 'card')
const selectedPriceTabs = ref<Record<string, string>>({})
const expandedLongContext = ref<Record<string, boolean>>({})
const language = computed<Language>(() => props.language ?? (locale.value.startsWith('zh') ? 'zh' : 'en'))
const text = computed(() => language.value === 'zh' ? {
  source: '渠道优先 · 官方兜底', demoSource: '本地演示数据', updated: '数据更新时间', connecting: '正在连接...', pending: '等待接口返回', cached: '', placeholder: '搜索模型，例如 gpt-5、claude-sonnet', clear: '清空搜索', card: '卡片视图', list: '列表视图', provider: '模型厂商', providerHint: '先选择厂商，再浏览对应分组', allProviders: '全部厂商', allGroups: '全部分组', noGroupsForProvider: '该厂商暂无可展示的分组', lowest: '最低', models: '个可用模型', groups: '个分组', input: '输入', cache: '缓存', cacheWrite: '写入（5m）', cacheWrite1h: '写入（1h）', cacheRead: '读取', output: '输出', original: '原价', actual: '实际', standard: '标准时段', current: '当前', model: '模型', rate: '费率倍数', group: '分组', unit: '单位', longContext: '长上下文阶梯', viewLongContext: '查看长上下文阶梯价', closeLongContext: '收起长上下文阶梯价', retry: '重新加载', failed: '暂时无法加载模型价格', empty: '没有符合条件的模型', emptyHint: '请更换分组或搜索关键词。', perImage: '按张', perVideo: '按秒', perImageUnit: '/ 张', perVideoUnit: '/ 秒', imageUnit: 'USD / 张', videoUnit: 'USD / 秒', tokenUnit: 'USD / 1M TOKENS', pricingNoticeTitle: '价格说明', pricingNoticeBody: '模型价格同步自 Sub2API。由于各渠道价格定义不统一且模型数量较多，本页仅展示针对性配置模型的长上下文价格；完整定义请参阅官方价格清单。', pricingNoticeData: 'Sub2API 价格同步数据', pricingNoticeOfficial: '官方价格清单', disclaimer: '本页价格优先采用渠道定价，渠道缺失字段回退 Sub2API 官方定价；实际价格按当前分组费率倍数计算，最终账单请以控制台和账单记录为准。',
} : {
  source: 'Channel first · official fallback', demoSource: 'Local demo data', updated: 'Data updated', connecting: 'Connecting...', pending: 'Waiting for response', cached: '', placeholder: 'Search models, e.g. gpt-5 or claude-sonnet', clear: 'Clear search', card: 'Card view', list: 'List view', provider: 'Model provider', providerHint: 'Choose a provider first, then browse its groups', allProviders: 'All providers', allGroups: 'All groups', noGroupsForProvider: 'No groups are available for this provider', lowest: 'LOWEST', models: 'available models', groups: 'groups', input: 'Input', cache: 'Cache', cacheWrite: 'Write (5m)', cacheWrite1h: 'Write (1h)', cacheRead: 'Read', output: 'Output', original: 'List', actual: 'Actual', standard: 'Standard period', current: 'CURRENT', model: 'Model', rate: 'Rate', group: 'Group', unit: 'Unit', longContext: 'Long-context tier', viewLongContext: 'View long-context tier prices', closeLongContext: 'Hide long-context tier prices', retry: 'Retry', failed: 'Model pricing is temporarily unavailable', empty: 'No matching models', emptyHint: 'Try another group or search term.', perImage: 'Per image', perVideo: 'Per second', perImageUnit: '/ image', perVideoUnit: '/ sec', imageUnit: 'USD / image', videoUnit: 'USD / second', tokenUnit: 'USD / 1M TOKENS', pricingNoticeTitle: 'Pricing note', pricingNoticeBody: 'Model prices are synchronized from Sub2API. Because channel definitions are not uniform and the catalog is large, this page only shows long-context prices for selected models with targeted configuration; see the official price lists for complete definitions.', pricingNoticeData: 'Sub2API synchronized pricing data', pricingNoticeOfficial: 'Official price lists', disclaimer: 'Prices use channel pricing first and fall back to Sub2API official pricing per missing field; actual charges use the current group rate multiplier and the console billing records remain authoritative.',
})

function isMediaGroup(group: ModelPlazaGroup): boolean {
  return group.models.some((model) => {
    const kind = modelPlazaEntryKind(group, model)
    return kind === 'image' || kind === 'video'
  })
}

function compareGroups(a: ModelPlazaGroup, b: ModelPlazaGroup): number {
  const mediaOrder = Number(isMediaGroup(b)) - Number(isMediaGroup(a))
  return mediaOrder
    || (a.user_rate_multiplier ?? a.rate_multiplier) - (b.user_rate_multiplier ?? b.rate_multiplier)
    || a.name.localeCompare(b.name)
}

const orderedGroups = computed(() => [...groups.value].sort(compareGroups))
const providers = computed(() => [...new Set(orderedGroups.value.map(modelPlazaProviderForGroup))].sort())
const providerGroups = computed(() => selectedProvider.value === 'all'
  ? orderedGroups.value
  : orderedGroups.value.filter((group) => modelPlazaProviderForGroup(group) === selectedProvider.value))

watch([selectedProvider, providerGroups], () => {
  if (selectedGroup.value !== 'all' && !providerGroups.value.some((group) => group.id === selectedGroup.value)) {
    selectedGroup.value = 'all'
  }
})

const filteredEntries = computed(() => {
  const term = query.value.trim().toLowerCase()
  return entries.value.filter((entry) => {
    const providerMatches = selectedProvider.value === 'all' || entry.provider === selectedProvider.value
    const groupMatches = selectedGroup.value === 'all' || entry.groupId === selectedGroup.value
    const searchMatches = !term || entry.id.toLowerCase().includes(term) || entry.groupName.toLowerCase().includes(term) || providerLabel(entry.provider).toLowerCase().includes(term)
    return providerMatches && groupMatches && searchMatches
  })
})
const visibleGroupCount = computed(() => new Set(filteredEntries.value.map((entry) => entry.groupId)).size)
const lowestRateMultiplier = computed(() => providerGroups.value.length > 0
  ? Math.min(...providerGroups.value.map((group) => group.user_rate_multiplier ?? group.rate_multiplier))
  : null)
const updatedLabel = computed(() => {
  if (loading.value) return text.value.connecting
  if (!updatedAt.value) return text.value.pending
  return `${updatedAt.value.toLocaleString(language.value === 'zh' ? 'zh-CN' : 'en-US', { hour12: false })}${stale.value ? text.value.cached : ''}`
})

function providerLabel(provider: string) { return ({ anthropic: 'Claude', openai: 'OpenAI', gemini: 'Gemini', google: 'Gemini', grok: 'Grok', deepseek: 'DeepSeek', kimi: 'Kimi', zhipu: '智谱 GLM', minimax: 'MiniMax', doubao: '豆包' } as Record<string, string>)[provider] ?? provider }
function formatPrice(value: string | null) { return formatDecimal(value, 2, true) }
function formatActualPrice(value: string | null, rateMultiplier: number, periodMultiplier = 1) { return formatActualModelPrice(value, rateMultiplier * periodMultiplier) }
function formatListedPrice(value: string | null, currency: ModelPlazaEntry['currency'], periodMultiplier = 1) {
  const formatted = formatPrice(value == null ? null : formatActualModelPrice(value, periodMultiplier))
  return formatted === '-' ? '-' : `${currency === 'CNY' ? '¥' : '$'}${formatted}`
}
function formatListedActual(value: string | null, rateMultiplier: number, periodMultiplier = 1, currency: ModelPlazaEntry['currency'] = 'USD') {
  const formatted = formatActualPrice(value, rateMultiplier, periodMultiplier)
  return formatted === '-' ? '-' : `${currency === 'CNY' ? '¥' : '$'}${formatted}`
}
type CachePriceSource = Pick<ModelPlazaEntry, 'cacheWrite' | 'cacheWrite1h' | 'cacheRead'>
type CachePriceLine = { key: string; label: string; value: string | null }
function cachePriceLines(source: CachePriceSource): CachePriceLine[] {
  const lines = [
    { key: 'write', label: text.value.cacheWrite, value: source.cacheWrite },
    { key: 'write-1h', label: text.value.cacheWrite1h, value: source.cacheWrite1h },
    { key: 'read', label: text.value.cacheRead, value: source.cacheRead },
  ].filter((line) => line.value != null)
  return lines.length > 0 ? lines : [{ key: 'none', label: '', value: null }]
}
function cacheGridClass(source: CachePriceSource) {
  const count = cachePriceLines(source).filter((line) => line.value != null).length
  return count >= 3 ? 'model-plaza__cache-grid--three' : count === 2 ? 'model-plaza__cache-grid--two' : 'model-plaza__cache-grid--one'
}
function unitLabel(entry: ModelPlazaEntry) { return entry.kind === 'image' ? `${entry.currency === 'CNY' ? 'CNY' : 'USD'} / ${text.value.perImageUnit.replace(/^\/ /, '')}` : entry.kind === 'video' ? `${entry.currency === 'CNY' ? 'CNY' : 'USD'} / ${text.value.perVideoUnit.replace(/^\/ /, '')}` : `${entry.currency === 'CNY' ? 'CNY' : 'USD'} / 1M TOKENS` }
function formatRate(value: number) { return `${Number(value).toFixed(4).replace(/\.?0+$/, '')}x` }
function isLowestRate(group: ModelPlazaGroup) { return lowestRateMultiplier.value !== null && Math.abs((group.user_rate_multiplier ?? group.rate_multiplier) - lowestRateMultiplier.value) < Number.EPSILON }
function setView(nextView: ViewMode) { view.value = nextView; localStorage.setItem('llmfree-model-view', nextView) }

type PriceTab = { key: string; label: string; selectLabel: string; multiplier: number; current: boolean }

function priceTabs(entry: ModelPlazaEntry): PriceTab[] {
  const schedule = entry.timePricing
  const periods = schedule?.periods ?? []
  const currentPeriod = periods.find((period) => isCurrentPeriod(period.start_time, period.end_time, schedule?.weekdays_only))
  const tabs: PriceTab[] = [{ key: 'standard', label: text.value.standard, selectLabel: text.value.standard, multiplier: 1, current: !currentPeriod }]
  periods.forEach((period, index) => {
    const weekdayPrefix = schedule?.weekdays_only ? (language.value === 'zh' ? '工作日' : 'Weekdays') : ''
    const periodLabel = formatPeriodLabel(period.start_time, period.end_time)
    const label = weekdayPrefix ? `${weekdayPrefix}\n${periodLabel}` : periodLabel
    tabs.push({ key: `period-${index}`, label, selectLabel: label.replace(/\n/g, ' '), multiplier: period.multiplier, current: period === currentPeriod })
  })
  return tabs
}

function formatPeriodLabel(start: string, end: string): string {
  const startDate = periodDatePart(start)
  const endDate = periodDatePart(end)
  const startTime = periodTimePart(start)
  const endTime = periodTimePart(end)
  if (startDate && endDate && startDate === endDate) return `${startDate}\n${startTime}–${endTime}`
  if (startDate || endDate) return `${periodDisplayPart(start)}\n–\n${periodDisplayPart(end)}`
  return `${startTime} - ${endTime}`
}

function periodDisplayPart(value: string): string {
  const date = periodDatePart(value)
  const time = periodTimePart(value)
  return date ? `${date} ${time}` : time
}

function periodDatePart(value: string): string {
  const match = value.trim().match(/^(\d{4})[-/](\d{1,2})[-/](\d{1,2})/)
  return match ? `${match[2].padStart(2, '0')}-${match[3].padStart(2, '0')}` : ''
}

function periodTimePart(value: string): string {
  const match = value.trim().match(/(?:T|\s|^)(\d{1,2}:\d{2})/)
  return match ? match[1].padStart(5, '0') : value.trim()
}

function priceSelectOptions(entry: ModelPlazaEntry) {
  return priceTabs(entry).map((tab) => ({ value: tab.key, label: `${tab.selectLabel}${tab.current ? ` · ${text.value.current}` : ''}` }))
}

function activePriceTab(entry: ModelPlazaEntry): PriceTab {
  const tabs = priceTabs(entry)
  const key = selectedPriceTabs.value[`${entry.groupId}:${entry.id}`]
  return tabs.find((tab) => tab.key === key) ?? tabs.find((tab) => tab.current) ?? tabs[0]
}

function selectPriceTab(entry: ModelPlazaEntry, key: string) {
  selectedPriceTabs.value[`${entry.groupId}:${entry.id}`] = key
}

function officialTierKey(entry: ModelPlazaEntry): string {
  return `${entry.groupId}:${entry.id}`
}

function hasLongContext(entry: ModelPlazaEntry): boolean {
  return entry.tokenTiers.length > 0 || entry.officialTokenTiers.length > 0
}

function longContextTiers(entry: ModelPlazaEntry): ModelPlazaEntry['tokenTiers'] {
  return entry.tokenTiers.length > 0 ? entry.tokenTiers : entry.officialTokenTiers
}

function isLongContextExpanded(entry: ModelPlazaEntry): boolean {
  return expandedLongContext.value[officialTierKey(entry)] === true
}

function toggleLongContext(entry: ModelPlazaEntry): void {
  const key = officialTierKey(entry)
  expandedLongContext.value[key] = !expandedLongContext.value[key]
}

function isCurrentPeriod(start: string, end: string, weekdaysOnly = false): boolean {
  const parts = new Intl.DateTimeFormat('en-US', { timeZone: 'Asia/Shanghai', weekday: 'short', hour: '2-digit', minute: '2-digit', hour12: false }).formatToParts(new Date())
  const weekday = parts.find((part) => part.type === 'weekday')?.value
  if (weekdaysOnly && (weekday === 'Sat' || weekday === 'Sun')) return false
  const hour = Number(parts.find((part) => part.type === 'hour')?.value ?? 0) % 24
  const minute = Number(parts.find((part) => part.type === 'minute')?.value ?? 0)
  const current = hour * 60 + minute
  const startMinutes = timeToMinutes(start)
  const endMinutes = timeToMinutes(end)
  if (startMinutes == null || endMinutes == null || startMinutes === endMinutes) return false
  return startMinutes < endMinutes ? current >= startMinutes && current < endMinutes : current >= startMinutes || current < endMinutes
}

function timeToMinutes(value: string): number | null {
  const match = value.trim().match(/^(\d{1,2}):(\d{2})/)
  if (!match) return null
  const hour = Number(match[1]); const minute = Number(match[2])
  return hour >= 0 && hour < 24 && minute >= 0 && minute < 60 ? hour * 60 + minute : null
}

async function loadData() {
  loading.value = true
  error.value = ''
  try {
    const plaza = await modelPlazaAPI.getModelPlaza()
    groups.value = plaza.groups
    entries.value = buildModelPlazaEntries(orderedGroups.value)
    updatedAt.value = new Date()
    stale.value = Boolean(plaza.demo)
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
    <div class="model-plaza__pricing-alert" role="note">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 10v6M12 7h.01"/></svg>
      <div>
        <strong>{{ text.pricingNoticeTitle }}</strong>
        <span>{{ text.pricingNoticeBody }}</span>
        <span class="model-plaza__pricing-links">
          <a href="https://github.com/Wei-Shaw/model-price-repo" target="_blank" rel="noopener noreferrer">{{ text.pricingNoticeData }}</a>
          <span aria-hidden="true">·</span>
          <a href="https://docs.anthropic.com/en/docs/about-claude/models/all-models" target="_blank" rel="noopener noreferrer">Anthropic</a>
          <a href="https://platform.openai.com/docs/pricing" target="_blank" rel="noopener noreferrer">OpenAI</a>
          <a href="https://ai.google.dev/gemini-api/docs/pricing" target="_blank" rel="noopener noreferrer">Gemini</a>
          <a href="https://docs.x.ai/docs/models" target="_blank" rel="noopener noreferrer">Grok</a>
          <a href="https://api-docs.deepseek.com/quick_start/pricing" target="_blank" rel="noopener noreferrer">DeepSeek</a>
          <a href="https://platform.moonshot.cn/docs/pricing" target="_blank" rel="noopener noreferrer">Kimi</a>
          <a href="https://open.bigmodel.cn/pricing" target="_blank" rel="noopener noreferrer">GLM</a>
        </span>
      </div>
    </div>
    <div class="model-plaza__meta">
      <span class="model-plaza__source"><i></i>{{ stale ? text.demoSource : text.source }}</span>
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
      <div class="model-plaza__catalog">
        <section class="model-plaza__providers" :aria-label="text.provider">
          <div class="model-plaza__catalog-heading">
            <div><span class="model-plaza__catalog-title"><span>{{ text.provider }}</span><strong>{{ providers.length }}</strong></span><small>{{ text.providerHint }}</small></div>
          </div>
          <div class="model-plaza__provider-list">
            <button type="button" :class="{ active: selectedProvider === 'all' }" @click="selectedProvider = 'all'">
              <span>{{ text.allProviders }}</span>
            </button>
            <button v-for="provider in providers" :key="provider" type="button" :class="{ active: selectedProvider === provider }" @click="selectedProvider = provider">
              <span><i class="model-plaza__provider-dot" :class="provider"></i>{{ providerLabel(provider) }}</span>
              <small>{{ groups.filter((group) => modelPlazaProviderForGroup(group) === provider).length }}</small>
            </button>
          </div>
        </section>
        <section class="model-plaza__group-panel" :aria-label="text.group">
          <div class="model-plaza__catalog-heading">
            <div><span class="model-plaza__catalog-title"><span>{{ text.group }}</span><strong>{{ providerGroups.length }}</strong></span><small>{{ selectedProvider === 'all' ? text.allProviders : providerLabel(selectedProvider) }}</small></div>
          </div>
          <div v-if="providerGroups.length" class="model-plaza__group-list">
            <button type="button" :class="{ active: selectedGroup === 'all' }" @click="selectedGroup = 'all'">
              <span>{{ text.allGroups }}</span><small>{{ providerGroups.length }}</small>
            </button>
            <button v-for="group in providerGroups" :key="group.id" type="button" :class="{ active: selectedGroup === group.id, 'is-lowest-rate': isLowestRate(group) }" @click="selectedGroup = group.id">
              <small v-if="isLowestRate(group)" class="model-plaza__lowest-marker">{{ text.lowest }}</small><span>{{ group.name }}</span><span class="model-plaza__group-meta"><small class="model-plaza__group-rate">{{ formatRate(group.user_rate_multiplier ?? group.rate_multiplier) }}</small></span>
            </button>
          </div>
          <p v-else class="model-plaza__no-groups">{{ text.noGroupsForProvider }}</p>
        </section>
      </div>
      <span class="model-plaza__count">{{ filteredEntries.length }} {{ text.models }} · {{ visibleGroupCount }} {{ text.groups }}</span>
    </div>
    <div v-if="loading" class="model-plaza__loading"><span></span></div>
    <div v-else-if="error" class="model-plaza__state"><strong>{{ text.failed }}</strong><span>{{ error }}</span><button type="button" @click="loadData">{{ text.retry }}</button></div>
    <div v-else-if="filteredEntries.length === 0" class="model-plaza__state"><strong>{{ text.empty }}</strong><span>{{ text.emptyHint }}</span></div>
    <div v-else-if="view === 'card'" class="model-plaza__grid">
      <article v-for="entry in filteredEntries" :key="`${entry.groupId}:${entry.id}`" class="model-plaza__card">
        <div class="model-plaza__card-head"><span class="model-plaza__provider-logo" :class="entry.provider"><ModelIcon :model="entry.provider" size="24px" /></span><span class="model-plaza__model-name"><strong :title="entry.id">{{ entry.id }}</strong><small>{{ providerLabel(entry.provider) }}</small></span><span class="model-plaza__rate" :title="text.rate">{{ formatRate(entry.rateMultiplier) }}</span></div>
        <div v-if="priceTabs(entry).length > 1" class="model-plaza__price-tabs" :class="{ 'model-plaza__price-tabs--many': priceTabs(entry).length >= 3 }" role="tablist">
          <button v-for="tab in priceTabs(entry)" :key="tab.key" type="button" role="tab" :aria-selected="activePriceTab(entry).key === tab.key" :class="{ active: activePriceTab(entry).key === tab.key }" @click="selectPriceTab(entry, tab.key)">
            <span>{{ tab.label }}</span><small v-if="tab.current">{{ text.current }}</small>
          </button>
        </div>
        <div v-if="priceTabs(entry).length > 1" class="model-plaza__price-select" :class="{ 'model-plaza__price-select--many': priceTabs(entry).length >= 3 }">
          <Select :model-value="activePriceTab(entry).key" :options="priceSelectOptions(entry)" :aria-label="text.longContext" :searchable="false" @update:model-value="selectPriceTab(entry, String($event))" />
        </div>
        <template v-if="entry.kind === 'token'">
          <div class="model-plaza__prices">
            <span class="model-plaza__price-item"><small>{{ text.input }}</small><span class="model-plaza__original-price"><i>{{ text.original }}</i><em>{{ formatListedPrice(entry.input, entry.currency, activePriceTab(entry).multiplier) }}</em></span><span class="model-plaza__actual-price"><i>{{ text.actual }}</i><strong>{{ formatListedActual(entry.input, entry.rateMultiplier, activePriceTab(entry).multiplier, entry.currency) }}</strong></span></span>
            <span class="model-plaza__price-item"><small>{{ text.output }}</small><span class="model-plaza__original-price"><i>{{ text.original }}</i><em>{{ formatListedPrice(entry.output, entry.currency, activePriceTab(entry).multiplier) }}</em></span><span class="model-plaza__actual-price"><i>{{ text.actual }}</i><strong>{{ formatListedActual(entry.output, entry.rateMultiplier, activePriceTab(entry).multiplier, entry.currency) }}</strong></span></span>
            <span class="model-plaza__cache-block">
              <span class="model-plaza__cache-heading"><small>{{ text.cache }}</small><button v-if="hasLongContext(entry)" type="button" class="model-plaza__long-context-toggle" :class="{ active: isLongContextExpanded(entry) }" :aria-pressed="isLongContextExpanded(entry)" @click="toggleLongContext(entry)">{{ isLongContextExpanded(entry) ? text.closeLongContext : text.viewLongContext }}</button></span>
              <span class="model-plaza__cache-grid" :class="cacheGridClass(entry)">
                <span v-for="line in cachePriceLines(entry)" :key="line.key" class="model-plaza__cache-line">
                <i>{{ line.label }}</i>
                <span><em>{{ text.original }} {{ formatListedPrice(line.value, entry.currency, activePriceTab(entry).multiplier) }}</em><strong>{{ text.actual }} {{ formatListedActual(line.value, entry.rateMultiplier, activePriceTab(entry).multiplier, entry.currency) }}</strong></span>
                </span>
              </span>
            </span>
          </div>
          <Transition name="model-plaza__long-context-slide">
            <div v-if="hasLongContext(entry) && isLongContextExpanded(entry)" class="model-plaza__official-context-panel">
              <div class="model-plaza__context-prices">
                <div v-for="tier in longContextTiers(entry)" :key="tier.label" class="model-plaza__context-tier">
                  <div class="model-plaza__context-head"><span>{{ text.longContext }}</span><strong>{{ tier.label }}</strong></div>
                  <div class="model-plaza__context-grid model-plaza__context-grid--token">
                    <span class="model-plaza__context-price-item"><small>{{ text.input }}</small><em>{{ text.original }} {{ formatListedPrice(tier.input, entry.currency, activePriceTab(entry).multiplier) }}</em><strong>{{ text.actual }} {{ formatListedActual(tier.input, entry.rateMultiplier, activePriceTab(entry).multiplier, entry.currency) }}</strong></span>
                    <span class="model-plaza__context-price-item"><small>{{ text.output }}</small><em>{{ text.original }} {{ formatListedPrice(tier.output, entry.currency, activePriceTab(entry).multiplier) }}</em><strong>{{ text.actual }} {{ formatListedActual(tier.output, entry.rateMultiplier, activePriceTab(entry).multiplier, entry.currency) }}</strong></span>
                    <span class="model-plaza__context-cache-block"><small>{{ text.cache }}</small><span class="model-plaza__cache-grid" :class="cacheGridClass(tier)"><span v-for="line in cachePriceLines(tier)" :key="line.key" class="model-plaza__cache-line model-plaza__cache-line--tier"><i>{{ line.label }}</i><span><em>{{ text.original }} {{ formatListedPrice(line.value, entry.currency, activePriceTab(entry).multiplier) }}</em><strong>{{ text.actual }} {{ formatListedActual(line.value, entry.rateMultiplier, activePriceTab(entry).multiplier, entry.currency) }}</strong></span></span></span></span>
                  </div>
                </div>
              </div>
            </div>
          </Transition>
        </template>
        <div v-else class="model-plaza__context-prices model-plaza__media-prices">
          <div class="model-plaza__context-tier">
            <div class="model-plaza__context-head"><span>{{ entry.kind === 'image' ? text.perImage : text.perVideo }}</span><strong>{{ entry.kind === 'image' ? text.perImageUnit : text.perVideoUnit }}</strong></div>
            <div class="model-plaza__context-grid">
              <span v-for="tier in entry.tiers" :key="tier.label"><small>{{ tier.label || (entry.kind === 'image' ? text.perImage : text.perVideo) }}</small><em>{{ text.original }} {{ formatListedPrice(tier.original, entry.currency, activePriceTab(entry).multiplier) }}</em><strong>{{ text.actual }} {{ formatListedActual(tier.original, entry.rateMultiplier, activePriceTab(entry).multiplier, entry.currency) }}</strong></span>
            </div>
          </div>
        </div>
        <div class="model-plaza__card-foot"><strong :title="entry.groupName">{{ entry.groupName }}</strong><span>{{ unitLabel(entry) }}</span></div>
      </article>
    </div>
    <div v-else class="model-plaza__table-wrap">
      <table class="model-plaza__table">
        <thead><tr><th>{{ text.model }}</th><th>{{ text.rate }}</th><th>{{ text.group }}</th><th>{{ text.input }}</th><th>{{ text.cache }}</th><th>{{ text.output }}</th><th>{{ text.unit }}</th></tr></thead>
        <tbody>
          <tr v-for="entry in filteredEntries" :key="`${entry.groupId}:${entry.id}`">
            <td><div class="model-plaza__table-model"><span class="model-plaza__provider-logo" :class="entry.provider"><ModelIcon :model="entry.provider" size="20px" /></span><span><code>{{ entry.id }}</code><small>{{ providerLabel(entry.provider) }}</small></span></div><button v-if="hasLongContext(entry)" type="button" class="model-plaza__long-context-toggle model-plaza__long-context-toggle--table" :class="{ active: isLongContextExpanded(entry) }" :aria-pressed="isLongContextExpanded(entry)" @click="toggleLongContext(entry)">{{ isLongContextExpanded(entry) ? text.closeLongContext : text.viewLongContext }}</button><div v-if="priceTabs(entry).length > 1" class="model-plaza__price-tabs model-plaza__price-tabs--table" :class="{ 'model-plaza__price-tabs--many': priceTabs(entry).length >= 3 }" role="tablist"><button v-for="tab in priceTabs(entry)" :key="tab.key" type="button" role="tab" :aria-selected="activePriceTab(entry).key === tab.key" :class="{ active: activePriceTab(entry).key === tab.key }" @click="selectPriceTab(entry, tab.key)"><span>{{ tab.label }}</span><small v-if="tab.current">{{ text.current }}</small></button></div><div v-if="priceTabs(entry).length > 1" class="model-plaza__price-select model-plaza__price-select--table" :class="{ 'model-plaza__price-select--many': priceTabs(entry).length >= 3 }"><Select :model-value="activePriceTab(entry).key" :options="priceSelectOptions(entry)" :aria-label="text.longContext" :searchable="false" @update:model-value="selectPriceTab(entry, String($event))" /></div></td>
            <td><span class="model-plaza__rate">{{ formatRate(entry.rateMultiplier) }}</span></td>
            <td>{{ entry.groupName }}</td>
            <template v-if="entry.kind === 'token'">
              <td><span class="model-plaza__table-original">{{ text.original }} {{ formatListedPrice(entry.input, entry.currency, activePriceTab(entry).multiplier) }}</span><strong class="model-plaza__table-actual">{{ text.actual }} {{ formatListedActual(entry.input, entry.rateMultiplier, activePriceTab(entry).multiplier, entry.currency) }}</strong><span v-for="tier in (isLongContextExpanded(entry) ? longContextTiers(entry) : [])" :key="tier.label" class="model-plaza__table-interval"><small>{{ text.longContext }} · {{ tier.label }}</small><span>{{ text.original }} {{ formatListedPrice(tier.input, entry.currency, activePriceTab(entry).multiplier) }}</span><strong>{{ text.actual }} {{ formatListedActual(tier.input, entry.rateMultiplier, activePriceTab(entry).multiplier, entry.currency) }}</strong></span></td>
              <td>
                <span v-for="line in cachePriceLines(entry)" :key="line.key" class="model-plaza__table-cache-line"><small>{{ line.label }}</small><span class="model-plaza__table-original">{{ text.original }} {{ formatListedPrice(line.value, entry.currency, activePriceTab(entry).multiplier) }}</span><strong class="model-plaza__table-actual">{{ text.actual }} {{ formatListedActual(line.value, entry.rateMultiplier, activePriceTab(entry).multiplier, entry.currency) }}</strong></span>
                <span v-for="tier in (isLongContextExpanded(entry) ? longContextTiers(entry) : [])" :key="tier.label" class="model-plaza__table-interval"><small>{{ text.longContext }} · {{ tier.label }}</small><span v-for="line in cachePriceLines(tier)" :key="line.key" class="model-plaza__table-cache-line"><small>{{ line.label }}</small><span>{{ text.original }} {{ formatListedPrice(line.value, entry.currency, activePriceTab(entry).multiplier) }}</span><strong>{{ text.actual }} {{ formatListedActual(line.value, entry.rateMultiplier, activePriceTab(entry).multiplier, entry.currency) }}</strong></span></span>
              </td>
              <td><span class="model-plaza__table-original">{{ text.original }} {{ formatListedPrice(entry.output, entry.currency, activePriceTab(entry).multiplier) }}</span><strong class="model-plaza__table-actual">{{ text.actual }} {{ formatListedActual(entry.output, entry.rateMultiplier, activePriceTab(entry).multiplier, entry.currency) }}</strong><span v-for="tier in (isLongContextExpanded(entry) ? longContextTiers(entry) : [])" :key="tier.label" class="model-plaza__table-interval"><small>{{ text.longContext }} · {{ tier.label }}</small><span>{{ text.original }} {{ formatListedPrice(tier.output, entry.currency, activePriceTab(entry).multiplier) }}</span><strong>{{ text.actual }} {{ formatListedActual(tier.output, entry.rateMultiplier, activePriceTab(entry).multiplier, entry.currency) }}</strong></span></td>
            </template>
            <td v-else colspan="3"><div class="model-plaza__table-media"><span v-for="tier in entry.tiers" :key="tier.label" class="model-plaza__media-chip"><small>{{ tier.label || (entry.kind === 'image' ? text.perImage : text.perVideo) }}</small><span class="model-plaza__table-original">{{ text.original }} {{ formatListedPrice(tier.original, entry.currency, activePriceTab(entry).multiplier) }}</span><strong class="model-plaza__table-actual">{{ text.actual }} {{ formatListedActual(tier.original, entry.rateMultiplier, activePriceTab(entry).multiplier, entry.currency) }}</strong></span></div></td>
            <td>{{ unitLabel(entry) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="model-plaza__disclaimer"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="9"/><path d="M12 10v6M12 7h.01"/></svg><span>{{ text.disclaimer }}</span></div>
  </section>
</template>

<style>
html.dark .model-plaza.model-plaza--console{--mp-bg:#1e293b;--mp-subtle:#0f172a;--mp-text:#f3f4f6;--mp-muted:#94a3b8;--mp-line:#334155;--mp-line-strong:#475569;--mp-accent:#2dd4bf;--mp-accent-soft:rgba(19,78,74,.55);--mp-green:#2dd4bf}
.dark .model-plaza__pricing-alert{border-color:color-mix(in srgb,#fbbf24 30%,var(--mp-line));border-left-color:#fbbf24;color:#fde68a;background:#33230b}.dark .model-plaza__pricing-alert svg{color:#fbbf24}.dark .model-plaza__pricing-alert strong,.dark .model-plaza__pricing-links a:hover{color:#fef3c7}
</style>

<style scoped>
.model-plaza{width:100%;min-width:0;--mp-bg:#fff;--mp-subtle:#f9fafb;--mp-text:#111827;--mp-muted:#6b7280;--mp-line:#f3f4f6;--mp-line-strong:#e5e7eb;--mp-accent:#0d9488;--mp-accent-soft:#ccfbf1;--mp-green:#0d9488;color:var(--mp-text);font-family:Inter,ui-sans-serif,system-ui,"PingFang SC","Microsoft YaHei",sans-serif}.model-plaza--home{--mp-bg:var(--surface);--mp-subtle:var(--code);--mp-text:var(--ink);--mp-muted:var(--muted);--mp-line:var(--line);--mp-line-strong:var(--line-strong);--mp-accent:var(--forest);--mp-accent-soft:var(--forest-soft);--mp-green:var(--forest)}.model-plaza *{box-sizing:border-box}
 .model-plaza__pricing-alert{display:flex;gap:10px;margin-bottom:16px;padding:12px 14px;border:1px solid color-mix(in srgb,#d97706 32%,var(--mp-line));border-left:3px solid #d97706;border-radius:7px;color:#713f12;background:#fffbeb;font-size:12px;line-height:1.7}.model-plaza__pricing-alert svg{width:16px;height:16px;flex:none;margin-top:2px;color:#b45309}.model-plaza__pricing-alert strong{display:block;margin-bottom:2px;color:#451a03;font-size:12px}.model-plaza__pricing-alert>div{min-width:0}.model-plaza__pricing-links{display:flex;flex-wrap:wrap;gap:4px 10px;margin-top:3px}.model-plaza__pricing-links a{color:var(--mp-accent);text-decoration:underline;text-underline-offset:2px}.model-plaza__pricing-links a:hover{color:#451a03}
.model-plaza__meta{display:flex;align-items:center;justify-content:space-between;gap:18px;margin-bottom:18px;color:var(--mp-muted);font-size:12px}.model-plaza__source{display:inline-flex;align-items:center;gap:8px;padding:7px 10px;border:1px solid var(--mp-line);border-radius:6px;background:var(--mp-bg)}.model-plaza__source i{width:7px;height:7px;border-radius:50%;background:var(--mp-green);box-shadow:0 0 0 3px color-mix(in srgb,var(--mp-green) 14%,transparent)}.model-plaza__updated{text-align:right}.model-plaza__updated strong{display:block;margin-top:3px;color:var(--mp-text);font-weight:650}
.model-plaza__toolbar{display:grid;grid-template-columns:minmax(260px,1fr) auto;align-items:center;gap:12px;margin-bottom:16px}.model-plaza__search-wrap{position:relative;min-width:0}.model-plaza__search-icon{position:absolute;left:14px;top:50%;width:18px;height:18px;color:var(--mp-muted);transform:translateY(-50%);pointer-events:none}.model-plaza__search{width:100%;height:44px;padding:0 44px 0 42px;border:1px solid var(--mp-line-strong);border-radius:7px;outline:none;color:var(--mp-text);background:var(--mp-bg);font-size:14px}.model-plaza__search:focus{border-color:var(--mp-accent);box-shadow:0 0 0 3px color-mix(in srgb,var(--mp-accent) 16%,transparent)}.model-plaza__search::placeholder{color:var(--mp-muted)}
.model-plaza__clear{position:absolute;right:7px;top:50%;display:flex;width:30px;height:30px;padding:0;align-items:center;justify-content:center;border:0;border-radius:5px;color:var(--mp-muted);background:transparent;line-height:0;transform:translateY(-50%);cursor:pointer}.model-plaza__clear:hover{color:var(--mp-text);background:var(--mp-subtle)}.model-plaza__clear svg{display:block;width:16px;height:16px;flex:none}.model-plaza__view-toggle{display:flex;gap:4px;padding:4px;border:1px solid var(--mp-line);border-radius:7px;background:var(--mp-bg)}.model-plaza__view-toggle button{display:grid;width:34px;height:34px;padding:0;place-items:center;border:0;border-radius:5px;color:var(--mp-muted);background:transparent;cursor:pointer}.model-plaza__view-toggle button:hover{color:var(--mp-text);background:var(--mp-subtle)}.model-plaza__view-toggle button.active{color:var(--mp-accent);background:var(--mp-accent-soft)}.model-plaza__view-toggle svg{width:18px;height:18px}
.model-plaza__filters{display:flex;align-items:center;justify-content:space-between;gap:18px;margin-bottom:20px}.model-plaza__groups{display:flex;min-width:0;gap:8px;padding:8px 6px 2px 0;overflow-x:auto;scrollbar-width:none}.model-plaza__groups::-webkit-scrollbar{display:none}.model-plaza__groups button{position:relative;min-height:34px;padding:0 13px;border:1px solid var(--mp-line);border-radius:6px;color:var(--mp-muted);background:var(--mp-bg);font-size:13px;white-space:nowrap;cursor:pointer}.model-plaza__groups button:hover{color:var(--mp-text);border-color:var(--mp-line-strong)}.model-plaza__groups button.active{color:var(--mp-text);border-color:var(--mp-accent);background:var(--mp-accent-soft);font-weight:650}.model-plaza__groups button.is-lowest-rate{padding-right:19px;border-color:color-mix(in srgb,var(--mp-green) 55%,var(--mp-line))}.model-plaza__lowest-badge{position:absolute;right:-6px;top:-9px;display:grid;width:19px;height:19px;place-items:center;border:2px solid var(--mp-bg);border-radius:50%;color:#fff;background:var(--mp-green);box-shadow:0 2px 6px color-mix(in srgb,var(--mp-green) 28%,transparent)}.model-plaza__lowest-badge svg{width:10px;height:10px}.model-plaza__count{flex:none;color:var(--mp-muted);font-size:12px;white-space:nowrap}
.model-plaza__loading{height:250px;overflow:hidden;border:1px solid var(--mp-line);border-radius:8px;background:var(--mp-bg)}.model-plaza__loading span{display:block;width:34%;height:2px;background:var(--mp-accent);animation:mp-loading 1s infinite ease-in-out}@keyframes mp-loading{from{transform:translateX(-110%)}to{transform:translateX(310%)}}.model-plaza__state{display:grid;min-height:250px;padding:30px;place-items:center;align-content:center;border:1px dashed var(--mp-line-strong);border-radius:8px;color:var(--mp-muted);background:var(--mp-bg);text-align:center}.model-plaza__state strong{color:var(--mp-text);font-size:14px}.model-plaza__state span{max-width:420px;margin-top:8px;font-size:12px;line-height:1.7}.model-plaza__state button{margin-top:16px;padding:8px 13px;border:1px solid var(--mp-line-strong);border-radius:6px;color:var(--mp-text);background:var(--mp-bg);cursor:pointer}
 .model-plaza__grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px}.model-plaza__card{display:flex;min-width:0;overflow:hidden;flex-direction:column;border:1px solid var(--mp-line);border-radius:8px;background:var(--mp-bg);box-shadow:0 1px 3px rgba(0,0,0,.04);transition:transform .16s,border-color .16s}.model-plaza__card:hover{transform:translateY(-2px);border-color:var(--mp-line-strong)}.model-plaza__card-head{display:flex;align-items:center;gap:12px;padding:18px 18px 16px;border-bottom:1px solid var(--mp-line)}.model-plaza__provider-logo{display:grid;width:38px;height:38px;flex:none;place-items:center;border:1px solid #d1d5db;border-radius:7px;color:var(--mp-green);background:#fff;box-shadow:0 1px 2px rgb(0 0 0 / 8%);font-size:12px;font-weight:800}.dark .model-plaza__provider-logo{border-color:#d1d5db;background:#fff}.model-plaza__provider-logo.anthropic{color:#b76035}.model-plaza__model-name{min-width:0;flex:1}.model-plaza__model-name strong{display:block;overflow:hidden;color:var(--mp-text);font:650 14px/1.4 ui-monospace,SFMono-Regular,Consolas,monospace;text-overflow:ellipsis;white-space:nowrap}.model-plaza__model-name small{display:block;margin-top:3px;color:var(--mp-muted);font-size:12px}.model-plaza__rate{flex:none;padding:5px 8px;border-radius:5px;color:var(--mp-green);background:color-mix(in srgb,var(--mp-green) 12%,transparent);font:750 12px/1 ui-monospace,SFMono-Regular,Consolas,monospace}
.model-plaza__provider-logo.deepseek{color:#4d6bfe}
.model-plaza__price-tabs{display:flex;flex-wrap:nowrap;gap:6px;padding:12px 18px 0;overflow:hidden}.model-plaza__price-tabs button{position:relative;display:inline-flex;flex:0 0 auto;min-width:88px;min-height:36px;align-items:center;justify-content:center;gap:6px;padding:5px 9px;border:1px solid var(--mp-line-strong);border-radius:5px;color:var(--mp-muted);background:var(--mp-bg);font-size:11px;line-height:1.2;text-align:center;white-space:pre-line;cursor:pointer}.model-plaza__price-tabs button span{display:block;white-space:pre-line}.model-plaza__price-tabs button:hover{color:var(--mp-text);border-color:var(--mp-accent)}.model-plaza__price-tabs button.active{color:var(--mp-text);border-color:var(--mp-accent);background:var(--mp-accent-soft);font-weight:650}.model-plaza__price-tabs button small{position:absolute;top:-8px;right:-5px;padding:2px 4px;border:2px solid var(--mp-bg);border-radius:4px;color:#fff;background:var(--mp-green);font-size:9px;line-height:1}.model-plaza__price-tabs button small::before{content:'•';margin-right:2px;font-size:10px}.model-plaza__price-tabs--table{padding:9px 0 0}.model-plaza__price-tabs--table button{min-height:30px;padding:3px 7px;font-size:10px}
.model-plaza__prices{display:grid;grid-template-columns:repeat(3,1fr);padding:15px 10px 17px}.model-plaza__prices>span{min-width:0;padding:0 8px;border-right:1px solid var(--mp-line)}.model-plaza__prices>span:last-child{border:0}.model-plaza__prices>span>small{display:block;margin-bottom:9px;color:var(--mp-muted);font-size:12px}.model-plaza__original-price,.model-plaza__actual-price{display:flex;min-width:0;align-items:baseline;justify-content:space-between;gap:5px;white-space:nowrap}.model-plaza__actual-price{margin-top:6px}.model-plaza__original-price i,.model-plaza__actual-price i{color:var(--mp-muted);font-size:12px;font-style:normal}.model-plaza__original-price em{overflow:hidden;color:var(--mp-muted);font-size:12px;font-style:normal;text-overflow:ellipsis}.model-plaza__actual-price strong{overflow:hidden;color:var(--mp-green);font-size:14px;text-overflow:ellipsis}.model-plaza__card-foot{display:flex;justify-content:space-between;gap:12px;margin-top:auto;padding:10px 18px;color:var(--mp-muted);background:var(--mp-subtle);font-size:12px}.model-plaza__card-foot strong{overflow:hidden;color:var(--mp-text);font-size:12px;text-overflow:ellipsis;white-space:nowrap}
.model-plaza__cache-line{display:grid;min-width:0;gap:3px;padding:0 0 7px;margin:0 0 7px;border-bottom:1px solid var(--mp-line)}.model-plaza__cache-line:last-child{padding-bottom:0;margin-bottom:0;border-bottom:0}.model-plaza__cache-line>i{color:var(--mp-muted);font-size:10px;font-style:normal;line-height:1.3}.model-plaza__cache-line>span{display:grid;min-width:0;gap:2px}.model-plaza__cache-line em,.model-plaza__cache-line strong{overflow:hidden;font-size:11px;font-style:normal;line-height:1.35;text-overflow:ellipsis;white-space:nowrap}.model-plaza__cache-line em{color:var(--mp-muted)}.model-plaza__cache-line strong{color:var(--mp-green)}.model-plaza__cache-line--tier{padding-bottom:5px;margin-bottom:5px}
.model-plaza__context-prices{padding:0 18px 15px}.model-plaza__context-tier{padding-top:12px;border-top:1px solid var(--mp-line)}.model-plaza__context-tier+.model-plaza__context-tier{margin-top:12px}.model-plaza__context-head{display:flex;align-items:center;justify-content:space-between;margin-bottom:10px;color:var(--mp-muted);font-size:12px}.model-plaza__context-head strong{padding:3px 6px;border-radius:4px;color:var(--mp-green);background:color-mix(in srgb,var(--mp-green) 12%,transparent);font-size:12px}.model-plaza__context-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px}.model-plaza__context-grid>span{display:grid;min-width:0;gap:3px}.model-plaza__context-grid small{color:var(--mp-muted);font-size:12px;font-weight:400;line-height:1.3}.model-plaza__context-grid em,.model-plaza__context-grid strong{overflow:hidden;font-style:normal;text-overflow:ellipsis;white-space:nowrap}.model-plaza__context-grid em{color:var(--mp-muted);font-size:12px}.model-plaza__context-grid strong{color:var(--mp-green);font-size:12px}
.model-plaza__table-wrap{overflow-x:auto;border:1px solid var(--mp-line);border-radius:8px;background:var(--mp-bg)}.model-plaza__table{width:100%;min-width:940px;border-collapse:collapse}.model-plaza__table th{padding:12px 16px;color:var(--mp-muted);background:var(--mp-subtle);font-size:11px;font-weight:650;text-align:left}.model-plaza__table td{padding:14px 16px;border-top:1px solid var(--mp-line);font-size:13px}.model-plaza__table-model{display:flex;align-items:center;gap:10px}.model-plaza__table-model .model-plaza__provider-logo{width:30px;height:30px;border-radius:6px;font-size:10px}.model-plaza__table-model code{display:block;color:var(--mp-text);font-size:12px}.model-plaza__table-model small{display:block;margin-top:3px;color:var(--mp-muted);font-size:10px}.model-plaza__table-original{display:block;color:var(--mp-muted);font-size:10px;white-space:nowrap}.model-plaza__table-actual{display:block;margin-top:5px;color:var(--mp-green);font-size:12px;white-space:nowrap}.model-plaza__disclaimer{display:flex;gap:10px;margin-top:22px;padding:13px 15px;border-left:3px solid var(--mp-line-strong);color:var(--mp-muted);background:var(--mp-subtle);font-size:11px;line-height:1.7}.model-plaza__disclaimer svg{width:16px;height:16px;flex:none;margin-top:1px}
.model-plaza__table-model .model-plaza__table-tier{margin-top:6px;color:var(--mp-green);font-weight:650}.model-plaza__table-interval{display:grid;margin-top:8px;padding-top:7px;border-top:1px solid var(--mp-line);color:var(--mp-muted);font-size:10px;white-space:nowrap}.model-plaza__table-interval small{margin-bottom:2px;color:var(--mp-green);font-weight:700}.model-plaza__table-interval strong{margin-top:2px;color:var(--mp-green);font-size:11px}.model-plaza__table-media{display:flex;flex-wrap:wrap;gap:8px}.model-plaza__media-chip{display:grid;min-width:92px;gap:2px;padding:6px 8px;border:1px solid var(--mp-line);border-radius:6px;background:var(--mp-subtle)}.model-plaza__media-chip small{color:var(--mp-green);font-size:10px;font-weight:700}
.model-plaza__table-cache-line{display:grid;gap:2px;padding-bottom:7px;margin-bottom:7px;border-bottom:1px solid var(--mp-line)}.model-plaza__table-cache-line:last-child{padding-bottom:0;margin-bottom:0;border-bottom:0}.model-plaza__table-cache-line>small{color:var(--mp-muted);font-size:10px}.model-plaza__table-interval .model-plaza__table-cache-line{padding-top:4px;margin-top:4px}
@media(max-width:900px){.model-plaza__grid{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:640px){.model-plaza__meta{align-items:flex-start;flex-direction:column}.model-plaza__updated{text-align:left}.model-plaza__updated strong{display:inline;margin-left:6px}.model-plaza__filters{align-items:flex-start;flex-direction:column;gap:10px}.model-plaza__groups{width:100%}.model-plaza__grid{grid-template-columns:1fr}.model-plaza__count{white-space:normal}}

/* Provider and group levels are stacked vertically; options wrap within each
   level so the page never needs horizontal scrolling. */
.model-plaza__filters{display:grid;width:100%;min-width:0;grid-template-columns:minmax(0,1fr);align-self:stretch;justify-self:stretch;align-items:start;gap:12px;margin-bottom:20px;padding:14px;border:1px solid var(--mp-line);border-radius:8px;background:var(--mp-bg)}
.model-plaza__catalog{display:block;width:100%;min-width:0;align-self:stretch;justify-self:stretch}
.model-plaza__providers{position:relative;width:100%;min-width:0;padding-bottom:14px}
.model-plaza__providers::after{position:absolute;right:0;bottom:0;left:0;height:1px;background:var(--mp-line);content:""}
.model-plaza__group-panel{width:100%;min-width:0;padding-top:14px}
.model-plaza__catalog-heading{display:flex;align-items:flex-end;gap:12px;margin-bottom:9px;color:var(--mp-text);font-size:12px;font-weight:700}
.model-plaza__catalog-title{display:flex;align-items:center;gap:7px}
.model-plaza__catalog-heading small{display:block;margin-top:4px;color:var(--mp-muted);font-size:10px;font-weight:400;line-height:1.4}
.model-plaza__catalog-heading strong{display:inline-grid;min-width:20px;height:20px;padding:0 6px;place-items:center;border-radius:999px;color:var(--mp-muted);background:var(--mp-subtle);font-size:10px;font-weight:700;line-height:1}
.model-plaza__provider-list,.model-plaza__group-list{display:flex;width:100%;min-width:0;flex-wrap:wrap;column-gap:7px;row-gap:14px;padding-top:6px}
.model-plaza__provider-list button,.model-plaza__group-list button{position:relative;display:inline-flex;min-height:36px;align-items:center;justify-content:space-between;gap:10px;width:auto;max-width:100%;padding:0 10px;border:1px solid var(--mp-line);border-radius:6px;color:var(--mp-muted);background:var(--mp-bg);font-size:12px;text-align:left;cursor:pointer;transition:background .15s,border-color .15s,color .15s}
.model-plaza__provider-list button:hover,.model-plaza__group-list button:hover{color:var(--mp-text);border-color:var(--mp-line-strong);background:var(--mp-subtle)}
.model-plaza__provider-list button.active,.model-plaza__group-list button.active{color:var(--mp-text);border-color:var(--mp-accent);background:var(--mp-accent-soft);font-weight:650}
.model-plaza__provider-list button span:first-child,.model-plaza__group-list button > span:not(.model-plaza__group-meta){display:flex;min-width:0;align-items:center;gap:8px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.model-plaza__provider-list button small,.model-plaza__group-list button small{flex:none;color:var(--mp-muted);font-size:10px;font-variant-numeric:tabular-nums}
.model-plaza__group-meta{display:inline-flex;flex:none;align-items:center;gap:5px}
.model-plaza__group-list button .model-plaza__lowest-marker{position:absolute;top:-12px;left:-5px;margin:0;padding:4px 6px;border:2px solid var(--mp-bg);border-radius:4px;color:#9a3412;background:#ffedd5;font-size:9px;font-weight:800;line-height:1;z-index:1}
.model-plaza__group-list button .model-plaza__group-rate{margin:0;padding:5px 7px;border-radius:5px;color:var(--mp-green);background:color-mix(in srgb,var(--mp-green) 13%,transparent);font-size:11px;font-weight:800;line-height:1;box-shadow:inset 0 0 0 1px color-mix(in srgb,var(--mp-green) 24%,transparent)}
.model-plaza__group-list button.active .model-plaza__group-rate{color:#fff;background:var(--mp-green);box-shadow:none}
.model-plaza__provider-dot{display:block;width:7px;height:7px;flex:none;border-radius:50%;background:var(--mp-accent)}
.model-plaza__provider-dot.anthropic{background:#b76035}.model-plaza__provider-dot.openai{background:#10a37f}.model-plaza__provider-dot.gemini{background:#4285f4}.model-plaza__provider-dot.grok{background:#111827}.model-plaza__provider-dot.deepseek{background:#4d6bfe}
.model-plaza__group-list{overflow:visible}
.model-plaza__group-list button.is-lowest-rate{padding-left:10px;border-color:color-mix(in srgb,var(--mp-green) 55%,var(--mp-line))}
.model-plaza__no-groups{padding:24px 12px;border:1px dashed var(--mp-line-strong);color:var(--mp-muted);font-size:12px;text-align:center}
.model-plaza__count{justify-self:end}
@media(max-width:640px){.model-plaza__count{justify-self:start}}

/* Keep the primary prices balanced and give cache variants their own full-width row. */
.model-plaza__price-select{display:none;width:calc(100% - 36px);margin:10px 18px 0}
.model-plaza__price-select :deep(.select-trigger){min-height:36px;padding:7px 10px;border-radius:5px;font-size:12px}
.model-plaza__prices{grid-template-columns:repeat(2,minmax(0,1fr));gap:16px 0;padding:15px 18px 17px}
.model-plaza__prices>span{padding:0 10px;border-right:0}
.model-plaza__prices>.model-plaza__price-item:first-child{padding-left:0;border-right:1px solid var(--mp-line)}
.model-plaza__prices>.model-plaza__price-item:nth-child(2){padding-right:0}
.model-plaza__cache-block{grid-column:1 / -1;padding:12px 0 0;border-top:1px solid var(--mp-line)}
.model-plaza__cache-grid{display:grid;gap:8px;margin-top:8px}
.model-plaza__cache-grid--three{grid-template-columns:repeat(3,minmax(0,1fr))}
.model-plaza__cache-grid--two{grid-template-columns:repeat(2,minmax(0,1fr))}
.model-plaza__cache-grid--one{grid-template-columns:minmax(0,1fr)}
.model-plaza__prices>.model-plaza__cache-block{padding:12px 0 0}
.model-plaza__cache-line{display:grid;min-width:0;gap:4px;padding:8px 9px;margin:0;border:1px solid var(--mp-line);border-radius:5px;background:var(--mp-subtle)}
.model-plaza__cache-grid .model-plaza__cache-line:last-child{padding:8px 9px;margin:0;border:1px solid var(--mp-line)}
.model-plaza__cache-line>i{font-size:10px;font-weight:600}
.model-plaza__cache-line>span{gap:2px}
.model-plaza__cache-line em,.model-plaza__cache-line strong{font-size:10px}
.model-plaza__context-grid--token{grid-template-columns:repeat(2,minmax(0,1fr));gap:12px 0}
.model-plaza__context-grid--token>span{padding:0 10px}
.model-plaza__context-grid--token>.model-plaza__context-price-item:first-child{padding-left:0;border-right:1px solid var(--mp-line)}
.model-plaza__context-grid--token>.model-plaza__context-price-item:nth-child(2){padding-right:0}
.model-plaza__context-grid--token>.model-plaza__context-cache-block{padding:10px 0 0}
.model-plaza__context-cache-block{grid-column:1 / -1;padding:10px 0 0;border-top:1px solid var(--mp-line)}
.model-plaza__context-cache-block>.model-plaza__cache-grid{margin-top:7px}
.model-plaza__card{container-type:inline-size}
@container (max-width:360px){.model-plaza__cache-grid--three{grid-template-columns:1fr}}
@container (max-width:320px){.model-plaza__price-tabs--many{display:none}.model-plaza__price-select--many{display:block}}
@media(max-width:640px){.model-plaza__cache-grid--three,.model-plaza__cache-grid--two{grid-template-columns:1fr}.model-plaza__prices,.model-plaza__context-grid--token{gap:12px 0}}
@media(max-width:640px){.model-plaza__price-tabs{display:none}.model-plaza__price-select{display:block}}
.model-plaza__card{position:relative}
.model-plaza__long-context-toggle{flex:none;max-width:150px;padding:5px 7px;border:1px solid var(--mp-line-strong);border-radius:5px;color:var(--mp-muted);background:var(--mp-bg);font-size:12px;line-height:1.25;text-align:center;cursor:pointer}.model-plaza__long-context-toggle:hover{color:var(--mp-text);border-color:var(--mp-accent)}.model-plaza__long-context-toggle.active{color:var(--mp-text);border-color:var(--mp-accent);background:var(--mp-accent-soft);font-weight:650}
.model-plaza__long-context-toggle--table{display:block;margin-top:8px;max-width:180px}
.model-plaza__official-context-panel{position:absolute;z-index:3;inset:72px 0 38px;overflow:auto;border-top:1px solid var(--mp-line-strong);background:var(--mp-bg);box-shadow:0 -8px 18px rgb(15 23 42 / 12%)}.model-plaza__official-context-panel .model-plaza__context-prices{padding-top:8px}.model-plaza__long-context-slide-enter-active,.model-plaza__long-context-slide-leave-active{transition:transform .24s ease,opacity .2s ease}.model-plaza__long-context-slide-enter-from,.model-plaza__long-context-slide-leave-to{opacity:0;transform:translateY(100%)}
.model-plaza__cache-heading{display:flex;align-items:center;justify-content:space-between;gap:8px;min-height:25px}.model-plaza__cache-heading small,.model-plaza__cache-heading .model-plaza__long-context-toggle{align-self:center;line-height:1.3}.model-plaza__cache-heading small{color:var(--mp-muted);font-size:12px;font-weight:400}.model-plaza__cache-heading .model-plaza__long-context-toggle{position:relative;z-index:4;margin:0}
.model-plaza__media-prices{padding-top:15px}
.model-plaza__official-context-panel{border-top:0}.model-plaza__official-context-panel .model-plaza__context-tier:first-child{padding-top:0;border-top:0}.model-plaza__context-prices>.model-plaza__context-tier:first-child{padding-top:0;border-top:0}
</style>
