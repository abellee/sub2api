import crypto from 'node:crypto'
import { deliverGroupNotification, groupConfig } from './group-notifications.mjs'

const ACTIVE = new Set(['queued', 'generating', 'sending'])

export function publicGroupJob(job) {
  const { id, status, createdAt, updatedAt, result, error } = job
  return { id, status, createdAt, updatedAt, result, error }
}

export function groupJobResult(job) {
  return { ...(job.result || { delivered: 0, failed: 0 }), queued: ACTIVE.has(job.status), jobID: job.id, status: job.status }
}

export function groupJobMessages(store) {
  const records = [...store.state.groupNotifies]
  for (const job of store.state.groupJobs) {
    for (const groupID of job.config.groupIDs) {
      const key = `${job.id}:${job.config.platformID}:${groupID}`
      if (records.some(record => record.key === key)) continue
      records.push({ key, groupID, title: job.notification.title, body: job.generatedText || job.notification.body,
        status: ACTIVE.has(job.status) ? 'sending' : job.status, phase: job.status, updatedAt: job.updatedAt })
    }
  }
  return records.sort((a, b) => b.updatedAt.localeCompare(a.updatedAt)).slice(0, 100)
}

export function createGroupJobs({ store, groupFetch, logger = console }) {
  let running = null
  let timer = null
  let stopped = false

  async function processJobs() {
    // Interrupted sends keep their per-destination receipt. The sender will
    // never replay a destination whose delivery may already have succeeded.
    while (!stopped) {
      const job = [...store.state.groupJobs].reverse().find(item => ACTIVE.has(item.status))
      if (!job) return
      try {
        await store.updateGroupJob(job.id, { status: job.generatedText || !job.notification.generate ? 'sending' : 'generating' })
        const result = await deliverGroupNotification({
          store, notification: job.notification, dedupeKey: job.id, config: job.config,
          generatedText: job.generatedText, fetchImpl: groupFetch,
          onGenerated: text => store.updateGroupJob(job.id, { generatedText: text }),
          onSending: () => store.updateGroupJob(job.id, { status: 'sending' }),
        })
        const status = result.uncertain ? 'uncertain' : result.failed ? 'failed' : 'sent'
        const error = result.generationFailed ? '群通知文案生成失败，请检查 AstrBot 模型、人设配置及密钥的 chat 权限'
          : result.uncertain ? '发送结果不确定，请先核对群消息，不要重复发送'
            : result.failed ? '群通知发送失败，请检查机器人连接和权限' : ''
        await store.updateGroupJob(job.id, { status, result, error })
        logger.info?.('group notification job finished', { id: job.id, status, ...result })
      } catch (error) {
        // Persistence or unexpected sender errors must not cause a resend loop.
        await store.updateGroupJob(job.id, { status: 'uncertain', error: '处理意外中断，请核对群消息后处理' })
        logger.error?.('group notification job failed', { id: job.id, message: error?.message })
      }
    }
  }

  function run() {
    if (!running && !stopped) {
      running = processJobs().catch(error => logger.error?.('group notification worker failed', error)).finally(() => { running = null })
    }
    return running || Promise.resolve()
  }

  return {
    async enqueue(notification, id = crypto.randomUUID()) {
      const job = await store.enqueueGroupJob(id, notification, groupConfig(store))
      // Finish the HTTP response before starting slow model requests.
      setImmediate(() => { void run() })
      return job
    },
    run,
    start() {
      stopped = false
      timer = setInterval(() => { void run() }, 1000)
      timer.unref()
      void run()
    },
    async stop() {
      stopped = true
      clearInterval(timer)
      await running
    },
  }
}
