import crypto from 'node:crypto'

function constantTimeEqual(left, right) {
  const a = Buffer.from(left)
  const b = Buffer.from(right)
  return a.length === b.length && crypto.timingSafeEqual(a, b)
}

function bearerToken(request) {
  const header = String(request.headers.authorization || '')
  return header.startsWith('Bearer ') ? header.slice(7).trim() : ''
}

export function createAdminAuthorizer({ devAdminToken, sub2apiAuthURL, fetchImpl = fetch }) {
  return async function authorizeAdmin(request) {
    const token = bearerToken(request)
    if (!token) return false

    if (devAdminToken && constantTimeEqual(token, devAdminToken)) return true

    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), 5000)
    try {
      const response = await fetchImpl(sub2apiAuthURL, {
        headers: { Authorization: `Bearer ${token}`, Accept: 'application/json' },
        signal: controller.signal,
      })
      if (!response.ok) return false
      const payload = await response.json()
      const user = payload?.data || payload
      return payload?.code !== undefined && payload.code !== 0 ? false : user?.role === 'admin'
    } catch {
      return false
    } finally {
      clearTimeout(timeout)
    }
  }
}
