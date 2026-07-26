import { describe, expect, it } from 'vitest'
import {
  closeFloatingWidget,
  getFloatingWidgetFingerprint,
  getFloatingWidgetStorageKey,
  parseBeijingTimestamp,
  parseFloatingWidgetConfig,
  readFloatingWidgetRecord,
  shouldShowFloatingWidget,
  type FloatingWidgetConfig
} from '../floatingWidgetVisibility'

const config: FloatingWidgetConfig = {
  id: 'float-size-test-v1',
  closable: true,
  startTime: '2026-07-26 00:00:00',
  endTime: '2026-07-26 23:59:59',
  displayCount: 3
}

describe('floating widget visibility', () => {
  it('parses Beijing time independently of browser timezone', () => {
    expect(parseBeijingTimestamp('2026-07-26 08:30:15')).toBe(
      Date.UTC(2026, 6, 26, 0, 30, 15)
    )
    expect(parseBeijingTimestamp('2026-02-30 08:30:15')).toBeNull()
  })

  it('rejects invalid remote configuration', () => {
    expect(parseFloatingWidgetConfig({ ...config, id: ' ' })).toBeNull()
    expect(parseFloatingWidgetConfig({ ...config, displayCount: 0 })).toBeNull()
    expect(parseFloatingWidgetConfig({ ...config, startTime: 'not-a-date' })).toBeNull()
  })

  it('shows inclusively only inside the configured window', () => {
    const record = { fingerprint: getFloatingWidgetFingerprint(config), closeCount: 0 }
    const start = parseBeijingTimestamp(config.startTime)!
    const end = parseBeijingTimestamp(config.endTime)!

    expect(shouldShowFloatingWidget(config, record, start - 1)).toBe(false)
    expect(shouldShowFloatingWidget(config, record, start)).toBe(true)
    expect(shouldShowFloatingWidget(config, record, end)).toBe(true)
    expect(shouldShowFloatingWidget(config, record, end + 1)).toBe(false)
  })

  it('stops showing after the configured number of closes', () => {
    const timestamp = parseBeijingTimestamp(config.startTime)!

    expect(
      shouldShowFloatingWidget(
        config,
        { fingerprint: getFloatingWidgetFingerprint(config), closeCount: 2 },
        timestamp
      )
    ).toBe(true)
    expect(
      shouldShowFloatingWidget(
        config,
        { fingerprint: getFloatingWidgetFingerprint(config), closeCount: 3 },
        timestamp
      )
    ).toBe(false)
  })

  it('resets the close count when any configuration value changes', () => {
    const storage = new Map<string, string>()
    const storageAdapter = { getItem: (key: string) => storage.get(key) ?? null }
    storage.set(
      getFloatingWidgetStorageKey(config.id),
      JSON.stringify({ fingerprint: getFloatingWidgetFingerprint(config), closeCount: 2 })
    )

    expect(readFloatingWidgetRecord(storageAdapter, config).closeCount).toBe(2)
    expect(readFloatingWidgetRecord(storageAdapter, { ...config, closable: false }).closeCount).toBe(0)
  })

  it('persists each close under the configuration id', () => {
    const storage = new Map<string, string>()
    const storageAdapter = { setItem: (key: string, value: string) => storage.set(key, value) }
    const record = closeFloatingWidget(storageAdapter, config, {
      fingerprint: getFloatingWidgetFingerprint(config),
      closeCount: 1
    })

    expect(record.closeCount).toBe(2)
    expect(JSON.parse(storage.get(getFloatingWidgetStorageKey(config.id))!)).toEqual(record)
  })
})
