<template>
  <AppLayout>
    <div class="creative-docs" :style="{ '--docs-group-top': `${groupBarTop}px` }">
      <CreativeOverview @start="scrollToSection('agent')" />
      <div v-if="loading" class="creative-docs-empty" role="status">正在加载分组文档…</div>
      <div v-else-if="loadError" class="creative-docs-empty" role="alert">{{ loadError }} <button type="button" @click="loadGroups">重新加载</button></div>
      <div v-else-if="!groups.length" class="creative-docs-empty">暂无可用的分组文档，请联系管理员为分组关联接入文档。</div>
      <template v-else>
      <nav ref="groupBar" class="creative-docs-groups" aria-label="切换文档分组">
        <div v-for="category in categories.filter(category => groups.some(group => group.kind === category.kind))" :key="category.kind" class="creative-docs-group-category" role="group" :aria-label="category.label">
          <span class="creative-docs-group-label">{{ category.label }}</span>
          <button v-for="group in groups.filter(group => group.kind === category.kind)" :key="group.id" type="button" :data-group-id="group.id" :aria-pressed="selectedGroupId === group.id" @click="selectGroup(group.id)">
            <span class="creative-docs-group-logo" aria-hidden="true"><img :src="`/creative-docs/assets/brands/${groupLogos[group.documentId] ?? 'openai'}.svg`" alt="" width="20" height="20"></span>
            <span>{{ group.name }}</span>
          </button>
        </div>
      </nav>
      <div class="creative-docs-workspace">
        <aside class="creative-docs-sidebar" :style="{ top: `${sidebarTop}px`, '--docs-sidebar-top': `${sidebarTop}px` }" aria-label="分组文档">
          <div class="creative-docs-sidebar-caption">文档导航 <span>DOCUMENTS</span></div>
          <div class="creative-docs-index-caption">章节目录</div>
          <nav class="creative-docs-index" aria-label="当前文档目录">
            <a v-for="section in sections" :key="section.label" :href="`#${section.id}`" :aria-current="activeSection === section.id ? 'location' : undefined" @click="navigateDocument">{{ section.label }}</a>
          </nav>
        </aside>
        <div class="creative-docs-content">
          <CreativeAgentPrompt v-if="selectedGroup" :group="selectedGroup" />
          <iframe ref="frame" src="/creative-docs/index.html" :title="t('creativeDocs.title')" class="creative-docs-frame" :style="{ height: `${frameHeight}px` }" @load="onFrameLoad"></iframe>
        </div>
      </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import CreativeOverview from '@/components/group/CreativeOverview.vue'
import CreativeAgentPrompt from '@/components/group/CreativeAgentPrompt.vue'
import { useTheme } from '@/composables/useTheme'
import { getAvailable, getCategories } from '@/api/groups'
import { resolveDocumentationGroups, type DocumentationGroup } from '@/utils/creativeDocumentation'
import { documentCatalog } from '../../../public/creative-docs/catalog.js'

const { t } = useI18n()
const { isDark } = useTheme()
const categories = [{ kind: 'image', label: '图片生成' }, { kind: 'video', label: '视频生成' }] as const
const groups = ref<DocumentationGroup[]>([])
const groupLogos: Record<string, string> = { banana: 'gemini', 'image-az': 'azure', 'image-adobe': 'adobe', seedream: 'bytedance', 'mj-v8.2': 'midjourney', video: 'alibaba', 'sd-video': 'bytedance' }
const selectedGroupId = ref('')
const selectedGroup = computed(() => groups.value.find(group => group.id === selectedGroupId.value))
const loading = ref(true)
const loadError = ref('')
const groupBar = ref<HTMLElement | null>(null)
const groupBarTop = ref(64)
const frame = ref<HTMLIFrameElement | null>(null)
const frameHeight = ref(1)
const sidebarTop = ref(84)
const sections = ref<{ id: string; label: string }[]>([])
const activeSection = ref('agent')
let navigating = false
let frameDocument: Document | null = null
let contentObserver: ResizeObserver | null = null
let groupBarObserver: ResizeObserver | null = null
let disposed = false

async function loadGroups() {
  loading.value = true
  loadError.value = ''
  try {
    const [available, snapshot] = await Promise.all([getAvailable(), getCategories()])
    if (disposed) return
    groups.value = resolveDocumentationGroups(available, snapshot.documentation ?? {})
    if (!groups.value.length && import.meta.env.DEV) {
      groups.value = documentCatalog.map(doc => ({ id: `preview:${doc.id}`, name: doc.label, description: '', documentId: doc.id, kind: doc.kind }))
    }
    selectedGroupId.value = groups.value[0]?.id ?? ''
  } catch {
    if (!disposed) loadError.value = '分组文档加载失败，请稍后重试。'
  } finally {
    if (!disposed) loading.value = false
  }
}

function syncTheme() {
  frameDocument?.documentElement.classList.toggle('dark', isDark.value)
}

function syncViewport() {
  const headerBottom = document.querySelector('.tokenfree-console header')?.getBoundingClientRect().bottom ?? 64
  groupBarTop.value = headerBottom
  sidebarTop.value = headerBottom + (groupBar.value?.getBoundingClientRect().height ?? 0) + 20
  if (navigating || !sections.value.length) return
  const boundary = sidebarTop.value + 1
  let current = sections.value[0].id
  for (const section of sections.value) {
    const top = sectionTop(section.id)
    if (top !== undefined && top <= boundary) current = section.id
  }
  activeSection.value = current
}

function sectionTop(id: string) {
  if (id === 'agent') return document.getElementById('agent')?.getBoundingClientRect().top
  const section = frameDocument?.getElementById(id)
  if (section && frame.value) return frame.value.getBoundingClientRect().top + section.getBoundingClientRect().top
}

function syncFrameHeight() {
  if (!frameDocument?.body) return
  const height = Math.ceil(frameDocument.body.getBoundingClientRect().height)
  if (height === frameHeight.value) return
  const scrollTop = window.scrollY
  frameHeight.value = height
  void nextTick(() => {
    window.scrollTo({ top: scrollTop, behavior: 'instant' })
    syncViewport()
  })
}

function scrollToSection(id: string, smooth = true) {
  const top = sectionTop(id)
  if (top === undefined) return
  syncViewport()
  const animate = smooth && !window.matchMedia('(prefers-reduced-motion: reduce)').matches
  // Keep the clicked chapter highlighted while the page passes other chapters.
  navigating = animate && Math.abs(top - sidebarTop.value) > 1
  activeSection.value = id
  window.scrollTo({
    top: window.scrollY + top - sidebarTop.value,
    behavior: animate ? 'smooth' : 'instant',
  })
}

function finishNavigation() {
  navigating = false
  syncViewport()
}

function navigateDocument(event: MouseEvent) {
  if (event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return
  const href = (event.target as Element | null)?.closest('a')?.getAttribute('href')
  if (!href?.startsWith('#')) return
  event.preventDefault()
  scrollToSection(href.slice(1))
}

function readDocumentIndex() {
  sections.value = [{ id: 'agent', label: 'Agent 快速接入' }, ...Array.from(frameDocument?.querySelectorAll<HTMLElement>('[data-doc-index]:not([hidden])') ?? []).filter(section => section.id !== 'agent').map(section => ({
    id: section.id, label: section.dataset.docIndex ?? '',
  }))]
  syncFrameHeight()
  void nextTick(syncViewport)
}

function sendSelection() {
  if (!frameDocument || !selectedGroup.value) return
  const event = frameDocument.createEvent('CustomEvent')
  event.initCustomEvent('creative-docs:select', false, false, {
    documentId: selectedGroup.value.documentId,
    groupName: selectedGroup.value.name,
  })
  frameDocument.dispatchEvent(event)
}

async function selectGroup(id: string) {
  const group = groups.value.find(group => group.id === id)
  if (!group || selectedGroupId.value === group.id) return
  navigating = false
  const index = Math.max(0, sections.value.findIndex(section => section.id === activeSection.value))
  const previousSections = sections.value.slice(0, index + 1).reverse()
  const followSection = (sectionTop(activeSection.value) ?? Infinity) <= sidebarTop.value + 1
  selectedGroupId.value = group.id
  sendSelection()
  // If this document lacks the current chapter, keep the nearest preceding shared chapter.
  const target = previousSections.map(previous => sections.value.find(section => section.label === previous.label)).find(Boolean) ?? sections.value[0]
  await nextTick()
  if (selectedGroupId.value !== group.id) return
  if (followSection && target) scrollToSection(target.id, false)
  syncViewport()
}

function disconnectDocument() {
  contentObserver?.disconnect()
  groupBarObserver?.disconnect()
  frameDocument?.removeEventListener('click', navigateDocument)
  frameDocument?.removeEventListener('creative-docs:rendered', readDocumentIndex)
  window.removeEventListener('scroll', syncViewport)
  window.removeEventListener('resize', syncViewport)
  document.removeEventListener('scrollend', finishNavigation)
  window.removeEventListener('wheel', finishNavigation)
  window.removeEventListener('touchstart', finishNavigation)
  navigating = false
}

function onFrameLoad() {
  disconnectDocument()
  frameDocument = frame.value?.contentDocument ?? null
  frameDocument?.documentElement.classList.add('console-embedded')
  syncTheme()
  frameDocument?.addEventListener('click', navigateDocument)
  frameDocument?.addEventListener('creative-docs:rendered', readDocumentIndex)
  sendSelection()
  readDocumentIndex()
  if (frameDocument?.body) {
    contentObserver = new ResizeObserver(syncFrameHeight)
    contentObserver.observe(frameDocument.body)
  }
  if (groupBar.value) {
    groupBarObserver = new ResizeObserver(syncViewport)
    groupBarObserver.observe(groupBar.value)
  }
  window.addEventListener('scroll', syncViewport, { passive: true })
  window.addEventListener('resize', syncViewport)
  document.addEventListener('scrollend', finishNavigation)
  window.addEventListener('wheel', finishNavigation, { passive: true })
  window.addEventListener('touchstart', finishNavigation, { passive: true })
}

watch(isDark, syncTheme)
onMounted(loadGroups)
onBeforeUnmount(() => { disposed = true; disconnectDocument() })
</script>

<style scoped>
@font-face { font-family: 'JetBrains Mono'; src: url('/tokenfree/fonts/JetBrainsMono-Variable.ttf') format('truetype'); font-weight: 100 800; font-display: swap; unicode-range: U+0000-024F,U+1E00-1EFF,U+2000-206F,U+20A0-20CF,U+2190-21FF; }
.creative-docs { width: 100%; max-width: 1480px; margin-inline: auto; font-family: 'JetBrains Mono', 'Segoe UI', 'Microsoft YaHei', sans-serif; color: rgb(var(--tf-console-neutral-900)); font-size: 12px; }
:global(.dark .creative-docs) { color: rgb(var(--tf-console-neutral-50)); }
.creative-docs-workspace { display: grid; grid-template-columns: 238px minmax(0,1fr); align-items: start; width: 100%; padding-top: 28px; }
.creative-docs-sidebar { position: sticky; min-width: 0; max-height: calc(100dvh - var(--docs-sidebar-top, 84px) - 20px); overflow-y: auto; overscroll-behavior-y: contain; padding: 4px 20px 30px 0; border-right: 1px solid var(--tf-console-line); }
.creative-docs-sidebar-caption { display: flex; justify-content: space-between; padding: 0 10px 20px; opacity: .6; }
.creative-docs-sidebar-caption span { font-size: 12px; letter-spacing: 1px; opacity: .65; }
.creative-docs-groups { position: sticky; top: var(--docs-group-top,64px); z-index: 20; display: flex; flex-wrap: wrap; align-items: center; gap: 10px 20px; margin-top: 18px; padding: 12px; border: 1px solid var(--tf-console-line); border-radius: 12px; background: var(--tf-console-surface); box-shadow: 0 8px 24px rgb(0 0 0 / .06); }
.creative-docs-group-category { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; min-width: 0; }
.creative-docs-group-label { padding: 0 8px; opacity: .55; white-space: nowrap; }
.creative-docs-group-category button { display: inline-flex; align-items: center; gap: 8px; padding: 8px 10px; border: 1px solid transparent; border-radius: 8px; font-size: 12px; line-height: 1.6; text-align: left; overflow-wrap: anywhere; transition: background .2s ease, border-color .2s ease; }
.creative-docs-group-category button:hover { background: var(--tf-console-table-head); }
.creative-docs-group-category button[aria-pressed="true"] { color: var(--tf-console-active-ink); background: var(--tf-console-active-bg); border-color: var(--tf-console-line); }
.creative-docs-group-logo { display: grid; place-items: center; flex: none; width: 28px; height: 28px; border: 1px solid #e5e7eb; border-radius: 7px; background: #fff; }
.creative-docs-group-logo img { display: block; }
.creative-docs-index-caption { margin: 20px 10px 10px; opacity: .6; }
.creative-docs-index { position: relative; display: flex; flex-direction: column; gap: 2px; }
.creative-docs-index a { padding: 8px 12px; border-left: 2px solid transparent; border-radius: 0 7px 7px 0; line-height: 1.7; opacity: .6; transition: none; animation: none; }
.creative-docs-index a[aria-current],.creative-docs-index a:hover { color: var(--tf-console-active-ink); background: var(--tf-console-active-bg); opacity: 1; }
.creative-docs-index a[aria-current] { border-left-color: var(--tf-console-active-ink); }
.creative-docs-content { min-width: 0; padding-left: 30px; }
.creative-docs-frame { display: block; width: 100%; border: 0; background: transparent; overflow-anchor: none; }
.creative-docs-empty { padding: 64px 24px; text-align: center; }
.creative-docs-empty button { text-decoration: underline; padding-left: 10px; }
.creative-docs a:focus-visible,.creative-docs button:focus-visible { outline: 2px solid var(--tf-console-active-ink); outline-offset: 3px; }
@media(max-width:1050px) { .creative-docs-workspace { grid-template-columns: 205px minmax(0,1fr); } .creative-docs-content { padding-left: 20px; } }
@media(max-width:700px) { .creative-docs-workspace { grid-template-columns: minmax(0,1fr); } .creative-docs-sidebar { position: static; max-height: none; overflow-y: visible; border-right: 0; border-bottom: 1px solid var(--tf-console-line); padding-right: 0; } .creative-docs-content { padding: 24px 0 0; } }
</style>
