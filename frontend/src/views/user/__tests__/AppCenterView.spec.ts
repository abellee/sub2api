import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AppCenterView from '../AppCenterView.vue'

const { listCatalogApps } = vi.hoisted(() => ({
  listCatalogApps: vi.fn(),
}))

vi.mock('@/api/appCatalog', () => ({
  listCatalogApps,
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

const dialogStub = {
  props: ['show', 'title'],
  inheritAttrs: false,
  template: '<div v-if="show" v-bind="$attrs"><h3>{{ title }}</h3><slot /><slot name="footer" /></div>',
}

function mountView() {
  return mount(AppCenterView, {
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

describe('用户侧应用中心', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listCatalogApps.mockResolvedValue({ items: [], total: 0 })
  })

  it('侧栏菜单项在模型广场下方', () => {
    const sidebar = readFileSync(
      resolve(__dirname, '../../../components/layout/AppSidebar.vue'),
      'utf8',
    )
    const plaza = sidebar.indexOf("{ path: '/model-plaza'")
    const center = sidebar.indexOf("{ path: '/app-center'")
    expect(plaza).toBeGreaterThan(-1)
    expect(center).toBeGreaterThan(plaza)
  })

  it('无应用时显示空状态', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="app-center-page"]').text()).toContain('appCenter.title')
    expect(wrapper.get('[data-testid="app-center-empty"]').text()).toContain('appCenter.empty')
    expect(wrapper.find('[data-testid="app-card"]').exists()).toBe(false)
  })

  it('展示已添加的应用，点击详情弹出 Logo 和截图', async () => {
    listCatalogApps.mockResolvedValue({
      items: [{
        id: 4,
        official_url: '',
        github_url: 'https://github.com/acme/demo',
        name: 'Grok App',
        icon_url: 'https://cdn.example/logo.png',
        description: '开源 **Grok App**\n\n- 核心亮点 A\n- 核心亮点 B',
        download_page_url: 'https://github.com/acme/demo/releases',
        screenshots: ['https://cdn.example/a.png', 'https://cdn.example/b.png'],
        changelog: '## 0.2.35\n\n- **修复** 崩溃',
        version: '0.2.35',
        created_at: '',
        updated_at: '2026-09-13T08:00:00Z',
      }],
      total: 1,
    })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="app-center-empty"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="app-updated-at"]').text()).toContain('admin.appCatalog.lastUpdated')
    const intro = wrapper.get('[data-testid="app-intro-preview"]')
    expect(intro.classes()).toContain('line-clamp-4')
    expect(intro.find('strong').text()).toBe('Grok App')
    expect(wrapper.find('[data-testid="edit-app-button"]').exists()).toBe(false)
    const card = wrapper.get('[data-testid="app-card"]')
    expect(card.text()).not.toContain('admin.appCatalog.download')
    const cardDownload = card.get('[data-testid="download-page-button"]')
    expect(cardDownload.text()).toBe('appCenter.openDownloadPage')
    expect(cardDownload.attributes('href')).toBe('https://github.com/acme/demo/releases')
    expect(cardDownload.classes()).toContain('btn-sm')
    expect(cardDownload.classes()).not.toContain('w-full')
    const footer = card.get('[data-testid="app-updated-at"]').element.parentElement
    expect(footer?.contains(cardDownload.element)).toBe(true)

    await wrapper.get('[data-testid="view-app-button"]').trigger('click')
    await flushPromises()

    const dialog = wrapper.get('[data-testid="app-detail-dialog"]')
    expect(dialog.text()).not.toContain('admin.appCatalog.download')
    const detailDownload = dialog.get('[data-testid="download-page-button"]')
    expect(detailDownload.text()).toBe('appCenter.openDownloadPage')
    expect(detailDownload.attributes('href')).toBe('https://github.com/acme/demo/releases')
    expect(detailDownload.classes()).not.toContain('w-full')
    const closeButton = dialog.findAll('button').find((button) => button.text() === 'common.close')
    expect(closeButton).toBeTruthy()
    expect(dialog.html().indexOf('common.close')).toBeLessThan(dialog.html().indexOf('data-testid="download-page-button"'))
    expect(wrapper.get('[data-testid="detail-logo"] img').attributes('src')).toBe('https://cdn.example/logo.png')
    const thumbs = wrapper.findAll('[data-testid="app-screenshot-thumb"]')
    expect(thumbs).toHaveLength(2)

    await thumbs[0].trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="lightbox-image"]').attributes('src')).toBe('https://cdn.example/a.png')
  })

  it('服务不可用时显示空状态而不是报错页', async () => {
    listCatalogApps.mockRejectedValue(new Error('down'))
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="app-center-empty"]').exists()).toBe(true)
  })
})
