export interface FloatingWidgetConfig {
  id: string
  closable: boolean
  startTime: string
  endTime: string
  displayCount: number
}

export interface FloatingWidgetStorageRecord {
  fingerprint: string
  closeCount: number
}

export const FLOATING_WIDGET_STORAGE_PREFIX = 'sub2api:floating-widget:'

const BEIJING_TIME_PATTERN =
  /^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2}):(\d{2})$/

export function parseFloatingWidgetConfig(value: unknown): FloatingWidgetConfig | null {
  if (!value || typeof value !== 'object') return null

  const config = value as Record<string, unknown>
  const id = typeof config.id === 'string' ? config.id.trim() : ''
  const displayCount = Number(config.displayCount)

  if (
    !id ||
    typeof config.closable !== 'boolean' ||
    typeof config.startTime !== 'string' ||
    typeof config.endTime !== 'string' ||
    !Number.isInteger(displayCount) ||
    displayCount < 1
  ) {
    return null
  }

  const parsedConfig: FloatingWidgetConfig = {
    id,
    closable: config.closable,
    startTime: config.startTime,
    endTime: config.endTime,
    displayCount
  }

  const startTimestamp = parseBeijingTimestamp(parsedConfig.startTime)
  const endTimestamp = parseBeijingTimestamp(parsedConfig.endTime)
  if (startTimestamp === null || endTimestamp === null || startTimestamp > endTimestamp) return null

  return parsedConfig
}

export function parseBeijingTimestamp(value: string): number | null {
  const match = BEIJING_TIME_PATTERN.exec(value)
  if (!match) return null

  const year = Number(match[1])
  const month = Number(match[2])
  const day = Number(match[3])
  const hour = Number(match[4])
  const minute = Number(match[5])
  const second = Number(match[6])
  const timestamp = Date.UTC(year, month - 1, day, hour - 8, minute, second)
  const beijingDate = new Date(timestamp + 8 * 60 * 60 * 1000)

  if (
    beijingDate.getUTCFullYear() !== year ||
    beijingDate.getUTCMonth() !== month - 1 ||
    beijingDate.getUTCDate() !== day ||
    beijingDate.getUTCHours() !== hour ||
    beijingDate.getUTCMinutes() !== minute ||
    beijingDate.getUTCSeconds() !== second
  ) {
    return null
  }

  return timestamp
}

export function getFloatingWidgetStorageKey(id: string): string {
  return `${FLOATING_WIDGET_STORAGE_PREFIX}${id}`
}

export function getFloatingWidgetFingerprint(config: FloatingWidgetConfig): string {
  return JSON.stringify(config)
}

export function readFloatingWidgetRecord(
  storage: Pick<Storage, 'getItem'>,
  config: FloatingWidgetConfig
): FloatingWidgetStorageRecord {
  const fingerprint = getFloatingWidgetFingerprint(config)

  try {
    const rawRecord = storage.getItem(getFloatingWidgetStorageKey(config.id))
    if (!rawRecord) return { fingerprint, closeCount: 0 }

    const record = JSON.parse(rawRecord) as Partial<FloatingWidgetStorageRecord>
    if (
      record.fingerprint !== fingerprint ||
      !Number.isInteger(record.closeCount) ||
      Number(record.closeCount) < 0
    ) {
      return { fingerprint, closeCount: 0 }
    }

    return { fingerprint, closeCount: Number(record.closeCount) }
  } catch {
    return { fingerprint, closeCount: 0 }
  }
}

export function shouldShowFloatingWidget(
  config: FloatingWidgetConfig,
  record: FloatingWidgetStorageRecord,
  now = Date.now()
): boolean {
  const startTimestamp = parseBeijingTimestamp(config.startTime)
  const endTimestamp = parseBeijingTimestamp(config.endTime)

  return (
    startTimestamp !== null &&
    endTimestamp !== null &&
    now >= startTimestamp &&
    now <= endTimestamp &&
    record.closeCount < config.displayCount
  )
}

export function closeFloatingWidget(
  storage: Pick<Storage, 'setItem'>,
  config: FloatingWidgetConfig,
  record: FloatingWidgetStorageRecord
): FloatingWidgetStorageRecord {
  const nextRecord = {
    fingerprint: getFloatingWidgetFingerprint(config),
    closeCount: Math.min(record.closeCount + 1, config.displayCount)
  }

  try {
    storage.setItem(getFloatingWidgetStorageKey(config.id), JSON.stringify(nextRecord))
  } catch {
    // Closing the current widget still works when storage is unavailable.
  }

  return nextRecord
}
