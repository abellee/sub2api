self.addEventListener('push', (event) => {
  let payload = {}
  try {
    payload = event.data ? event.data.json() : {}
  } catch {
    payload = { title: 'Sub2API', body: event.data ? event.data.text() : '' }
  }

  const title = String(payload.title || 'Sub2API')
  const image = String(payload.image || '').trim()
  const options = {
    body: String(payload.body || ''),
    icon: '/favicon.ico',
    badge: '/favicon.ico',
    tag: String(payload.tag || 'sub2api-notification'),
    renotify: true,
    data: {
      url: String(payload.url || '/'),
    },
  }
  if (image) options.image = image

  event.waitUntil(self.registration.showNotification(title, options))
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const targetURL = new URL(event.notification.data?.url || '/', self.location.origin).href

  event.waitUntil((async () => {
    const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
    const sameOriginWindow = windows.find((client) => new URL(client.url).origin === self.location.origin)
    if (sameOriginWindow) {
      await sameOriginWindow.navigate(targetURL)
      return sameOriginWindow.focus()
    }
    return self.clients.openWindow(targetURL)
  })())
})
