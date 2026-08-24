import crypto from 'node:crypto'
import http from 'node:http'
import webpush from 'web-push'

const API_PREFIX = '/push-api/v1'
const MAX_BODY_BYTES = 32 * 1024

function sendJSON(response, status, data) {
  response.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Cache-Control': 'no-store',
    'X-Content-Type-Options': 'nosniff',
  })
  response.end(JSON.stringify(data))
}

async function readJSON(request) {
  const chunks = []
  let size = 0
  for await (const chunk of request) {
    size += chunk.length
    if (size > MAX_BODY_BYTES) throw Object.assign(new Error('request body too large'), { status: 413 })
    chunks.push(chunk)
  }
  if (chunks.length === 0) return {}
  try {
    return JSON.parse(Buffer.concat(chunks).toString('utf8'))
  } catch {
    throw Object.assign(new Error('invalid JSON body'), { status: 400 })
  }
}

function validateSubscription(subscription) {
  if (!subscription || typeof subscription !== 'object') return 'subscription is required'
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
  return { title, body, url, image }
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

export function createServer({ store, authorizeAdmin, vapidSubject, sender = defaultSender, logger = console }) {
  return http.createServer(async (request, response) => {
    const requestURL = new URL(request.url || '/', 'http://localhost')
    try {
      if (request.method === 'GET' && requestURL.pathname === '/health') {
        return sendJSON(response, 200, { status: 'ok' })
      }
      if (request.method === 'GET' && requestURL.pathname === `${API_PREFIX}/config`) {
        return sendJSON(response, 200, { publicKey: store.getVapidKeys().publicKey })
      }
      if (request.method === 'POST' && requestURL.pathname === `${API_PREFIX}/subscriptions`) {
        const payload = await readJSON(request)
        const validationError = validateSubscription(payload)
        if (validationError) return sendJSON(response, 400, { message: validationError })
        const subscription = await store.upsertSubscription(payload)
        return sendJSON(response, 201, { subscription })
      }
      if (request.method === 'DELETE' && requestURL.pathname === `${API_PREFIX}/subscriptions`) {
        const payload = await readJSON(request)
        if (!payload.endpoint) return sendJSON(response, 400, { message: 'endpoint is required' })
        const removed = await store.removeSubscription(String(payload.endpoint))
        return sendJSON(response, 200, { removed })
      }

      if (requestURL.pathname.startsWith(`${API_PREFIX}/admin/`)) {
        if (!(await authorizeAdmin(request))) return sendJSON(response, 403, { message: 'administrator access required' })

        if (request.method === 'GET' && requestURL.pathname === `${API_PREFIX}/admin/overview`) {
          return sendJSON(response, 200, store.overview())
        }
        if (request.method === 'GET' && requestURL.pathname === `${API_PREFIX}/admin/messages`) {
          const limit = Math.min(Math.max(Number(requestURL.searchParams.get('limit')) || 20, 1), 100)
          return sendJSON(response, 200, { messages: store.listMessages(limit) })
        }
        const messagePathPrefix = `${API_PREFIX}/admin/messages/`
        if (request.method === 'DELETE' && requestURL.pathname.startsWith(messagePathPrefix)) {
          const messageID = requestURL.pathname.slice(messagePathPrefix.length)
          if (!messageID || messageID.includes('/')) {
            return sendJSON(response, 400, { message: 'invalid message id' })
          }
          const removed = await store.removeMessage(messageID)
          if (!removed) return sendJSON(response, 404, { message: 'message not found' })
          return sendJSON(response, 200, { removed, overview: store.overview() })
        }
        if (request.method === 'POST' && requestURL.pathname === `${API_PREFIX}/admin/broadcast`) {
          const validation = validateBroadcast(await readJSON(request))
          if (validation.error) return sendJSON(response, 400, { message: validation.error })

          const subscriptions = store.listSubscriptions()
          const vapid = store.getVapidKeys()
          const messageID = crypto.randomUUID()
          const notification = JSON.stringify({
            id: messageID,
            title: validation.title,
            body: validation.body,
            url: validation.url,
            image: validation.image,
            tag: `sub2api-broadcast-${messageID}`,
          })
          const sent = await sendBroadcast({
            subscriptions,
            payload: notification,
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
          const removed = await store.removeSubscriptionsByID(sent.expiredIDs)
          const message = await store.addMessage({
            ...validation,
            delivered: sent.delivered,
            failed: sent.failed,
            removed,
          })
          return sendJSON(response, 201, { message })
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
