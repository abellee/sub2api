import assert from 'node:assert/strict'
import { mkdtemp, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { createServer, deliverNotification } from '../src/server.mjs'
import { groupConfig, validateGroupConfig } from '../src/group-notifications.mjs'
import { PushStore } from '../src/store.mjs'

const config = { enabled: true, baseURL: 'http://astrbot:6185', apiKey: 'test-only-key', platformID: 'napcat', groupIDs: ['764163343', '1132213496'], configID: 'default' }
const ok = () => Response.json({ status: 'ok' })

async function fixture(run, groupFetch = async () => ok(), sender = async () => {}) {
  const directory = await mkdtemp(path.join(os.tmpdir(), 'push-groups-'))
  const store = await new PushStore(path.join(directory, 'store.json')).init()
  const server = createServer({ store, groupFetch, sender, notifyToken: 'internal-test',
    authorizeAdmin: async req => req.headers.authorization === 'Bearer admin', logger: { error() {} } })
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
  const base = `http://127.0.0.1:${server.address().port}/push-api/v1`
  const request = (route, body, method = 'POST', admin = true) => fetch(base + route, {
    method, headers: { 'Content-Type': 'application/json', ...(admin ? { Authorization: 'Bearer admin' } : {}) },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  })
  try { await run({ store, base, request }) }
  finally {
    await new Promise(resolve => server.close(resolve))
    await rm(directory, { recursive: true, force: true })
  }
}

test('group admin settings require authorization, redact and preserve keys, and survive reload', async () => {
  await fixture(async ({ store, request }) => {
    for (const [route, method, body] of [
      ['/config', 'GET'], ['/config', 'PUT', config], ['/messages', 'GET'],
      ['/send', 'POST', { title: 'Test', body: 'Test', requestID: 'test-request' }],
    ]) {
      assert.equal((await request('/admin/group-notifications' + route, body, method, false)).status, 403)
    }
    const saved = await request('/admin/group-notifications/config', config, 'PUT')
    assert.equal(saved.status, 200)
    const visible = await saved.json()
    assert.equal(visible.keyConfigured, true)
    assert.equal(Object.hasOwn(visible, 'apiKey'), false)
    await request('/admin/group-notifications/config', { ...visible, apiKey: '' }, 'PUT')
    const restored = await new PushStore(store.filePath).init()
    assert.equal(groupConfig(restored).apiKey, config.apiKey)
    assert.deepEqual(groupConfig(restored, true), visible)
    const publicResponse = await request('/config', undefined, 'GET', false).then(r => r.json())
    assert.deepEqual(Object.keys(publicResponse), ['publicKey'])
  })
})

test('invalid group config and messages never send; disabled channel rejects manual sends', async () => {
  let calls = 0
  await fixture(async ({ request }) => {
    for (const change of [{ baseURL: 'file:///tmp' }, { baseURL: 'http://user:pass@host' }, { baseURL: 'http://host?token=secret' },
      { groupIDs: ['abc'] }, { groupIDs: [] }, { platformID: 'a:b' }, { apiKey: '' }, { enabled: 'true' }]) {
      assert.equal((await request('/admin/group-notifications/config', { ...config, ...change }, 'PUT')).status, 400)
    }
    assert.equal((await request('/admin/group-notifications/send', { title: 'X', body: 'Y', requestID: 'manual-001' })).status, 400)
    await request('/admin/group-notifications/config', config, 'PUT')
    for (const change of [{ title: '' }, { body: '' }, { body: 'x'.repeat(16001) }, { requestID: 'bad' }]) {
      assert.equal((await request('/admin/group-notifications/send', { title: 'X', body: 'Y', requestID: 'manual-001', ...change })).status, 400)
    }
    assert.equal(calls, 0)
  }, async () => { calls++; return ok() })
})

test('four activity stages send public group text with no push recipients and dedupe concurrent/reloaded calls', async () => {
  const calls = []
  await fixture(async ({ store, base }) => {
    await store.saveGroupConfig(config)
    const notify = (stage, target = store) => deliverNotification({ store: target, validation: { title: '抽奖', body: 'private winner@example.com CODE123', url: '/lottery', dedupeKey: `lottery:1:${stage}` },
      userIDs: [], groupNotification: { title: '抽奖', body: `public ${stage}`, url: '/lottery' }, groupFetch: async (url, options) => {
        calls.push({ url, options, payload: JSON.parse(options.body) }); return ok()
      } })
    for (const stage of ['before_start', 'started', 'before_draw', 'results']) {
      const [first, second] = await Promise.all([notify(stage), notify(stage)])
      assert.equal(first.group.delivered, 2)
      assert.deepEqual(second, first)
      assert.equal(first.delivered, 0)
      await notify(stage, await new PushStore(store.filePath).init())
    }
    assert.equal(calls.length, 8)
    assert.equal(calls[0].url, 'http://astrbot:6185/api/v1/im/messages')
    assert.equal(calls[0].options.headers.Authorization, `Bearer ${config.apiKey}`)
    assert.equal(calls[0].payload.umo, 'napcat:GroupMessage:764163343')
    assert.ok(!calls[0].payload.message.includes('https://'))
    assert.ok(calls.every(call => !call.payload.message.includes('private') && !call.payload.message.includes('CODE123')))
    const response = await fetch(base + '/internal/notify', { method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Push-Token': 'internal-test' },
      body: JSON.stringify({ title: 'Private task', body: 'Task reward', userIDs: [7], dedupeKey: 'task:1' }) })
    assert.equal(response.status, 200)
    assert.equal(Object.hasOwn(await response.json(), 'group'), false)
  }, async () => { assert.fail('private task must not send to group') })
})

test('shared internal endpoint retries only failed groups, retaining successful browser and group receipts', async () => {
  let pushes = 0
  const groups = []
  await fixture(async ({ store, base }) => {
    await store.saveGroupConfig(config)
    await store.upsertSubscription({ endpoint: 'https://push.example.test/one', keys: { p256dh: 'pk', auth: 'ak' }, user: { id: 7, username: 'tester' } })
    const input = { title: 'Results', body: '你中奖了 SECRET', url: '/lottery', userIDs: [7], dedupeKey: 'lottery:2:results', groupNotification: { body: '结果去活动页看一下。' } }
    const send = () => fetch(base + '/internal/notify', { method: 'POST', headers: { 'X-Push-Token': 'internal-test', 'Content-Type': 'application/json' }, body: JSON.stringify(input) })
    assert.equal((await send()).status, 502)
    assert.equal((await send()).status, 200)
    assert.equal(pushes, 1)
    assert.deepEqual(groups, ['764163343', '1132213496', '1132213496'])
  }, async (_, options) => {
    const body = JSON.parse(options.body)
    assert.ok(!body.message.includes('SECRET'))
    const group = body.umo.split(':')[2]
    groups.push(group)
    return group === '1132213496' && groups.length === 2 ? Response.json({ status: 'error' }, { status: 403 }) : ok()
  }, async () => { pushes++; return { statusCode: 201 } })
})

test('manual sends share dispatch, retain uncertain receipts and do not resend successful groups on retry', async () => {
  const attempts = []
  await fixture(async ({ store, request }) => {
    await store.saveGroupConfig(config)
    const input = { title: '活动提醒', body: '双11充值活动0点结束。需要的抓紧。', requestID: 'manual-unique-001' }
    assert.equal((await request('/admin/group-notifications/send', input)).status, 502)
    assert.equal((await request('/admin/group-notifications/send', input)).status, 502)
    assert.equal(attempts.length, 2)
    const restored = await new PushStore(store.filePath).init()
    assert.equal(restored.state.groupNotifies.find(r => r.groupID === '1132213496').status, 'uncertain')
    const history = await request('/admin/group-notifications/messages', undefined, 'GET').then(r => r.json())
    assert.equal(history.messages.length, 2)
    assert.ok(!JSON.stringify(history).includes(config.apiKey))
  }, async (url, options) => {
    const body = JSON.parse(options.body)
    if (url.endsWith('/chat')) return chatResponse('双11充值活动0点结束。需要的抓紧。')
    attempts.push(body)
    assert.equal(body.message, '双11充值活动0点结束。需要的抓紧。')
    if (body.umo.endsWith('1132213496')) throw new Error('connection lost after send')
    return ok()
  })
})

test('an interrupted sending receipt is never automatically replayed after restart', async () => {
  await fixture(async ({ store }) => {
    await store.saveGroupConfig({ ...config, groupIDs: ['764163343'] })
    await store.saveGroupReceipt({ key: 'event-1:napcat:764163343', groupID: '764163343', status: 'sending' })
    const restored = await new PushStore(store.filePath).init()
    const result = await deliverNotification({ store: restored, browserEnabled: false, validation: { dedupeKey: 'event-1' },
      groupNotification: { title: 'X', body: 'Y' }, groupFetch: async () => assert.fail('must not resend') })
    assert.equal(result.group.uncertain, 1)
  })
})

test('disabled group delivery remains compatible with private push requests', async () => {
  await fixture(async ({ store }) => {
    const result = await deliverNotification({ store, validation: { title: 'X', body: 'Y' }, userIDs: [], groupNotification: { title: 'X', body: 'Y' }, groupFetch: async () => assert.fail('disabled') })
    assert.equal(result.group.skipped, true)
    const defaults = groupConfig(store)
    assert.equal(validateGroupConfig({ ...config, clearApiKey: true, enabled: false }, defaults).apiKey, '')
  })
})

function chatResponse(text, complete = true) {
  const events = [{ type: 'session_id', session_id: 'test' }, { type: 'plain', data: '私密思考', chain_type: 'reasoning' }, { type: 'plain', data: text, streaming: false, chain_type: 'llm_result' }, { type: 'complete', data: text }]
  if (complete) events.push({ type: 'end', data: '' })
  const encoded = new TextEncoder().encode(events.map(event => `data: ${JSON.stringify(event)}\n\n`).join(''))
  // Split bytes within Chinese characters to cover incremental decoding.
  return new Response(new ReadableStream({ start(controller) {
    for (let offset = 0; offset < encoded.length; offset += 7) controller.enqueue(encoded.slice(offset, offset + 7))
    controller.close()
  } }), { headers: { 'Content-Type': 'text/event-stream' } })
}

test('announcement and manual notices always use the persona model and dedupe across retries', async () => {
  const chatCalls = [], messages = []
  await fixture(async ({ store, request }) => {
    await store.saveGroupConfig(config)
    const input = { title: '群通知', body: JSON.stringify({ 类型: '公告', 标题: '维护公告', 内容: '凌晨维护，请提前保存。' }), requestID: 'announcement-42', generate: false }
    assert.equal((await request('/admin/group-notifications/send', input)).status, 200)
    assert.equal((await request('/admin/group-notifications/send', input)).status, 200)
    assert.equal(chatCalls.length, 1)
    assert.match(chatCalls[0].message, /当前的人设/)
    assert.match(chatCalls[0].message, /公告不得编造成抽奖活动/)
    assert.match(chatCalls[0].message, /维护公告/)
    assert.deepEqual(messages, ['凌晨要维护。记得提前保存。', '凌晨要维护。记得提前保存。'])
    assert.ok(store.state.groupNotifies.every(record => record.generated))
  }, async (url, options) => {
    const payload = JSON.parse(options.body)
    if (url.endsWith('/chat')) { chatCalls.push(payload); return chatResponse('凌晨要维护。记得提前保存。') }
    messages.push(payload.message)
    return ok()
  })
})

test('announcement generation failure reports an error and never sends the raw announcement', async () => {
  await fixture(async ({ store, request }) => {
    await store.saveGroupConfig(config)
    const response = await request('/admin/group-notifications/send', { title: '公告', body: '维护事项', requestID: 'announcement-43' })
    assert.equal(response.status, 502)
    assert.match((await response.json()).message, /文案生成失败/)
    assert.equal(store.state.groupNotifies.length, 0)
  }, async url => {
    assert.ok(url.endsWith('/chat'))
    return Response.json({ status: 'error' }, { status: 503 })
  })
})

test('model creates persona copy once; retries reuse generated copy and never append activity links', async () => {
  let generated = 0, sent = 0, pushes = 0
  await fixture(async ({ store, base }) => {
    await store.saveGroupConfig(config)
    await store.upsertSubscription({ endpoint: 'https://push.example.test/persona', keys: { p256dh: 'pk', auth: 'ak' }, user: { id: 7, username: 'tester' } })
    const input = { title: '即将开奖', body: 'Browser stays unchanged', url: '/lottery', userIDs: [7], dedupeKey: 'lottery:9:before_draw', groupNotification: { body: JSON.stringify({ 活动名称: '双11', 阶段: '开奖前5分钟', 奖品: '余额5美元' }), generate: true } }
    const notify = () => fetch(base + '/internal/notify', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-Push-Token': 'internal-test' }, body: JSON.stringify(input) })
    assert.equal((await notify()).status, 502)
    assert.equal((await notify()).status, 200)
    assert.equal(generated, 1)
    assert.equal(sent, 3)
    assert.equal(pushes, 1)
    const restored = await new PushStore(store.filePath).init()
    assert.ok(restored.state.groupNotifies.every(record => record.body === '双11还有5分钟开奖。想参加的抓紧。'))
    assert.equal(Object.hasOwn(groupConfig(restored, true), 'siteURL'), false)
  }, async (url, options) => {
    const payload = JSON.parse(options.body)
    if (url.endsWith('/chat')) {
      generated++
      assert.equal(payload.config_id, 'default')
      assert.equal(payload.username, 'tokenfree-notifications')
      assert.equal(payload.flags.enable_default_system_prompt, true)
      assert.match(payload.message, /当前的人设/)
      assert.match(payload.message, /余额5美元/)
      assert.ok(!payload.message.includes('Browser stays unchanged'))
      return chatResponse('双11还有5分钟开奖。想参加的抓紧。')
    }
    sent++
    assert.equal(payload.message, '双11还有5分钟开奖。想参加的抓紧。')
    return sent === 2 ? Response.json({ status: 'error' }, { status: 403 }) : ok()
  }, async (_, payload) => {
    pushes++
    assert.equal(JSON.parse(payload).body, 'Browser stays unchanged')
    assert.equal(JSON.parse(payload).url, '/lottery')
    return { statusCode: 201 }
  })
})

test('failed or incomplete generation never sends draft, reasoning, errors, or a link to the group', async () => {
  for (const response of [Response.json({ status: 'error' }), chatResponse('partial', false), chatResponse('https://example.com'), chatResponse('')]) {
    await fixture(async ({ store }) => {
      await store.saveGroupConfig(config)
      const result = await deliverNotification({ store, browserEnabled: false, validation: { dedupeKey: 'generated-test' }, groupNotification: { title: 'Test', body: 'Public facts', generate: true }, groupFetch: async url => {
        assert.ok(url.endsWith('/chat'))
        return response
      } })
      assert.equal(result.group.generationFailed, true)
      assert.equal(result.group.delivered, 0)
      assert.equal(store.state.groupNotifies.length, 0)
    })
  }
})

test('browser-disabled requests send only groups, and browser-only requests never call AstrBot', async () => {
  let groups = 0, pushes = 0
  await fixture(async ({ store, base }) => {
    await store.saveGroupConfig(config)
    await store.upsertSubscription({ endpoint: 'https://push.example.test/routing', keys: { p256dh: 'pk', auth: 'ak' }, user: { id: 7, username: 'tester' } })
    const notify = body => fetch(base + '/internal/notify', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-Push-Token': 'internal-test' }, body: JSON.stringify({ title: 'Test', body: 'Message', userIDs: [7], ...body }) })
    assert.equal((await notify({ browserNotification: false, groupNotification: { body: 'Public' } })).status, 200)
    assert.equal(groups, 2)
    assert.equal(pushes, 0)
    assert.equal((await notify({ browserNotification: true })).status, 200)
    assert.equal(groups, 2)
    assert.equal(pushes, 1)
    assert.equal((await notify({ browserNotification: 'bad' })).status, 400)
  }, async () => { groups++; return ok() }, async () => { pushes++; return { statusCode: 201 } })
})
