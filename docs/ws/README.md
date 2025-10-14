# WebSocket Delivery Model

The WebSocket server accepts HTTP upgrade requests through `Server.Handle`, requiring clients to supply a bearer token in the `Sec-WebSocket-Protocol` header. The handler authenticates the token, echoes the `bearer` subprotocol, and subscribes the connection to user-specific channels plus the administrative broadcast channel when the principal has the `admin` role.【F:internal/websocket/server.go†L32-L99】

## Fan-Out Sources

Inbound events originate from the background worker. The queue registry installs the `notification_fanout` job, which iterates through the provided user IDs and invokes the notifier implementation bound to the worker process. In the default worker, notifications are logged to stdout, but production deployments replace the notifier with the WebSocket broadcaster so queued events reach active connections.【F:internal/queue/notification.go†L5-L31】【F:cmd/worker/main.go†L36-L70】

## Settings-Driven Preferences

Baseline delivery preferences live inside the `settings` table under the `notifications.defaults` key. The database seeder upserts this entry so that the notification service can honor organization-wide overrides (for example, disabling email fan-out while leaving WebSocket events enabled). Administrators with the required permissions can update the row, and the worker can pull the latest JSON payload to adapt fan-out behaviour without redeploying the worker or API.【F:seeds/seeder.go†L231-L270】

## Operational Notes

* WebSocket write deadlines default to five seconds; adjust the worker publish cadence if clients experience frequent disconnects.【F:internal/websocket/server.go†L24-L91】
* The `make run-worker` task boots the Redis-backed queue consumer, Cron scheduler bootstrapper, and demo notifiers to exercise fan-out locally.【F:Makefile†L21-L24】【F:cmd/worker/main.go†L32-L81】
