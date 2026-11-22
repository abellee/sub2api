<template>
  <AppLayout>
    <div class="space-y-6" data-testid="documentation-management">
      <header class="flex flex-wrap items-center justify-between gap-3">
        <div><h2 class="text-lg font-semibold">文档管理</h2><p class="mt-1 text-sm text-gray-500">管理文章、分类和 Agent 提示词。页面通过接口读取数据并自行展示。</p></div>
        <div class="flex items-center gap-2"><span v-if="dirty" class="text-xs text-amber-600">有未保存的修改</span><button type="button" class="btn btn-secondary" :disabled="loading || saving" @click="reload">重新加载</button><button type="button" class="btn btn-primary" :disabled="!loaded || !dirty || saving" data-testid="save-documents" @click="save">{{ saving ? '正在保存…' : '保存全部' }}</button></div>
      </header>
      <p v-if="error" class="rounded-lg bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300" role="alert">{{ error }}</p>
      <p v-if="loading" class="py-12 text-center" role="status">正在加载文档…</p>
      <fieldset v-else-if="loaded" :disabled="saving" class="grid min-w-0 items-start gap-6 xl:grid-cols-[310px,minmax(0,1fr)]">
        <aside class="space-y-5">
          <section class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <h3 class="mb-3 font-semibold">分类</h3>
            <form class="mb-4 flex gap-2" @submit.prevent="addCategory"><input v-model="newCategoryName" class="input min-w-0" aria-label="新分类名称" placeholder="新分类名称" maxlength="80"><button type="submit" class="btn btn-secondary" :disabled="!newCategoryName.trim()">添加</button></form>
            <div v-for="(category, index) in snapshot.categories" :key="category.id" class="mb-4 space-y-2">
              <input v-model="category.name" class="input" aria-label="分类名称" maxlength="80">
              <div class="flex flex-wrap items-center gap-2 text-xs"><button type="button" :disabled="index === 0" aria-label="上移分类" @click="move(snapshot.categories, index, -1)">↑</button><button type="button" :disabled="index === snapshot.categories.length - 1" aria-label="下移分类" @click="move(snapshot.categories, index, 1)">↓</button><button type="button" class="text-red-600" @click="removeCategory(category.id)">删除分类</button></div>
              <label class="block text-xs text-gray-500">分类标识<input :value="category.id" readonly class="input mt-1 text-xs" aria-label="分类标识" @focus="($event.target as HTMLInputElement).select()"></label>
              <label class="block text-xs text-gray-500">文章数据接口<input :value="categoryAPI(category.id)" readonly class="input mt-1 text-xs" aria-label="文章数据接口" @focus="($event.target as HTMLInputElement).select()"></label>
            </div>
            <label class="block text-xs text-gray-500">分类列表接口<input value="/api/v1/documentation/categories" readonly class="input mt-1 text-xs" aria-label="分类列表接口" @focus="($event.target as HTMLInputElement).select()"></label>
            <p class="mt-3 text-xs leading-5 text-gray-500">接口需要登录，文章接口仅返回已发布文章。分类不自动生成页面或菜单，展示方式由调用页面决定。</p>
          </section>
          <section class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <div class="mb-3 flex items-center justify-between"><h3 class="font-semibold">文章（{{ snapshot.articles.length }}）</h3><button type="button" class="btn btn-secondary btn-sm" :disabled="!snapshot.categories.length" data-testid="add-article" @click="addArticle">新增文章</button></div>
            <input v-model="search" class="input mb-3" aria-label="搜索文章" placeholder="搜索文章名称">
            <button v-for="article in filteredArticles" :key="article.id" type="button" class="mb-1 flex w-full items-center gap-2 rounded-lg border p-3 text-left text-sm" :class="article.id === selectedID ? 'border-primary-400 bg-primary-50 dark:bg-primary-950/20' : 'border-transparent hover:bg-gray-50 dark:hover:bg-dark-700'" :data-article-id="article.id" :aria-pressed="article.id === selectedID" @click="selectedID = article.id">
              <img v-if="article.logo" :src="'/creative-docs/assets/brands/' + article.logo + '.svg'" alt="" width="22" height="22" class="rounded bg-white p-0.5"><span class="min-w-0 flex-1">{{ article.name || '未命名文章' }}<small class="block text-gray-500">{{ categoryName(article.category_id) }} · {{ article.published ? '已发布' : '草稿' }}</small></span>
            </button>
          </section>
        </aside>
        <section v-if="selected" class="min-w-0 space-y-5 rounded-xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
          <div class="flex flex-wrap items-center justify-between gap-2"><h3 class="font-semibold">编辑文章</h3><div class="flex flex-wrap gap-3 text-sm"><button type="button" :disabled="selectedIndex === 0" @click="move(snapshot.articles, selectedIndex, -1)">上移</button><button type="button" :disabled="selectedIndex === snapshot.articles.length - 1" @click="move(snapshot.articles, selectedIndex, 1)">下移</button><RouterLink :to="{ path: '/docs/creative', query: { article: selected.id } }" target="_blank" class="text-primary-600 underline">查看已保存页面</RouterLink><button type="button" class="text-red-600" data-testid="delete-article" @click="removeArticle">删除文章</button></div></div>
          <div class="grid gap-4 sm:grid-cols-2">
            <label><span class="input-label">文章名称</span><input v-model="selected.name" class="input" maxlength="160" data-testid="article-name"></label>
            <label><span class="input-label">分类</span><Select :model-value="selected.category_id" :options="categoryOptions" aria-label="文章分类" @update:model-value="selected!.category_id = String($event)" /></label>
            <label><span class="input-label">品牌 Logo</span><Select :model-value="selected.logo" :options="documentationLogos" aria-label="品牌 Logo" @update:model-value="selected!.logo = String($event)" /></label>
            <label class="flex items-center gap-2 pt-6"><input v-model="selected.published" type="checkbox">发布文章（取消后仅管理员可见）</label>
          </div>
          <div><div class="mb-2 flex items-center justify-between"><label for="article-content" class="input-label mb-0">文档内容（Markdown）</label><button type="button" class="text-sm text-primary-600" @click="showPreview = !showPreview">{{ showPreview ? '返回编辑' : '预览内容' }}</button></div><p class="mb-2 text-xs text-gray-500">使用 #、##、### 标题自动生成章节目录；支持代码块、表格和列表。</p><div v-if="showPreview" class="markdown-body max-h-[650px] overflow-auto rounded-lg border p-4 dark:border-dark-600" v-html="preview.html"></div><textarea v-else id="article-content" v-model="selected.content" class="input min-h-[460px] font-mono text-sm leading-6" spellcheck="false" data-testid="article-content"></textarea></div>
          <section class="space-y-4 border-t border-gray-200 pt-5 dark:border-dark-600">
            <label class="flex items-center gap-2 font-semibold"><input v-model="selected.agent.enabled" type="checkbox" data-testid="agent-enabled">显示页面顶部的 Agent 提示词</label>
            <template v-if="selected.agent.enabled">
              <label class="block"><span class="input-label">提示词区域标题</span><input v-model="selected.agent.title" class="input" maxlength="160"></label>
              <label class="block"><span class="input-label">说明文字</span><input v-model="selected.agent.description" class="input" maxlength="1000"></label>
              <label class="block"><span class="input-label">接口地址（Base URL）</span><input v-model="selected.agent.base_url" class="input" type="url" placeholder="https://creative.tokenfree.dev/"></label>
              <label class="block"><span class="input-label">Key 读取方式</span><Select :model-value="selected.agent.key_mode" :options="keyModeOptions" aria-label="Key 读取方式" @update:model-value="selected!.agent.key_mode = $event as DocumentationAgent['key_mode']" /></label>
              <div v-if="selected.agent.key_mode === 'groups'" class="space-y-2"><p class="text-sm">允许选择这些分组下的个人 Key：</p><p v-if="groupsError" class="text-sm text-red-600" role="alert">{{ groupsError }} <button type="button" class="underline" @click="loadGroups">重试</button></p><label v-for="group in groups" :key="group.id" class="flex items-center gap-2 text-sm"><input v-model="selected.agent.group_ids" type="checkbox" :value="group.id">{{ group.name }}</label><p v-for="id in selected.agent.group_ids.filter(id => !groups.some(group => group.id === id))" :key="id" class="text-sm text-amber-600">分组 {{ id }} 已不可用 <button type="button" @click="selected!.agent.group_ids = selected!.agent.group_ids.filter(value => value !== id)">移除</button></p></div>
              <p class="text-xs leading-5 text-gray-500">自动读取仅使用当前登录用户自己的有效 Key。页面显示 Key 名称，复制时才填入真实值。指定分组只限制 Key 选择范围，不影响文章展示。</p>
              <label class="block"><span class="input-label">Agent 提示词模板</span><textarea v-model="selected.agent.template" class="input min-h-[280px] font-mono text-sm leading-6" spellcheck="false" data-testid="agent-template"></textarea></label>
              <p class="text-xs leading-6 text-gray-500" v-pre>占位符：{{article_name}} 文章名、{{base_url}} 接口地址、{{api_key}} 用户选中的 Key、{{group_name}} Key 的分组名、{{document}} 完整 Markdown 文档。按需保留，不会执行脚本。</p>
              <p class="text-xs leading-5 text-amber-600">接口地址用于提示词中的占位符；正文里直接写出的地址需要一并修改。</p>
              <details><summary class="cursor-pointer text-sm">预览复制文本（使用 Key 占位符）</summary><pre class="mt-3 max-h-80 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-gray-50 p-4 text-xs leading-6 dark:bg-dark-900">{{ promptPreview }}</pre></details>
            </template>
          </section>
        </section>
        <p v-else class="p-10 text-center text-gray-500">请选择文章，或先添加分类和文章。</p>
      </fieldset>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select from '@/components/common/Select.vue'
import { getAdminDocumentation, saveDocumentation, type DocumentationAgent, type DocumentationSnapshot } from '@/api/documentation'
import { getAllIncludingInactive } from '@/api/admin/groups'
import { defaultDocumentationAgent, documentationLogos, renderAgentTemplate, renderDocumentationMarkdown } from '@/utils/creativeDocumentation'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import '@/styles/announcement-markdown.css'

const app = useAppStore()
const snapshot = ref<DocumentationSnapshot>({ revision: 0, categories: [], articles: [] })
const baseline = ref('')
const loaded = ref(false)
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const selectedID = ref('')
const newCategoryName = ref('')
const search = ref('')
const showPreview = ref(false)
const groups = ref<{ id: number; name: string }[]>([])
const groupsError = ref('')
const dirty = computed(() => loaded.value && JSON.stringify(snapshot.value) !== baseline.value)
const selectedIndex = computed(() => snapshot.value.articles.findIndex(article => article.id === selectedID.value))
const selected = computed(() => snapshot.value.articles[selectedIndex.value])
const filteredArticles = computed(() => snapshot.value.articles.filter(article => article.name.toLocaleLowerCase().includes(search.value.toLocaleLowerCase())))
const categoryOptions = computed(() => snapshot.value.categories.map(category => ({ value: category.id, label: category.name })))
const preview = computed(() => renderDocumentationMarkdown(selected.value?.content ?? ''))
const promptPreview = computed(() => selected.value ? renderAgentTemplate(selected.value) : '')
const keyModeOptions = [{ value: 'all', label: '读取用户全部可用 Key' }, { value: 'groups', label: '只读取指定分组下的用户 Key' }, { value: 'manual', label: '不读取 Key，复制后由用户填写' }, { value: 'none', label: '不使用 Key' }]
const categoryAPI = (id: string) => '/api/v1/documentation?category=' + encodeURIComponent(id)
const categoryName = (id: string) => snapshot.value.categories.find(category => category.id === id)?.name ?? '未分类'
const errorMessage = extractApiErrorMessage

async function load() {
  loading.value = true; error.value = ''
  try {
    const data = await getAdminDocumentation()
    snapshot.value = data; baseline.value = JSON.stringify(data); loaded.value = true
    if (!data.articles.some(article => article.id === selectedID.value)) selectedID.value = data.articles[0]?.id ?? ''
  } catch (e) { error.value = errorMessage(e, '文档加载失败，请重试。') }
  finally { loading.value = false }
}
async function reload() { if (!dirty.value || window.confirm('重新加载会丢弃未保存的修改，继续吗？')) await load() }
async function loadGroups() {
  groupsError.value = ''
  try { groups.value = await getAllIncludingInactive() } catch { groupsError.value = '分组列表加载失败。' }
}
async function save() {
  saving.value = true; error.value = ''
  try {
    const saved = await saveDocumentation(JSON.parse(JSON.stringify(snapshot.value)))
    snapshot.value = saved; baseline.value = JSON.stringify(saved)
    app.showSuccess('文档已保存')
  } catch (e) { error.value = errorMessage(e, '保存失败，请检查内容后重试。') }
  finally { saving.value = false }
}
function addCategory() {
  const name = newCategoryName.value.trim()
  if (!name) return
  if (snapshot.value.categories.some(category => category.name === name)) { error.value = '已有同名分类。'; return }
  snapshot.value.categories.push({ id: crypto.randomUUID(), name }); newCategoryName.value = ''; error.value = ''
}
function removeCategory(id: string) {
  if (snapshot.value.articles.some(article => article.category_id === id)) { error.value = '请先移走或删除该分类下的文章。'; return }
  if (window.confirm('删除这个分类？保存后生效。')) snapshot.value.categories = snapshot.value.categories.filter(category => category.id !== id)
}
function addArticle() {
  const id = crypto.randomUUID()
  snapshot.value.articles.push({ id, name: '新文章', logo: '', category_id: snapshot.value.categories[0].id, content: '# 新文章\n\n## 开始使用\n\n在这里输入 Markdown 文档。', published: false, agent: defaultDocumentationAgent() })
  selectedID.value = id; search.value = ''; showPreview.value = false
}
function removeArticle() {
  if (!selected.value || !window.confirm('删除文章「' + selected.value.name + '」？保存后生效。')) return
  snapshot.value.articles.splice(selectedIndex.value, 1); selectedID.value = snapshot.value.articles[0]?.id ?? ''
}
function move<T>(items: T[], index: number, direction: number) {
  const destination = index + direction
  if (index < 0 || destination < 0 || destination >= items.length) return
  ;[items[index], items[destination]] = [items[destination], items[index]]
}
function beforeUnload(event: BeforeUnloadEvent) { if (dirty.value) { event.preventDefault(); event.returnValue = '' } }
onBeforeRouteLeave(() => !dirty.value || window.confirm('有未保存的文档修改，确定离开？'))
onMounted(() => { void load(); void loadGroups(); window.addEventListener('beforeunload', beforeUnload) })
onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))
</script>
