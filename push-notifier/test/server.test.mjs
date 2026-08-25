import assert from 'node:assert/strict'
import { mkdtemp, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { createServer } from '../src/server.mjs'
import { PushStore } from '../src/store.mjs'

async function withServer(run, sender = async () => ({ statusCode: 201, resume() {} })) {
  const directory = await mkdtemp(path.join(os.tmpdir(), 'push-notifier-'))
  const store = await new PushStore(path.join(directory, 'store.json')).init()
  const server = createServer({
    store,
    authorizeAdmin: async (request) => request.headers.authorization === 'Bearer admin',
    authorizeInternal: async (request) => request.headers.authorization === 'Bearer internal',
    vapidSubject: 'mailto:test@example.com',
    sender,
    logger: { error() {} },
  })
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
  const address = server.address()
  try {
    await run(`http://127.0.0.1:${address.port}`, store)
  } finally {
    await new Promise((resolve) => server.close(resolve))
    await rm(directory, { recursive: true, force: true })
  }
}

test('publishes VAPID configuration and stores subscriptions', async () => {
  await withServer(async (baseURL, store) => {
    const configResponse = await fetch(`${baseURL}/push-api/v1/config`)
    const config = await configResponse.json()
    assert.equal(configResponse.status, 200)
    assert.ok(config.publicKey)

    const subscribeResponse = await fetch(`${baseURL}/push-api/v1/subscriptions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        endpoint: 'https://push.example.test/subscription/1',
        keys: { p256dh: 'public-key', auth: 'auth-key' },
      }),
    })
    assert.equal(subscribeResponse.status, 201)
    assert.equal(store.overview().activeSubscriptions, 1)
  })
})

test('broadcast requires an administrator and records delivery totals', async () => {
  await withServer(async (baseURL) => {
    const forbidden = await fetch(`${baseURL}/push-api/v1/admin/broadcast`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: 'Test', body: 'Message', url: '/' }),
    })
    assert.equal(forbidden.status, 403)

    const response = await fetch(`${baseURL}/push-api/v1/admin/broadcast`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer admin' },
      body: JSON.stringify({
        title: 'Rate updated',
        body: 'The rate is now 0.08x',
        url: '/model-plaza',
        image: 'https://cdn.example.test/rate-update.jpg',
      }),
    })
    const payload = await response.json()
    assert.equal(response.status, 201)
    assert.equal(payload.message.title, 'Rate updated')
    assert.equal(payload.message.image, 'https://cdn.example.test/rate-update.jpg')
    assert.equal(payload.message.delivered, 0)

    const messages = await fetch(`${baseURL}/push-api/v1/admin/messages`, {
      headers: { Authorization: 'Bearer admin' },
    }).then((result) => result.json())
    assert.equal(messages.messages.length, 1)

    const messageID = messages.messages[0].id
    const forbiddenDelete = await fetch(`${baseURL}/push-api/v1/admin/messages/${messageID}`, {
      method: 'DELETE',
    })
    assert.equal(forbiddenDelete.status, 403)

    const deleteResponse = await fetch(`${baseURL}/push-api/v1/admin/messages/${messageID}`, {
      method: 'DELETE',
      headers: { Authorization: 'Bearer admin' },
    })
    const deleteResult = await deleteResponse.json()
    assert.equal(deleteResponse.status, 200)
    assert.equal(deleteResult.removed, true)
    assert.equal(deleteResult.overview.messageCount, 0)

    const missingDelete = await fetch(`${baseURL}/push-api/v1/admin/messages/${messageID}`, {
      method: 'DELETE',
      headers: { Authorization: 'Bearer admin' },
    })
    assert.equal(missingDelete.status, 404)
  })
})

test('channel-monitor delivery is not added to manual history and history can be cleared', async () => {
  await withServer(async (baseURL) => {
    const manualResponse = await fetch(`${baseURL}/push-api/v1/admin/broadcast`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer admin' },
      body: JSON.stringify({ title: 'Manual', body: 'Message', url: '/' }),
    })
    assert.equal(manualResponse.status, 201)

    const channelResponse = await fetch(`${baseURL}/push-api/v1/internal/channel-check-completed`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer internal' },
      body: JSON.stringify({
        groupName: 'CC-Kiro',
        recentStatuses: ['operational'],
        currentStatus: 'operational',
      }),
    })
    assert.equal(channelResponse.status, 201)

    const beforeClear = await fetch(`${baseURL}/push-api/v1/admin/messages`, {
      headers: { Authorization: 'Bearer admin' },
    }).then((result) => result.json())
    assert.equal(beforeClear.messages.length, 1)
    assert.equal(beforeClear.messages[0].title, 'Manual')

    const clearResponse = await fetch(`${baseURL}/push-api/v1/admin/messages`, {
      method: 'DELETE',
      headers: { Authorization: 'Bearer admin' },
    })
    const clearResult = await clearResponse.json()
    assert.equal(clearResponse.status, 200)
    assert.equal(clearResult.removed, 1)
    assert.equal(clearResult.overview.messageCount, 0)
  })
})

test('rejects image URLs that cannot be displayed by a browser notification', async () => {
  await withServer(async (baseURL) => {
    const response = await fetch(`${baseURL}/push-api/v1/admin/broadcast`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer admin' },
      body: JSON.stringify({ title: 'Test', body: 'Message', url: '/', image: 'file:///private/image.jpg' }),
    })
    assert.equal(response.status, 400)
    assert.match((await response.json()).message, /image url/)
  })
})

test('scheduled channel completion uses internal auth and records the status timeline', async () => {
  await withServer(async (baseURL) => {
    const endpoint = `${baseURL}/push-api/v1/internal/channel-check-completed`
    const forbidden = await fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        groupName: 'CC-Kiro',
        recentStatuses: ['operational'],
        currentStatus: 'operational',
      }),
    })
    assert.equal(forbidden.status, 403)

    const response = await fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer internal' },
      body: JSON.stringify({
        groupName: 'CC-Kiro',
        recentStatuses: ['operational', 'operational', 'degraded', 'operational', 'failed'],
        currentStatus: 'degraded',
      }),
    })
    const payload = await response.json()
    assert.equal(response.status, 201)
    assert.equal(payload.message.title, '渠道检测完成')
    assert.equal(payload.message.body, 'CC-Kiro 🟩🟩🟨🟩🟥 降级')
    assert.equal(payload.message.url, '/monitor')
  })
})

test('channel-monitor delivery respects per-device monitor selection and mute period', async () => {
  const sentEndpoints = []
  await withServer(async (baseURL) => {
    const subscribe = async (suffix) => fetch(`${baseURL}/push-api/v1/subscriptions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ endpoint: `https://push.example.test/subscription/${suffix}`, keys: { p256dh: 'public-key', auth: 'auth-key' } }),
    })
    await subscribe('one')
    await subscribe('two')

    const updatePreferences = (endpoint, monitorIDs, muteUntil = null, range = {}) => fetch(`${baseURL}/push-api/v1/subscriptions/preferences`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ endpoint, monitorIDs, muteUntil, ...range }),
    })
    await updatePreferences('https://push.example.test/subscription/one', [1])
    await updatePreferences('https://push.example.test/subscription/two', [2])

    const response = await fetch(`${baseURL}/push-api/v1/internal/channel-check-completed`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer internal' },
      body: JSON.stringify({ monitorID: 1, groupName: 'CC-Kiro', recentStatuses: ['operational'], currentStatus: 'operational' }),
    })
    assert.equal(response.status, 201)
    assert.deepEqual(sentEndpoints, ['https://push.example.test/subscription/one'])

    await updatePreferences('https://push.example.test/subscription/one', [1], null, {
      muteStart: new Date(Date.now() - 60_000).toISOString(),
      muteEnd: new Date(Date.now() + 60_000).toISOString(),
      muteDaily: false,
    })
    sentEndpoints.length = 0
    const mutedResponse = await fetch(`${baseURL}/push-api/v1/internal/channel-check-completed`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer internal' },
      body: JSON.stringify({ monitorID: 1, groupName: 'CC-Kiro', recentStatuses: ['degraded'], currentStatus: 'degraded' }),
    })
    assert.equal(mutedResponse.status, 201)
    assert.deepEqual(sentEndpoints, [])
  }, async (subscription) => {
    sentEndpoints.push(subscription.endpoint)
    return { statusCode: 201, resume() {} }
  })
})

test('channel-monitor delivery can be limited to status changes per subscription', async () => {
  const sentEndpoints = []
  await withServer(async (baseURL) => {
    const endpoint = 'https://push.example.test/subscription/change-only'
    await fetch(`${baseURL}/push-api/v1/subscriptions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ endpoint, keys: { p256dh: 'public-key', auth: 'auth-key' } }),
    })
    const update = (notifyOnlyOnChange) => fetch(`${baseURL}/push-api/v1/subscriptions/preferences`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ endpoint, monitorIDs: [7], notifyOnlyOnChange }),
    })
    await update(true)

    const check = (status) => fetch(`${baseURL}/push-api/v1/internal/channel-check-completed`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer internal' },
      body: JSON.stringify({ monitorID: 7, groupName: 'CC-Kiro', recentStatuses: [status], currentStatus: status }),
    })

    await check('operational')
    assert.deepEqual(sentEndpoints, [endpoint])
    sentEndpoints.length = 0

    await check('operational')
    assert.deepEqual(sentEndpoints, [])

    await check('degraded')
    assert.deepEqual(sentEndpoints, [endpoint])
  }, async (subscription) => {
    sentEndpoints.push(subscription.endpoint)
    return { statusCode: 201, resume() {} }
  })
})
