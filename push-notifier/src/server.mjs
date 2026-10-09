import crypto from 'node:crypto'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import http from 'node:http'
import path from 'node:path'
import webpush from 'web-push'
import { normalizeClientID } from './store.mjs'

const API_PREFIX = '/push-api/v1'
const MAX_BODY_BYTES = 32 * 1024
const MAX_IMAGE_BYTES = 2 * 1024 * 1024
const IMAGE_NAME = /^[a-f0-9]{32}\.(jpg|png|gif|webp)$/
const IMAGE_SIGNATURES = {
  'image/jpeg': {
    ext: 'jpg',
    match: (body) => body.length >= 3 && body[0] === 0xff && body[1] === 0xd8 && body[2] === 0xff,
  },
  'image/png': {
    ext: 'png',
    match: (body) => body.length >= 8 && body[0] === 0x89 && body[1] === 0x50 && body[2] === 0x4e && body[3] === 0x47,
  },
  'image/gif': {
    ext: 'gif',
    match: (body) => body.length >= 6 && body.toString('ascii', 0, 4) === 'GIF8',
  },
  'image/webp': {
    ext: 'webp',
    match: (body) => body.length >= 12 && body.toString('ascii', 0, 4) === 'RIFF' && body.toString('ascii', 8, 12) === 'WEBP',
  },
}

function sendJSON(response, status, data) {
  response.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Cache-Control': 'no-store',
    'X-Content-Type-Options': 'nosniff',
  })
  response.end(JSON.stringify(data))
}

async function readRaw(request, maxBytes = MAX_BODY_BYTES) {
  const chunks = []
  let size = 0
  for await (const chunk of request) {
    size += chunk.length
    if (size > maxBytes) throw Object.assign(new Error('request body too large'), { status: 413 })
    chunks.push(chunk)
  }
  return Buffer.concat(chunks)
}

async function readJSON(request, maxBytes = MAX_BODY_BYTES) {
  const body = await readRaw(request, maxBytes)
  if (body.length === 0) return {}
  try {
    return JSON.parse(body.toString('utf8'))
  } catch {
    throw Object.assign(new Error('invalid JSON body'), { status: 400 })
  }
}

function imageDirectory(store) {
  return path.join(path.dirname(store.filePath), 'images')
}

function imageContentType(filename) {
  const ext = path.extname(filename)
  if (ext === '.jpg') return 'image/jpeg'
  if (ext === '.png') return 'image/png'
  if (ext === '.gif') return 'image/gif'
  if (ext === '.webp') return 'image/webp'
  return ''
}

async function savePushImage(store, contentType, body) {
  const type = String(contentType || '').split(';')[0].trim().toLowerCase()
  const signature = IMAGE_SIGNATURES[type]
  if (!signature || !signature.match(body)) {
    throw Object.assign(new Error('image must be jpeg, png, gif, or webp'), { status: 415 })
  }
  const filename = `${crypto.randomBytes(16).toString('hex')}.${signature.ext}`
  const directory = imageDirectory(store)
  await mkdir(directory, { recursive: true })
  await writeFile(path.join(directory, filename), body)
  return { url: `${API_PREFIX}/images/${filename}` }
}

function sendFile(response, body, contentType) {
  response.writeHead(200, {
    'Content-Type': contentType,
    'Cache-Control': 'public, max-age=31536000, immutable',
    'X-Content-Type-Options': 'nosniff',
    'Content-Length': body.length,
  })
  response.end(body)
}

function validateSubscription(subscription) {
  if (!subscription || typeof subscription !== 'object') return 'subscription is required'
  if (subscription.clientId != null && subscription.clientId !== '' && !normalizeClientID(subscription.clientId)) {
    return 'clientId must contain 8 to 128 letters, numbers, dashes, or underscores'
  }
  if (subscription.enabled != null && typeof subscription.enabled !== 'boolean') return 'enabled must be a boolean'
  try {
    const endpoint = new URL(subscription.endpoint)
    if (endpoint.protocol !== 'https:') return 'subscription endpoint must use HTTPS'
  } catch {
    return 'invalid subscription endpoint'
  }
  if (!subscription.keys?.p256dh || !subscription.keys?.auth) return 'subscription keys are required'
  if (subscription.endpoint.length > 4096) return 'subscription endpoint is too long'
  return null
}

function validateBroadcast(payload) {
  const title = String(payload.title || '').trim()
  const body = String(payload.body || '').trim()
  const url = String(payload.url || '/').trim()
  const image = String(payload.image || '').trim()
  if (!title || title.length > 100) return { error: 'title must contain 1 to 100 characters' }
  if (!body || body.length > 500) return { error: 'body must contain 1 to 500 characters' }
  if (url.length > 2048) return { error: 'url is too long' }
  if (image.length > 2048) return { error: 'image url is too long' }
  if (!(url.startsWith('/') && !url.startsWith('//'))) {
    try {
      const parsed = new URL(url)
      if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') return { error: 'url must use HTTP or HTTPS' }
    } catch {
      return { error: 'invalid url' }
    }
  }
  if (image) {
    try {
      const parsed = new URL(image)
      if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') {
        return { error: 'image url must use HTTP or HTTPS' }
      }
    } catch {
      return { error: 'invalid image url' }
    }
  }
  const channels = normalizeChannels(payload.channels)
  if (channels.error) return channels
  return { title, body, url, image, channels: channels.channels, channelsSpecified: payload.channels != null }
}

function normalizeChannels(value) {
  if (value == null) return { channels: ['browser'] }
  if (!Array.isArray(value)) return { error: 'channels must be an array' }
  const channels = []
  for (const item of value) {
    if (item !== 'browser' && item !== 'page') return { error: 'channels must be browser or page' }
    if (!channels.includes(item)) channels.push(item)
  }
  if (channels.length === 0) return { error: 'select at least one channel' }
  return { channels }
}

function validateSchedule(payload) {
  const broadcast = validateBroadcast(payload)
  if (broadcast.error) return broadcast
  const scheduledAt = new Date(String(payload.scheduledAt || ''))
  if (Number.isNaN(scheduledAt.getTime())) return { error: 'scheduledAt must be a valid date' }
  if (scheduledAt.getTime() <= Date.now()) return { error: 'scheduledAt must be in the future' }
  return { ...broadcast, scheduledAt: scheduledAt.toISOString() }
}

async function defaultSender(subscription, payload, options) {
  return webpush.sendNotification(subscription, payload, options)
}

async function sendBroadcast({ subscriptions, payload, options, sender }) {
  const result = { delivered: 0, failed: 0, expiredIDs: [] }
  for (let index = 0; index < subscriptions.length; index += 10) {
    const batch = subscriptions.slice(index, index + 10)
    const outcomes = await Promise.allSettled(
      batch.map((subscription) => sender(subscription, payload, options)),
    )
    outcomes.forEach((outcome, offset) => {
      const subscription = batch[offset]
      if (outcome.status === 'fulfilled') {
        const statusCode = Number(outcome.value?.statusCode || 201)
        outcome.value?.resume?.()
        if (statusCode >= 200 && statusCode < 300) result.delivered += 1
        else if (statusCode === 404 || statusCode === 410) result.expiredIDs.push(subscription.id)
        else result.failed += 1
        return
      }
      const statusCode = Number(outcome.reason?.statusCode || 0)
      if (statusCode === 404 || statusCode === 410) result.expiredIDs.push(subscription.id)
      else result.failed += 1
    })
  }
  return result
}

function countNotified(subscriptions) {
  const userIDs = new Set()
  let anonymous = 0
  for (const subscription of subscriptions) {
    const id = Number(subscription.user?.id)
    if (Number.isSafeInteger(id) && id > 0) userIDs.add(id)
    else anonymous += 1
  }
  return userIDs.size + anonymous
}

async function deliverToSubscriptions({ store, subscriptions, validation, vapidSubject, sender, tag, notificationID }) {
  const vapid = store.getVapidKeys()
  const notification = {
    id: notificationID || crypto.randomUUID(),
    title: validation.title,
    body: validation.body,
    url: validation.url,
    image: validation.image || '',
    tag,
  }
  if (validation.channelsSpecified && Array.isArray(validation.channels)) {
    notification.channels = validation.channels
  }
  const encoded = JSON.stringify(notification)
  const sent = await sendBroadcast({
    subscriptions,
    payload: encoded,
    options: {
      TTL: 24 * 60 * 60,
      vapidDetails: {
        subject: vapidSubject,
        publicKey: vapid.publicKey,
        privateKey: vapid.privateKey,
      },
    },
    sender,
  })
  const removed = await store.clearExpiredSubscriptions(sent.expiredIDs)
  return { delivered: sent.delivered, failed: sent.failed, removed }
}

export async function deliverBroadcast({ store, validation, vapidSubject, sender = defaultSender }) {
  const channels = validation.channels?.length ? validation.channels : ['browser']
  const messageID = crypto.randomUUID()
  let sent = { delivered: 0, failed: 0, removed: 0 }
  if (channels.includes('browser')) {
    sent = await deliverToSubscriptions({
      store,
      subscriptions: store.listDeliverableSubscriptions(),
      validation: validation.channelsSpecified
        ? { ...validation, channels: ['browser'], channelsSpecified: true }
        : validation,
      vapidSubject,
      sender,
      tag: `sub2api-broadcast-${messageID}`,
      notificationID: messageID,
    })
  }
  if (validation.channelsSpecified && channels.includes('page')) {
    await store.publishPageNotice({
      id: messageID,
      title: validation.title,
      body: validation.body,
      url: validation.url,
      image: validation.image || '',
    })
  }
  return store.addMessage({
    id: messageID,
    title: validation.title,
    body: validation.body,
    url: validation.url,
    image: validation.image || '',
    channels,
    delivered: sent.delivered,
    failed: sent.failed,
    removed: sent.removed,
  })
}

function tokenMatches(expected, provided) {
  const left = Buffer.from(String(expected || ''))
  const right = Buffer.from(String(provided || ''))
  if (left.length === 0 || left.length !== right.length) return false
  return crypto.timingSafeEqual(left, right)
}

function normalizeUserIDs(value) {
  if (value == null) return { ids: null }
  if (!Array.isArray(value)) return { error: 'userIDs must be an array' }
  const ids = []
  for (const item of value) {
    const id = Number(item)
    if (!Number.isSafeInteger(id) || id <= 0) return { error: 'userIDs must contain positive integers' }
    ids.push(id)
  }
  return { ids }
}

export async function deliverTargeted({ store, validation, userIDs, vapidSubject, sender = defaultSender }) {
  const wanted = userIDs == null ? null : new Set(userIDs)
  const subscriptions = store.listDeliverableSubscriptions().filter((subscription) => {
    if (!wanted) return true
    return wanted.has(Number(subscription.user?.id))
  })
  const dedupeKey = String(validation.dedupeKey || '').trim()
  if (dedupeKey) {
    const existing = store.findNotify(dedupeKey)
    if (existing) return existing
  }
  const sent = await deliverToSubscriptions({
    store,
    subscriptions,
    validation,
    vapidSubject,
    sender,
    tag: dedupeKey ? `sub2api-notify-${dedupeKey}` : `sub2api-notify-${crypto.randomUUID()}`,
  })
  return store.saveNotifyReceipt({
    dedupeKey,
    notified: countNotified(subscriptions),
    delivered: sent.delivered,
    failed: sent.failed,
  })
}

export function createServer({
  store,
  authorizeAdmin,
  resolveSubscriberUser = async () => null,
  vapidSubject,
  notifyToken = '',
  sender = defaultSender,
  logger = console,
}) {
  return http.createServer(async (request, response) => {
    const requestURL = new URL(request.url || '/', 'http://localhost')
    try {
      if (request.method === 'GET' && requestURL.pathname === '/health') {
        return sendJSON(response, 200, { status: 'ok' })
      }
      if (request.method === 'GET' && requestURL.pathname === `${API_PREFIX}/config`) {
        return sendJSON(response, 200, { publicKey: store.getVapidKeys().publicKey })
      }
      const imagePrefix = `${API_PREFIX}/images/`
      if (request.method === 'GET' && requestURL.pathname.startsWith(imagePrefix)) {
        const filename = requestURL.pathname.slice(imagePrefix.length)
        const contentType = imageContentType(filename)
        if (!IMAGE_NAME.test(filename) || !contentType) return sendJSON(response, 404, { message: 'not found' })
        try {
          const body = await readFile(path.join(imageDirectory(store), filename))
          return sendFile(response, body, contentType)
        } catch (error) {
          if (error?.code === 'ENOENT') return sendJSON(response, 404, { message: 'not found' })
          throw error
        }
      }
      if (request.method === 'GET' && requestURL.pathname === `${API_PREFIX}/page-notifications`) {
        const user = await resolveSubscriberUser(request)
        if (!user) return sendJSON(response, 401, { message: 'sign in required' })
        const now = new Date().toISOString()
        const since = requestURL.searchParams.get('since') || ''
        return sendJSON(response, 200, { notices: store.listPageNotices(since, now), now })
      }
      if (request.method === 'GET' && requestURL.pathname === `${API_PREFIX}/subscriptions/status`) {
        return sendJSON(response, 200, store.getSubscriptionStatus(requestURL.searchParams.get('clientId')))
      }
      if (request.method === 'POST' && requestURL.pathname === `${API_PREFIX}/subscriptions`) {
        const payload = await readJSON(request)
        const validationError = validateSubscription(payload)
        if (validationError) return sendJSON(response, 400, { message: validationError })
        // The first successful account lookup binds this client id. Later
        // toggles keep that account, including its email, unchanged.
        const resolved = await resolveSubscriberUser(request)
        const user = resolved || payload.user || null
        const subscription = await store.upsertSubscription({ ...payload, user })
        return sendJSON(response, 201, { subscription })
      }
      if (request.method === 'DELETE' && requestURL.pathname === `${API_PREFIX}/subscriptions`) {
        const payload = await readJSON(request)
        const clientId = normalizeClientID(payload.clientId)
        if (!payload.endpoint && !clientId) return sendJSON(response, 400, { message: 'endpoint or clientId is required' })
        const disabled = await store.disableSubscription({
          endpoint: payload.endpoint ? String(payload.endpoint) : '',
          clientId,
        })
        return sendJSON(response, 200, { removed: false, disabled })
      }
      if (request.method === 'GET' && requestURL.pathname === `${API_PREFIX}/internal/subscribers`) {
        const token = request.headers['x-push-token']
        const allowed = tokenMatches(notifyToken, token) || await authorizeAdmin(request)
        if (!allowed) return sendJSON(response, 403, { message: 'notify access required' })
        return sendJSON(response, 200, { users: store.listEnabledSubscriberUsers() })
      }
      if (request.method === 'POST' && requestURL.pathname === `${API_PREFIX}/internal/notify`) {
        const token = request.headers['x-push-token']
        const allowed = tokenMatches(notifyToken, token) || await authorizeAdmin(request)
        if (!allowed) return sendJSON(response, 403, { message: 'notify access required' })
        const payload = await readJSON(request, 1024 * 1024)
        const validation = validateBroadcast(payload)
        if (validation.error) return sendJSON(response, 400, { message: validation.error })
        const dedupeKey = String(payload.dedupeKey || '').trim()
        if (dedupeKey.length > 200) return sendJSON(response, 400, { message: 'dedupeKey is too long' })
        const userIDs = normalizeUserIDs(payload.userIDs)
        if (userIDs.error) return sendJSON(response, 400, { message: userIDs.error })
        const result = await deliverTargeted({
          store,
          validation: { ...validation, dedupeKey },
          userIDs: userIDs.ids,
          vapidSubject,
          sender,
        })
        return sendJSON(response, 200, {
          notified: result.notified || 0,
          delivered: result.delivered || 0,
          failed: result.failed || 0,
          deduped: Boolean(result.deduped),
        })
      }

      if (requestURL.pathname.startsWith(`${API_PREFIX}/admin/`)) {
        if (!(await authorizeAdmin(request))) return sendJSON(response, 403, { message: 'administrator access required' })

        if (request.method === 'GET' && requestURL.pathname === `${API_PREFIX}/admin/overview`) {
          return sendJSON(response, 200, store.overview())
        }
        if (request.method === 'GET' && requestURL.pathname === `${API_PREFIX}/admin/subscriptions`) {
          return sendJSON(response, 200, { subscriptions: store.listSubscriptionSummaries() })
        }
        if (request.method === 'GET' && requestURL.pathname === `${API_PREFIX}/admin/schedules`) {
          return sendJSON(response, 200, { schedules: store.listSchedules() })
        }
        if (request.method === 'GET' && requestURL.pathname === `${API_PREFIX}/admin/messages`) {
          const limit = Math.min(Math.max(Number(requestURL.searchParams.get('limit')) || 20, 1), 100)
          return sendJSON(response, 200, { messages: store.listMessages(limit) })
        }
        const messagePathPrefix = `${API_PREFIX}/admin/messages/`
        if (request.method === 'DELETE' && requestURL.pathname === `${API_PREFIX}/admin/messages`) {
          const removed = await store.clearMessages()
          return sendJSON(response, 200, { removed, overview: store.overview() })
        }
        if (request.method === 'DELETE' && requestURL.pathname.startsWith(messagePathPrefix)) {
          const messageID = requestURL.pathname.slice(messagePathPrefix.length)
          if (!messageID || messageID.includes('/')) {
            return sendJSON(response, 400, { message: 'invalid message id' })
          }
          const removed = await store.removeMessage(messageID)
          if (!removed) return sendJSON(response, 404, { message: 'message not found' })
          return sendJSON(response, 200, { removed, overview: store.overview() })
        }
        if (request.method === 'POST' && requestURL.pathname === `${API_PREFIX}/admin/images`) {
          const saved = await savePushImage(store, request.headers['content-type'], await readRaw(request, MAX_IMAGE_BYTES))
          return sendJSON(response, 201, saved)
        }
        if (request.method === 'POST' && requestURL.pathname === `${API_PREFIX}/admin/broadcast`) {
          const validation = validateBroadcast(await readJSON(request))
          if (validation.error) return sendJSON(response, 400, { message: validation.error })
          const message = await deliverBroadcast({ store, validation, vapidSubject, sender })
          return sendJSON(response, 201, { message })
        }
        if (request.method === 'POST' && requestURL.pathname === `${API_PREFIX}/admin/schedules`) {
          const validation = validateSchedule(await readJSON(request))
          if (validation.error) return sendJSON(response, 400, { message: validation.error })
          const schedule = await store.addSchedule(validation)
          return sendJSON(response, 201, { schedule })
        }
        const schedulePathPrefix = `${API_PREFIX}/admin/schedules/`
        if (request.method === 'DELETE' && requestURL.pathname.startsWith(schedulePathPrefix)) {
          const scheduleID = requestURL.pathname.slice(schedulePathPrefix.length)
          if (!scheduleID || scheduleID.includes('/')) return sendJSON(response, 400, { message: 'invalid schedule id' })
          const removed = await store.removeSchedule(scheduleID)
          if (!removed) return sendJSON(response, 404, { message: 'schedule not found' })
          return sendJSON(response, 200, { removed })
        }
      }

      return sendJSON(response, 404, { message: 'not found' })
    } catch (error) {
      logger.error?.('push notifier request failed', error)
      return sendJSON(response, Number(error?.status || 500), {
        message: Number(error?.status || 500) >= 500 ? 'internal server error' : error.message,
      })
    }
  })
}
