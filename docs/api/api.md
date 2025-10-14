# Yamato API Endpoint Catalog

This catalog enumerates the HTTP endpoints exposed by the Yamato Go Gin service and summarizes the headers, request bodies, and query parameters required to interact with them. Authentication-sensitive handlers assume a Bearer token authentication middleware that injects an `auth.Principal` into the Gin context; provide an `Authorization: Bearer <access token>` header when a handler calls `internalauth.PrincipalFromContext` or the admin RBAC middleware. 【F:internal/http/auth/handlers.go†L312-L325】【F:internal/http/admin/handlers.go†L239-L270】

## Conventions

- **Content type** – JSON endpoints expect `Content-Type: application/json` and respond using the ADR-003 success and error envelopes described in the handler implementations. 【F:internal/http/auth/handlers.go†L126-L137】【F:internal/http/joinrequests/handlers.go†L106-L120】
- **Pagination** – Admin and notification listings accept `page` and `per_page` query parameters with positive integer values, defaulting to page 1 and 20 items per page. 【F:internal/http/admin/handlers.go†L216-L236】【F:internal/http/notifications/handlers.go†L157-L182】
- **Status filter** – Join request listings allow an optional `status` filter accepting `pending`, `approved`, or `declined`. 【F:internal/http/joinrequests/handlers.go†L185-L213】

## Public and Diagnostics Endpoints

| Method | Path | Description | Headers | Query/Body |
| --- | --- | --- | --- | --- |
| GET | `/` | Returns the welcome banner for the API. | None required. | No body. 【F:routes/web.go†L38-L42】 |
| GET | `/api/health` | Reports service and dependency health using the diagnostics handler. | None required. | No body; response includes database and Redis component statuses. 【F:routes/web.go†L44-L49】【F:internal/http/diagnostics/handlers.go†L60-L115】 |
| GET | `/ready` | Lightweight readiness probe that reuses diagnostics checks. | None required. | No body. 【F:routes/web.go†L50-L51】【F:internal/http/diagnostics/handlers.go†L116-L170】 |
| GET | `/metrics` | Prometheus scrape endpoint exposed when metrics are configured. | None required. | No body; returns Prometheus metrics exposition format. 【F:routes/web.go†L52-L55】 |

## Authentication Endpoints (`/v1/auth`)

All authentication endpoints consume and emit JSON payloads using the ADR-003 envelopes; provide `Content-Type: application/json` for POST requests. 【F:internal/http/auth/handlers.go†L126-L137】

| Method | Path | Description | Headers | Request Body |
| --- | --- | --- | --- | --- |
| POST | `/v1/auth/register` | Creates a user and issues an access/refresh token pair. | `Content-Type: application/json` | `{ "email": string, "password": string }` – both trimmed and required. 【F:internal/http/auth/handlers.go†L149-L199】【F:internal/http/auth/handlers.go†L54-L64】 |
| POST | `/v1/auth/login` | Authenticates credentials and rotates tokens. | `Content-Type: application/json` | `{ "email": string, "password": string }` – required. 【F:internal/http/auth/handlers.go†L201-L244】【F:internal/http/auth/handlers.go†L60-L64】 |
| POST | `/v1/auth/refresh` | Exchanges a refresh token for a new token pair. | `Content-Type: application/json` | `{ "refresh_token": string }` – required. 【F:internal/http/auth/handlers.go†L246-L278】【F:internal/http/auth/handlers.go†L66-L69】 |
| POST | `/v1/auth/logout` | Revokes the supplied access and refresh tokens. | `Content-Type: application/json` | `{ "refresh_token": string, "access_token": string }` – both required. 【F:internal/http/auth/handlers.go†L280-L309】【F:internal/http/auth/handlers.go†L71-L75】 |

### Current Principal (`/v1/user`)

| Method | Path | Description | Headers | Request |
| --- | --- | --- | --- | --- |
| GET | `/v1/user` | Returns the authenticated subject with roles and permissions. | `Authorization: Bearer <access token>` | No body. 【F:internal/http/auth/handlers.go†L312-L325】【F:internal/httpserver/router.go†L21-L23】 |

## Notification Endpoints (`/v1/notifications`)

Authenticated users can page through notifications and mark items as read. 【F:internal/http/notifications/handlers.go†L89-L155】 Tests exercise these routes at `/v1/notifications` and `/v1/notifications/{id}`. 【F:internal/http/notifications/handlers_test.go†L100-L156】

| Method | Path | Description | Headers | Query/Body |
| --- | --- | --- | --- | --- |
| GET | `/v1/notifications` | Lists notifications for the principal, returning pagination metadata. | `Authorization: Bearer <access token>` | Query: `page`, `per_page` (optional, positive integers). Body: none. 【F:internal/http/notifications/handlers.go†L89-L123】【F:internal/http/notifications/handlers.go†L157-L182】 |
| PATCH | `/v1/notifications/{id}` | Marks a notification as read. | `Authorization: Bearer <access token>` | No body required; supply the notification ID in the path. 【F:internal/http/notifications/handlers.go†L126-L155】【F:internal/http/notifications/handlers_test.go†L139-L156】 |

## Join Request Endpoints

### Public Submission

| Method | Path | Description | Headers | Request Body |
| --- | --- | --- | --- | --- |
| POST | `/join-requests` | Accepts a join request submission without authentication, normalizing casing and validating email format. | `Content-Type: application/json` | `{ "user": string, "email": string, "payload": object }` – `user` and `email` are required; payload is arbitrary metadata. 【F:internal/http/joinrequests/handlers.go†L143-L183】【F:internal/http/joinrequests/handlers_test.go†L125-L153】 |

### Administrative Review

Administrative handlers require an authenticated principal (Bearer token) and are commonly mounted under `/admin/join-requests`, as demonstrated by the tests. 【F:internal/http/joinrequests/handlers.go†L215-L303】【F:internal/http/joinrequests/handlers_test.go†L155-L216】

| Method | Path | Description | Headers | Request |
| --- | --- | --- | --- | --- |
| GET | `/admin/join-requests` | Lists join requests, optionally filtered by `status` (`pending`, `approved`, `declined`). | `Authorization: Bearer <access token>` | Query: `status` (optional). Body: none. 【F:internal/http/joinrequests/handlers.go†L185-L213】 |
| POST | `/admin/join-requests/{id}/approve` | Approves a join request and records an optional decision note. | `Authorization: Bearer <access token>`, `Content-Type: application/json` if a note is supplied | `{ "note": string }` (optional). 【F:internal/http/joinrequests/handlers.go†L215-L258】【F:internal/http/joinrequests/handlers_test.go†L155-L185】 |
| POST | `/admin/join-requests/{id}/decline` | Declines a join request and records an optional decision note. | `Authorization: Bearer <access token>`, `Content-Type: application/json` if a note is supplied | `{ "note": string }` (optional). 【F:internal/http/joinrequests/handlers.go†L260-L303】【F:internal/http/joinrequests/handlers_test.go†L187-L216】 |

## Administrative Management Endpoints (`/admin`)

Admin routes enforce permission-specific RBAC using the `RBAC` middleware, which requires an authenticated principal and validates that the user holds the appropriate permission slug. 【F:internal/http/admin/handlers.go†L239-L270】 Tests mount these handlers under `/admin` paths (for example, `/admin/users`). 【F:internal/http/admin/handlers_test.go†L200-L384】 The following resources share consistent JSON structures:

- **User** – `{ "email": string, "roles": [string], "teams": [string] }`; email is required and normalized to lowercase, while role/team arrays are deduplicated. 【F:internal/http/admin/handlers.go†L24-L40】【F:internal/http/admin/handlers.go†L286-L341】
- **Role** – `{ "name": string, "permissions": [string] }`; `name` is required. 【F:internal/http/admin/handlers.go†L26-L76】【F:internal/http/admin/handlers.go†L360-L399】
- **Permission** – `{ "name": string }`; required. 【F:internal/http/admin/handlers.go†L32-L103】【F:internal/http/admin/handlers.go†L400-L439】
- **Team** – `{ "name": string }`; required. 【F:internal/http/admin/handlers.go†L34-L103】【F:internal/http/admin/handlers.go†L440-L479】

| Method | Path | Description | Headers | Request |
| --- | --- | --- | --- | --- |
| POST | `/admin/users` | Create a user with roles and teams. | `Authorization: Bearer <access token>`, `Content-Type: application/json` | User payload with required `email`. 【F:internal/http/admin/handlers.go†L286-L309】【F:internal/http/admin/handlers_test.go†L200-L217】 |
| PUT | `/admin/users/{id}` | Update an existing user. | `Authorization`, `Content-Type: application/json` | User payload with required `email`. 【F:internal/http/admin/handlers.go†L312-L341】【F:internal/http/admin/handlers_test.go†L239-L258】 |
| DELETE | `/admin/users/{id}` | Delete a user. | `Authorization` | No body. 【F:internal/http/admin/handlers.go†L343-L358】【F:internal/http/admin/handlers_test.go†L260-L278】 |
| GET | `/admin/users` | List users with pagination metadata supplied in the response. | `Authorization` | Query: `page`, `per_page` optional. 【F:internal/http/admin/handlers.go†L360-L372】 |
| POST | `/admin/roles` | Create a role. | `Authorization`, `Content-Type: application/json` | Role payload with required `name`. 【F:internal/http/admin/handlers.go†L374-L392】【F:internal/http/admin/handlers_test.go†L334-L353】 |
| PUT | `/admin/roles/{id}` | Update a role. | `Authorization`, `Content-Type: application/json` | Role payload with required `name`. 【F:internal/http/admin/handlers.go†L394-L420】 |
| DELETE | `/admin/roles/{id}` | Delete a role. | `Authorization` | No body. 【F:internal/http/admin/handlers.go†L422-L436】 |
| GET | `/admin/roles` | List roles with pagination. | `Authorization` | Query: `page`, `per_page` optional. 【F:internal/http/admin/handlers.go†L437-L449】 |
| POST | `/admin/permissions` | Create a permission. | `Authorization`, `Content-Type: application/json` | Permission payload with required `name`. 【F:internal/http/admin/handlers.go†L451-L469】 |
| PUT | `/admin/permissions/{id}` | Update a permission. | `Authorization`, `Content-Type: application/json` | Permission payload with required `name`. 【F:internal/http/admin/handlers.go†L471-L495】【F:internal/http/admin/handlers_test.go†L356-L373】 |
| DELETE | `/admin/permissions/{id}` | Delete a permission. | `Authorization` | No body. 【F:internal/http/admin/handlers.go†L497-L511】 |
| GET | `/admin/permissions` | List permissions with pagination. | `Authorization` | Query: `page`, `per_page` optional. 【F:internal/http/admin/handlers.go†L513-L525】 |
| POST | `/admin/teams` | Create a team. | `Authorization`, `Content-Type: application/json` | Team payload with required `name`. 【F:internal/http/admin/handlers.go†L527-L545】 |
| PUT | `/admin/teams/{id}` | Update a team. | `Authorization`, `Content-Type: application/json` | Team payload with required `name`. 【F:internal/http/admin/handlers.go†L547-L571】 |
| DELETE | `/admin/teams/{id}` | Delete a team. | `Authorization` | No body. 【F:internal/http/admin/handlers.go†L573-L587】【F:internal/http/admin/handlers_test.go†L375-L393】 |
| GET | `/admin/teams` | List teams with pagination. | `Authorization` | Query: `page`, `per_page` optional. 【F:internal/http/admin/handlers.go†L589-L601】 |

