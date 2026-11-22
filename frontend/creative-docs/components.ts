import { createApp, h, reactive, Teleport } from 'vue'
import { createI18n } from 'vue-i18n'
import Select from '../src/components/common/Select.vue'
import Input from '../src/components/common/Input.vue'
import Icon from '../src/components/icons/Icon.vue'
import { initThemeClass, useTheme } from '../src/composables/useTheme'
import '../src/styles/tokenfree-console.css'
import './utilities.css'

initThemeClass()

export function mountModelSelect(
  target: HTMLElement,
  options: string[],
  initialValue: string,
  onChange: (value: string) => void
) {
  const state = reactive({ options, value: initialValue })
  const app = createApp({
    setup() {
      const { isDark, setTheme } = useTheme()
      return () => [
        h(Select, {
          id: 'model',
          ariaLabel: '选择模型',
          modelValue: state.value,
          options: state.options.map(value => ({ value, label: value })),
          searchable: true,
          searchPlaceholder: '搜索模型',
          emptyText: '没有匹配的模型',
          'onUpdate:modelValue': (value) => {
            state.value = String(value)
            onChange(state.value)
          }
        }, {
          search: ({ modelValue, update, placeholder }: { modelValue: string; update: (value: string) => void; placeholder: string }) => [
            h('label', { for: 'model-search', class: 'sr-only' }, placeholder),
            h(Input, {
              id: 'model-search',
              class: 'model-search-field',
              modelValue,
              placeholder,
              autocomplete: 'off',
              'onUpdate:modelValue': update
            }, { prefix: () => h(Icon, { name: 'search', size: 'sm' }) })
          ]
        }),
        h(Teleport, { to: '#theme-toggle' }, h('button', {
          type: 'button',
          class: 'theme-toggle',
          'aria-label': isDark.value ? '切换浅色模式' : '切换深色模式',
          onClick: () => setTheme(isDark.value ? 'light' : 'dark')
        }, [h(Icon, { name: isDark.value ? 'sun' : 'moon', size: 'sm' }), h('span', isDark.value ? '浅色' : '深色')]))
      ]
    }
  })
  app.use(createI18n({ legacy: false, locale: 'zh', messages: { zh: { common: { selectOption: '选择模型', searchPlaceholder: '搜索模型', noOptionsFound: '没有匹配的模型' } } } }))
  app.mount(target)
  return {
    update(options: string[], value: string) {
      state.options = options
      state.value = value
    }
  }
}
