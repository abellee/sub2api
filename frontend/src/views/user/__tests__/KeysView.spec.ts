import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'

import type { ApiKey, Group } from '@/types'
import { keysAPI } from '@/api'
import KeysView from '../KeysView.vue'

let intersectionCallback: IntersectionObserverCallback
const reachEnd = async () => {
  intersectionCallback([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver)
  await flushPromises()
}

const {
  listKeys,
  updateKey,
  getPublicSettings,
  getDashboardApiKeysUsage,
  getAvailableGroups,
  getUserGroupRates,
  getGroupRecommendations,
  getGroupCategories,
  showError,
  showSuccess,
  copyToClipboard,
  isCurrentStep,
  nextStep,
} = vi.hoisted(() => ({
  listKeys: vi.fn(),
  updateKey: vi.fn(),
  getPublicSettings: vi.fn(),
  getDashboardApiKeysUsage: vi.fn(),
  getAvailableGroups: vi.fn(),
  getUserGroupRates: vi.fn(),
  getGroupRecommendations: vi.fn(),
  getGroupCategories: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  copyToClipboard: vi.fn(),
  isCurrentStep: vi.fn(),
  nextStep: vi.fn(),
}))

const messages: Record<string, string> = {
  'common.actions': 'Actions',
  'common.name': 'Name',
  'common.refresh': 'Refresh',
  'common.status': 'Status',
  'keys.apiKey': 'API Key',
  'keys.totalKeys': '{count} keys',
  'keys.allGroups': 'All Groups',
  'keys.allStatus': 'All Status',
  'keys.columnSettings': 'Column Settings',
  'keys.createKey': 'Create API Key',
  'keys.created': 'Created',
  'keys.expiresAt': 'Expires',
  'keys.group': 'Group',
  'keys.requiredProtocolTag': '仅限{protocol}协议',
  'keys.blockedClientTag': '排除客户端：{client}',
  'keys.id': 'ID',
  'keys.currentConcurrency': 'Current Concurrency',
  'keys.lastUsedAt': 'Last Used',
  'keys.lastUsedIP': 'Last Used IP',
  'keys.rateLimitColumn': 'Rate Limit',
  'keys.searchPlaceholder': 'Search name or key...',
  'keys.status.active': 'Active',
  'keys.status.expired': 'Expired',
  'keys.status.inactive': 'Inactive',
  'keys.status.quota_exhausted': 'Quota exhausted',
  'keys.usage': 'Usage',
}

vi.mock('@/api', () => ({
  keysAPI: {
    list: listKeys,
    create: vi.fn(),
    update: updateKey,
    delete: vi.fn(),
    toggleStatus: vi.fn(),
  },
  authAPI: {
    getPublicSettings,
  },
  usageAPI: {
    getDashboardApiKeysUsage,
  },
  userGroupsAPI: {
    getAvailable: getAvailableGroups,
    getUserGroupRates,
    getRecommendations: getGroupRecommendations,
    getCategories: getGroupCategories,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({
    isCurrentStep,
    nextStep,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        const text = messages[key] ?? key
        return text.replace(/\{(\w+)\}/g, (_, name) => String(params?.[name] ?? ''))
      },
    }),
  }
})

const createApiKey = (): ApiKey => ({
  id: 1,
  user_id: 1,
  key: 'sk-test-key',
  name: 'test-key',
  group_id: null,
  status: 'active',
  ip_whitelist: [],
  ip_blacklist: [],
  last_used_at: null,
  last_used_ip: null,
  quota: 0,
  quota_used: 0,
  expires_at: null,
  created_at: '2026-06-27T00:00:00Z',
  updated_at: '2026-06-27T00:00:00Z',
  current_concurrency: 3,
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0,
  usage_5h: 0,
  usage_1d: 0,
  usage_7d: 0,
  window_5h_start: null,
  window_1d_start: null,
  window_7d_start: null,
  reset_5h_at: null,
  reset_1d_at: null,
  reset_7d_at: null,
})

const StudioLayoutStub = {
  template: '<div><header><slot name="filters" /></header><slot /></div>',
}

const KeyCardGridStub = {
  name: 'KeyCardGrid',
  props: { columns: Array, data: Array, selectedKeys: Array, loading: Boolean },
  emits: ['update:selectedKeys'],
  template: `
    <div>
      <div data-test="columns">{{ columns.map((col) => col.key).join(',') }}</div>
      <div data-test="columns-meta">{{ JSON.stringify(columns.map((col) => ({ key: col.key, sortable: !!col.sortable }))) }}</div>
      <slot name="selection-actions" />
      <div v-for="row in data" :key="row.id">
        <div
          v-if="columns.some((col) => col.key === 'id')"
          data-test="key-id"
        >
          <slot name="cell-id" :value="row.id" :row="row" />
        </div>
        <slot name="cell-name" :value="row.name" :row="row" />
        <div v-if="columns.some((col) => col.key === 'group')" data-test="key-group">
          <slot name="cell-group" :row="row" />
        </div>
        <slot name="cell-actions" :row="row" />
        <div data-test="current-concurrency">
          <slot name="cell-current_concurrency" :value="row.current_concurrency" :row="row" />
        </div>
        <div data-test="usage">
          <slot name="cell-usage" :row="row" />
        </div>
        <div
          v-if="columns.some((col) => col.key === 'last_used_ip')"
          data-test="last-used-ip"
        >
          <slot name="cell-last_used_ip" :value="row.last_used_ip" :row="row" />
        </div>
      </div>
      <slot name="empty" />
    </div>
  `,
}

const SelectStub = {
  name: 'Select',
  props: ['modelValue', 'options'],
  emits: ['update:modelValue'],
  template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"></select>',
}

const SearchInputStub = {
  name: 'SearchInput',
  props: ['modelValue'],
  emits: ['update:modelValue', 'search'],
  template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
}

const IconStub = {
  props: ['name'],
  template: '<span data-test="icon">{{ name }}</span>',
}

const mountView = async () => {
  const wrapper = mount(KeysView, {
    global: {
      stubs: {
        StudioLayout: StudioLayoutStub,
        KeyCardGrid: KeyCardGridStub,
        BaseDialog: {
          props: ['show', 'title'],
          emits: ['close'],
          template: '<div v-if="show" role="dialog"><button data-test="close-dialog" @click="$emit(\'close\')">Close</button><slot /><slot name="footer" /></div>',
        },
        ConfirmDialog: true,
        EmptyState: true,
        Select: SelectStub,
        SearchInput: SearchInputStub,
        Icon: IconStub,
        UseKeyModal: true,
        BulkEditKeysModal: true,
        EndpointPopover: {
          name: 'EndpointPopover',
          template: '<div data-test="endpoint-popover" />',
        },
        GroupBadge: true,
        GroupOptionItem: true,
        Teleport: true,
      },
    },
  })
  await flushPromises()
  await nextTick()
  return wrapper
}

const visibleColumnKeys = (wrapper: VueWrapper) =>
  wrapper.get('[data-test="columns"]').text().split(',').filter(Boolean)

const visibleColumnMeta = (wrapper: VueWrapper): Array<{ key: string; sortable: boolean }> =>
  JSON.parse(wrapper.get('[data-test="columns-meta"]').text())

const getButtonByText = (wrapper: VueWrapper, text: string) => {
  const button = wrapper.findAll('button').find((item) => item.text().includes(text))
  if (!button) {
    throw new Error(`Button not found: ${text}`)
  }
  return button
}

describe('user KeysView column settings', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.stubGlobal('IntersectionObserver', class {
      constructor(callback: IntersectionObserverCallback) { intersectionCallback = callback }
      observe = vi.fn()
      disconnect = vi.fn()
    })

    listKeys.mockReset()
    updateKey.mockReset()
    vi.mocked(keysAPI.create).mockReset()
    getPublicSettings.mockReset()
    getDashboardApiKeysUsage.mockReset()
    getAvailableGroups.mockReset()
    getUserGroupRates.mockReset()
    getGroupRecommendations.mockReset()
    getGroupCategories.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    copyToClipboard.mockReset()
    isCurrentStep.mockReset()
    nextStep.mockReset()

    listKeys.mockResolvedValue({
      items: [createApiKey()],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getPublicSettings.mockResolvedValue({})
    getDashboardApiKeysUsage.mockResolvedValue({ stats: {} })
    getAvailableGroups.mockResolvedValue([])
    getUserGroupRates.mockResolvedValue({})
    getGroupRecommendations.mockResolvedValue([])
    getGroupCategories.mockResolvedValue({ categories: [], assignments: {} })
    isCurrentStep.mockReturnValue(false)
  })

  it('shows the model icon before the card title and all tags below the group name', async () => {
    const group = {
      id: 1, name: 'OpenAI group', platform: 'openai', rate_multiplier: 0.3,
      required_protocol: 'openai', blocked_user_agents: ' Claude, , cursor, CLAUDE ',
    } as Group
    getAvailableGroups.mockResolvedValue([group])
    getGroupCategories.mockResolvedValue({
      categories: [], assignments: {}, group_caps: { '1': 0.8 },
      group_tags: { '1': [
        { name: '快速', description: '优先处理请求' },
        { name: '稳定', description: '' },
        { name: '支持长上下文', description: '' },
        { name: '完整显示的第四个标签', description: '' },
      ] },
    })
    listKeys.mockResolvedValue({
      items: [{ ...createApiKey(), group_id: 1, group }, createApiKey()], total: 2,
      page: 1, page_size: 20, pages: 1,
    })
    const wrapper = await mountView()
    const cards = wrapper.findAll('[data-test="key-group"]')
    const titles = wrapper.findAll('.key-card-name')
    expect(titles[0].findComponent({ name: 'PlatformIcon' }).props('platform')).toBe('openai')
    expect(titles[0].element.firstElementChild?.classList.contains('key-card-model-icon')).toBe(true)
    expect(titles[0].get('span').text()).toBe('test-key')
    expect(cards[0].findComponent({ name: 'PlatformIcon' }).exists()).toBe(false)
    const tags = cards[0].get('.key-group-tags')
    expect(tags.find('.keys-group-cap-tag').exists()).toBe(true)
    expect(tags.findAll('.key-group-custom-tag').map(tag => tag.text()))
      .toEqual(['快速', '稳定', '支持长上下文', '完整显示的第四个标签'])
    expect(tags.get('.key-group-custom-tag').attributes('title')).toBe('优先处理请求')
    expect(tags.get('.key-group-protocol-tag').text()).toBe('仅限OpenAI协议')
    expect(tags.findAll('.key-group-client-tag').map(tag => tag.text()))
      .toEqual(['排除客户端：claude', '排除客户端：cursor'])
    expect(cards[0].get('.key-group-trigger').element.nextElementSibling).toBe(tags.element)
    expect(cards[1].find('.key-group-tags').exists()).toBe(false)
    expect(titles[1].find('.key-card-model-icon').exists()).toBe(false)
    wrapper.unmount()
  })

  it.each([
    { initialStatus: 'quota_exhausted', status: 'active', formStatus: 'active' },
    { initialStatus: 'inactive', status: 'inactive', formStatus: 'inactive' },
    { initialStatus: 'active', status: 'active', formStatus: 'inactive' },
  ] as const)('syncs quota reset from $initialStatus to $status with form status $formStatus', async ({ initialStatus, status, formStatus }) => {
    const key: ApiKey = {
      ...createApiKey(), group_id: 1, quota: 10, quota_used: 10,
      status: initialStatus,
    }
    listKeys.mockResolvedValueOnce({ items: [key], total: 1, page: 1, page_size: 20, pages: 1 })
    updateKey.mockResolvedValue({ ...key, status, quota_used: 0 })
    const wrapper = await mountView()
    await getButtonByText(wrapper, 'common.edit').trigger('click')
    // 编辑模式与创建一致：额度/限速等区块在「更多设置」折叠区内，先展开
    await getButtonByText(wrapper, 'keys.moreSettings').trigger('click')
    await wrapper.get('[data-tour="key-form-name"]').setValue('Unsaved name')
    const statusSelect = wrapper.findAllComponents({ name: 'Select' })
      .find((select) => select.props('options').length === 2 &&
        select.props('options')[0].value === 'active')!
    statusSelect.vm.$emit('update:modelValue', 'inactive')
    await wrapper.get('button[title="keys.resetQuotaUsed"]').trigger('click')
    const confirmation = wrapper.findAllComponents({ name: 'ConfirmDialog' })
      .find((dialog) => dialog.props('title') === 'keys.resetQuotaTitle')!
    confirmation.vm.$emit('confirm')
    await flushPromises()

    expect(updateKey).toHaveBeenNthCalledWith(1, key.id, { reset_quota: true })
    expect(wrapper.findComponent({ name: 'KeyCardGrid' }).props('data')[0])
      .toMatchObject({ status, quota_used: 0 })
    expect(statusSelect.props('modelValue')).toBe(formStatus)
    expect((wrapper.get('[data-tour="key-form-name"]').element as HTMLInputElement).value)
      .toBe('Unsaved name')

    await wrapper.get('#key-form').trigger('submit')
    await flushPromises()
    expect(updateKey).toHaveBeenNthCalledWith(2, key.id, expect.objectContaining({ name: 'Unsaved name', status: formStatus }))
    wrapper.unmount()
  })

  it('uses the default API key columns with low-frequency columns hidden', async () => {
    const wrapper = await mountView()

    expect(visibleColumnKeys(wrapper)).toEqual([
      'name',
      'key',
      'group',
      'current_concurrency',
      'usage',
      'expires_at',
      'status',
      'created_at',
      'actions',
    ])
    expect(visibleColumnKeys(wrapper)).not.toContain('rate_limit')
    expect(visibleColumnKeys(wrapper)).not.toContain('last_used_at')
    expect(visibleColumnKeys(wrapper)).not.toContain('last_used_ip')
    expect(visibleColumnKeys(wrapper)).not.toContain('id')
  })

  it('opens bulk editing with only selected visible keys', async () => {
    const wrapper = await mountView()
    const table = wrapper.findComponent({ name: 'KeyCardGrid' })
    table.vm.$emit('update:selectedKeys', [1, 99])
    await nextTick()
    await wrapper.get('[data-test="bulk-edit-keys"]').trigger('click')
    const modal = wrapper.findComponent({ name: 'BulkEditKeysModal' })
    expect(modal.props('show')).toBe(true)
    expect(modal.props('selectedKeys').map((key: ApiKey) => key.id)).toEqual([1])
    wrapper.unmount()
  })

  it('clears selection on filter changes', async () => {
    const wrapper = await mountView()
    const table = wrapper.findComponent({ name: 'KeyCardGrid' })
    table.vm.$emit('update:selectedKeys', [1])
    await nextTick()
    wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('search')
    await flushPromises()
    expect(table.props('selectedKeys')).toEqual([])
    expect(wrapper.find('[data-test="bulk-edit-keys"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('removes successful keys from the selection and refreshes the table', async () => {
    listKeys.mockResolvedValue({
      items: [createApiKey(), { ...createApiKey(), id: 2, name: 'Second' }],
      total: 2, pages: 1
    })
    const wrapper = await mountView()
    const table = wrapper.findComponent({ name: 'KeyCardGrid' })
    table.vm.$emit('update:selectedKeys', [1, 2])
    await nextTick()
    await wrapper.get('[data-test="bulk-edit-keys"]').trigger('click')
    wrapper.findComponent({ name: 'BulkEditKeysModal' }).vm.$emit('updated', [1])
    await flushPromises()
    expect(listKeys).toHaveBeenCalledTimes(2)
    expect(table.props('selectedKeys')).toEqual([2])
    wrapper.unmount()
  })

  it('drops keys that are no longer visible after a refresh', async () => {
    const wrapper = await mountView()
    const table = wrapper.findComponent({ name: 'KeyCardGrid' })
    table.vm.$emit('update:selectedKeys', [1])
    await nextTick()
    listKeys.mockResolvedValue({ items: [], total: 0, pages: 0 })
    await wrapper.get('button[title="Refresh"]').trigger('click')
    await flushPromises()
    expect(table.props('selectedKeys')).toEqual([])
    wrapper.unmount()
  })

  it('shows a hidden column when toggled and persists the preference', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'Rate Limit').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('rate_limit')
    expect(localStorage.getItem('api-key-hidden-columns')).toBe(
      JSON.stringify(['id', 'last_used_at', 'last_used_ip'])
    )
    expect(localStorage.getItem('api-key-column-settings-version')).toBe('3')
  })

  it('shows the API key ID column when toggled', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'ID').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('id')
    expect(wrapper.get('[data-test="key-id"]').text()).toBe('#1')
    expect(visibleColumnMeta(wrapper).find((column) => column.key === 'id')?.sortable).toBe(false)
  })

  it('shows the last used IP column when toggled', async () => {
    listKeys.mockResolvedValueOnce({
      items: [{ ...createApiKey(), last_used_ip: '203.0.113.10' }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'Last Used IP').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('last_used_ip')
    expect(wrapper.get('[data-test="last-used-ip"]').text()).toBe('203.0.113.10')
  })

  it('restores column preferences from localStorage on mount', async () => {
    localStorage.setItem('api-key-hidden-columns', JSON.stringify(['group', 'created_at']))
    localStorage.setItem('api-key-column-settings-version', '1')

    const wrapper = await mountView()

    expect(visibleColumnKeys(wrapper)).toEqual([
      'name',
      'key',
      'current_concurrency',
      'usage',
      'rate_limit',
      'expires_at',
      'status',
      'last_used_at',
      'actions',
    ])
    expect(localStorage.getItem('api-key-hidden-columns')).toBe(
      JSON.stringify(['group', 'created_at', 'last_used_ip', 'id'])
    )
    expect(localStorage.getItem('api-key-column-settings-version')).toBe('3')
  })

  it('does not include always-visible columns in the toggleable menu', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await nextTick()

    const columnMenuText = wrapper.text()
    expect(columnMenuText).toContain('API Key')
    expect(columnMenuText).toContain('ID')
    expect(columnMenuText).toContain('Current Concurrency')
    expect(columnMenuText).toContain('Rate Limit')
    expect(columnMenuText).toContain('Last Used IP')
    expect(columnMenuText).not.toContain('Name')
    expect(columnMenuText).not.toContain('Actions')
  })

  it('renders the current concurrency value', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-test="current-concurrency"]').text()).toBe('3')
  })

  it('uses plain card fields without sorting controls', async () => {
    const wrapper = await mountView()
    expect(visibleColumnMeta(wrapper).every(column => !column.sortable)).toBe(true)
    expect(listKeys).toHaveBeenLastCalledWith(1, 20, { sort_by: 'created_at', sort_order: 'desc' }, expect.anything())
  })

  it('places controls in the header and the total opposite the endpoints', async () => {
    getPublicSettings.mockResolvedValue({
      api_base_url: 'https://api.example.com/v1',
      custom_endpoints: [],
    })
    const wrapper = await mountView()

    const row = wrapper.get('[data-test="keys-filter-row"]')
    expect(row.findComponent({ name: 'SearchInput' }).exists()).toBe(false)
    expect(row.get('[data-test="endpoint-popover"]').exists()).toBe(true)
    expect(row.get('[data-test="keys-total"]').text()).toBe('1 keys')
    const header = wrapper.get('header')
    expect(header.findComponent({ name: 'SearchInput' }).exists()).toBe(true)
    expect(header.findAllComponents({ name: 'Select' })).toHaveLength(2)
    expect(header.get('button[title="Refresh"]').exists()).toBe(true)
    expect(header.get('button[title="Column Settings"]').exists()).toBe(true)
    expect(header.find('[data-tour="keys-create-btn"]').exists()).toBe(false)
    const actions = row.get('.keys-summary-actions')
    expect(actions.element.children[0]).toBe(row.get('[data-test="keys-total"]').element)
    expect(actions.element.children[1]).toBe(row.get('[data-tour="keys-create-btn"]').element)
  })

  describe('infinite scrolling', () => {
    it('appends pages once, deduplicates keys and retains selected cards', async () => {
      listKeys.mockResolvedValueOnce({ items: [createApiKey()], total: 3, pages: 2 })
      let resolvePage!: (value: unknown) => void
      listKeys.mockReturnValueOnce(new Promise(resolve => { resolvePage = resolve }))
      const wrapper = await mountView()
      const cards = wrapper.findComponent({ name: 'KeyCardGrid' })
      cards.vm.$emit('update:selectedKeys', [1])
      await reachEnd()
      await reachEnd()
      expect(listKeys).toHaveBeenCalledTimes(2)
      expect(listKeys.mock.calls[1]?.slice(0, 2)).toEqual([2, 20])
      resolvePage({ items: [createApiKey(), { ...createApiKey(), id: 2 }], total: 3, pages: 2 })
      await flushPromises()
      expect(cards.props('data').map((key: ApiKey) => key.id)).toEqual([1, 2])
      expect(cards.props('selectedKeys')).toEqual([1])
      await reachEnd()
      expect(listKeys).toHaveBeenCalledTimes(2)
      wrapper.unmount()
    })

    it('rejects a stale page after filters change and cancels it on unmount', async () => {
      listKeys.mockResolvedValueOnce({ items: [createApiKey()], total: 2, pages: 2 })
      const wrapper = await mountView()
      let resolvePage!: (value: unknown) => void
      listKeys.mockReturnValueOnce(new Promise(resolve => { resolvePage = resolve }))
      await reachEnd()
      const pageSignal = listKeys.mock.calls[1]?.[3].signal
      listKeys.mockResolvedValueOnce({ items: [{ ...createApiKey(), id: 9 }], total: 1, pages: 1 })
      wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('update:modelValue', 'new')
      wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('search')
      await flushPromises()
      expect(pageSignal.aborted).toBe(true)
      resolvePage({ items: [{ ...createApiKey(), id: 2 }], total: 2, pages: 2 })
      await flushPromises()
      expect(wrapper.findComponent({ name: 'KeyCardGrid' }).props('data').map((key: ApiKey) => key.id)).toEqual([9])
      const latestSignal = listKeys.mock.calls[2]?.[3].signal
      wrapper.unmount()
      expect(latestSignal.aborted).toBe(true)
    })

    it('retains loaded cards after a page fails and retries the same page', async () => {
      listKeys.mockResolvedValueOnce({ items: [createApiKey()], total: 2, pages: 2 })
      const wrapper = await mountView()
      listKeys.mockRejectedValueOnce(new Error('offline'))
      await reachEnd()
      expect(wrapper.findComponent({ name: 'KeyCardGrid' }).props('data')).toHaveLength(1)
      listKeys.mockResolvedValueOnce({ items: [{ ...createApiKey(), id: 2 }], total: 2, pages: 2 })
      await wrapper.get('[data-test="keys-retry-more"]').trigger('click')
      await flushPromises()
      expect(listKeys.mock.calls[2]?.[0]).toBe(2)
      expect(wrapper.findComponent({ name: 'KeyCardGrid' }).props('data')).toHaveLength(2)
      wrapper.unmount()
    })

    it('merges slow usage responses from multiple pages without cancelling earlier usage', async () => {
      listKeys.mockResolvedValueOnce({ items: [createApiKey()], total: 2, pages: 2 })
      let resolveFirst!: (value: unknown) => void
      getDashboardApiKeysUsage.mockReturnValueOnce(new Promise(resolve => { resolveFirst = resolve }))
      const wrapper = await mountView()
      const usageSignal = getDashboardApiKeysUsage.mock.calls[0]?.[1].signal
      listKeys.mockResolvedValueOnce({ items: [{ ...createApiKey(), id: 2 }], total: 2, pages: 2 })
      getDashboardApiKeysUsage.mockResolvedValueOnce({ stats: { 2: { today_actual_cost: 2, total_actual_cost: 4 } } })
      await reachEnd()
      expect(usageSignal.aborted).toBe(false)
      resolveFirst({ stats: { 1: { today_actual_cost: 1, total_actual_cost: 3 } } })
      await flushPromises()
      const usage = wrapper.findAll('[data-test="usage"]')
      expect(usage[0]?.text()).toContain('$3.0000')
      expect(usage[1]?.text()).toContain('$4.0000')
      wrapper.unmount()
    })
  })

  describe('create group selection', () => {
    const platforms = ['anthropic', 'openai', 'kimi', 'zhipu', 'deepseek', 'minimax', 'gemini', 'grok', 'antigravity', 'composite', 'opencode_go', 'typesafe']
    const availableGroups = platforms.map((platform, index) => ({
      id: index + 1,
      name: `Shared group ${index + 1}`,
      platform,
      rate_multiplier: 1,
      subscription_type: 'standard',
    }))
    const groupRows = (wrapper: VueWrapper) => wrapper.findAll('[data-group-id]')
    const optionIds = (wrapper: VueWrapper) => groupRows(wrapper).map((row) => Number(row.attributes('data-group-id')))
    const selectedGroupIds = (wrapper: VueWrapper) =>
      groupRows(wrapper)
        .filter((row) => row.classes().includes('group-row-selected'))
        .map((row) => Number(row.attributes('data-group-id')))
    const openCreate = async () => {
      const wrapper = await mountView()
      await wrapper.get('[data-tour="keys-create-btn"]').trigger('click')
      return wrapper
    }

    beforeEach(() => {
      localStorage.setItem('api-key-group-picker-mode', 'native')
      getAvailableGroups.mockResolvedValue(availableGroups)
    })

    it('removes the provider selector and exposes every available group', async () => {
      const wrapper = await openCreate()
      expect(wrapper.find('[data-tour="key-form-provider"]').exists()).toBe(false)
      expect(wrapper.findAll('input[name="key-provider"]')).toHaveLength(0)
      expect([...optionIds(wrapper)].sort((a, b) => a - b)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12])
      expect(wrapper.findAllComponents({ name: 'Select' })[0].props('options')).toHaveLength(14)
    })

    it('submits the selected group without a provider step', async () => {
      const wrapper = await openCreate()
      await wrapper.get('[data-tour="key-form-name"]').setValue('My key')
      await wrapper.get('#key-form').trigger('submit')
      expect(keysAPI.create).not.toHaveBeenCalled()
      expect(showError).toHaveBeenCalledWith('keys.groupRequired')

      await wrapper.get('[data-group-id="2"]').trigger('click')
      vi.mocked(keysAPI.create).mockResolvedValue({ ...createApiKey(), group_id: 2 })
      await wrapper.get('#key-form').trigger('submit')
      await flushPromises()
      expect(keysAPI.create).toHaveBeenCalledOnce()
      expect(vi.mocked(keysAPI.create).mock.calls[0].slice(0, 2)).toEqual(['My key', 2])
    })

    it('shows configured protocol and client tags on each group card', async () => {
      getAvailableGroups.mockResolvedValue([
        { ...availableGroups[0], required_protocol: 'anthropic', blocked_user_agents: ' Claude, , cursor, CLAUDE ' },
        { ...availableGroups[1], required_protocol: 'openai', blocked_user_agents: '' },
        { ...availableGroups[6], required_protocol: 'google' },
        { ...availableGroups[2], blocked_user_agents: 'codex' },
        { ...availableGroups[3], required_protocol: '', blocked_user_agents: ' , ' },
      ])
      const wrapper = await openCreate()
      const tags = wrapper.get('[data-group-id="1"] [data-test="group-policy-tags"]')
      expect(tags.text()).toContain('仅限Anthropic协议')
      expect(tags.text()).toContain('排除客户端：claude')
      expect(tags.text()).toContain('排除客户端：cursor')
      expect(tags.findAll('span')).toHaveLength(3)
      expect(wrapper.get('[data-group-id="2"]').text()).toContain('仅限OpenAI协议')
      expect(wrapper.get('[data-group-id="7"]').text()).toContain('仅限Google协议')
      const clientOnly = wrapper.get('[data-group-id="3"] [data-test="group-policy-tags"]')
      expect(clientOnly.text()).toBe('排除客户端：codex')
      expect(wrapper.find('[data-group-id="4"] [data-test="group-policy-tags"]').exists()).toBe(false)
    })

    it('shows custom tags beside the cap with descriptions only on hover', async () => {
      getGroupCategories.mockResolvedValue({
        categories: [], assignments: {}, group_caps: { '1': 0.8 },
        group_tags: { '1': [{ name: '快速', description: '优先处理请求' }, { name: '稳定', description: '' }] },
      })
      const wrapper = await openCreate()
      const footer = wrapper.get('[data-group-id="1"] .group-row-footer')
      expect(footer.find('.group-cap-tag').exists()).toBe(true)
      const tags = footer.findAll('[data-test="group-custom-tag"]')
      expect(tags.map(tag => tag.text())).toEqual(['快速', '稳定'])
      expect(tags[0].attributes('title')).toBe('优先处理请求')
      expect(tags[1].attributes('title')).toBeUndefined()
      expect(footer.text()).not.toContain('优先处理请求')
      expect(wrapper.find('[data-group-id="2"] [data-test="group-custom-tag"]').exists()).toBe(false)
    })

    it('shows only the groups returned by the API', async () => {
      getAvailableGroups.mockResolvedValue([availableGroups[5]])
      const wrapper = await openCreate()
      expect(optionIds(wrapper)).toEqual([6])
    })

    it('shows no group options when no groups are available', async () => {
      getAvailableGroups.mockResolvedValue([])
      const wrapper = await openCreate()
      expect(optionIds(wrapper)).toEqual([])
    })

    it('updates group options when groups arrive after opening', async () => {
      let resolveGroups!: (value: typeof availableGroups) => void
      getAvailableGroups.mockReturnValue(new Promise((resolve) => { resolveGroups = resolve }))
      const wrapper = await openCreate()
      resolveGroups([availableGroups[1]])
      await flushPromises()
      expect(optionIds(wrapper)).toEqual([2])
    })

    it('resets the group when reopening create and preserves edit options', async () => {
      const wrapper = await openCreate()
      await wrapper.get('[data-group-id="5"]').trigger('click')
      expect(selectedGroupIds(wrapper)).toEqual([5])
      await wrapper.get('[data-test="close-dialog"]').trigger('click')
      await wrapper.get('[data-tour="keys-create-btn"]').trigger('click')
      expect([...optionIds(wrapper)].sort((a, b) => a - b)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12])
      expect(selectedGroupIds(wrapper)).toEqual([])
      await wrapper.get('[data-test="close-dialog"]').trigger('click')
      await getButtonByText(wrapper, 'common.edit').trigger('click')
      expect(wrapper.find('[data-tour="key-form-provider"]').exists()).toBe(false)
      // 编辑模式也使用左右布局分组选择：12 行分组列表
      expect(groupRows(wrapper)).toHaveLength(12)
    })
  })

  describe('usage stats loading', () => {
    it('renders the key list without waiting for slow usage stats', async () => {
      let resolveUsage!: (value: { stats: Record<string, unknown> }) => void
      getDashboardApiKeysUsage.mockReturnValue(new Promise((resolve) => { resolveUsage = resolve }))
      const wrapper = await mountView()

      const table = wrapper.findComponent({ name: 'KeyCardGrid' })
      expect(table.props('loading')).toBe(false)
      expect(table.props('data')).toHaveLength(1)
      expect(wrapper.find('[data-test="usage-pending"]').exists()).toBe(true)

      resolveUsage({
        stats: { 1: { api_key_id: 1, today_actual_cost: 0.5, total_actual_cost: 12.25 } },
      })
      await flushPromises()
      expect(wrapper.find('[data-test="usage-pending"]').exists()).toBe(false)
      expect(wrapper.get('[data-test="usage"]').text()).toContain('$12.2500')
    })

    it('stops showing usage placeholders when the usage request fails', async () => {
      const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
      getDashboardApiKeysUsage.mockRejectedValue(new Error('timeout'))
      const wrapper = await mountView()

      expect(wrapper.find('[data-test="usage-pending"]').exists()).toBe(false)
      expect(wrapper.get('[data-test="usage"]').text()).toContain('$0.0000')
      consoleError.mockRestore()
    })
  })
})
