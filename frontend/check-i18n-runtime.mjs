import assert from 'node:assert/strict'
import { build } from 'vite'

// Use the production config so a unit-test-only compiler flag cannot hide this regression.
const entry = 'virtual:i18n-runtime-check'
const result = await build({
  configFile: 'vite.config.ts',
  logLevel: 'error',
  plugins: [{
    name: 'i18n-runtime-check',
    configResolved(config) { delete config.build.rollupOptions.output.manualChunks },
    resolveId: id => id === entry ? entry : undefined,
    load: id => id === entry ? `
      import { createI18n } from 'vue-i18n'
      import iq from '/src/i18n/locales/zh/admin/iqDetection.ts'
      import monitor from '/src/i18n/locales/zh/channelMonitorV2.ts'
      const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh: { admin: iq, ...monitor } } })
      export const values = [
        i18n.global.t('admin.iqDetection.testResultsTotal', { n: 48 }),
        i18n.global.t('channelMonitorV2.bootstrap.progress', { percent: 72 }),
        i18n.global.t('channelMonitorV2.metrics.errorRateValue', { value: '1.25%' })
      ]
    ` : undefined,
  }],
  build: {
    write: false,
    rollupOptions: {
      input: entry,
      preserveEntrySignatures: 'strict',
      output: { inlineDynamicImports: true, format: 'es' },
    },
  },
})
const code = result.output.find(chunk => chunk.type === 'chunk' && chunk.isEntry).code
const originalFunction = globalThis.Function
globalThis.Function = () => { throw new Error('unsafe-eval is disabled') }
try {
  const { values } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
  assert.deepEqual(values, ['共 48 条', '进度 72%', '错误率 1.25%'])
} finally {
  globalThis.Function = originalFunction
}
console.log('Production i18n interpolation passed without unsafe-eval')
