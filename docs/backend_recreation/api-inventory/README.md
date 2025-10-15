# API Inventory

## Overview
This inventory captures the publicly mounted HTTP routes defined in `routes/web.go` along with their handler implementations in `internal/http/diagnostics`, `internal/http/tasks`, and `internal/http/notifications`. The list reflects the current behaviour of the development bootstrap that wires in-memory services and the authentication middleware used under `/v1`.

## Route Catalogue
| Method | Path | Handler | Authentication | Description |
| --- | --- | --- | --- | --- |
| GET | `/` | inline handler in `routes/web.go` | None | Returns the static welcome banner "Welcome to the Larago API" as plain text. |
| GET | `/api/health` | `diagnostics.Handler.Health` | None | Emits dependency health alongside service metadata using the canonical success envelope. |
| GET | `/ready` | `diagnostics.Handler.Ready` | None | Reports readiness by evaluating database and Redis checks when configured. |
| GET | `/metrics` | `observability.Metrics.Handler` (optional) | None | Publishes Prometheus metrics when `WithMetrics` is supplied during router setup. |
| GET | `/api/tasks` | `tasks.Handler.List` | None | Lists curated dashboard tasks for the frontend without requiring authentication. |
| GET | `/v1/tasks` | `tasks.Handler.List` | Bearer token validated by `middleware.Authentication` | Authenticated variant of the task list used by the operator dashboard. |
| GET | `/v1/notifications` | `notifications.Handler.List` | Bearer token validated by `middleware.Authentication` | Returns paginated notifications for the authenticated principal. |
| PATCH | `/v1/notifications/:id` | `notifications.Handler.MarkRead` | Bearer token validated by `middleware.Authentication` | Marks the specified notification as read for the authenticated principal. |

## Authentication Notes
All `/v1` routes are wrapped by `middleware.Authentication`, which expects an `Authorization: Bearer <token>` header. Tokens are validated by `internal/auth.Service.ValidateAccessToken`; successful validation stores an `auth.Principal` on the Gin context for downstream handlers. Public routes under `/` and `/api` remain accessible without credentials in the local bootstrap.
