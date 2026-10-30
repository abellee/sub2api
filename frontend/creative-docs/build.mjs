import { build } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from 'tailwindcss'
import autoprefixer from 'autoprefixer'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'
import loadConfig from 'tailwindcss/loadConfig.js'

const root = fileURLToPath(new URL('..', import.meta.url))
const consoleTheme = loadConfig(resolve(root, 'tailwind.config.js'))
await build({
  configFile: false,
  root,
  publicDir: false,
  plugins: [vue()],
  resolve: { alias: { '@': resolve(root, 'src'), 'vue-i18n': 'vue-i18n/dist/vue-i18n.runtime.esm-bundler.js' } },
  define: { __INTLIFY_JIT_COMPILATION__: true, 'process.env.NODE_ENV': '"production"' },
  css: { postcss: { plugins: [tailwindcss({
    ...consoleTheme,
    content: ['src/components/common/Select.vue', 'src/components/common/Input.vue', 'src/components/icons/Icon.vue', 'creative-docs/components.ts'].map(path => resolve(root, path)),
    corePlugins: { preflight: false }
  }), autoprefixer()] } },
  build: {
    outDir: resolve(root, 'public/creative-docs/components'),
    emptyOutDir: true,
    lib: { entry: resolve(root, 'creative-docs/components.ts'), formats: ['es'], fileName: () => 'components.js' }
  }
})
