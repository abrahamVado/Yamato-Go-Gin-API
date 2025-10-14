# Authorization Policy Overview

The authorization layer centers on the reusable `Policy` type, which caches permission sets per principal and evaluates `Gate` requirements. Middleware such as `RequireRole` and `RequirePermission` injects gate checks into Gin routes, delegating to the shared policy instance so repeated requests do not rebuild permission lookups. Cache entries are invalidated explicitly when upstream services mutate a subject's assignments.【F:internal/authorization/policy.go†L9-L70】【F:internal/middleware/role.go†L13-L26】【F:internal/middleware/permission.go†L13-L26】

## Default Roles and Permissions

Database seeds provision the foundational RBAC catalog: the `admin` role, granular permissions (e.g., `settings.manage`), and the administrator user binding. This bootstrap process is idempotent, allowing repeated runs without duplicating data. Because the `settings` table also receives baseline configuration in the same transaction, policy decisions can safely guard settings updates—the `settings.manage` permission is guaranteed to exist and can be assigned to additional roles when overrides are delegated.【F:seeds/seeder.go†L57-L170】

## Settings-Aware Workflows

Runtime overrides (such as rate limit changes or notification preferences) live in the `settings` table. Policy-protected admin routes should require the `settings.manage` permission so only authorized operators can mutate these overrides. When the worker or API fetches configuration via their respective providers, they inherit the guarantees enforced by the policy middleware: unprivileged principals cannot alter the JSON payloads that drive runtime behaviour.【F:internal/middleware/rate_limit.go†L27-L113】【F:seeds/seeder.go†L231-L270】

## Operational Practices

* Share a single `Policy` instance per process—its internal cache is concurrency-safe and keeps permission checks fast for HTTP handlers, background workers, and WebSocket authenticators.【F:internal/authorization/policy.go†L15-L70】
* After mutating a user's roles or permissions, call `Policy.Invalidate` with the subject identifier so the cache refreshes on the next request.【F:internal/authorization/policy.go†L72-L86】
