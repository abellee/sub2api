# Push Notifier

Standalone Web Push broadcaster for Sub2API. It owns subscriptions, VAPID keys, delivery history, and push delivery. It does not read or modify Sub2API billing data.

## Local development

```powershell
cd push-notifier
$env:PUSH_DEV_ADMIN_TOKEN='sub2api-local-admin-mock-token'
pnpm install
pnpm start
```

The service listens on `127.0.0.1:8091` by default. The frontend development server proxies `/push-api` to it.

Production should omit `PUSH_DEV_ADMIN_TOKEN`, persist `PUSH_DATA_FILE` on a mounted volume, set a valid `PUSH_VAPID_SUBJECT`, and set `SUB2API_AUTH_URL` to the existing Sub2API `/api/v1/auth/me` endpoint. The broadcast API forwards the current Sub2API bearer token and only accepts users whose existing profile reports the `admin` role.

Set `PUSH_INTERNAL_TOKEN` to a strong random secret to accept scheduled channel-check completion events from Sub2API. Configure the same value as `PUSH_NOTIFIER_INTERNAL_TOKEN` in the Sub2API container. `PUSH_NOTIFIER_INTERNAL_URL` can override the internal endpoint address when the services do not share the host network. Notifications show the group name, the primary model's latest five results (`🟩` operational, `🟨` degraded, `🟥` failed/error), and the current status. Manual "run now" checks do not emit these events.

Route `/push-api/*` to this service on the same public HTTPS origin as Sub2API. The Service Worker remains at `/push-worker.js`, so subscriptions and notifications belong to the main site rather than a separate notification-service domain.
