import crypto from 'node:crypto'
import { mkdir, readFile, rename, writeFile } from 'node:fs/promises'
import path from 'node:path'
import webpush from 'web-push'

const EMPTY_STATE = Object.freeze({
  version: 1,
  vapid: null,
  subscriptions: [],
  messages: [],
  schedules: [],
  notifies: [],
  pageNotices: [],
})

function cloneEmptyState() {
  return JSON.parse(JSON.stringify(EMPTY_STATE))
}

const CLIENT_ID_PATTERN = /^[A-Za-z0-9_-]{8,128}$/

export function normalizeClientID(value) {
  const clientId = String(value || '').trim()
  return CLIENT_ID_PATTERN.test(clientId) ? clientId : ''
}

function subscriptionID(endpoint) {
  return crypto.createHash('sha256').update(String(endpoint || '')).digest('hex')
}

function clientSubscriptionID(clientId) {
  return crypto.createHash('sha256').update(`client:${clientId}`).digest('hex')
}

function normalizeUser(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const id = Number(value.id)
  const username = String(value.username || '').trim()
  const email = String(value.email || '').trim()
  if (!Number.isSafeInteger(id) || id <= 0 || (!username && !email)) return null
  return { id, username, email }
}

function isEnabled(subscription) {
  return subscription?.enabled !== false
}

export function isDeliverable(subscription) {
  return Boolean(
    isEnabled(subscription)
    && subscription.endpoint
    && subscription.keys?.p256dh
    && subscription.keys?.auth,
  )
}

function findSubscriptionIndex(subscriptions, { clientId, endpoint }) {
  if (clientId) {
    const id = clientSubscriptionID(clientId)
    const byClient = subscriptions.findIndex((item) => item.id === id || item.clientId === clientId)
    if (byClient >= 0) return byClient
  }
  if (endpoint) {
    const id = subscriptionID(endpoint)
    const byEndpoint = subscriptions.findIndex((item) => item.id === id || item.endpoint === endpoint)
    if (byEndpoint >= 0) return byEndpoint
  }
  return -1
}

function nextEnabled(existing, requested) {
  if (requested === false) return false
  if (requested === true) return true
  return !existing || existing.enabled !== false
}

function subscriptionSummary(subscription) {
  return {
    id: subscription.id,
    user: normalizeUser(subscription.user),
    enabled: isEnabled(subscription),
    createdAt: subscription.createdAt,
    lastSeenAt: subscription.lastSeenAt,
  }
}

export class PushStore {
  constructor(filePath) {
    this.filePath = filePath
    this.state = cloneEmptyState()
    this.writeQueue = Promise.resolve()
  }

  async init() {
    try {
      const raw = await readFile(this.filePath, 'utf8')
      const parsed = JSON.parse(raw)
      this.state = {
        version: 1,
        vapid: parsed.vapid || null,
        subscriptions: Array.isArray(parsed.subscriptions) ? parsed.subscriptions : [],
        messages: Array.isArray(parsed.messages) ? parsed.messages : [],
        schedules: Array.isArray(parsed.schedules) ? parsed.schedules : [],
        notifies: Array.isArray(parsed.notifies) ? parsed.notifies : [],
        pageNotices: Array.isArray(parsed.pageNotices) ? parsed.pageNotices : [],
      }
    } catch (error) {
      if (error?.code !== 'ENOENT') throw error
    }

    if (!this.state.vapid?.publicKey || !this.state.vapid?.privateKey) {
      this.state.vapid = webpush.generateVAPIDKeys()
      await this.persist()
    }
    return this
  }

  getVapidKeys() {
    return { ...this.state.vapid }
  }

  listSubscriptions() {
    return this.state.subscriptions.map((subscription) => ({
      ...subscription,
      keys: subscription.keys ? { ...subscription.keys } : null,
      user: subscription.user ? { ...subscription.user } : null,
    }))
  }

  listDeliverableSubscriptions() {
    return this.listSubscriptions().filter((subscription) => isDeliverable(subscription))
  }

  listEnabledSubscriberUsers() {
    const users = new Map()
    for (const subscription of this.listDeliverableSubscriptions()) {
      const user = normalizeUser(subscription.user)
      if (!user || users.has(user.id)) continue
      users.set(user.id, { id: user.id, email: user.email })
    }
    return [...users.values()]
  }

  listSubscriptionSummaries() {
    return this.state.subscriptions.map((subscription) => subscriptionSummary(subscription))
  }

  getSubscriptionStatus(clientId) {
    const normalized = normalizeClientID(clientId)
    if (!normalized) return { found: false, enabled: false, bound: false }
    const index = findSubscriptionIndex(this.state.subscriptions, { clientId: normalized, endpoint: '' })
    if (index < 0) return { found: false, enabled: false, bound: false }
    const subscription = this.state.subscriptions[index]
    return {
      found: true,
      enabled: isEnabled(subscription),
      bound: Boolean(normalizeUser(subscription.user)),
    }
  }

  findNotify(dedupeKey) {
    const key = String(dedupeKey || '').trim()
    if (!key) return null
    const found = this.state.notifies.find((item) => item.dedupeKey === key)
    return found ? { ...found, deduped: true } : null
  }

  listMessages(limit = 20) {
    return this.state.messages.slice(0, limit).map((message) => ({ ...message }))
  }

  listSchedules() {
    return this.state.schedules.map((schedule) => ({ ...schedule }))
  }

  overview() {
    return {
      activeSubscriptions: this.state.subscriptions.filter((subscription) => isDeliverable(subscription)).length,
      messageCount: this.state.messages.length,
      lastSentAt: this.state.messages[0]?.createdAt || null,
    }
  }

  async upsertSubscription(subscription) {
    return this.mutate(() => {
      const now = new Date().toISOString()
      const clientId = normalizeClientID(subscription.clientId)
      const endpoint = String(subscription.endpoint || '')
      let index = findSubscriptionIndex(this.state.subscriptions, { clientId, endpoint })
      const existing = index >= 0 ? this.state.subscriptions[index] : null
      const resolvedClientId = clientId || normalizeClientID(existing?.clientId)
      const id = resolvedClientId ? clientSubscriptionID(resolvedClientId) : subscriptionID(endpoint)
      let user = normalizeUser(existing?.user)
      if (!user) {
        const donor = this.state.subscriptions.find((item, itemIndex) => itemIndex !== index && (
          (resolvedClientId && (item.clientId === resolvedClientId || item.id === id))
          || (endpoint && item.endpoint === endpoint)
        ))
        user = normalizeUser(donor?.user)
      }
      if (!user) user = normalizeUser(subscription.user)
      const next = {
        id,
        clientId: resolvedClientId,
        endpoint,
        expirationTime: subscription.expirationTime ?? null,
        keys: { p256dh: subscription.keys.p256dh, auth: subscription.keys.auth },
        user,
        enabled: nextEnabled(existing, subscription.enabled),
        createdAt: existing?.createdAt || now,
        lastSeenAt: now,
      }
      if (index >= 0) this.state.subscriptions[index] = next
      else {
        this.state.subscriptions.push(next)
        index = this.state.subscriptions.length - 1
      }
      this.state.subscriptions = this.state.subscriptions.filter((item, itemIndex) => {
        if (itemIndex === index) return true
        if (item.id === id) return false
        if (resolvedClientId && item.clientId === resolvedClientId) return false
        if (endpoint && item.endpoint === endpoint) return false
        return true
      })
      return {
        id,
        createdAt: next.createdAt,
        lastSeenAt: next.lastSeenAt,
        enabled: next.enabled !== false,
        bound: Boolean(user),
      }
    })
  }

  async disableSubscription({ endpoint = '', clientId = '' } = {}) {
    return this.mutate(() => {
      const normalizedClientId = normalizeClientID(clientId)
      const index = findSubscriptionIndex(this.state.subscriptions, {
        clientId: normalizedClientId,
        endpoint: String(endpoint || ''),
      })
      if (index < 0) return false
      this.state.subscriptions[index].enabled = false
      this.state.subscriptions[index].lastSeenAt = new Date().toISOString()
      return true
    })
  }

  async clearExpiredSubscriptions(ids) {
    const idSet = new Set(ids)
    if (idSet.size === 0) return 0
    return this.mutate(() => {
      let cleared = 0
      for (const subscription of this.state.subscriptions) {
        if (!idSet.has(subscription.id)) continue
        if (subscription.user || subscription.clientId) {
          subscription.endpoint = ''
          subscription.keys = null
          subscription.expirationTime = null
          cleared += 1
        } else {
          subscription.dropExpired = true
          cleared += 1
        }
      }
      this.state.subscriptions = this.state.subscriptions.filter((subscription) => !subscription.dropExpired)
      return cleared
    })
  }

  async saveNotifyReceipt(receipt) {
    return this.mutate(() => {
      const key = String(receipt.dedupeKey || '').trim()
      if (key) {
        const existing = this.state.notifies.find((item) => item.dedupeKey === key)
        if (existing) return { ...existing, deduped: true }
      }
      const record = {
        dedupeKey: key,
        notified: receipt.notified || 0,
        delivered: receipt.delivered || 0,
        failed: receipt.failed || 0,
        sentAt: new Date().toISOString(),
      }
      if (key) {
        this.state.notifies.unshift(record)
        if (this.state.notifies.length > 5000) this.state.notifies.length = 5000
      }
      return { ...record, deduped: false }
    })
  }

  async removeSubscriptionsByID(ids) {
    const idSet = new Set(ids)
    if (idSet.size === 0) return 0
    return this.mutate(() => {
      const previousLength = this.state.subscriptions.length
      this.state.subscriptions = this.state.subscriptions.filter((item) => !idSet.has(item.id))
      return previousLength - this.state.subscriptions.length
    })
  }

  listPageNotices(since, until) {
    const start = Date.parse(since)
    const end = Date.parse(until)
    if (!Array.isArray(this.state.pageNotices) || Number.isNaN(start)) return []
    return this.state.pageNotices
      .filter((item) => {
        const time = Date.parse(item.createdAt)
        return time > start && (Number.isNaN(end) || time <= end)
      })
      .sort((left, right) => Date.parse(left.createdAt) - Date.parse(right.createdAt))
      .map((item) => ({ ...item }))
  }

  async publishPageNotice(notice) {
    return this.mutate(() => {
      if (!Array.isArray(this.state.pageNotices)) this.state.pageNotices = []
      const record = {
        id: notice.id,
        title: notice.title,
        body: notice.body,
        url: notice.url || '/',
        image: notice.image || '',
        createdAt: new Date().toISOString(),
      }
      this.state.pageNotices.unshift(record)
      this.state.pageNotices = this.state.pageNotices.slice(0, 50)
      return { ...record }
    })
  }

  async addMessage(message) {
    return this.mutate(() => {
      const record = {
        id: message.id || crypto.randomUUID(),
        title: message.title,
        body: message.body,
        url: message.url,
        image: message.image || '',
        channels: Array.isArray(message.channels) && message.channels.length ? message.channels : ['browser'],
        createdAt: new Date().toISOString(),
        delivered: message.delivered || 0,
        failed: message.failed || 0,
        removed: message.removed || 0,
      }
      this.state.messages.unshift(record)
      this.state.messages = this.state.messages.slice(0, 100)
      return { ...record }
    })
  }

  async removeMessage(id) {
    return this.mutate(() => {
      const previousLength = this.state.messages.length
      this.state.messages = this.state.messages.filter((message) => message.id !== id)
      return previousLength !== this.state.messages.length
    })
  }

  async clearMessages() {
    return this.mutate(() => {
      const removed = this.state.messages.length
      this.state.messages = []
      return removed
    })
  }

  async addSchedule(schedule) {
    return this.mutate(() => {
      const record = {
        id: crypto.randomUUID(),
        title: schedule.title,
        body: schedule.body,
        url: schedule.url,
        image: schedule.image || '',
        channels: Array.isArray(schedule.channels) && schedule.channels.length ? schedule.channels : ['browser'],
        channelsSpecified: schedule.channelsSpecified === true,
        scheduledAt: new Date(schedule.scheduledAt).toISOString(),
        status: 'pending',
        createdAt: new Date().toISOString(),
      }
      this.state.schedules.push(record)
      this.state.schedules.sort((left, right) => new Date(left.scheduledAt).getTime() - new Date(right.scheduledAt).getTime())
      return { ...record }
    })
  }

  async claimDueSchedules(now = new Date()) {
    return this.mutate(() => {
      const timestamp = now.getTime()
      const due = []
      for (const schedule of this.state.schedules) {
        if (schedule.status !== 'pending' || new Date(schedule.scheduledAt).getTime() > timestamp) continue
        schedule.status = 'processing'
        due.push({ ...schedule })
      }
      return due
    })
  }

  async completeSchedule(id) {
    return this.mutate(() => {
      const previousLength = this.state.schedules.length
      this.state.schedules = this.state.schedules.filter((schedule) => schedule.id !== id)
      return previousLength !== this.state.schedules.length
    })
  }

  async failSchedule(id, error) {
    return this.mutate(() => {
      const schedule = this.state.schedules.find((item) => item.id === id)
      if (!schedule) return false
      schedule.status = 'failed'
      schedule.error = String(error || 'delivery failed').slice(0, 500)
      schedule.failedAt = new Date().toISOString()
      return true
    })
  }

  async removeSchedule(id) {
    return this.mutate(() => {
      const previousLength = this.state.schedules.length
      this.state.schedules = this.state.schedules.filter((schedule) => schedule.id !== id)
      return previousLength !== this.state.schedules.length
    })
  }

  async mutate(operation) {
    const task = this.writeQueue.then(async () => {
      const result = operation()
      await this.persist()
      return result
    })
    this.writeQueue = task.catch(() => {})
    return task
  }

  async persist() {
    await mkdir(path.dirname(this.filePath), { recursive: true })
    const temporaryPath = `${this.filePath}.${process.pid}.tmp`
    await writeFile(temporaryPath, `${JSON.stringify(this.state, null, 2)}\n`, { mode: 0o600 })
    await rename(temporaryPath, this.filePath)
  }
}
