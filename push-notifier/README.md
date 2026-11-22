# Push Notifier

Standalone Web Push broadcaster for Sub2API. It owns subscriptions, VAPID keys, delivery history, and push delivery. It does not read or modify Sub2API billing data.

## Group notifications

Open **Push notifications → Group notifications** as an administrator to save the AstrBot URL, a dedicated API key with `im` and `chat` scopes, platform ID (normally `napcat`), QQ group numbers, and the AstrBot configuration ID (normally `default`). The bot must already belong to the groups. The URL must be reachable **from this service**, e.g. `http://astrbot:6185` on a shared Docker network; `localhost` inside a container is that container. No public port needs to be opened. Configuration is persisted with the existing private store. API responses never return the saved key; a blank key preserves it.

The tab supports manual messages (rewritten using the current bot persona), a recipient confirmation and the latest 100 delivery records. All stages and manual group notices use `deliverNotification`, which dispatches through the same AstrBot sender. It calls `POST /api/v1/im/messages` with `umo: <platformID>:GroupMessage:<groupID>`.

Admin endpoints under `/push-api/v1` (existing admin authentication):

- `GET/PUT /admin/group-notifications/config`: `{enabled,baseURL,apiKey?,platformID,groupIDs,configID}`; responses replace `apiKey` with `keyConfigured`.
- `POST /admin/group-notifications/send`: `{title,body,requestID}`; reuse the same request ID when retrying unchanged content to avoid duplicate deliveries.
- `GET /admin/group-notifications/messages`: per-group delivery status.

`POST /internal/notify` optionally accepts `groupNotification: {body,generate?}` and `browserNotification` (default true). Omit `groupNotification` for private messages such as task rewards. Lottery creation/editing has independent browser/group checkboxes, both selected by default, alongside the existing four stages. The selection is persisted for manual and daily activities, including explicitly empty channel selections. Old records default to both channels. Browser recipients, private results copy, and browser links remain unchanged. Group-only delivery does not query browser recipient eligibility. Already recorded stages are not replayed.

For lottery group notices, `generate:true` passes a JSON summary (activity name, description, Beijing times, stage, conditions and public prizes) to AstrBot `POST /api/v1/chat` in a fresh session. It uses the selected AstrBot configuration's current model and persona, then sends the finished text through the common IM sender. No winner identities, codes, private browser text or separate activity links are sent to the model. Generation failures do not send draft/error text. Generated copy is saved per destination and reused on delivery retries. Announcements and manual notifications use the same persona generation and dispatch path. The manual endpoint always generates persona copy, including when the client supplies `generate:false`; it accepts up to 16000 characters of source facts and has no separate link field or website setting.

When creating an announcement, the optional **Group notification** checkbox is available for active announcements. Saving sends an immediate notice to all configured groups; future start times are announced as previews, with Beijing times included. Draft and archived announcements never send. This is a creation-time action, not a scheduled delivery setting. The announcement is saved first; if sending fails, the dialog retains the saved ID and offers a group-only retry with the same dedupe key, without creating another announcement. Website popup/silent behavior is unchanged.

Lottery delivery requires `LOTTERY_PUSH_URL` to reach this service and `LOTTERY_PUSH_TOKEN` to match `PUSH_NOTIFY_TOKEN`. Enable and save the group settings before future stages become due. A disabled group channel is skipped.

Successful destinations are retained across retries and restarts. Explicit rejections can be retried; interrupted or ambiguous sends are recorded as `uncertain` (or `sending` if the process stopped) and are **not resent automatically**. Check the group before manually composing another message. A group failure returns HTTP 502 without repeating successful browser or group deliveries on retry. Different groups send in parallel within a 15-second request timeout. Model generation has a 60-second timeout; lottery calls that request generation allow 90 seconds overall.

## Local development

```powershell
cd push-notifier
$env:PUSH_DEV_ADMIN_TOKEN='sub2api-local-admin-mock-token'
pnpm install
pnpm start
```

The service listens on `127.0.0.1:8091` by default. The frontend development server proxies `/push-api` to it.

Production should omit `PUSH_DEV_ADMIN_TOKEN`, persist `PUSH_DATA_FILE` on a mounted volume, set a valid `PUSH_VAPID_SUBJECT`, and set `SUB2API_AUTH_URL` to the existing Sub2API `/api/v1/auth/me` endpoint. The broadcast API forwards the current Sub2API bearer token and only accepts users whose existing profile reports the `admin` role.

Route `/push-api/*` to this service on the same public HTTPS origin as Sub2API. The Service Worker remains at `/push-worker.js`, so subscriptions and notifications belong to the main site rather than a separate notification-service domain. Users turn delivery on or off from the account menu. The service does not filter by channel monitor or quiet hours.

Administrators can select specific notification users for immediate or scheduled delivery. The frontend uses `/push-api/v1/admin/targeted` and `/push-api/v1/admin/targeted/schedules`, which require a non-empty `userIDs` array of positive integer account IDs. These separate routes ensure an older service rejects targeted requests instead of broadcasting them. The existing broadcast and schedule routes also accept optional `userIDs`; omit it to send to everyone. Browser delivery filters enabled subscriptions by account, and in-page delivery checks the signed-in account before returning a notice. Scheduled jobs and delivery history retain the selected account IDs.
