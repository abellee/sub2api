import assert from 'node:assert/strict'
import { mkdtemp, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { createServer } from '../src/server.mjs'
import { PushStore } from '../src/store.mjs'

async function withServer(run) {
  const directory = await mkdtemp(path.join(os.tmpdir(), 'push-notifier-'))
  const store = await new PushStore(path.join(directory, 'store.json')).init()
  const sender = async () => ({ statusCode: 201, resume() {} })
  const server = createServer({
    store,
    authorizeAdmin: async (request) => request.headers.authorization === 'Bearer admin',
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
