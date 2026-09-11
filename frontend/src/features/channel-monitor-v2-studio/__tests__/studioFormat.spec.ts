import { describe, expect, it } from 'vitest'
import {
  cacheTone,
  errorRateTone,
  metricRingProgress,
  overallToneFromRow,
  sparklinePoints,
  sparklineValues,
  statusFace,
  hexLuminance,
  collectStudioGroupRates,
  formatStudioCacheRate,
  formatStudioErrorRate,
  formatStudioMultiplier,
  formatStudioSuccessRate,
  groupStudioCardsByBrand,
  sortStudioCardsByStatus,
  studioAccentFromState,
  studioSparkPaint,
  allocStudioSparkIds,
  lookupStudioGroupRate,
  mergeSelectedStudioGroups,
  pickStudioActiveGroups,
  studioGroupActivityScore,
  studioChartGap,
  studioChartDots,
  studioChartArea,
  studioChartTrackWidth,
  studioFaceDepth,
  studioFacePalette,
  studioValueShade,
  truncateStudioGroupName,
  studioTextDisplayWidth,
  ttftTone,
} from '../studioFormat'
import type { MonitorHealth, MonitorMetric } from '@/api/channelMonitorV2'

function metric(partial: Partial<MonitorMetric> = {}): MonitorMetric {
  return {
    success_requests: 0,
    error_requests: 0,
    request_count: 0,
    token_count: 0,
    rpm: 0,
    tpm: 0,
    error_rate: 0,
    cache_rate: 0,
    cache_rate_numerator: 0,
    cache_rate_denominator: 0,
    ttft: { sample_count: 0, p50_ms: null, p95_ms: null, avg_ms: null },
    duration: { sample_count: 0, p50_ms: null, p95_ms: null, avg_ms: null },
    ...partial,
  }
}

function health(partial: Partial<MonitorHealth> = {}): MonitorHealth {
  return {
    overall: 'unknown',
    error_rate: 'unknown',
    ttft: 'unknown',
    minimum_sample: 1,
    ...partial,
  }
}

describe('studioFormat', () => {
  it('truncates group names to 8 Chinese-character widths with English at half', () => {
    expect(studioTextDisplayWidth('默认分组')).toBe(4)
    expect(studioTextDisplayWidth('OpenAI')).toBe(3)
    expect(truncateStudioGroupName('默认分组名称')).toBe('默认分组名称')
    expect(truncateStudioGroupName('默认分组名称测试用字多')).toBe('默认分组名称测试…')
    expect(truncateStudioGroupName('ABCDEFGHIJKLMNOP')).toBe('ABCDEFGHIJKLMNOP')
    expect(truncateStudioGroupName('ABCDEFGHIJKLMNOPQ')).toBe('ABCDEFGHIJKLMNOP…')
    expect(truncateStudioGroupName('GPT线路测试分组名称')).toBe('GPT线路测试分组…')
  })

  it('maps success and cache rates onto 0–100 rings', () => {
    expect(metricRingProgress('success', metric({ error_rate: 0.02 }))).toBeCloseTo(98)
    expect(metricRingProgress('cache', metric({ cache_rate: 0.4 }))).toBeCloseTo(40)
    expect(metricRingProgress('tps', metric({ tpm: 1200 }))).toBeNull()
  })

  it('uses ttft score for the latency ring and skips missing scores', () => {
    expect(metricRingProgress('ttft', metric(), health({ ttft_score: 72 }))).toBeCloseTo(72)
    expect(metricRingProgress('ttft', metric(), health())).toBeNull()
  })

  it('builds a sparkline polyline from trend points', () => {
    const values = sparklineValues(
      [{ metrics: metric({ error_rate: 0.2 }) }, { metrics: metric({ error_rate: 0.1 }) }, { metrics: metric({ error_rate: 0 }) }],
      (item) => 1 - item.error_rate,
    )
    const points = sparklinePoints(values)
    expect(values).toEqual([0.8, 0.9, 1])
    expect(points.split(' ')).toHaveLength(3)
  })

  it('places a chart dot on every time slot including empty ones', () => {
    const dots = studioChartDots([0.8, null, 1])
    expect(dots).toHaveLength(3)
    expect(dots[0].value).toBe(0.8)
    expect(dots[1].value).toBeNull()
    expect(dots[2].value).toBe(1)
    expect(dots[0].x).toBeLessThan(dots[1].x)
    expect(dots[1].x).toBeLessThan(dots[2].x)
  })

  it('maps health scores onto status faces without treating missing samples as critical', () => {
    expect(statusFace(95)).toEqual({ mood: 'joy', tone: 'healthy' })
    expect(statusFace(82)).toEqual({ mood: 'happy', tone: 'healthy' })
    expect(statusFace(66)).toEqual({ mood: 'neutral', tone: 'warning' })
    expect(statusFace(66, 'healthy')).toEqual({ mood: 'smile', tone: 'healthy' })
    expect(statusFace(50)).toEqual({ mood: 'neutral', tone: 'warning' })
    expect(statusFace(40)).toEqual({ mood: 'sad', tone: 'critical' })
    expect(statusFace(40, 'warning')).toEqual({ mood: 'worried', tone: 'warning' })
    expect(statusFace(10)).toEqual({ mood: 'sad', tone: 'critical' })
    expect(statusFace(null)).toEqual({ mood: 'blank', tone: 'unknown' })
    expect(statusFace(Number.NaN)).toEqual({ mood: 'blank', tone: 'unknown' })
    expect(statusFace(96, 'critical')).toEqual({ mood: 'sad', tone: 'critical' })
    expect(statusFace(10, 'healthy')).toEqual({ mood: 'smile', tone: 'healthy' })
  })

  it('colors success / TTFT / cache from V2 health thresholds', () => {
    expect(errorRateTone(0)).toBe('healthy')
    expect(errorRateTone(0.05)).toBe('warning')
    expect(errorRateTone(0.2)).toBe('critical')
    expect(cacheTone(0.9)).toBe('healthy')
    expect(cacheTone(0.7)).toBe('warning')
    expect(cacheTone(0)).toBe('critical')
    expect(ttftTone(400)).toBe('healthy')
    expect(ttftTone(4500)).toBe('warning')
    expect(ttftTone(12000)).toBe('critical')
    expect(ttftTone(null, null, { p50_ms: null, sample_count: 0 })).toBe('unknown')
  })

  it('darkens face skin as the score rises', () => {
    const low = studioFacePalette('healthy', 20)
    const high = studioFacePalette('healthy', 96)
    const warnLow = studioFacePalette('warning', 50)
    const warnHigh = studioFacePalette('warning', 78)
    const critLow = studioFacePalette('critical', 8)
    const critHigh = studioFacePalette('critical', 40)
    expect(hexLuminance(high.skin)).toBeLessThan(hexLuminance(low.skin))
    expect(hexLuminance(warnHigh.skin)).toBeLessThan(hexLuminance(warnLow.skin))
    expect(hexLuminance(critHigh.skin)).toBeLessThan(hexLuminance(critLow.skin))
    expect(studioFacePalette('healthy').skin).toBe('#6EE7B7')
    expect(studioFaceDepth('healthy', 80)).toBeLessThan(studioFaceDepth('healthy', 100))
    expect(studioValueShade(1, 0, 1)).toBeGreaterThan(studioValueShade(0, 0, 1))
    expect(hexLuminance(studioFacePalette('healthy', 100).skin)).toBeGreaterThan(0.5)
    expect(hexLuminance(studioFacePalette('healthy', 100).skin)).toBeLessThan(0.7)
    expect(
      hexLuminance(studioFacePalette('healthy', 80).skin) - hexLuminance(studioFacePalette('healthy', 100).skin),
    ).toBeGreaterThan(0.18)
    expect(hexLuminance(studioFacePalette('critical', 0).skin)).toBeGreaterThan(0.7)
    expect(hexLuminance(studioFacePalette('critical', 40).skin)).toBeGreaterThan(0.42)
    expect(hexLuminance(studioFacePalette('critical', 40).skin)).toBeLessThan(0.72)
  })

  it('widens the K-line track so dots keep a minimum gap', () => {
    expect(studioChartTrackWidth(2, 240, 40, 14)).toBe(240)
    expect(studioChartTrackWidth(12, 240, 40, 14)).toBe(14 * 2 + 11 * 40)
  })

  it('extends the K-line fill to both track edges so side fades have color to blend', () => {
    const dots = studioChartDots([0.4, 0.9], 200, 44, 14, 8)
    const area = studioChartArea(dots, 44, 200)
    expect(area.startsWith('0,44 0,')).toBe(true)
    expect(area.endsWith(',200.00,44') || area.endsWith(' 200.00,44')).toBe(true)
    expect(area).toContain('200.00,')
  })

  it('shrinks K-line gap on a short card so five dots fit, and keeps 40px on a wide card', () => {
    expect(studioChartGap(160, 40, 14, 5)).toBe((160 - 28) / 4)
    expect(studioChartGap(400, 40, 14, 5)).toBe(40)
  })

  it('formats a group multiplier next to the brand name', () => {
    expect(formatStudioMultiplier(0.08)).toBe('用户倍率0.08x')
    expect(formatStudioMultiplier(1)).toBe('用户倍率1x')
    expect(formatStudioMultiplier('0.1')).toBe('用户倍率0.1x')
    expect(formatStudioMultiplier(0.08, '用户倍率{n}x')).toBe('用户倍率0.08x')
    expect(formatStudioMultiplier(null)).toBe('')
  })

  it('looks up a group multiplier by id, custom override, then name', () => {
    const rates = collectStudioGroupRates(
      [
        { id: '2', name: 'GPT 标准', rate_multiplier: '1' },
        { id: 5, name: 'Claude Code 标准', rate_multiplier: 1 },
      ],
      { 5: 0.1 },
    )
    expect(lookupStudioGroupRate(rates, 2)).toBe(1)
    expect(lookupStudioGroupRate(rates, 5)).toBe(0.1)
    expect(lookupStudioGroupRate(rates, undefined, 'GPT 标准')).toBe(1)
    expect(lookupStudioGroupRate(rates, undefined, '示例 · 正常', 0.08)).toBe(0.08)
    expect(lookupStudioGroupRate(rates, 99)).toBeUndefined()
  })

  it('keeps every selected monitor group even when the matrix omitted idle rows', () => {
    const live = {
      platform: 'openai',
      group_id: 2,
      group_name: 'GPT 标准',
      metrics: metric({ rpm: 4, request_count: 10 }),
      health: health({ overall: 'healthy' }),
      buckets: [{ bucket_start: '2026-09-10T10:00:00.000Z', metrics: metric({ request_count: 1 }), health: health() }],
    }
    const rows = mergeSelectedStudioGroups(
      [live],
      [2, 5, 7, 9, 11, 13],
      [
        { id: 5, name: 'Claude 标准', platform: 'anthropic' },
        { id: 7, name: 'Grok 线路', platform: 'grok' },
      ],
    )
    expect(rows.map((row) => row.group_id)).toEqual([2, 5, 7, 9, 11, 13])
    expect(rows[0]).toEqual(live)
    expect(rows[1]).toMatchObject({ group_id: 5, group_name: 'Claude 标准', platform: 'anthropic', buckets: [] })
    expect(rows[2]?.group_name).toBe('Grok 线路')
    expect(rows[3]?.group_name).toBe('#9')
    expect(rows[3]?.metrics.request_count).toBe(0)
    expect(rows[3]?.health.overall).toBe('unknown')
  })

  it('groups cards by model brand and keeps first-seen brand order', () => {
    const sections = groupStudioCardsByBrand([
      { key: 'a', platform: 'openai', brandLabel: 'OpenAI' },
      { key: 'b', platform: 'anthropic', brandLabel: 'Anthropic' },
      { key: 'c', platform: 'openai', brandLabel: 'OpenAI' },
    ])
    expect(sections.map((section) => section.key)).toEqual(['openai', 'anthropic'])
    expect(sections[0]?.cards.map((card) => card.key)).toEqual(['a', 'c'])
    expect(sections[1]?.cards.map((card) => card.key)).toEqual(['b'])
    expect(sections[0]?.brandLabel).toBe('OpenAI')
  })

  it('sorts brand cards 正常 → 降级 → 失败 → 样本不足', () => {
    expect(
      sortStudioCardsByStatus([
        { key: 'idle', state: 'unknown' },
        { key: 'bad', state: 'critical' },
        { key: 'ok', state: 'healthy' },
        { key: 'warn', state: 'warning' },
        { key: 'ok-2', state: 'healthy' },
      ]).map((card) => card.key),
    ).toEqual(['ok', 'ok-2', 'warn', 'bad', 'idle'])
    const sections = groupStudioCardsByBrand([
      { key: 'idle', platform: 'openai', brandLabel: 'OpenAI', state: 'unknown' },
      { key: 'bad', platform: 'openai', brandLabel: 'OpenAI', state: 'critical' },
      { key: 'ok', platform: 'openai', brandLabel: 'OpenAI', state: 'healthy' },
      { key: 'warn', platform: 'anthropic', brandLabel: 'Anthropic', state: 'warning' },
    ])
    expect(sections[0]?.cards.map((card) => card.key)).toEqual(['ok', 'bad', 'idle'])
    expect(sections[1]?.cards.map((card) => card.key)).toEqual(['warn'])
  })

  it('paints each K-line from its own status accent', () => {
    expect(studioSparkPaint('teal').fill).toBe('#14b8a6')
    expect(studioSparkPaint('amber').fill).toBe('#d97706')
    expect(studioSparkPaint('coral').fill).toBe('#e11d48')
    expect(studioSparkPaint('slate').fill).toBe('#64748b')
    const a = allocStudioSparkIds()
    const b = allocStudioSparkIds()
    expect(a.fillId).not.toBe(b.fillId)
    expect(a.fadeId).not.toBe(b.fadeId)
  })

  it('maps card wash to status, not card index', () => {
    expect(studioAccentFromState('healthy')).toBe('teal')
    expect(studioAccentFromState('warning')).toBe('amber')
    expect(studioAccentFromState('critical')).toBe('coral')
    expect(studioAccentFromState('unknown')).toBe('slate')
    expect(studioAccentFromState(null)).toBe('teal')
  })

  it('picks the currently busiest groups for the active section', () => {
    const idle = {
      key: 'idle',
      metrics: metric(),
      health: health(),
      buckets: [{ metrics: metric(), health: health() }],
    }
    const live = {
      key: 'live',
      metrics: metric({ rpm: 4, request_count: 20 }),
      health: health({ overall: 'healthy', score: 90 }),
      buckets: [{ metrics: metric({ rpm: 4 }), health: health({ overall: 'healthy', score: 90 }) }],
    }
    const hot = {
      key: 'hot',
      metrics: metric({ rpm: 40, request_count: 200 }),
      health: health({ overall: 'healthy', score: 80 }),
      buckets: [{ metrics: metric({ rpm: 40 }), health: health({ overall: 'healthy', score: 80 }) }],
    }
    expect(studioGroupActivityScore(idle)).toBe(0)
    expect(pickStudioActiveGroups([idle, live, hot]).map((card) => card.key)).toEqual(['hot', 'live'])
    expect(pickStudioActiveGroups([live, hot, idle, { ...live, key: 'live-2' }, { ...hot, key: 'hot-2' }, { ...live, key: 'live-3' }], 4).map((card) => card.key)).toEqual([
      'hot',
      'hot-2',
      'live',
      'live-2',
    ])
    const mixed = pickStudioActiveGroups([
      { ...hot, key: 'bad', state: 'critical' },
      { ...live, key: 'ok', state: 'healthy' },
      { ...live, key: 'warn', state: 'warning', metrics: metric({ rpm: 12, request_count: 60 }) },
    ])
    expect(mixed.map((card) => card.key)).toEqual(['bad', 'warn', 'ok'])
    expect(sortStudioCardsByStatus(mixed).map((card) => card.key)).toEqual(['ok', 'warn', 'bad'])
  })

  it('sorts brand sections OpenAI → Anthropic → Grok even when cards arrive alphabetically', () => {
    const sections = groupStudioCardsByBrand([
      { key: 'a', platform: 'anthropic', brandLabel: 'Anthropic' },
      { key: 'g', platform: 'grok', brandLabel: 'Grok' },
      { key: 'o', platform: 'openai', brandLabel: 'OpenAI' },
    ])
    expect(sections.map((section) => section.key)).toEqual(['openai', 'anthropic', 'grok'])
  })

  it('reads success and error percents from success_rate after volume redaction', () => {
    const redacted = metric({
      request_count: 0,
      success_requests: 0,
      error_requests: 0,
      error_rate: 0,
      success_rate: 0.92,
      cache_rate: 0.4,
      cache_rate_denominator: 0,
      rpm: 3,
    })
    const live = health({ overall: 'healthy', score: 90 })
    expect(formatStudioSuccessRate(redacted, live)).toBe('92.0%')
    expect(formatStudioErrorRate(redacted, live)).toBe('8.0%')
    expect(formatStudioCacheRate(redacted, live)).toBe('40.0%')
    expect(formatStudioSuccessRate(metric(), health())).toBe('-')
    expect(formatStudioErrorRate(metric(), health())).toBe('-')
  })

  it('falls back to metric thresholds when API overall is unknown', () => {
    const unknown = health({ overall: 'unknown', error_rate: 'unknown', ttft: 'unknown' })
    expect(overallToneFromRow(metric({ error_rate: 0, cache_rate: 0.9, ttft: { sample_count: 10, p50_ms: 200, p95_ms: 300, avg_ms: 220 } }), unknown)).toBe('healthy')
    expect(overallToneFromRow(metric({ error_rate: 0, cache_rate: 0, ttft: { sample_count: 0, p50_ms: null, p95_ms: null, avg_ms: null } }), unknown)).toBe('unknown')
    expect(overallToneFromRow(metric({ error_rate: 0.08, cache_rate: 0.9 }), unknown)).toBe('warning')
    expect(
      overallToneFromRow(
        metric({ error_rate: 0, success_rate: 0.99, cache_rate: 0, rpm: 3 }),
        unknown,
      ),
    ).toBe('healthy')
    expect(
      overallToneFromRow(
        metric({ error_rate: 0, success_rate: 0, rpm: 4 }),
        health({ overall: 'healthy', error_rate: 'healthy', score: 92 }),
      ),
    ).toBe('critical')
  })
})
