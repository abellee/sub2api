<template>
  <StudioLayout>
    <div class="creative-docs" :style="{ '--docs-group-top': groupBarTop + 'px' }">
      <CreativeOverview @start="scrollToSection(sections[0]?.id ?? '')" />
      <div v-if="loading" class="creative-docs-empty" role="status">正在加载文档…</div>
      <div v-else-if="loadError" class="creative-docs-empty" role="alert">{{ loadError }} <button type="button" @click="loadDocuments">重新加载</button></div>
      <div v-else-if="!articles.length" class="creative-docs-empty">暂无已发布的文章。</div>
      <template v-else>
        <nav ref="groupBar" class="creative-docs-groups" aria-label="切换文档">
          <div v-for="category in visibleCategories" :key="category.id" class="creative-docs-group-category" role="group" :aria-label="category.name">
            <span class="creative-docs-group-label">{{ category.name }}</span>
            <button v-for="article in articles.filter(item => item.category_id === category.id)" :key="article.id" type="button" :data-article-id="article.id" :aria-pressed="selectedArticleId === article.id" @click="selectArticle(article.id)">
              <span v-if="article.logo" class="creative-docs-group-logo" aria-hidden="true"><img :src="'/creative-docs/assets/brands/' + article.logo + '.svg'" alt="" width="20" height="20"></span>
              <span>{{ article.name }}</span>
            </button>
          </div>
        </nav>
        <div class="creative-docs-workspace">
          <aside class="creative-docs-sidebar" :style="{ top: sidebarTop + 'px', '--docs-sidebar-top': sidebarTop + 'px' }" aria-label="文档导航">
            <div class="creative-docs-sidebar-caption">文档导航 <span>DOCUMENTS</span></div>
            <div class="creative-docs-index-caption">章节目录</div>
            <nav class="creative-docs-index" aria-label="当前文档目录">
              <a v-for="section in sections" :key="section.id" :href="'#' + section.id" :style="{ paddingLeft: 12 + Math.max(0, section.level - 2) * 12 + 'px' }" :aria-current="activeSection === section.id ? 'location' : undefined" @click="navigateDocument">{{ section.label }}</a>
            </nav>
          </aside>
          <div v-if="selectedArticle" class="creative-docs-content">
            <CreativeAgentPrompt :key="selectedArticle.id" :article="selectedArticle" />
            <article ref="contentElement" class="markdown-body creative-docs-markdown" :aria-label="selectedArticle.name" @click="navigateDocument" v-html="rendered.html"></article>
          </div>
        </div>
      </template>
    </div>
  </StudioLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import StudioLayout from '@/components/user/dashboard/studio/DashboardStudioLayout.vue'
import CreativeOverview from '@/components/group/CreativeOverview.vue'
import CreativeAgentPrompt from '@/components/group/CreativeAgentPrompt.vue'
import { getDocumentation, type DocumentationSnapshot } from '@/api/documentation'
import { renderDocumentationMarkdown } from '@/utils/creativeDocumentation'
import '@/styles/announcement-markdown.css'

const route = useRoute()
const router = useRouter()
const snapshot = ref<DocumentationSnapshot>({ revision: 0, categories: [], articles: [] })
const articles = computed(() => snapshot.value.articles.filter(article => article.published))
const visibleCategories = computed(() => snapshot.value.categories.filter(category => articles.value.some(article => article.category_id === category.id)))
const selectedArticleId = ref('')
const selectedArticle = computed(() => articles.value.find(article => article.id === selectedArticleId.value))
const rendered = computed(() => renderDocumentationMarkdown(selectedArticle.value?.content ?? '', true))
const sections = computed(() => [...(selectedArticle.value?.agent.enabled ? [{ id: 'agent', label: 'Agent 快速接入', level: 2 }] : []), ...rendered.value.sections])
const loading = ref(true)
const loadError = ref('')
const groupBar = ref<HTMLElement | null>(null)
const contentElement = ref<HTMLElement | null>(null)
const groupBarTop = ref(64)
const sidebarTop = ref(84)
const activeSection = ref('')
let navigating = false
let disposed = false
let observer: ResizeObserver | null = null
let navigationTimer: ReturnType<typeof setTimeout> | undefined

async function loadDocuments() {
  loading.value = true
  loadError.value = ''
  try {
    const data = await getDocumentation()
    if (disposed) return
    snapshot.value = data
    chooseInitialArticle()
  } catch {
    if (!disposed) loadError.value = '文档加载失败，请稍后重试。'
  } finally {
    if (!disposed) { loading.value = false; await nextTick(); observeContent() }
  }
}
function chooseInitialArticle() {
  clearTimeout(navigationTimer); navigating = false
  const requested = String(route.query.article ?? '')
  selectedArticleId.value = articles.value.find(article => article.id === requested)?.id ?? articles.value[0]?.id ?? ''
  activeSection.value = sections.value[0]?.id ?? ''
}
function sectionTop(id: string) { return document.getElementById(id)?.getBoundingClientRect().top }
function syncViewport() {
  const headerBottom = document.querySelector('.tokenfree-console header')?.getBoundingClientRect().bottom ?? 64
  groupBarTop.value = headerBottom
  sidebarTop.value = headerBottom + (groupBar.value?.getBoundingClientRect().height ?? 0) + 20
  if (navigating || !sections.value.length) return
  let current = sections.value[0].id
  for (const section of sections.value) { const top = sectionTop(section.id); if (top !== undefined && top <= sidebarTop.value + 1) current = section.id }
  activeSection.value = current
}
function scrollToSection(id: string, smooth = true) {
  const top = sectionTop(id)
  if (top === undefined) return
  syncViewport()
  const animate = smooth && !window.matchMedia('(prefers-reduced-motion: reduce)').matches
  navigating = animate && Math.abs(top - sidebarTop.value) > 1
  activeSection.value = id
  window.scrollTo({ top: window.scrollY + top - sidebarTop.value, behavior: animate ? 'smooth' : 'instant' })
  clearTimeout(navigationTimer)
  // Browsers without scrollend still resume chapter tracking.
  if (navigating) navigationTimer = setTimeout(finishNavigation, 1800)
}
function finishNavigation() { clearTimeout(navigationTimer); navigating = false; syncViewport() }
function navigateDocument(event: MouseEvent) {
  if (event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return
  const button = (event.target as Element | null)?.closest<HTMLButtonElement>('button[data-copy-code]')
  if (button) {
    const code = button.closest('.docs-code-block')?.querySelector('pre code')?.textContent ?? ''
    void navigator.clipboard.writeText(code).then(() => { button.textContent = '已复制' }).catch(() => { button.textContent = '复制失败，请重试' })
    return
  }
  const href = (event.target as Element | null)?.closest('a')?.getAttribute('href')
  if (!href?.startsWith('#') || !sections.value.some(section => section.id === href.slice(1))) return
  event.preventDefault()
  scrollToSection(href.slice(1))
}
async function selectArticle(id: string) {
  if (selectedArticleId.value === id) return
  const index = Math.max(0, sections.value.findIndex(section => section.id === activeSection.value))
  const previous = sections.value.slice(0, index + 1).reverse()
  const follow = (sectionTop(activeSection.value) ?? Infinity) <= sidebarTop.value + 1
  navigating = false
  selectedArticleId.value = id
  const target = previous.map(section => sections.value.find(next => next.label === section.label)).find(Boolean) ?? sections.value[0]
  await router.replace({ query: { ...route.query, article: id }, hash: '' })
  await nextTick()
  observeContent()
  if (follow && target) scrollToSection(target.id, false)
}
function observeContent() {
  observer?.disconnect()
  if (groupBar.value) observer?.observe(groupBar.value)
  if (contentElement.value) observer?.observe(contentElement.value)
  syncViewport()
}
watch(() => route.query.article, async () => {
  if (route.query.article !== selectedArticleId.value || !articles.value.some(article => article.id === selectedArticleId.value)) chooseInitialArticle()
  await nextTick(); observeContent()
})
onMounted(() => {
  observer = new ResizeObserver(syncViewport)
  window.addEventListener('scroll', syncViewport, { passive: true })
  window.addEventListener('resize', syncViewport)
  document.addEventListener('scrollend', finishNavigation)
  window.addEventListener('wheel', finishNavigation, { passive: true })
  window.addEventListener('touchstart', finishNavigation, { passive: true })
  void loadDocuments()
})
onBeforeUnmount(() => {
  disposed = true; observer?.disconnect(); clearTimeout(navigationTimer)
  window.removeEventListener('scroll', syncViewport)
  window.removeEventListener('resize', syncViewport)
  document.removeEventListener('scrollend', finishNavigation)
  window.removeEventListener('wheel', finishNavigation)
  window.removeEventListener('touchstart', finishNavigation)
})
</script>

<style scoped>
@font-face { font-family: 'JetBrains Mono'; src: url('/tokenfree/fonts/JetBrainsMono-Variable.ttf') format('truetype'); font-weight: 100 800; font-display: swap; unicode-range: U+0000-024F,U+1E00-1EFF,U+2000-206F,U+20A0-20CF,U+2190-21FF; }
.creative-docs { width: 100%; max-width: 1200px; margin-inline: auto; font-family: 'TokenFree Console Mono', 'JetBrains Mono', 'Segoe UI', 'Microsoft YaHei', sans-serif; color: rgb(var(--tf-console-neutral-900)); font-size: 12px; }
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
.creative-docs-markdown { margin-top: 32px; overflow-wrap: anywhere; }
.creative-docs-empty { padding: 64px 24px; text-align: center; }
.creative-docs-empty button { text-decoration: underline; padding-left: 10px; }
.creative-docs a:focus-visible,.creative-docs button:focus-visible { outline: 2px solid var(--tf-console-active-ink); outline-offset: 3px; }
@media(max-width:1050px) { .creative-docs-workspace { grid-template-columns: 205px minmax(0,1fr); } .creative-docs-content { padding-left: 20px; } }
@media(max-width:700px) { .creative-docs-workspace { grid-template-columns: minmax(0,1fr); } .creative-docs-sidebar { position: static; max-height: none; overflow-y: visible; border-right: 0; border-bottom: 1px solid var(--tf-console-line); padding-right: 0; } .creative-docs-content { padding: 24px 0 0; } .creative-docs-groups { position: static; z-index: auto; flex-direction: column; align-items: stretch; gap: 8px; padding: 10px; } .creative-docs-group-category { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: start; gap: 6px; } .creative-docs-group-label { grid-column: 1 / -1; padding: 0 2px; } }
</style>
