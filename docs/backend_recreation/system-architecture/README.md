# Backend Reconstruction Blueprint

## Overview
This blueprint maps the concrete Go subsystems that power the Yamato API. The application boots from `main.go`, assembles shared infrastructure in `bootstrap/app.go`, and wires HTTP routes in `routes/web.go`. Each subsection below calls out the files and types that must be recreated for a compatible backend.

## Startup and Configuration Flow
- **Entry point (`main.go`)**
  - Calls `bootstrap.SetupRouter()` to receive a configured `*gin.Engine` and the resolved `config.AppConfig`.
  - Flushes the global Zap logger with `zap.L().Sync()` before exiting and starts the HTTP server via `router.Run(":" + config.Port)`.【F:main.go†L1-L21】
- **Bootstrap (`bootstrap/app.go`)**
  - Loads application settings through `config.LoadAppConfig()` and builds a structured logger with `observability.NewLogger` before replacing the global Zap instance.【F:bootstrap/app.go†L1-L42】
  - Creates the Gin engine (`gin.Default()`), then attaches middleware from `internal/middleware`: `RequestID()` for trace correlation, `ErrorHandler()` for envelope rendering, and `CORS(resolveCORSConfig())` to expose the REST API to the Next.js frontend.【F:bootstrap/app.go†L34-L56】【F:internal/middleware/request.go†L23-L57】【F:internal/middleware/error.go†L9-L33】【F:internal/middleware/cors.go†L1-L28】
  - Initializes Prometheus instrumentation with `observability.NewMetrics` and hands it to `routes.RegisterRoutes` so `/metrics` can be exposed when configured.【F:bootstrap/app.go†L57-L69】【F:internal/observability/metrics.go†L17-L109】
  - Emits a startup banner (`fmt.Printf("Starting %s on port %s\n", ...)`) to highlight the running service and port.【F:bootstrap/app.go†L71-L73】

## Observability and Diagnostics
- **Logging (`internal/observability/logging.go`)**
  - `NewLogger` selects development or production Zap presets, injects the service label, and normalizes timestamp rendering. `bootstrap.SetupRouter` sets this logger as the global default so `zap.L()` works across packages.【F:internal/observability/logging.go†L1-L66】【F:bootstrap/app.go†L24-L42】
  - `ContextWithRequestID` and `WithContext` bridge Gin request metadata into structured logs, complementing the `RequestID` middleware.【F:internal/observability/logging.go†L70-L130】【F:internal/middleware/request.go†L40-L53】
- **Metrics (`internal/observability/metrics.go`)**
  - `NewMetrics` registers HTTP, database, Redis, WebSocket, and background job collectors against a scoped Prometheus registry and exposes an HTTP handler via `Metrics.Handler()`. Routes mount it at `/metrics` when instrumentation is available.【F:internal/observability/metrics.go†L17-L134】【F:routes/web.go†L58-L66】
  - Helper methods such as `RecordHTTP` and `RecordDBQuery` wrap metric recordings with optional tracing spans for richer telemetry.【F:internal/observability/metrics.go†L136-L201】
- **Diagnostics (`internal/http/diagnostics/handlers.go`)**
  - `Handler` exposes `Health` and `Ready` endpoints that probe optional database (`DBPinger`) and Redis (`RedisPinger`) dependencies, emitting ADR-003 compliant envelopes through the shared respond package. Rate limiting hooks are available via `WithRateLimiter` to protect the probes.【F:internal/http/diagnostics/handlers.go†L1-L146】【F:routes/web.go†L41-L55】

## Middleware and Transport Assembly
- **Core middleware (`internal/middleware`)**
  - `RequestID` mirrors inbound `X-Request-ID` headers or generates UUIDs, storing them on the Gin context and the standard library context for downstream logging.【F:internal/middleware/request.go†L33-L57】
  - `ErrorHandler` inspects `ctx.Errors` after handler execution and renders structured envelopes defined by `internal/http/respond`.【F:internal/middleware/error.go†L9-L33】
  - `CORS` adapts `internal/config.CORSConfig` values into a `gin-contrib/cors` middleware to support the local Next.js dashboard.【F:internal/middleware/cors.go†L1-L28】【F:bootstrap/app.go†L44-L56】
  - `Authentication` validates bearer tokens with `auth.Service`, loads user context from the injected `authhttp.UserStore`, and attaches an `auth.Principal` to the Gin context for downstream handlers.【F:internal/middleware/authentication.go†L1-L43】【F:internal/auth/context.go†L1-L32】
- **HTTP server glue (`internal/httpserver/router.go`)**
  - `RegisterAuthRoutes` mounts the authentication handler on `/v1/auth`, surfaces `/v1/user` behind the authentication middleware, and keeps Laravel-compatible verification endpoints at `/email/verify/:id/:hash` and `/email/verification-notification`. This helper is invoked by `routes.RegisterRoutes` during startup.【F:internal/httpserver/router.go†L1-L34】【F:routes/web.go†L74-L90】

## Authentication Subsystem
- **Core service (`internal/auth/service.go`)**
  - `Service` manages bcrypt hashing, JWT issuance, refresh token rotation, and blacklist bookkeeping against a `RedisCommander`. Constructors enforce secrets and expiration defaults, while methods like `Login`, `Refresh`, `Logout`, and `ValidateAccessToken` provide the workflows consumed by middleware and handlers.【F:internal/auth/service.go†L1-L161】
- **HTTP handlers (`internal/http/auth/handlers.go`)**
  - `Handler` coordinates credential registration, login, logout, refresh, email verification, and principal introspection. It depends on an `AuthService`, a `UserStore`, and an `EmailVerificationService`, with responses formatted per ADR-003 envelopes.【F:internal/http/auth/handlers.go†L1-L142】【F:routes/web.go†L74-L90】
- **In-memory adapters (`internal/platform/memory`)**
  - `NewRedis` offers an in-memory Redis facade satisfying the `auth.Service` contract during local development.【F:internal/platform/memory/redis.go†L1-L61】
  - `NewUserStore` and `UserStore` implement the `authhttp.UserStore` interface with uniqueness enforcement, while `NewVerificationService` provides deterministic verification hashes with resend throttling for UI flows.【F:internal/platform/memory/user_store.go†L1-L45】【F:internal/platform/memory/verification.go†L1-L63】
  - `routes.RegisterRoutes` stitches these adapters together, creating the auth service, handler, and middleware before delegating to `httpserver.RegisterAuthRoutes`.【F:routes/web.go†L67-L92】

## Tasks and Notifications
- **Tasks (`internal/http/tasks/handlers.go`)**
  - `Handler.List` retrieves a curated task collection from a `tasks.Service` and returns it under the `/api/tasks` and `/v1/tasks` routes. The development stack uses `memory.NewTaskService(memory.DefaultTasks())` for deterministic responses.【F:internal/http/tasks/handlers.go†L1-L45】【F:routes/web.go†L95-L105】【F:internal/platform/memory/tasks.go†L1-L36】
- **Notifications (`internal/http/notifications/handlers.go`)**
  - `Handler.List` and `Handler.MarkRead` expose paginated notification feeds and read acknowledgements for the authenticated principal. They rely on `notifications.Service` for persistence and leverage `auth.PrincipalFromContext` to resolve the user ID.【F:internal/http/notifications/handlers.go†L1-L109】【F:internal/auth/context.go†L1-L32】
  - `memory.NewNotificationService(memory.DefaultNotifications())` supplies an in-memory implementation that clones seeded alerts per user, supporting the `/v1/notifications` routes grouped in `routes/web.go`.【F:internal/platform/memory/notifications.go†L1-L70】【F:routes/web.go†L106-L112】

## Diagnostics, Health, and Metrics Endpoints
- `routes/web.go` binds `diagnostics.Handler.Health` to `/api/health` for liveness checks, `diagnostics.Handler.Ready` to `/ready` for orchestrator readiness, and conditionally publishes `/metrics` using the Prometheus handler returned by `observability.Metrics`. These endpoints reuse the middleware stack registered during bootstrap, ensuring consistent envelopes and request tracing.【F:routes/web.go†L35-L66】

## Recreation Checklist
1. Implement `bootstrap.SetupRouter` to load configuration, configure logging via `observability.NewLogger`, attach the middleware stack, initialize metrics with `observability.NewMetrics`, and call `routes.RegisterRoutes` with any optional instrumentation.【F:bootstrap/app.go†L19-L73】
2. Reproduce `routes.RegisterRoutes` to mount diagnostics, metrics, public task listings, and the protected `/v1` group that houses tasks and notification APIs. Ensure authentication is wired by composing `auth.Service`, `authhttp.Handler`, and `middleware.Authentication` with the in-memory platform adapters for local parity.【F:routes/web.go†L35-L112】
3. Surface the authentication workflows through `internal/httpserver.RegisterAuthRoutes` so registration, login, logout, refresh, verification, and principal inspection match the current API contracts.【F:internal/httpserver/router.go†L1-L34】
4. Provide development-friendly implementations for Redis, user storage, verification, tasks, and notifications under `internal/platform/memory` so the service remains functional without external dependencies during reconstruction.【F:internal/platform/memory/redis.go†L1-L61】【F:internal/platform/memory/user_store.go†L1-L45】【F:internal/platform/memory/tasks.go†L1-L36】【F:internal/platform/memory/notifications.go†L1-L70】
