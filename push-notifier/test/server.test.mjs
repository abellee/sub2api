import assert from 'node:assert/strict'
import { mkdtemp, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { createServer } from '../src/server.mjs'
import { PushStore } from '../src/store.mjs'

async function withServer(run, sender = async () => ({ statusCode: 201, resume() {} }), resolveSubscriberUser = async () => null, notifyToken = '') {
  const directory = await mkdtemp(path.join(os.tmpdir(), 'push-notifier-'))
  const store = await new PushStore(path.join(directory, 'store.json')).init()
  const server = createServer({
    store,
    authorizeAdmin: async (request) => request.headers.authorization === 'Bearer admin',
    resolveSubscriberUser: resolveSubscriberUser || (async () => null),
    vapidSubject: 'mailto:test@example.com',
    notifyToken,
    sender: sender || (async () => ({ statusCode: 201, resume() {} })),
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

test('a partial profile cannot replace a stored email', async () => {
  await withServer(async (baseURL) => {
    const body = {
      endpoint: 'https://push.example.test/subscription/email',
      keys: { p256dh: 'public-key', auth: 'auth-key' },
    }
    const first = await fetch(`${baseURL}/push-api/v1/subscriptions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer user' },
      body: JSON.stringify({ ...body, user: { id: 7, username: 'ada', email: 'ada@example.com' } }),
    })
    assert.equal(first.status, 201)

    const second = await fetch(`${baseURL}/push-api/v1/subscriptions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer user' },
      body: JSON.stringify({ ...body, user: { id: 7, username: 'ada', email: '' } }),
    })
    assert.equal(second.status, 201)

    const listed = await fetch(`${baseURL}/push-api/v1/admin/subscriptions`, {
      headers: { Authorization: 'Bearer admin' },
    }).then((result) => result.json())
    assert.equal(listed.subscriptions.length, 1)
    assert.deepEqual(listed.subscriptions[0].user, { id: 7, username: 'ada', email: 'ada@example.com' })
  })
})

test('the signed-in account email wins over a stale browser profile', async () => {
  const resolveSubscriberUser = async () => ({ id: 7, username: 'ada', email: 'ada@example.com' })
  await withServer(async (baseURL) => {
    const response = await fetch(`${baseURL}/push-api/v1/subscriptions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer user' },
      body: JSON.stringify({
        endpoint: 'https://push.example.test/subscription/account',
        keys: { p256dh: 'public-key', auth: 'auth-key' },
        user: { id: 7, username: 'cached', email: 'stale@example.com' },
      }),
    })
    assert.equal(response.status, 201)

    const listed = await fetch(`${baseURL}/push-api/v1/admin/subscriptions`, {
      headers: { Authorization: 'Bearer admin' },
    }).then((result) => result.json())
    assert.deepEqual(listed.subscriptions[0].user, { id: 7, username: 'ada', email: 'ada@example.com' })
  }, undefined, resolveSubscriberUser)
})

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

test('manual history can be cleared', async () => {
  await withServer(async (baseURL) => {
    const manualResponse = await fetch(`${baseURL}/push-api/v1/admin/broadcast`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer admin' },
      body: JSON.stringify({ title: 'Manual', body: 'Message', url: '/' }),
    })
    assert.equal(manualResponse.status, 201)

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

test('administrators can schedule and cancel a future broadcast', async () => {
  await withServer(async (baseURL) => {
    const scheduledAt = new Date(Date.now() + 60 * 60 * 1000).toISOString()
    const forbidden = await fetch(`${baseURL}/push-api/v1/admin/schedules`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: 'Later', body: 'Message', url: '/', scheduledAt }),
    })
    assert.equal(forbidden.status, 403)

    const response = await fetch(`${baseURL}/push-api/v1/admin/schedules`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer admin' },
      body: JSON.stringify({ title: 'Later', body: 'Message', url: '/keys', scheduledAt }),
    })
    const payload = await response.json()
    assert.equal(response.status, 201)
    assert.equal(payload.schedule.title, 'Later')
    assert.equal(payload.schedule.status, 'pending')

    const listed = await fetch(`${baseURL}/push-api/v1/admin/schedules`, {
      headers: { Authorization: 'Bearer admin' },
    }).then((result) => result.json())
    assert.equal(listed.schedules.length, 1)

    const removed = await fetch(`${baseURL}/push-api/v1/admin/schedules/${payload.schedule.id}`, {
      method: 'DELETE',
      headers: { Authorization: 'Bearer admin' },
    })
    assert.equal(removed.status, 200)
  })
})

test('a bound client id keeps the original account when the switch is updated', async () => {
  let email = 'ada@example.com'
  const resolveSubscriberUser = async () => ({ id: 7, username: 'ada', email })
  await withServer(async (baseURL) => {
    const body = {
      clientId: 'client-ada-0001',
      endpoint: 'https://push.example.test/subscription/bound',
      keys: { p256dh: 'public-key', auth: 'auth-key' },
      enabled: true,
      user: { id: 7, username: 'cached', email: 'stale@example.com' },
    }
    const first = await fetch(`${baseURL}/push-api/v1/subscriptions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer user' },
      body: JSON.stringify(body),
    })
    assert.equal(first.status, 201)
    assert.equal((await first.json()).subscription.bound, true)

    email = 'new@example.com'
    const second = await fetch(`${baseURL}/push-api/v1/subscriptions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer user' },
      body: JSON.stringify({
        ...body,
        endpoint: 'https://push.example.test/subscription/bound-rotated',
        user: { id: 99, username: 'other', email: 'other@example.com' },
      }),
    })
    assert.equal(second.status, 201)

    const listed = await fetch(`${baseURL}/push-api/v1/admin/subscriptions`, {
      headers: { Authorization: 'Bearer admin' },
    }).then((result) => result.json())
    assert.equal(listed.subscriptions.length, 1)
    assert.equal(listed.subscriptions[0].enabled, true)
    assert.deepEqual(listed.subscriptions[0].user, { id: 7, username: 'ada', email: 'ada@example.com' })
  }, undefined, resolveSubscriberUser)
})

test('turning notifications off keeps the user and reports disabled', async () => {
  await withServer(async (baseURL, store) => {
    const subscribed = await fetch(`${baseURL}/push-api/v1/subscriptions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        clientId: 'client-off-0001',
        endpoint: 'https://push.example.test/subscription/off',
        keys: { p256dh: 'public-key', auth: 'auth-key' },
        enabled: true,
        user: { id: 8, username: 'bea', email: 'bea@example.com' },
      }),
    })
    assert.equal(subscribed.status, 201)

    const disabled = await fetch(`${baseURL}/push-api/v1/subscriptions`, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ clientId: 'client-off-0001' }),
    })
    const payload = await disabled.json()
    assert.equal(disabled.status, 200)
    assert.equal(payload.removed, false)
    assert.equal(payload.disabled, true)
    assert.equal(store.overview().activeSubscriptions, 0)

    const listed = await fetch(`${baseURL}/push-api/v1/admin/subscriptions`, {
      headers: { Authorization: 'Bearer admin' },
    }).then((result) => result.json())
    assert.equal(listed.subscriptions.length, 1)
    assert.equal(listed.subscriptions[0].enabled, false)
    assert.equal(listed.subscriptions[0].user.email, 'bea@example.com')

    const status = await fetch(`${baseURL}/push-api/v1/subscriptions/status?clientId=client-off-0001`).then((result) => result.json())
    assert.deepEqual(status, { found: true, enabled: false, bound: true })
  })
})

test('broadcast and targeted notify skip disabled subscriptions and dedupe', async () => {
  const sent = []
  const sender = async (subscription) => {
    sent.push(subscription.endpoint)
    return { statusCode: 201, resume() {} }
  }
  await withServer(async (baseURL) => {
    for (const user of [
      { id: 7, endpoint: 'https://push.example.test/subscription/ada', clientId: 'client-ada-0002' },
      { id: 8, endpoint: 'https://push.example.test/subscription/bea', clientId: 'client-bea-0002' },
    ]) {
      const response = await fetch(`${baseURL}/push-api/v1/subscriptions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          clientId: user.clientId,
          endpoint: user.endpoint,
          keys: { p256dh: 'public-key', auth: 'auth-key' },
          enabled: user.id === 7,
          user: { id: user.id, username: `user-${user.id}`, email: `user${user.id}@example.com` },
        }),
      })
      assert.equal(response.status, 201)
    }

    const broadcast = await fetch(`${baseURL}/push-api/v1/admin/broadcast`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer admin' },
      body: JSON.stringify({ title: 'Hello', body: 'Everyone enabled', url: '/lottery' }),
    })
    assert.equal(broadcast.status, 201)
    assert.deepEqual(sent, ['https://push.example.test/subscription/ada'])

    const notify = await fetch(`${baseURL}/push-api/v1/internal/notify`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Push-Token': 'notify-secret' },
      body: JSON.stringify({
        title: 'You won',
        body: 'Open the result',
        url: '/lottery',
        dedupeKey: 'lottery:1:results',
        userIDs: [7, 8],
      }),
    })
    const first = await notify.json()
    assert.equal(notify.status, 200)
    assert.equal(first.notified, 1)
    assert.equal(first.delivered, 1)
    assert.equal(first.deduped, false)
    assert.equal(sent.length, 2)

    const repeat = await fetch(`${baseURL}/push-api/v1/internal/notify`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Push-Token': 'notify-secret' },
      body: JSON.stringify({
        title: 'You won',
        body: 'Open the result',
        url: '/lottery',
        dedupeKey: 'lottery:1:results',
        userIDs: [7],
      }),
    })
    const second = await repeat.json()
    assert.equal(second.deduped, true)
    assert.equal(second.notified, 1)
    assert.equal(sent.length, 2)
  }, sender, undefined, 'notify-secret')
})

test('an expired push endpoint keeps the bound user', async () => {
  const sender = async () => {
    throw Object.assign(new Error('gone'), { statusCode: 410 })
  }
  await withServer(async (baseURL, store) => {
    const subscribed = await fetch(`${baseURL}/push-api/v1/subscriptions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        clientId: 'client-gone-0001',
        endpoint: 'https://push.example.test/subscription/gone',
        keys: { p256dh: 'public-key', auth: 'auth-key' },
        user: { id: 7, username: 'ada', email: 'ada@example.com' },
      }),
    })
    assert.equal(subscribed.status, 201)

    const broadcast = await fetch(`${baseURL}/push-api/v1/admin/broadcast`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer admin' },
      body: JSON.stringify({ title: 'Hello', body: 'Gone', url: '/' }),
    })
    assert.equal(broadcast.status, 201)
    const listed = store.listSubscriptionSummaries()
    assert.equal(listed.length, 1)
    assert.equal(listed[0].user.email, 'ada@example.com')
    assert.equal(store.listDeliverableSubscriptions().length, 0)
  }, sender)
})

test('subscriber list includes only enabled deliverable users', async () => {
  await withServer(async (baseURL) => {
    for (const user of [
      { id: 7, enabled: true, endpoint: 'https://push.example.test/subscription/ada' },
      { id: 8, enabled: false, endpoint: 'https://push.example.test/subscription/bea' },
    ]) {
      const response = await fetch(`${baseURL}/push-api/v1/subscriptions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          clientId: `client-user-${user.id}`,
          endpoint: user.endpoint,
          keys: { p256dh: 'public-key', auth: 'auth-key' },
          enabled: user.enabled,
          user: { id: user.id, username: `user-${user.id}`, email: `user${user.id}@example.com` },
        }),
      })
      assert.equal(response.status, 201)
    }

    const denied = await fetch(`${baseURL}/push-api/v1/internal/subscribers`)
    assert.equal(denied.status, 403)

    const listed = await fetch(`${baseURL}/push-api/v1/internal/subscribers`, {
      headers: { 'X-Push-Token': 'notify-secret' },
    })
    assert.equal(listed.status, 200)
    assert.deepEqual(await listed.json(), {
      users: [{ id: 7, email: 'user7@example.com' }],
    })
  }, undefined, undefined, 'notify-secret')
})

test('browser and page channels are delivered separately', async () => {
  const sent = []
  const sender = async (_subscription, payload) => {
    sent.push(JSON.parse(payload))
    return { statusCode: 201, resume() {} }
  }
  const resolveSubscriberUser = async (request) => (
    request.headers.authorization === 'Bearer user'
      ? { id: 7, username: 'ada', email: 'ada@example.com' }
      : null
  )
  await withServer(async (baseURL) => {
    const subscribed = await fetch(`${baseURL}/push-api/v1/subscriptions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        endpoint: 'https://push.example.test/subscription/channels',
        keys: { p256dh: 'public-key', auth: 'auth-key' },
        user: { id: 7, username: 'ada', email: 'ada@example.com' },
      }),
    })
    assert.equal(subscribed.status, 201)

    const anonymous = await fetch(`${baseURL}/push-api/v1/page-notifications`)
    assert.equal(anonymous.status, 401)
    const cursor = await fetch(`${baseURL}/push-api/v1/page-notifications`, {
      headers: { Authorization: 'Bearer user' },
    }).then((result) => result.json())
    assert.deepEqual(cursor.notices, [])
    const since = new Date(Date.now() - 1000).toISOString()

    const pageOnly = await fetch(`${baseURL}/push-api/v1/admin/broadcast`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer admin' },
      body: JSON.stringify({
        title: 'Page only',
        body: 'Shown inside the page',
        url: '/lottery',
        image: 'https://cdn.example.test/page.jpg',
        channels: ['page'],
      }),
    })
    const pageMessage = await pageOnly.json()
    assert.equal(pageOnly.status, 201)
    assert.deepEqual(pageMessage.message.channels, ['page'])
    assert.equal(pageMessage.message.delivered, 0)
    assert.equal(sent.length, 0)

    const both = await fetch(`${baseURL}/push-api/v1/admin/broadcast`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer admin' },
      body: JSON.stringify({
        title: 'Both',
        body: 'System and page',
        url: '/',
        channels: ['browser', 'page'],
      }),
    })
    assert.equal(both.status, 201)
    assert.equal(sent.length, 1)
    assert.deepEqual(sent[0].channels, ['browser'])
    assert.equal(sent[0].title, 'Both')

    const notices = await fetch(`${baseURL}/push-api/v1/page-notifications?since=${encodeURIComponent(since)}`, {
      headers: { Authorization: 'Bearer user' },
    }).then((result) => result.json())
    assert.deepEqual(notices.notices.map((item) => item.title), ['Page only', 'Both'])
    assert.equal(notices.notices[0].image, 'https://cdn.example.test/page.jpg')
    assert.equal(notices.notices[0].id, pageMessage.message.id)

    const invalid = await fetch(`${baseURL}/push-api/v1/admin/broadcast`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer admin' },
      body: JSON.stringify({ title: 'Nope', body: 'Missing channel', url: '/', channels: [] }),
    })
    assert.equal(invalid.status, 400)
  }, sender, resolveSubscriberUser)
})

test('administrators can upload an image that is then served by its public link', async () => {
  await withServer(async (baseURL) => {
    const png = Buffer.from(
      '89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000a49444154789c63000100000500010d0a2db40000000049454e44ae426082',
      'hex',
    )
    const forbidden = await fetch(`${baseURL}/push-api/v1/admin/images`, {
      method: 'POST',
      headers: { 'Content-Type': 'image/png' },
      body: png,
    })
    assert.equal(forbidden.status, 403)

    const uploaded = await fetch(`${baseURL}/push-api/v1/admin/images`, {
      method: 'POST',
      headers: { 'Content-Type': 'image/png', Authorization: 'Bearer admin' },
      body: png,
    })
    assert.equal(uploaded.status, 201)
    const payload = await uploaded.json()
    assert.match(payload.url, /^\/push-api\/v1\/images\/[a-f0-9]{32}\.png$/)

    const image = await fetch(`${baseURL}${payload.url}`)
    assert.equal(image.status, 200)
    assert.equal(image.headers.get('content-type'), 'image/png')
    assert.equal(Buffer.from(await image.arrayBuffer()).equals(png), true)

    const traversal = await fetch(`${baseURL}/push-api/v1/images/..%2Fstore.json`)
    assert.equal(traversal.status, 404)

    const wrongType = await fetch(`${baseURL}/push-api/v1/admin/images`, {
      method: 'POST',
      headers: { 'Content-Type': 'image/png', Authorization: 'Bearer admin' },
      body: Buffer.from('not-a-png'),
    })
    assert.equal(wrongType.status, 415)
  })
})
