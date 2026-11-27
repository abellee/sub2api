import assert from 'node:assert/strict'
import { mkdtemp, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { createServer } from '../src/server.mjs'
import { PushStore } from '../src/store.mjs'
import { createGroupJobs } from '../src/group-jobs.mjs'

const config = { enabled: true, baseURL: 'http://astrbot:6185', apiKey: 'private-test-key',
  platformID: 'napcat', groupIDs: ['764163343'], configID: 'default' }
const notice = { title: '维护公告', body: '请提前保存', generate: true }
const quiet = { error() {}, info() {} }
function generated(text) {
  return new Response(['plain', 'end'].map(type => `data: ${JSON.stringify({ type, data: type === 'plain' ? text : '' })}\n\n`).join(''),
    { headers: { 'content-type': 'text/event-stream' } })
}
async function fixture(run) {
  const directory = await mkdtemp(path.join(os.tmpdir(), 'group-jobs-'))
  try {
    const store = await new PushStore(path.join(directory, 'push.json')).init()
    await store.saveGroupConfig(config)
    await run(store)
  } finally { await rm(directory, { recursive: true, force: true }) }
}

test('slow generation outlives HTTP acceptance; duplicate submits and restart never resend', { timeout: 10000 }, async () => {
  await fixture(async store => {
    let release
    const gate = new Promise(resolve => { release = resolve })
    let markStarted
    const started = new Promise(resolve => { markStarted = resolve })
    let generations = 0
    const messages = []
    const server = createServer({ store, logger: { ...quiet, error: (...args) => console.error(...args) },
      authorizeAdmin: async req => req.headers.authorization === 'Bearer user-2-admin',
      groupFetch: async (url, options) => {
        if (url.endsWith('/chat')) {
          generations++
          markStarted()
          await gate
          assert.equal(options.signal.aborted, false)
          return generated('维护前记得保存。')
        }
        messages.push(JSON.parse(options.body))
        return Response.json({ status: 'ok' })
      } })
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
    const base = `http://127.0.0.1:${server.address().port}/push-api/v1/admin/group-notifications`
    const auth = { Authorization: 'Bearer user-2-admin', 'Content-Type': 'application/json' }
    const send = () => fetch(base + '/send', { method: 'POST', headers: auth,
      body: JSON.stringify({ ...notice, requestID: 'announcement-132' }) })
    try {
      const [first, duplicate] = await Promise.all([send(), send()])
      assert.equal(first.status, 202)
      assert.equal(duplicate.status, 202)
      const accepted = await first.json()
      assert.equal(accepted.group.queued, true)
      assert.equal(accepted.group.delivered, 0)
      assert.equal(messages.length, 0)
      await started
      const persisted = await new PushStore(store.filePath).init()
      assert.equal(persisted.state.groupJobs.length, 1)
      assert.ok(['queued', 'generating'].includes(persisted.state.groupJobs[0].status))
      assert.equal((await fetch(base + '/jobs')).status, 403)
      const pending = await fetch(base + '/jobs', { headers: auth }).then(r => r.json())
      assert.ok(!JSON.stringify(pending).includes(config.apiKey))
      assert.ok(!JSON.stringify(pending).includes(config.baseURL))
      const history = await fetch(base + '/messages', { headers: auth }).then(r => r.json())
      assert.equal(history.messages[0].status, 'sending')
      // Configuration changes must not redirect an already submitted notice.
      await store.saveGroupConfig({ ...config, groupIDs: ['1132213496'] })
      release()
      await server.groupJobs.run()
      assert.equal(generations, 1)
      assert.deepEqual(messages, [{ umo: 'napcat:GroupMessage:764163343', message: '维护前记得保存。' }])
      const completed = await fetch(base + '/jobs/' + encodeURIComponent(accepted.job.id), { headers: auth }).then(r => r.json())
      assert.equal(completed.job.status, 'sent')
      assert.equal(completed.job.result.delivered, 1)
      assert.equal((await send()).status, 200)
      await server.groupJobs.run()
      assert.equal(messages.length, 1)
      const restored = await new PushStore(store.filePath).init()
      const worker = createGroupJobs({ store: restored, logger: quiet, groupFetch: async () => assert.fail('must not replay') })
      await worker.run()
      await worker.stop()
    } finally {
      release()
      await server.groupJobs.stop()
      await new Promise(resolve => server.close(resolve))
    }
  })
})

test('restart resumes queued and generated work without losing copy or replaying ambiguous sends', async () => {
  await fixture(async store => {
    const destinations = { ...config, groupIDs: ['764163343', '1132213496'] }
    await store.enqueueGroupJob('resume-job', notice, destinations)
    await store.updateGroupJob('resume-job', { status: 'sending', generatedText: '已经生成好的通知。' })
    await store.saveGroupReceipt({ key: 'resume-job:napcat:764163343', groupID: '764163343', status: 'sent' })
    await store.saveGroupReceipt({ key: 'resume-job:napcat:1132213496', groupID: '1132213496', status: 'sending' })
    await store.enqueueGroupJob('queued-job', notice, config)
    await store.enqueueGroupJob('copy-job', notice, config)
    await store.updateGroupJob('copy-job', { status: 'sending', generatedText: '重启前已经保存的文案。' })
    const restored = await new PushStore(store.filePath).init()
    let generations = 0
    const messages = []
    const worker = createGroupJobs({ store: restored, logger: quiet, groupFetch: async (url, options) => {
      if (url.endsWith('/chat')) { generations++; return generated('新任务的通知。') }
      messages.push(JSON.parse(options.body).message)
      return Response.json({ status: 'ok' })
    } })
    await worker.run()
    await worker.stop()
    assert.equal(restored.state.groupJobs.find(j => j.id === 'resume-job').status, 'uncertain')
    assert.equal(restored.state.groupJobs.find(j => j.id === 'queued-job').status, 'sent')
    assert.equal(generations, 1)
    assert.deepEqual(messages, ['新任务的通知。', '重启前已经保存的文案。'])
  })
})

test('explicit failures retry with persisted generated copy and original destinations', async () => {
  await fixture(async store => {
    let generations = 0, sends = 0
    const worker = createGroupJobs({ store, logger: quiet, groupFetch: async url => {
      if (url.endsWith('/chat')) { generations++; return generated('保存一次的通知。') }
      sends++
      return sends === 1 ? Response.json({ status: 'error' }, { status: 403 }) : Response.json({ status: 'ok' })
    } })
    await worker.enqueue(notice, 'retry-job')
    await worker.run()
    assert.equal(store.state.groupJobs[0].status, 'failed')
    assert.equal(store.state.groupJobs[0].generatedText, '保存一次的通知。')
    await worker.enqueue(notice, 'retry-job')
    await worker.run()
    await worker.stop()
    assert.equal(store.state.groupJobs[0].status, 'sent')
    assert.equal(generations, 1)
    assert.equal(sends, 2)
  })
})
