import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import UserNavSections from '../UserNavSections.vue'

describe('UserNavSections', () => {
  it('renders configured titles with the My Account section styles and retains link order', async () => {
    const wrapper = mount(UserNavSections, {
      props: {
        collapsed: false,
        sections: [
          { id: 'services', title: '模型与服务', items: [{ path: '/keys' }, { path: '/model-plaza' }] },
          { id: 'default', title: '我的账户', items: [{ path: '/profile' }] },
        ],
      },
      slots: { default: '<template #default="{ item }"><a :href="item.path">{{ item.path }}</a></template>' },
    })
    expect(wrapper.findAll('.sidebar-section-title-text').map(title => title.text())).toEqual(['模型与服务', '我的账户'])
    expect(wrapper.findAll('a').map(link => link.attributes('href'))).toEqual(['/keys', '/model-plaza', '/profile'])
    await wrapper.setProps({ collapsed: true })
    expect(wrapper.findAll('.sidebar-section-title').every(title => title.attributes('aria-hidden') === 'true')).toBe(true)
    expect(wrapper.findAll('a')).toHaveLength(3)
  })
})
