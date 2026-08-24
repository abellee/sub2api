import { createAdminAuthorizer } from './auth.mjs'
import { loadConfig } from './config.mjs'
import { createServer } from './server.mjs'
import { PushStore } from './store.mjs'

const config = loadConfig()
const store = await new PushStore(config.dataFile).init()
const authorizeAdmin = createAdminAuthorizer(config)
const server = createServer({ store, authorizeAdmin, vapidSubject: config.vapidSubject })

server.listen(config.port, config.host, () => {
  console.log(`push notifier listening on http://${config.host}:${config.port}`)
})

function shutdown() {
  server.close((error) => {
    if (error) {
      console.error(error)
      process.exitCode = 1
    }
  })
}

process.on('SIGINT', shutdown)
process.on('SIGTERM', shutdown)
