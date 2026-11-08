import { describe, expect, it, vi } from 'vitest'
import { i18n, loadLocaleMessages } from '..'

describe('locale message interpolation', () => {
  it.each([
    ['zh', '共 48 条', '已选 2/5', '鹈鹕渲染 · Account'],
    ['en', '48 in total', '2/5 selected', 'Pelican render · Account'],
  ] as const)('replaces %s placeholders without dynamic code execution', async (locale, total, selected, title) => {
    await loadLocaleMessages(locale)
    const dynamicFunction = vi.fn(() => { throw new Error('unsafe-eval is disabled') })
    vi.stubGlobal('Function', dynamicFunction)
    try {
      expect(i18n.global.t('admin.iqDetection.testResultsTotal', { n: 48 }, { locale })).toBe(total)
      expect(i18n.global.t('admin.iqDetection.selectedCount', { selected: 2, total: 5 }, { locale })).toBe(selected)
      expect(i18n.global.t('admin.iqDetection.pelicanRenderTitle', { name: 'Account' }, { locale })).toBe(title)
      expect(i18n.global.t('channelMonitorV2.bootstrap.progress', { percent: 72 }, { locale })).toBe(
        locale === 'zh' ? '进度 72%' : '72% complete',
      )
      expect(i18n.global.t('channelMonitorV2.metrics.errorRateValue', { value: '1.25%' }, { locale })).toBe(
        locale === 'zh' ? '错误率 1.25%' : 'Error rate 1.25%',
      )
      expect(i18n.global.t('channelMonitorV2.bucket.minutes', { count: 5 }, { locale })).toBe(
        locale === 'zh' ? '5 分钟粒度' : '5-minute buckets',
      )
      expect(dynamicFunction).not.toHaveBeenCalled()
    } finally {
      vi.unstubAllGlobals()
    }
  })
})
