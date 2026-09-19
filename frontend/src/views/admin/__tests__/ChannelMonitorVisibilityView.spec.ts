import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import ChannelMonitorVisibilityView from '@/views/admin/ChannelMonitorVisibilityView.vue'

vi.mock('@/features/channel-monitor-v2/ChannelMonitorVisibilityPanel.vue', () => ({
  default: { name: 'ChannelMonitorVisibilityPanel', template: '<div data-testid="visibility-panel" />' },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('ChannelMonitorVisibilityView', () => {
  it('renders the visibility panel on its own admin page', () => {
    const wrapper = mount(ChannelMonitorVisibilityView, {
      global: {
        stubs: {
          AppLayout: defineComponent({ template: '<main><slot /></main>' }),
          Icon: true,
        },
      },
    })

    expect(wrapper.text()).toContain('admin.channelMonitor.visibility.title')
    expect(wrapper.find('[data-testid="visibility-panel"]').exists()).toBe(true)
  })
})
