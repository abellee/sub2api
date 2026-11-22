/** 抽奖和任务中心的业务时间。北京时间是固定 UTC+8，不跟浏览器时区走。 */
const BEIJING_OFFSET_MS = 8 * 60 * 60 * 1000

function pad(n: number): string {
  return String(n).padStart(2, '0')
}

function beijingWall(date: Date): Date {
  return new Date(date.getTime() + BEIJING_OFFSET_MS)
}

/** 北京时间日历日，YYYY-MM-DD。 */
export function beijingToday(now = new Date()): string {
  const wall = beijingWall(now)
  return `${wall.getUTCFullYear()}-${pad(wall.getUTCMonth() + 1)}-${pad(wall.getUTCDate())}`
}

/** ISO 时刻格式化为北京时间 YYYY/MM/DD HH:mm:ss。纯日期串只换分隔符。 */
export function formatBeijingDateTime(value: string): string {
  if (!value) return '—'
  if (/^\d{4}-\d{2}-\d{2}$/.test(value)) return value.slice(0, 10).replace(/-/g, '/')
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const wall = beijingWall(date)
  return `${wall.getUTCFullYear()}/${pad(wall.getUTCMonth() + 1)}/${pad(wall.getUTCDate())} ${pad(wall.getUTCHours())}:${pad(wall.getUTCMinutes())}:${pad(wall.getUTCSeconds())}`
}

/** ISO 时刻转成 datetime-local 的北京时间钟面。 */
export function isoToBeijingInput(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const wall = beijingWall(date)
  return `${wall.getUTCFullYear()}-${pad(wall.getUTCMonth() + 1)}-${pad(wall.getUTCDate())}T${pad(wall.getUTCHours())}:${pad(wall.getUTCMinutes())}`
}

/** datetime-local 的值按北京时间解释，输出 UTC ISO。 */
export function beijingInputToISO(value: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(value)
  if (!match) return new Date(value).toISOString()
  const utc = Date.UTC(
    Number(match[1]),
    Number(match[2]) - 1,
    Number(match[3]),
    Number(match[4]) - 8,
    Number(match[5])
  )
  return new Date(utc).toISOString()
}

/** 日历日加减，不经过浏览器时区。 */
export function addCalendarDays(date: string, days: number): string {
  const [y, m, d] = date.slice(0, 10).split('-').map(Number)
  return new Date(Date.UTC(y, m - 1, d + days)).toISOString().slice(0, 10)
}
