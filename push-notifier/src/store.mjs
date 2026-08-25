import crypto from 'node:crypto'
import { mkdir, readFile, rename, writeFile } from 'node:fs/promises'
import path from 'node:path'
import webpush from 'web-push'

const EMPTY_STATE = Object.freeze({
  version: 1,
  vapid: null,
  subscriptions: [],
  messages: [],
})

function cloneEmptyState() {
  return JSON.parse(JSON.stringify(EMPTY_STATE))
}

function subscriptionID(endpoint) {
  return crypto.createHash('sha256').update(endpoint).digest('hex')
}

function normalizeMonitorIDs(value) {
  if (value === null || value === undefined) return null
  if (!Array.isArray(value)) return null
  return [...new Set(value.map((id) => Number(id)).filter((id) => Number.isSafeInteger(id) && id > 0))]
}

function normalizeMuteUntil(value) {
  if (!value) return null
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? null : parsed.toISOString()
}

function normalizeMuteTime(value) {
  const text = String(value || '').trim()
  return /^([01]\d|2[0-3]):[0-5]\d$/.test(text) ? text : null
}

function normalizeMuteDate(value) {
  if (!value) return null
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? null : parsed.toISOString()
}

function normalizeMuteTimezone(value) {
  const timezone = String(value || '').trim()
  if (!timezone) return null
  try {
    new Intl.DateTimeFormat('en-US', { timeZone: timezone }).format()
    return timezone
  } catch {
    return null
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
    return this.state.subscriptions.map((subscription) => ({ ...subscription, keys: { ...subscription.keys } }))
  }

  listMessages(limit = 20) {
    return this.state.messages.slice(0, limit).map((message) => ({ ...message }))
  }

  overview() {
    return {
      activeSubscriptions: this.state.subscriptions.length,
      messageCount: this.state.messages.length,
      lastSentAt: this.state.messages[0]?.createdAt || null,
    }
  }

  async upsertSubscription(subscription) {
    return this.mutate(() => {
      const now = new Date().toISOString()
      const id = subscriptionID(subscription.endpoint)
      const index = this.state.subscriptions.findIndex((item) => item.id === id)
      const existing = index >= 0 ? this.state.subscriptions[index] : subscription
      const muteDaily = Boolean(existing.muteDaily)
      const next = {
        id,
        endpoint: subscription.endpoint,
        expirationTime: subscription.expirationTime ?? null,
        keys: { p256dh: subscription.keys.p256dh, auth: subscription.keys.auth },
        monitorIDs: normalizeMonitorIDs(index >= 0 ? this.state.subscriptions[index].monitorIDs : subscription.monitorIDs),
        muteUntil: normalizeMuteUntil(index >= 0 ? this.state.subscriptions[index].muteUntil : subscription.muteUntil),
        muteStart: muteDaily ? normalizeMuteTime(existing.muteStart) : normalizeMuteDate(existing.muteStart),
        muteEnd: muteDaily ? normalizeMuteTime(existing.muteEnd) : normalizeMuteDate(existing.muteEnd),
        muteDaily,
        muteTimezone: normalizeMuteTimezone(existing.muteTimezone),
        createdAt: index >= 0 ? this.state.subscriptions[index].createdAt : now,
        lastSeenAt: now,
      }
      if (index >= 0) this.state.subscriptions[index] = next
      else this.state.subscriptions.push(next)
      return { id, createdAt: next.createdAt, lastSeenAt: next.lastSeenAt }
    })
  }

  getSubscriptionPreferences(endpoint) {
    const id = subscriptionID(endpoint)
    const subscription = this.state.subscriptions.find((item) => item.id === id)
    if (!subscription) return null
    return {
      monitorIDs: normalizeMonitorIDs(subscription.monitorIDs),
      muteUntil: normalizeMuteUntil(subscription.muteUntil),
      muteStart: subscription.muteStart || null,
      muteEnd: subscription.muteEnd || null,
      muteDaily: Boolean(subscription.muteDaily),
      muteTimezone: normalizeMuteTimezone(subscription.muteTimezone),
    }
  }

  async updateSubscriptionPreferences(endpoint, preferences) {
    return this.mutate(() => {
      const id = subscriptionID(endpoint)
      const subscription = this.state.subscriptions.find((item) => item.id === id)
      if (!subscription) return null
      subscription.monitorIDs = normalizeMonitorIDs(preferences.monitorIDs)
      subscription.muteUntil = normalizeMuteUntil(preferences.muteUntil)
      subscription.muteStart = preferences.muteDaily ? normalizeMuteTime(preferences.muteStart) : normalizeMuteDate(preferences.muteStart)
      subscription.muteEnd = preferences.muteDaily ? normalizeMuteTime(preferences.muteEnd) : normalizeMuteDate(preferences.muteEnd)
      subscription.muteDaily = Boolean(preferences.muteDaily)
      subscription.muteTimezone = normalizeMuteTimezone(preferences.muteTimezone)
      subscription.updatedAt = new Date().toISOString()
      return {
        monitorIDs: subscription.monitorIDs,
        muteUntil: subscription.muteUntil,
        muteStart: subscription.muteStart,
        muteEnd: subscription.muteEnd,
        muteDaily: subscription.muteDaily,
        muteTimezone: subscription.muteTimezone,
      }
    })
  }

  async removeSubscription(endpoint) {
    return this.mutate(() => {
      const id = subscriptionID(endpoint)
      const previousLength = this.state.subscriptions.length
      this.state.subscriptions = this.state.subscriptions.filter((item) => item.id !== id)
      return previousLength !== this.state.subscriptions.length
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

  async addMessage(message) {
    return this.mutate(() => {
      const record = {
        id: crypto.randomUUID(),
        title: message.title,
        body: message.body,
        url: message.url,
        image: message.image || '',
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
