import path from 'node:path'

function parseAddress(value) {
  const raw = String(value || '127.0.0.1:8091').trim()
  const separator = raw.lastIndexOf(':')
  if (separator < 0) return { host: raw, port: 8091 }
  const host = raw.slice(0, separator) || '127.0.0.1'
  const port = Number(raw.slice(separator + 1))
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error(`invalid PUSH_ADDR port: ${raw}`)
  }
  return { host, port }
}

export function loadConfig(environment = process.env) {
  const address = parseAddress(environment.PUSH_ADDR)
  return {
    ...address,
    dataFile: path.resolve(environment.PUSH_DATA_FILE || './data/push-notifier.json'),
    vapidSubject: String(environment.PUSH_VAPID_SUBJECT || 'mailto:admin@example.com').trim(),
    sub2apiAuthURL: String(
      environment.SUB2API_AUTH_URL || 'http://127.0.0.1:8080/api/v1/auth/me',
    ).trim(),
    devAdminToken: String(environment.PUSH_DEV_ADMIN_TOKEN || '').trim(),
    internalToken: String(environment.PUSH_INTERNAL_TOKEN || '').trim(),
  }
}
