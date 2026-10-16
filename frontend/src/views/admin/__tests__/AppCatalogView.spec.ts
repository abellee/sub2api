import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AppCatalogView from '../AppCatalogView.vue'

const { getHealth, list, create, update, fetchPreview, remove, showError, showSuccess } = vi.hoisted(() => ({
  getHealth: vi.fn(),
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  fetchPreview: vi.fn(),
  remove: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    appCatalog: { getHealth, list, create, update, fetchPreview, remove },
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError, showSuccess, showInfo: vi.fn() }),
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params ? `${key}:${JSON.stringify(params)}` : key,
  }),
}))

const dialogStub = {
  props: ['show', 'title'],
  inheritAttrs: false,
  template: '<div v-if="show" v-bind="$attrs"><h3>{{ title }}</h3><slot /><slot name="footer" /></div>',
}

function mountView() {
  return mount(AppCatalogView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        Teleport: { template: '<div><slot /></div>' },
        BaseDialog: dialogStub,
      },
    },
  })
}

async function openAddDialog(wrapper: ReturnType<typeof mountView>) {
  await wrapper.get('[data-testid="add-app-button"]').trigger('click')
  await flushPromises()
}

describe('应用管理', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getHealth.mockResolvedValue({ enabled: true, base_url: 'http://127.0.0.1:18099' })
    list.mockResolvedValue({ items: [], total: 0 })
  })

  it('默认不展示表单，点击添加后在弹框中打开', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="crawl-status"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="add-app-form"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="empty-state"]').text()).toContain('admin.appCatalog.empty')

    await openAddDialog(wrapper)

    expect(wrapper.get('[data-testid="add-app-dialog"]').text()).toContain('admin.appCatalog.add')
    expect(wrapper.get('[data-testid="app-name-input"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="app-intro-input"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="app-screenshots-input"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="app-changelog-input"]').exists()).toBe(true)
  })

  it('服务未启动时显示提示', async () => {
    getHealth.mockResolvedValue({ enabled: false, reason: 'APP_CATALOG_AGENT_UNAVAILABLE', base_url: 'http://127.0.0.1:18099' })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="service-down"]').text()).toBe('admin.appCatalog.serviceDown')
    expect(list).not.toHaveBeenCalled()
    expect((wrapper.get('[data-testid="add-app-button"]').element as HTMLButtonElement).disabled).toBe(true)
  })

  it('两者都空时不允许提交', async () => {
    const wrapper = mountView()
    await flushPromises()
    await openAddDialog(wrapper)

    await wrapper.get('[data-testid="add-app-form"]').trigger('submit')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('admin.appCatalog.requiredUrl')
    expect(create).not.toHaveBeenCalled()
  })

  it('提交后保存手填资料并刷新列表', async () => {
    create.mockResolvedValue({ id: 1, name: 'Demo' })
    const wrapper = mountView()
    await flushPromises()
    await openAddDialog(wrapper)

    await wrapper.get('#app-github-url').setValue('https://github.com/acme/demo')
    await wrapper.get('[data-testid="app-name-input"]').setValue('Demo')
    await wrapper.get('[data-testid="app-intro-input"]').setValue('简介')
    await wrapper.get('[data-testid="app-screenshots-input"]').setValue('https://cdn.example/a.png')
    await wrapper.get('[data-testid="app-changelog-input"]').setValue('v1')
    await wrapper.get('[data-testid="add-app-form"]').trigger('submit')
    await flushPromises()

    expect(create).toHaveBeenCalledWith({
      official_url: '',
      github_url: 'https://github.com/acme/demo',
      name: 'Demo',
      icon_url: '',
      description: '简介',
      download_page_url: '',
      screenshots: ['https://cdn.example/a.png'],
      changelog: 'v1',
      version: '',
    })
    expect(showSuccess).toHaveBeenCalledWith('admin.appCatalog.added')
    expect(list).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="add-app-form"]').exists()).toBe(false)
  })

  it('抓取后自动填入表单', async () => {
    fetchPreview.mockResolvedValue({
      official_url: 'https://demo.example',
      github_url: 'https://github.com/acme/demo',
      name: 'Fetched App',
      icon_url: 'https://cdn.example/logo.png',
      description: 'from web',
      screenshots: ['https://cdn.example/shot.png'],
      changelog: 'release notes',
      version: '1.0.0',
    })
    const wrapper = mountView()
    await flushPromises()
    await openAddDialog(wrapper)

    await wrapper.get('#app-github-url').setValue('https://github.com/acme/demo')
    await wrapper.get('[data-testid="fetch-fill-button"]').trigger('click')
    await flushPromises()

    expect(fetchPreview).toHaveBeenCalledWith({
      official_url: '',
      github_url: 'https://github.com/acme/demo',
    })
    expect((wrapper.get('#app-official-url').element as HTMLInputElement).value).toBe('https://demo.example')
    expect((wrapper.get('#app-github-url').element as HTMLInputElement).value).toBe('https://github.com/acme/demo')
    expect((wrapper.get('[data-testid="app-name-input"]').element as HTMLInputElement).value).toBe('Fetched App')
    expect((wrapper.get('[data-testid="app-intro-input"]').element as HTMLTextAreaElement).value).toBe('from web')
    expect((wrapper.get('[data-testid="app-screenshots-input"]').element as HTMLTextAreaElement).value).toBe(
      'https://cdn.example/shot.png',
    )
    expect((wrapper.get('[data-testid="app-changelog-input"]').element as HTMLTextAreaElement).value).toBe('release notes')
    expect(showSuccess).toHaveBeenCalledWith('admin.appCatalog.fetchFilled')
  })

  it('点击编辑打开同一个添加弹框并保存', async () => {
    list.mockResolvedValue({
      items: [{
        id: 9,
        official_url: '',
        github_url: 'https://github.com/acme/demo',
        name: 'Old',
        icon_url: '',
        description: 'old intro',
        download_page_url: '',
        screenshots: [],
        changelog: 'old notes',
        version: '',
        created_at: '',
        updated_at: '',
      }],
      total: 1,
    })
    update.mockResolvedValue({ id: 9, name: 'New' })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="add-app-form"]').exists()).toBe(false)
    await wrapper.get('[data-testid="edit-app-button"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="add-app-dialog"]').text()).toContain('admin.appCatalog.editTitle')
    await wrapper.get('[data-testid="app-name-input"]').setValue('New')
    await wrapper.get('[data-testid="add-app-form"]').trigger('submit')
    await flushPromises()

    expect(update).toHaveBeenCalledWith(9, expect.objectContaining({
      github_url: 'https://github.com/acme/demo',
      name: 'New',
      description: 'old intro',
      changelog: 'old notes',
    }))
    expect(showSuccess).toHaveBeenCalledWith('admin.appCatalog.updated')
  })

  it('列表卡片简介最多四行，详情弹框展示完整内容和更新说明', async () => {
    list.mockResolvedValue({
      items: [{
        id: 3,
        official_url: '',
        github_url: 'https://github.com/acme/demo',
        name: 'Demo',
        icon_url: '',
        description: 'line1\nline2\nline3\nline4\nline5 should be clamped',
        download_page_url: '',
        screenshots: [],
        changelog: 'full release notes body',
        version: '1.2.0',
        created_at: '',
        updated_at: '2026-09-13T08:00:00Z',
      }],
      total: 1,
    })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="app-updated-at"]').text()).toContain('admin.appCatalog.lastUpdated')
    const intro = wrapper.get('[data-testid="app-intro-preview"]')
    expect(intro.classes()).toContain('line-clamp-4')
    expect(wrapper.find('[data-testid="app-detail-dialog"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="app-card"]').text()).not.toContain('full release notes body')

    await wrapper.get('[data-testid="view-app-button"]').trigger('click')
    await flushPromises()

    const dialog = wrapper.get('[data-testid="app-detail-dialog"]')
    expect(dialog.text()).toContain('Demo')
    expect(wrapper.get('[data-testid="app-detail-intro"]').classes()).not.toContain('line-clamp-4')
    expect(wrapper.get('[data-testid="changelog-dialog-body"]').text()).toBe('full release notes body')
  })

  it('详情弹框顶部展示 Logo，后面是截图，点击进入灯箱', async () => {
    list.mockResolvedValue({
      items: [{
        id: 4,
        official_url: '',
        github_url: 'https://github.com/acme/demo',
        name: 'Grok App',
        icon_url: 'https://cdn.example/logo.png',
        description: '开源 **Grok App**\n\n- 核心亮点 A\n- 核心亮点 B',
        download_page_url: '',
        screenshots: ['https://cdn.example/a.png', 'https://cdn.example/b.png'],
        changelog: '## 0.2.35\n\n- **修复** 崩溃',
        version: '0.2.35',
        created_at: '',
        updated_at: '',
      }],
      total: 1,
    })
    const wrapper = mountView()
    await flushPromises()

    const intro = wrapper.get('[data-testid="app-intro-preview"]')
    expect(intro.classes()).toContain('line-clamp-4')
    expect(intro.find('strong').text()).toBe('Grok App')
    expect(wrapper.find('[data-testid="app-screenshot-thumb"]').exists()).toBe(false)

    await wrapper.get('[data-testid="view-app-button"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="detail-logo"] img').attributes('src')).toBe('https://cdn.example/logo.png')
    const thumbs = wrapper.findAll('[data-testid="app-screenshot-thumb"]')
    expect(thumbs).toHaveLength(2)
    expect(thumbs[0].element.tagName).toBe('BUTTON')

    const detailIntro = wrapper.get('[data-testid="app-detail-intro"]')
    expect(detailIntro.classes()).toContain('markdown-body')
    expect(detailIntro.findAll('li')).toHaveLength(2)

    const changelogBody = wrapper.get('[data-testid="changelog-dialog-body"]')
    expect(changelogBody.find('h2').text()).toContain('0.2.35')
    expect(changelogBody.find('strong').text()).toBe('修复')

    await thumbs[0].trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="screenshot-lightbox"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="lightbox-image"]').attributes('src')).toBe('https://cdn.example/a.png')

    await wrapper.get('[data-testid="lightbox-next"]').trigger('click')
    expect(wrapper.get('[data-testid="lightbox-image"]').attributes('src')).toBe('https://cdn.example/b.png')
  })
})
