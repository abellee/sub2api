import { createAdminAuthorizer, createInternalAuthorizer, createSubscriberUserResolver } from './auth.mjs'
import { loadConfig } from './config.mjs'
import { createServer, deliverBroadcast } from './server.mjs'
import { PushStore } from './store.mjs'

const config = loadConfig()
const store = await new PushStore(config.dataFile).init()
const authorizeAdmin = createAdminAuthorizer(config)
const authorizeInternal = createInternalAuthorizer(config)
const resolveSubscriberUser = createSubscriberUserResolver(config)
const server = createServer({ store, authorizeAdmin, authorizeInternal, resolveSubscriberUser, vapidSubject: config.vapidSubject })

server.listen(config.port, config.host, () => {
  console.log(`push notifier listening on http://${config.host}:${config.port}`)
})

let scheduleRunnerActive = false
async function runDueSchedules() {
  if (scheduleRunnerActive) return
  scheduleRunnerActive = true
  try {
    const schedules = await store.claimDueSchedules()
    for (const schedule of schedules) {
      try {
        await deliverBroadcast({
          store,
          validation: schedule,
          vapidSubject: config.vapidSubject,
          sender: undefined,
        })
        await store.completeSchedule(schedule.id)
      } catch (error) {
        console.error(`scheduled push ${schedule.id} failed`, error)
        await store.failSchedule(schedule.id, error?.message || error)
      }
    }
  } catch (error) {
    console.error('scheduled push runner failed', error)
  } finally {
    scheduleRunnerActive = false
  }
}

const scheduleTimer = setInterval(() => { void runDueSchedules() }, 5000)
void runDueSchedules()

function shutdown() {
  clearInterval(scheduleTimer)
  server.close((error) => {
    if (error) {
      console.error(error)
      process.exitCode = 1
    }
  })
}

process.on('SIGINT', shutdown)
process.on('SIGTERM', shutdown)
